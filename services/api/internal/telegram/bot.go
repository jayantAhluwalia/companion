package telegram

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"companion/api/internal/ai"
	"companion/api/internal/store"
)

// Bot runs the long-polling loop and the user-facing behaviour:
// capture -> instant ack -> async interpretation -> reply with buttons,
// plus the status commands. Everything is persisted through the store.
type Bot struct {
	client      *Client
	ai          *ai.Client
	db          *store.Store
	log         *slog.Logger
	allowedChat int64 // 0 = not locked yet; the bot then only tells callers their id
}

func NewBot(client *Client, aiClient *ai.Client, db *store.Store, allowedChat int64, log *slog.Logger) *Bot {
	return &Bot{client: client, ai: aiClient, db: db, log: log, allowedChat: allowedChat}
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
		{Command: "start", Description: "What this bot does"},
		{Command: "list", Description: "Your active thoughts"},
		{Command: "done", Description: "/done 12 — mark thought 12 done"},
		{Command: "snooze", Description: "/snooze 12 7 — quiet for 7 days (default 3)"},
		{Command: "drop", Description: "/drop 12 — no longer relevant"},
		{Command: "nudge", Description: "Send yourself a sample nudge"},
		{Command: "cost", Description: "This month's AI spend"},
		{Command: "id", Description: "Show this chat's id"},
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

	cmd, arg, _ := strings.Cut(text, " ")
	switch {
	case cmd == "/start":
		b.reply(ctx, chat, "Hi. Send me anything on your mind: something you want to do, try, learn or look into. I'll remember it and, later, help you act on it.\n\nNo need to organise it. Just type it. /list shows what I'm holding.")
	case cmd == "/id":
		b.reply(ctx, chat, fmt.Sprintf("Chat id: %d", chat))
	case cmd == "/list":
		b.list(ctx, user)
	case cmd == "/done" || cmd == "/drop" || cmd == "/snooze":
		b.statusCommand(ctx, user, cmd, arg)
	case cmd == "/nudge":
		b.SendSampleNudge(ctx)
	case cmd == "/cost":
		cost, n, err := b.db.MonthlyCostUSD(ctx)
		if err != nil {
			b.reply(ctx, chat, "Couldn't read the cost log.")
			return
		}
		b.reply(ctx, chat, fmt.Sprintf("This month: %d AI calls, about $%.3f.", n, cost))
	case strings.HasPrefix(cmd, "/"):
		b.reply(ctx, chat, "I don't know that command. Just send me a thought, or /list.")
	default:
		b.captureThought(ctx, user, text)
	}
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
	b.reply(ctx, user.TelegramChatID, "Got it.")

	go func() {
		ictx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		interp, info, err := b.ai.Interpret(ictx, raw, time.Now())
		_ = b.db.LogLLMCall(ictx, store.LLMCall{
			UserID: &user.ID, ThoughtID: &t.ID, Purpose: "interpret", Model: info.Model, PromptVer: info.PromptVersion,
			InputTokens: info.InputTokens, OutputTokens: info.OutputTokens, CostUSD: info.CostUSD(),
			Latency: info.Latency, StopReason: info.StopReason, Err: err,
		})
		if err != nil {
			b.log.Error("interpret", "err", err, "thought", t.ID)
			b.reply(ictx, user.TelegramChatID, "Saved, but I couldn't make sense of it just now. I'll keep the original wording.")
			return
		}
		stored, err := b.db.AddInterpretation(ictx, t.ID, "ai", interp, info.Model, info.PromptVersion)
		if err != nil {
			b.log.Error("store interpretation", "err", err, "thought", t.ID)
			return
		}
		b.log.Info("interpreted", "thought", t.ID, "version", stored.Version, "type", interp.Type,
			"confidence", interp.Confidence, "in", info.InputTokens, "out", info.OutputTokens, "ms", info.Latency.Milliseconds())
		b.sendInterpretation(ictx, user.TelegramChatID, t.ID, interp)
	}()
}

func (b *Bot) sendInterpretation(ctx context.Context, chat int64, thoughtID int64, i ai.Interpretation) {
	var sb strings.Builder
	fmt.Fprintf(&sb, "#%d  %s\n%s · confidence %.0f%%\n", thoughtID, i.Title, humanType(i.Type), i.Confidence*100)
	if i.ClarificationNeeded {
		fmt.Fprintf(&sb, "\nQuick question: %s\n", i.ClarificationQuestion)
	}
	if len(i.NextSteps) > 0 {
		sb.WriteString("\nIf you ever want to move on it:\n")
		for _, s := range i.NextSteps {
			fmt.Fprintf(&sb, "• %s (~%d min)\n", s.Label, s.EstimatedMinutes)
		}
	}
	if len(i.Assumptions) > 0 {
		sb.WriteString("\nI assumed: " + strings.Join(i.Assumptions, "; "))
	}
	kb := &InlineKeyboardMarkup{InlineKeyboard: [][]InlineKeyboardButton{{
		{Text: "Looks right", CallbackData: fmt.Sprintf("ok:%d", thoughtID)},
		{Text: "Not quite", CallbackData: fmt.Sprintf("wrong:%d", thoughtID)},
	}}}
	if _, err := b.client.SendMessage(ctx, chat, sb.String(), kb); err != nil {
		b.log.Error("send interpretation", "err", err)
	}
}

func (b *Bot) list(ctx context.Context, user store.User) {
	ts, err := b.db.ListThoughts(ctx, user.ID, []string{"active", "snoozed"}, 50)
	if err != nil {
		b.log.Error("list", "err", err)
		b.reply(ctx, user.TelegramChatID, "Couldn't load the list.")
		return
	}
	if len(ts) == 0 {
		b.reply(ctx, user.TelegramChatID, "Nothing on the list yet. Send me a thought.")
		return
	}
	groups := map[ai.ThoughtType][]store.Thought{}
	order := []ai.ThoughtType{ai.TypeActionable, ai.TypeExperience, ai.TypeIdea, ai.TypeDevelopment, ai.TypeUnclear}
	for _, t := range ts {
		typ := ai.TypeUnclear
		if t.Interpretation != nil {
			typ = t.Interpretation.Type
		}
		groups[typ] = append(groups[typ], t)
	}
	var sb strings.Builder
	for _, typ := range order {
		items := groups[typ]
		if len(items) == 0 {
			continue
		}
		fmt.Fprintf(&sb, "%s\n", strings.ToUpper(humanType(typ)))
		for _, t := range items {
			title := t.RawContent
			if t.Interpretation != nil {
				title = t.Interpretation.Title
			}
			suffix := ""
			if t.Status == "snoozed" && t.SnoozedUntil != nil {
				suffix = fmt.Sprintf("  (quiet until %s)", t.SnoozedUntil.Format("2 Jan"))
			}
			fmt.Fprintf(&sb, "#%d  %s%s\n", t.ID, title, suffix)
		}
		sb.WriteString("\n")
	}
	sb.WriteString("Use /done, /snooze or /drop with a number.")
	b.reply(ctx, user.TelegramChatID, sb.String())
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
		b.reply(ctx, user.TelegramChatID, fmt.Sprintf("I don't have a thought #%d.", id))
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
		b.reply(ctx, user.TelegramChatID, fmt.Sprintf("Nice. #%d \"%s\" is done.", id, title))
	case "/drop":
		err = b.db.SetStatus(ctx, id, "archived", nil)
		b.reply(ctx, user.TelegramChatID, fmt.Sprintf("Okay, letting #%d \"%s\" go. It stays in the archive if you ever want it back.", id, title))
	case "/snooze":
		days := 3
		if len(fields) > 1 {
			if d, err := strconv.Atoi(fields[1]); err == nil && d > 0 && d <= 90 {
				days = d
			}
		}
		until := time.Now().Add(time.Duration(days) * 24 * time.Hour)
		err = b.db.SetStatus(ctx, id, "snoozed", &until)
		b.reply(ctx, user.TelegramChatID, fmt.Sprintf("Quiet on #%d \"%s\" for %d days.", id, title, days))
	}
	if err != nil {
		b.log.Error("set status", "err", err, "thought", id)
	}
}

// SendSampleNudge proves the proactive path with the latest interpreted
// thought. The Phase 2 evaluator replaces the "latest" choice with scoring.
func (b *Bot) SendSampleNudge(ctx context.Context) {
	if b.allowedChat == 0 {
		return
	}
	user, err := b.db.GetOrCreateUser(ctx, b.allowedChat, "")
	if err != nil {
		return
	}
	t, err := b.db.LatestInterpreted(ctx, user.ID)
	text := "Sample nudge: this is what a proactive check-in will look like."
	var id int64
	if err == nil && t.Interpretation != nil {
		id = t.ID
		step := "a small first step"
		if len(t.Interpretation.NextSteps) > 0 {
			step = t.Interpretation.NextSteps[0].Label
		}
		text = fmt.Sprintf("Still thinking about \"%s\"? If you have ten minutes, you could: %s.", t.Interpretation.Title, step)
	}
	kb := &InlineKeyboardMarkup{InlineKeyboard: [][]InlineKeyboardButton{
		{{Text: "Let's do it", CallbackData: fmt.Sprintf("do_it:%d", id)}, {Text: "Show next steps", CallbackData: fmt.Sprintf("next_steps:%d", id)}},
		{{Text: "Later", CallbackData: fmt.Sprintf("later:%d", id)}, {Text: "Not interested", CallbackData: fmt.Sprintf("not_interested:%d", id)}},
	}}
	if _, err := b.client.SendMessage(ctx, b.allowedChat, text, kb); err != nil {
		b.log.Error("send nudge", "err", err)
	}
}

func (b *Bot) handleCallback(ctx context.Context, q *CallbackQuery) {
	if q.Message == nil || !b.allowed(q.Message.Chat.ID) {
		_ = b.client.AnswerCallbackQuery(ctx, q.ID, "")
		return
	}
	action, idText, _ := strings.Cut(q.Data, ":")
	id, _ := strconv.ParseInt(idText, 10, 64)
	chat := q.Message.Chat.ID

	toast := "Noted."
	var dbErr error
	switch action {
	case "ok":
		toast = "Thanks, noted."
		dbErr = b.db.AddEvent(ctx, id, nil, "interpretation_confirmed", nil)
	case "wrong":
		toast = "Noted. Correcting it arrives in the next version."
		dbErr = b.db.AddEvent(ctx, id, nil, "interpretation_flagged", nil)
	case "do_it":
		toast = "Nice. Let's go."
		dbErr = b.db.AddEvent(ctx, id, nil, "nudge_do_it", nil)
	case "next_steps":
		toast = "Steps are in the message above."
		dbErr = b.db.AddEvent(ctx, id, nil, "nudge_next_steps", nil)
	case "later":
		toast = "No problem. Quiet for three days."
		until := time.Now().Add(72 * time.Hour)
		dbErr = b.db.SetStatus(ctx, id, "snoozed", &until)
	case "not_interested":
		toast = "Understood. I won't bring this one up again."
		dbErr = b.db.SetStatus(ctx, id, "archived", nil)
	}
	if id != 0 && dbErr == nil {
		_ = b.db.Touch(ctx, id)
	}
	if dbErr != nil && !errors.Is(dbErr, store.ErrNotFound) {
		b.log.Error("callback persist", "err", dbErr, "action", action, "thought", id)
	}
	b.log.Info("callback", "action", action, "thought", id)
	_ = b.client.AnswerCallbackQuery(ctx, q.ID, toast)
	_ = b.client.ClearKeyboard(ctx, chat, q.Message.MessageID)
	if action == "do_it" {
		b.reply(ctx, chat, "This is where the per-thought conversation will start (Phase 4). For now: pick the first step and go.")
	}
}

func (b *Bot) reply(ctx context.Context, chat int64, text string) {
	if _, err := b.client.SendMessage(ctx, chat, text, nil); err != nil {
		b.log.Error("send", "err", err)
	}
}

func humanType(t ai.ThoughtType) string {
	switch t {
	case ai.TypeActionable:
		return "something to do"
	case ai.TypeExperience:
		return "an experience or interest"
	case ai.TypeIdea:
		return "an idea"
	case ai.TypeDevelopment:
		return "personal development"
	default:
		return "not sure yet"
	}
}
