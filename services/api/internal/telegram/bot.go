package telegram

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"

	"companion/api/internal/ai"
	"companion/api/internal/engine"
	"companion/api/internal/signals"
	"companion/api/internal/store"
)

// Bot runs the long-polling loop and the user-facing behaviour:
// capture -> instant ack -> async interpretation -> reply with buttons,
// plus onboarding, corrections, status commands and nudge delivery.
// Everything is persisted through the store.
type Bot struct {
	client       *Client
	ai           *ai.Client
	db           *store.Store
	eng          *engine.Engine
	weather      *signals.Weather
	log          *slog.Logger
	allowedChat  int64   // the owner's chat; 0 = not configured yet, the bot then only tells callers their id
	globalBudget float64 // service-wide monthly model budget, for refusing work and for /cost
	username     string  // the bot's own @name, for building invite links

	mu       sync.Mutex
	followOn map[int64]*time.Timer // per user: a pending "while you're here" check
	alerted  map[string]time.Time  // last time each kind of alert went to the owner
	lastPoll atomic.Int64          // unix time of the last successful poll, for the health check
}

func NewBot(client *Client, aiClient *ai.Client, db *store.Store, eng *engine.Engine, weather *signals.Weather, ownerChat int64, globalBudget float64, log *slog.Logger) *Bot {
	return &Bot{client: client, ai: aiClient, db: db, eng: eng, weather: weather, log: log, allowedChat: ownerChat,
		globalBudget: globalBudget, followOn: map[int64]*time.Timer{}, alerted: map[string]time.Time{}}
}

// AllowedChat is the chat the bot will proactively message (nudges).
func (b *Bot) AllowedChat() int64 { return b.allowedChat }

// Run polls until ctx is cancelled. Errors are logged and retried; a Telegram
// outage must never take the API process down.
func (b *Bot) Run(ctx context.Context) {
	if me, err := b.client.GetMe(ctx); err == nil {
		b.username = me.Username
		b.log.Info("telegram bot ready", "username", me.Username)
	} else {
		b.log.Error("telegram getMe failed", "err", err)
	}
	if b.allowedChat != 0 {
		if _, err := b.db.EnsureOwner(ctx, b.allowedChat, ""); err != nil {
			b.log.Error("ensure owner", "err", err)
		}
	}
	_ = b.client.SetMyCommands(ctx, []BotCommand{
		{Command: "list", Description: "Your active thoughts"},
		{Command: "nudge", Description: "Pick the best thing to bring up right now"},
		{Command: "why", Description: "Why I did or didn't nudge you"},
		{Command: "review", Description: "Go through what I'm holding, now"},
		{Command: "routine", Description: "Sleep, work hours, days off, city, review day"},
		{Command: "done", Description: "/done 12 — mark thought 12 done"},
		{Command: "snooze", Description: "/snooze 12 7 — quiet for 7 days (default 3)"},
		{Command: "drop", Description: "/drop 12 — no longer relevant"},
		{Command: "pause", Description: "Stop nudges until /resume"},
		{Command: "resume", Description: "Turn nudges back on"},
		{Command: "cost", Description: "This month's AI spend"},
		{Command: "export", Description: "Send me everything you hold about me"},
		{Command: "forget", Description: "Delete everything you hold about me"},
		{Command: "start", Description: "What this bot does"},
	})

	go b.sweep(ctx)

	// The offset is kept in the database so a restart resumes where the
	// last process stopped instead of receiving handled updates again.
	var offset int64
	if v, err := b.db.GetKV(ctx, offsetKey); err == nil && v != "" {
		offset, _ = strconv.ParseInt(v, 10, 64)
	}
	failingSince := time.Time{}
	for {
		if ctx.Err() != nil {
			return
		}
		updates, err := b.client.GetUpdates(ctx, offset, 30)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			if failingSince.IsZero() {
				failingSince = time.Now()
			}
			b.log.Error("getUpdates", "err", err, "failing_for", time.Since(failingSince).Round(time.Second).String())
			time.Sleep(3 * time.Second)
			continue
		}
		failingSince = time.Time{}
		b.lastPoll.Store(time.Now().Unix())
		for _, u := range updates {
			offset = u.UpdateID + 1
			b.handle(ctx, u)
		}
		if len(updates) > 0 {
			if err := b.db.SetKV(ctx, offsetKey, strconv.FormatInt(offset, 10)); err != nil {
				b.log.Warn("save offset", "err", err)
			}
		}
	}
}

const offsetKey = "telegram_offset"

// LastPoll is when Telegram last answered a poll; zero before the first one.
func (b *Bot) LastPoll() time.Time {
	if v := b.lastPoll.Load(); v != 0 {
		return time.Unix(v, 0)
	}
	return time.Time{}
}

// Alert tells the owner that something needs attention, at most once an
// hour per kind so a persistent fault does not become a flood. Errors must
// already be free of secrets: this text is shown in the chat.
func (b *Bot) Alert(kind, text string) {
	if b.allowedChat == 0 {
		return
	}
	b.mu.Lock()
	if last, ok := b.alerted[kind]; ok && time.Since(last) < time.Hour {
		b.mu.Unlock()
		return
	}
	b.alerted[kind] = time.Now()
	b.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	b.reply(ctx, b.allowedChat, "⚠️  <b>Something needs a look</b>\n"+Esc(clip(text, 600)))
}

// sweep retries captures that never got an interpretation: the first
// attempt failed, timed out, or was cut off by a restart. It runs at start
// (to pick up what a deploy interrupted) and every few minutes after.
func (b *Bot) sweep(ctx context.Context) {
	for {
		stuck, err := b.db.Uninterpreted(ctx, 2*time.Minute)
		if err != nil {
			b.log.Warn("sweep", "err", err)
		}
		for _, t := range stuck {
			user, err := b.db.GetUser(ctx, t.UserID)
			if err != nil {
				continue
			}
			if over, _ := b.overBudget(ctx, user); over {
				continue // kept as written; picked up again when the budget returns
			}
			b.log.Info("retrying interpretation", "thought", t.ID)
			b.interpret(user, t.ID, t.RawContent, "")
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(3 * time.Minute):
		}
	}
}

func (b *Bot) handle(ctx context.Context, u Update) {
	switch {
	case u.Message != nil:
		b.handleMessage(ctx, u.Message)
	case u.CallbackQuery != nil:
		b.handleCallback(ctx, u.CallbackQuery)
	}
}

func (b *Bot) handleMessage(ctx context.Context, m *Message) {
	chat := m.Chat.ID
	text := strings.TrimSpace(m.Text)
	user, ok := b.resolveUser(ctx, m)
	if !ok {
		return
	}
	if text == "" {
		// A photo, voice note, sticker or location. Say so, or the person
		// will think it was saved.
		b.reply(ctx, chat, "🙈  I can only read text for now, so that one wasn't saved. Type it and I'll keep it.")
		return
	}

	// A typed answer to a question the bot just asked is not a new thought.
	// Any command abandons the question.
	pending, err := b.db.TakePending(ctx, user.ID)
	if err != nil {
		b.log.Warn("pending", "err", err)
	}
	if pending != nil && !strings.HasPrefix(text, "/") {
		b.handlePending(ctx, user, *pending, text, m.MessageID)
		return
	}

	cmd, arg, _ := strings.Cut(text, " ")
	switch {
	case cmd == "/start":
		b.start(ctx, user)
	case cmd == "/cancel":
		b.reply(ctx, chat, "👌  Cancelled.")
	case cmd == "/id":
		b.reply(ctx, chat, fmt.Sprintf("Chat id: %d", chat))
	case cmd == "/list":
		b.list(ctx, user)
	case cmd == "/done" || cmd == "/drop" || cmd == "/snooze":
		b.statusCommand(ctx, user, cmd, arg)
	case cmd == "/nudge":
		b.nudgeNow(ctx, user)
	case cmd == "/why":
		b.why(ctx, user)
	case cmd == "/routine":
		b.routine(ctx, user)
	case cmd == "/review":
		b.reviewNow(ctx, user)
	case cmd == "/pause" || cmd == "/resume":
		on := cmd == "/resume"
		if err := b.db.SetNotifications(ctx, user.ID, on); err != nil {
			b.log.Error("set notifications", "err", err)
			return
		}
		if on {
			b.reply(ctx, chat, "🔔  Nudges are back on.")
		} else {
			b.reply(ctx, chat, "🔕  Nudges paused. I'll keep collecting your thoughts.\n<i>/resume turns them back on.</i>")
		}
	case cmd == "/cost":
		b.cost(ctx, user)
	case cmd == "/invite":
		b.invite(ctx, user)
	case cmd == "/users":
		b.users(ctx, user)
	case cmd == "/forget":
		b.forgetAsk(ctx, user)
	case cmd == "/export":
		b.export(ctx, user)
	case strings.HasPrefix(cmd, "/"):
		b.reply(ctx, chat, "🤷  I don't know that one. Just send me a thought, or /list.")
	case isChatter(text):
		// "ok", "thanks", a lone emoji: answer, but remember nothing.
		b.reply(ctx, chat, "🙂  <i>Not saved. Send me a thought whenever one comes up.</i>")
	default:
		b.captureThought(ctx, user, text, m.MessageID)
	}
}

// userNow is the current time in the user's own timezone.
func userNow(u store.User) time.Time {
	loc, err := time.LoadLocation(u.Timezone)
	if err != nil {
		loc = time.UTC
	}
	return time.Now().In(loc)
}

// captureThought follows product principle 1: save and acknowledge instantly,
// then let the interpretation arrive when it is ready.
func (b *Bot) captureThought(ctx context.Context, user store.User, raw string, messageID int64) {
	// A daily ceiling per person. It exists for the case that should never
	// happen (a stuck client, a script) and is set far above real use.
	if prefs, err := b.db.GetPrefs(ctx, user.ID); err == nil {
		if n, err := b.db.CapturesSince(ctx, user.ID, time.Now().Add(-24*time.Hour)); err == nil && n >= prefs.CapturesPerDay {
			b.reply(ctx, user.TelegramChatID, fmt.Sprintf("🧺  That's %d thoughts in a day, which is my limit. I haven't saved this one; send it again tomorrow.", n))
			return
		}
	}
	t, created, err := b.db.CreateThought(ctx, user.ID, raw, "telegram", messageID)
	if err != nil {
		b.log.Error("create thought", "err", err)
		b.reply(ctx, user.TelegramChatID, "I couldn't save that. Try again in a moment.")
		return
	}
	if !created {
		b.log.Info("duplicate delivery ignored", "message", messageID)
		return
	}
	// Saving is free; reading it costs money. Out of budget, the thought is
	// kept in the person's own words and read when the budget returns.
	if over, why := b.overBudget(ctx, user); over {
		b.reply(ctx, user.TelegramChatID, "✍️  Saved in your own words. I can't read it properly yet because "+why+"; I'll catch up when it resets.")
		return
	}
	b.reply(ctx, user.TelegramChatID, "✍️  Got it.")
	go b.interpret(user, t.ID, raw, "")
	b.scheduleFollowOn(user)
}

// followOnDelay is how long the chat must be quiet before the bot adds
// something of its own. Long enough for the interpretation to arrive and
// for a second thought to be typed; the research on interruptions says to
// speak after an exchange ends, not in the middle of it.
const followOnDelay = 90 * time.Second

// scheduleFollowOn arranges one engine pass shortly after the user stops
// typing. They are already looking at the chat, so a nudge now costs no
// interruption. Each new message pushes the moment back.
func (b *Bot) scheduleFollowOn(user store.User) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if t, ok := b.followOn[user.ID]; ok {
		t.Stop()
	}
	b.followOn[user.ID] = time.AfterFunc(followOnDelay, func() {
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
		defer cancel()
		res, err := b.eng.Evaluate(ctx, user, engine.Options{Trigger: "message", Present: true})
		if err != nil {
			b.log.Error("follow-on evaluate", "err", err)
			return
		}
		b.log.Info("follow-on", "user", user.ID, "outcome", res.Outcome, "reason", res.Reason, "thought", res.ThoughtID)
	})
}

// interpret reads a thought (optionally with the user's clarification),
// stores the result as a new version and shows it.
func (b *Bot) interpret(user store.User, thoughtID int64, raw, clarification string) {
	// Bookkeeping uses its own context. If it shared the model call's
	// deadline, a timeout would also silence the log entry and the message
	// that tells the person what happened.
	ctx, cancelAll := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancelAll()
	if err := b.db.CountInterpretAttempt(ctx, thoughtID); err != nil {
		b.log.Warn("count attempt", "err", err)
	}
	home := ""
	if p, err := b.db.GetPrefs(ctx, user.ID); err == nil {
		home = p.HomeCity
	}
	callCtx, cancelCall := context.WithTimeout(ctx, 90*time.Second)
	interp, info, err := b.ai.Interpret(callCtx, raw, ai.InterpretContext{Today: userNow(user), Home: home, Clarification: clarification})
	cancelCall()
	_ = b.db.LogLLMCall(ctx, store.LLMCall{
		UserID: &user.ID, ThoughtID: &thoughtID, Purpose: "interpret", Model: info.Model, PromptVer: info.PromptVersion,
		InputTokens: info.InputTokens, OutputTokens: info.OutputTokens, CostUSD: info.CostUSD(),
		Latency: info.Latency, StopReason: info.StopReason, Err: err,
	})
	if err != nil {
		b.log.Error("interpret", "err", err, "thought", thoughtID)
		// A failure is not final: the sweep tries again, up to
		// store.MaxInterpretAttempts. Say what is true at each stage.
		t, getErr := b.db.GetThought(ctx, user.ID, thoughtID)
		switch {
		case getErr != nil:
		case t.Interpretation != nil:
			b.reply(ctx, user.TelegramChatID, "I couldn't re-read that just now, so the earlier reading stays.")
		case t.InterpretAttempts >= store.MaxInterpretAttempts:
			b.reply(ctx, user.TelegramChatID, "Saved in your own words. I couldn't make sense of it after a few tries, so it shows in /list as you wrote it.")
			b.Alert("interpret", fmt.Sprintf("Interpretation failed %d times for one thought. Last error: %v", t.InterpretAttempts, err))
		case t.InterpretAttempts == 1:
			b.reply(ctx, user.TelegramChatID, "Saved. I couldn't read it properly just now; I'll try again in a few minutes.")
		}
		return
	}
	stored, err := b.db.AddInterpretation(ctx, thoughtID, "ai", interp, info.Model, info.PromptVersion)
	if err != nil {
		b.log.Error("store interpretation", "err", err, "thought", thoughtID)
		return
	}
	b.log.Info("interpreted", "thought", thoughtID, "version", stored.Version, "type", interp.Type, "not_a_thought", interp.NotAThought,
		"confidence", interp.Confidence, "hooks", len(interp.Hooks), "in", info.InputTokens, "out", info.OutputTokens, "ms", info.Latency.Milliseconds())

	// Talk aimed at the bot is set aside rather than kept, with a way back
	// in case the reading was wrong. A clarified thought is never set aside:
	// the user has just told us it matters.
	if interp.NotAThought && clarification == "" {
		_ = b.db.AddEvent(ctx, thoughtID, nil, "not_a_thought", nil)
		if err := b.db.SetStatus(ctx, thoughtID, "archived", nil); err != nil {
			b.log.Error("set aside", "err", err, "thought", thoughtID)
		}
		kb := &InlineKeyboardMarkup{InlineKeyboard: [][]InlineKeyboardButton{{
			{Text: "📌 Keep it as a thought", CallbackData: fmt.Sprintf("keep:%d", thoughtID)},
		}}}
		text := "💬  That read like a message to me rather than something to remember, so I set it aside.\n<i>I can't hold a conversation yet. /list, /why and /routine are what I can show you.</i>"
		if _, err := b.client.SendMessage(ctx, user.TelegramChatID, text, kb); err != nil {
			b.log.Error("send set-aside", "err", err)
		}
		return
	}
	b.sendInterpretation(ctx, user.TelegramChatID, thoughtID, b.numberOf(ctx, user.ID, thoughtID), interp)
}

// numberOf returns the number a person sees for one of their thoughts.
func (b *Bot) numberOf(ctx context.Context, userID, thoughtID int64) int {
	if t, err := b.db.GetThought(ctx, userID, thoughtID); err == nil {
		return t.Number
	}
	return 0
}

// chatter is the set of messages that are plainly talk, not thoughts. It is
// deliberately small: anything not on it goes to the interpreter, which
// makes the subtler call. A wrongly dropped thought is worse than an extra
// model call.
var chatter = map[string]bool{
	"ok": true, "okay": true, "k": true, "kk": true, "okk": true, "ok thanks": true, "ok thank you": true, "okay thanks": true,
	"thanks": true, "thank you": true, "thankyou": true, "thx": true, "ty": true, "thanks a lot": true, "thank you so much": true,
	"cool": true, "nice": true, "great": true, "good": true, "awesome": true, "perfect": true, "wow": true, "super": true,
	"got it": true, "noted": true, "sure": true, "fine": true, "alright": true, "right": true, "np": true, "no problem": true,
	"yes": true, "no": true, "yep": true, "yeah": true, "yup": true, "nope": true, "nah": true, "haan": true, "nahi": true, "theek hai": true, "thik hai": true, "accha": true, "acha": true,
	"hi": true, "hello": true, "hey": true, "hii": true, "yo": true, "bye": true, "good morning": true, "good night": true, "gm": true, "gn": true,
	"hmm": true, "hm": true, "hmmm": true, "lol": true, "haha": true, "hahaha": true, "test": true, "testing": true,
	"thats fine": true, "ok thats fine": true, "okay thats fine": true, "its fine": true, "all good": true, "sounds good": true,
	"ok cool": true, "okay cool": true, "ok great": true, "okay great": true, "ok good": true, "ok nice": true, "ok got it": true, "okay got it": true,
}

// isChatter reports whether a message is an acknowledgment, a greeting or
// only emoji and punctuation.
func isChatter(text string) bool {
	var sb strings.Builder
	for _, r := range strings.ToLower(text) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			sb.WriteRune(r)
		case unicode.IsSpace(r):
			sb.WriteRune(' ')
		}
	}
	words := strings.Join(strings.Fields(sb.String()), " ")
	return words == "" || chatter[words]
}

// interpretationKeyboard is the row under a fresh reading. An urgent reading
// gets a second row, because urgency changes how soon the thought comes
// back and the person should be able to correct it with one tap.
func interpretationKeyboard(thoughtID int64, i ai.Interpretation) *InlineKeyboardMarkup {
	rows := [][]InlineKeyboardButton{{
		{Text: "👍 Looks right", CallbackData: fmt.Sprintf("ok:%d", thoughtID)},
		{Text: "✏️ Not quite", CallbackData: fmt.Sprintf("wrong:%d", thoughtID)},
	}}
	if i.Urgency == ai.UrgencyNow {
		rows = append(rows, []InlineKeyboardButton{{Text: "🕰 No rush on this", CallbackData: fmt.Sprintf("nr:%d", thoughtID)}})
	}
	return &InlineKeyboardMarkup{InlineKeyboard: rows}
}

func (b *Bot) sendInterpretation(ctx context.Context, chat int64, thoughtID int64, number int, i ai.Interpretation) {
	if _, err := b.client.SendMessage(ctx, chat, formatInterpretation(number, i), interpretationKeyboard(thoughtID, i)); err != nil {
		b.log.Error("send interpretation", "err", err)
	}
}

func (b *Bot) list(ctx context.Context, user store.User) {
	ts, err := b.db.ListThoughts(ctx, user.ID, []string{"active", "snoozed"}, 50)
	if err != nil {
		b.log.Error("list", "err", err)
		b.reply(ctx, user.TelegramChatID, "Couldn't load the list just now.")
		return
	}
	if len(ts) == 0 {
		b.reply(ctx, user.TelegramChatID, "📭  Nothing here yet. Send me a thought.")
		return
	}
	b.reply(ctx, user.TelegramChatID, formatList(ts, userNow(user).Location()))
}

func (b *Bot) statusCommand(ctx context.Context, user store.User, cmd, arg string) {
	fields := strings.Fields(arg)
	if len(fields) == 0 {
		b.reply(ctx, user.TelegramChatID, fmt.Sprintf("Which one? e.g. %s 12 (numbers are in /list)", cmd))
		return
	}
	number, err := strconv.Atoi(strings.TrimPrefix(fields[0], "#"))
	if err != nil {
		b.reply(ctx, user.TelegramChatID, "That doesn't look like a thought number.")
		return
	}
	t, err := b.db.GetThoughtByNumber(ctx, user.ID, number)
	if errors.Is(err, store.ErrNotFound) {
		b.reply(ctx, user.TelegramChatID, fmt.Sprintf("I don't have a thought #%d. /list shows the numbers.", number))
		return
	}
	id := t.ID
	if err != nil {
		b.log.Error("get thought", "err", err)
		return
	}
	title := t.RawContent
	if t.Interpretation != nil {
		title = t.Interpretation.Title
	}
	switch cmd {
	case "/done":
		err = b.db.SetStatus(ctx, id, "done", nil)
		b.reply(ctx, user.TelegramChatID, fmt.Sprintf("🎉  Done: <b>%s</b>", Esc(clean(title))))
	case "/drop":
		err = b.db.SetStatus(ctx, id, "archived", nil)
		b.reply(ctx, user.TelegramChatID, fmt.Sprintf("🗂  Letting go of <b>%s</b>.\n<i>It stays in the archive if you ever want it back.</i>", Esc(clean(title))))
	case "/snooze":
		days := 3
		if len(fields) > 1 {
			if d, err := strconv.Atoi(fields[1]); err == nil && d > 0 && d <= 90 {
				days = d
			}
		}
		until := time.Now().Add(time.Duration(days) * 24 * time.Hour)
		err = b.db.SetStatus(ctx, id, "snoozed", &until)
		b.reply(ctx, user.TelegramChatID, fmt.Sprintf("😴  Quiet on <b>%s</b> for %d days.", Esc(clean(title)), days))
	}
	if err != nil {
		b.log.Error("set status", "err", err, "thought", id)
	}
}

func (b *Bot) handleCallback(ctx context.Context, q *CallbackQuery) {
	if q.Message == nil || q.Message.Chat.Type != "private" {
		_ = b.client.AnswerCallbackQuery(ctx, q.ID, "")
		return
	}
	chat, msgID := q.Message.Chat.ID, q.Message.MessageID
	user, found, err := b.db.UserByChat(ctx, chat)
	if err != nil || !found {
		_ = b.client.AnswerCallbackQuery(ctx, q.ID, "")
		return
	}
	// Callback data is "action:arg1:arg2:arg3"; the last part may itself
	// contain colons (times like 23:00-07:00).
	parts := strings.SplitN(q.Data, ":", 4)
	for len(parts) < 4 {
		parts = append(parts, "")
	}
	action := parts[0]
	id, _ := strconv.ParseInt(parts[1], 10, 64)
	b.log.Info("callback", "action", action, "user", user.ID)

	// Nothing below runs until every id in the button data has been checked
	// against the person who pressed it.
	if refusal := b.authorize(ctx, user, action, parts); refusal != "" {
		b.log.Warn("callback refused", "action", action, "user", user.ID, "data", q.Data)
		_ = b.client.AnswerCallbackQuery(ctx, q.ID, refusal)
		return
	}

	toast := "Noted."
	switch action {
	case "ok":
		toast = "Thanks, noted."
		if err := b.db.AddEvent(ctx, id, nil, "interpretation_confirmed", nil); err == nil {
			_ = b.db.Touch(ctx, id)
		}
		_ = b.client.ClearKeyboard(ctx, chat, msgID)
	case "keep":
		toast = "Couldn't find it."
		if t, err := b.db.GetThought(ctx, user.ID, id); err == nil && t.Interpretation != nil {
			if err := b.db.SetStatus(ctx, id, "active", nil); err == nil {
				// The model read this as chatter, so its type is "unclear"
				// and an unclear thought is never nudged or reviewed. Ask
				// what it is, or keeping it would change nothing.
				kb := correctionKeyboard(id)
				_ = b.client.EditText(ctx, chat, msgID, formatInterpretation(t.Number, t.Interpretation.Interpretation)+"\n\n<i>Kept. What kind of thing is it?</i>", &kb)
				toast = "Kept."
			}
		}
	case "wrong", "ct", "cn", "cx":
		toast = b.correction(ctx, user, action, id, parts[2], msgID)
	case "nr":
		toast = "Couldn't find it."
		if t, err := b.db.GetThought(ctx, user.ID, id); err == nil && t.Interpretation != nil {
			calm := t.Interpretation.Interpretation
			calm.Urgency = ai.UrgencyWhenever
			if _, err := b.db.AddInterpretation(ctx, id, "user", calm, "", ""); err == nil {
				_ = b.client.EditText(ctx, chat, msgID, formatInterpretation(t.Number, calm), interpretationKeyboard(id, calm))
				toast = "Okay, no rush."
			}
		}
	case "do_it", "next_steps", "not_interested":
		nudgeID, _ := strconv.ParseInt(parts[2], 10, 64)
		toast = b.nudgeResponse(ctx, user, action, id, nudgeID)
		_ = b.client.ClearKeyboard(ctx, chat, msgID)
	case "later":
		nudgeID, _ := strconv.ParseInt(parts[2], 10, 64)
		toast = b.nudgeResponse(ctx, user, action, id, nudgeID)
		_ = b.client.EditKeyboard(ctx, chat, msgID, laterKeyboard(id))
	case "lt":
		toast = b.laterChoice(ctx, user, id, parts[2])
		_ = b.client.ClearKeyboard(ctx, chat, msgID)
	case "rv":
		thoughtID, _ := strconv.ParseInt(parts[2], 10, 64)
		toast = b.reviewAnswer(ctx, user, id, thoughtID, parts[3], q.Message)
	case "ob":
		toast = b.onboardingAnswer(ctx, user, parts[1] == "c", parts[2], parts[3], msgID)
	case "rt":
		toast = ""
		b.ask(ctx, user, parts[1], false)
	case "fg":
		toast = b.forgetAnswer(ctx, user, parts[1], msgID)
	}
	_ = b.client.AnswerCallbackQuery(ctx, q.ID, toast)
}

func (b *Bot) reply(ctx context.Context, chat int64, text string) {
	if _, err := b.client.SendMessage(ctx, chat, text, nil); err != nil {
		b.log.Error("send", "err", err)
	}
}
