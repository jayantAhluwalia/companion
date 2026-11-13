package telegram

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
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
	client      *Client
	ai          *ai.Client
	db          *store.Store
	eng         *engine.Engine
	weather     *signals.Weather
	log         *slog.Logger
	allowedChat int64 // 0 = not locked yet; the bot then only tells callers their id
}

func NewBot(client *Client, aiClient *ai.Client, db *store.Store, eng *engine.Engine, weather *signals.Weather, allowedChat int64, log *slog.Logger) *Bot {
	return &Bot{client: client, ai: aiClient, db: db, eng: eng, weather: weather, log: log, allowedChat: allowedChat}
}

// AllowedChat is the chat the bot will proactively message (nudges).
func (b *Bot) AllowedChat() int64 { return b.allowedChat }

// Run polls until ctx is cancelled. Errors are logged and retried; a Telegram
// outage must never take the API process down.
func (b *Bot) Run(ctx context.Context) {
	if me, err := b.client.GetMe(ctx); err == nil {
		b.log.Info("telegram bot ready", "username", me.Username)
	} else {
		b.log.Error("telegram getMe failed", "err", err)
	}
	_ = b.client.SetMyCommands(ctx, []BotCommand{
		{Command: "list", Description: "Your active thoughts"},
		{Command: "nudge", Description: "Pick the best thing to bring up right now"},
		{Command: "why", Description: "Why I did or didn't nudge you"},
		{Command: "routine", Description: "Sleep, work hours, days off, city"},
		{Command: "done", Description: "/done 12 — mark thought 12 done"},
		{Command: "snooze", Description: "/snooze 12 7 — quiet for 7 days (default 3)"},
		{Command: "drop", Description: "/drop 12 — no longer relevant"},
		{Command: "pause", Description: "Stop nudges until /resume"},
		{Command: "resume", Description: "Turn nudges back on"},
		{Command: "cost", Description: "This month's AI spend"},
		{Command: "start", Description: "What this bot does"},
	})

	var offset int64
	for {
		if ctx.Err() != nil {
			return
		}
		updates, err := b.client.GetUpdates(ctx, offset, 30)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			b.log.Error("getUpdates", "err", err)
			time.Sleep(3 * time.Second)
			continue
		}
		for _, u := range updates {
			offset = u.UpdateID + 1
			b.handle(ctx, u)
		}
	}
}

func (b *Bot) allowed(chatID int64) bool { return b.allowedChat != 0 && chatID == b.allowedChat }

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
	if text == "" {
		return
	}
	if !b.allowed(chat) {
		b.reply(ctx, chat, fmt.Sprintf("This bot is private. Chat id: %d\nSet TELEGRAM_CHAT_ID to this value on the server to enable it.", chat))
		return
	}
	name := ""
	if m.From != nil {
		name = m.From.FirstName
	}
	user, err := b.db.GetOrCreateUser(ctx, chat, name)
	if err != nil {
		b.log.Error("get user", "err", err)
		b.reply(ctx, chat, "Something went wrong on my side. Try again in a moment.")
		return
	}

	// A typed answer to a question the bot just asked is not a new thought.
	// Any command abandons the question.
	pending, err := b.db.TakePending(ctx, user.ID)
	if err != nil {
		b.log.Warn("pending", "err", err)
	}
	if pending != nil && !strings.HasPrefix(text, "/") {
		b.handlePending(ctx, user, *pending, text)
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
		cost, n, err := b.db.MonthlyCostUSD(ctx)
		if err != nil {
			b.reply(ctx, chat, "Couldn't read the cost log.")
			return
		}
		b.reply(ctx, chat, fmt.Sprintf("💸  <b>This month</b>\n%d AI calls · about $%.2f", n, cost))
	case strings.HasPrefix(cmd, "/"):
		b.reply(ctx, chat, "🤷  I don't know that one. Just send me a thought, or /list.")
	case isChatter(text):
		// "ok", "thanks", a lone emoji: answer, but remember nothing.
		b.reply(ctx, chat, "🙂  <i>Not saved. Send me a thought whenever one comes up.</i>")
	default:
		b.captureThought(ctx, user, text)
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
func (b *Bot) captureThought(ctx context.Context, user store.User, raw string) {
	t, err := b.db.CreateThought(ctx, user.ID, raw, "telegram")
	if err != nil {
		b.log.Error("create thought", "err", err)
		b.reply(ctx, user.TelegramChatID, "I couldn't save that. Try again in a moment.")
		return
	}
	b.reply(ctx, user.TelegramChatID, "✍️  Got it.")
	go b.interpret(user, t.ID, raw, "")
}

// interpret reads a thought (optionally with the user's clarification),
// stores the result as a new version and shows it.
func (b *Bot) interpret(user store.User, thoughtID int64, raw, clarification string) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	home := ""
	if p, err := b.db.GetPrefs(ctx, user.ID); err == nil {
		home = p.HomeCity
	}
	interp, info, err := b.ai.Interpret(ctx, raw, ai.InterpretContext{Today: userNow(user), Home: home, Clarification: clarification})
	_ = b.db.LogLLMCall(ctx, store.LLMCall{
		UserID: &user.ID, ThoughtID: &thoughtID, Purpose: "interpret", Model: info.Model, PromptVer: info.PromptVersion,
		InputTokens: info.InputTokens, OutputTokens: info.OutputTokens, CostUSD: info.CostUSD(),
		Latency: info.Latency, StopReason: info.StopReason, Err: err,
	})
	if err != nil {
		b.log.Error("interpret", "err", err, "thought", thoughtID)
		b.reply(ctx, user.TelegramChatID, "Saved. I couldn't make sense of it just now, so I'll keep your original wording.")
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
	b.sendInterpretation(ctx, user.TelegramChatID, thoughtID, interp)
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

func (b *Bot) sendInterpretation(ctx context.Context, chat int64, thoughtID int64, i ai.Interpretation) {
	kb := &InlineKeyboardMarkup{InlineKeyboard: [][]InlineKeyboardButton{{
		{Text: "👍 Looks right", CallbackData: fmt.Sprintf("ok:%d", thoughtID)},
		{Text: "✏️ Not quite", CallbackData: fmt.Sprintf("wrong:%d", thoughtID)},
	}}}
	if _, err := b.client.SendMessage(ctx, chat, formatInterpretation(thoughtID, i), kb); err != nil {
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
	b.reply(ctx, user.TelegramChatID, formatList(ts))
}

func (b *Bot) statusCommand(ctx context.Context, user store.User, cmd, arg string) {
	fields := strings.Fields(arg)
	if len(fields) == 0 {
		b.reply(ctx, user.TelegramChatID, fmt.Sprintf("Which one? e.g. %s 12 (numbers are in /list)", cmd))
		return
	}
	id, err := strconv.ParseInt(strings.TrimPrefix(fields[0], "#"), 10, 64)
	if err != nil {
		b.reply(ctx, user.TelegramChatID, "That doesn't look like a thought number.")
		return
	}
	t, err := b.db.GetThought(ctx, user.ID, id)
	if errors.Is(err, store.ErrNotFound) {
		b.reply(ctx, user.TelegramChatID, fmt.Sprintf("I don't have a thought #%d. /list shows the numbers.", id))
		return
	}
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
	if q.Message == nil || !b.allowed(q.Message.Chat.ID) {
		_ = b.client.AnswerCallbackQuery(ctx, q.ID, "")
		return
	}
	chat, msgID := q.Message.Chat.ID, q.Message.MessageID
	user, err := b.db.GetOrCreateUser(ctx, chat, "")
	if err != nil {
		_ = b.client.AnswerCallbackQuery(ctx, q.ID, "Something went wrong.")
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
	b.log.Info("callback", "action", action, "data", q.Data)

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
				_ = b.client.EditText(ctx, chat, msgID, formatInterpretation(id, t.Interpretation.Interpretation))
				toast = "Kept."
			}
		}
	case "wrong", "ct", "cn", "cx":
		toast = b.correction(ctx, user, action, id, parts[2], msgID)
	case "do_it", "next_steps", "later", "not_interested":
		nudgeID, _ := strconv.ParseInt(parts[2], 10, 64)
		toast = b.nudgeResponse(ctx, user, action, id, nudgeID)
		_ = b.client.ClearKeyboard(ctx, chat, msgID)
	case "ob":
		toast = b.onboardingAnswer(ctx, user, parts[1] == "c", parts[2], parts[3], msgID)
	case "rt":
		toast = ""
		b.ask(ctx, user, parts[1], false)
	}
	_ = b.client.AnswerCallbackQuery(ctx, q.ID, toast)
}

func (b *Bot) reply(ctx context.Context, chat int64, text string) {
	if _, err := b.client.SendMessage(ctx, chat, text, nil); err != nil {
		b.log.Error("send", "err", err)
	}
}
