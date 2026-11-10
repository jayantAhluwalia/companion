package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"companion/api/internal/ai"
)

// Bot runs the long-polling loop and the Phase 0 behaviour:
// capture -> instant ack -> async interpretation -> reply with buttons.
// Phase 1 replaces the in-memory map with Postgres; the handlers keep their shape.
type Bot struct {
	client      *Client
	ai          *ai.Client
	log         *slog.Logger
	allowedChat int64 // 0 = not locked yet; the bot then only tells callers their id

	mu       sync.Mutex
	nextID   int
	thoughts map[int]capturedThought
}

type capturedThought struct {
	Raw            string
	Interpretation *ai.Interpretation
	CapturedAt     time.Time
}

func NewBot(client *Client, aiClient *ai.Client, allowedChat int64, log *slog.Logger) *Bot {
	return &Bot{client: client, ai: aiClient, log: log, allowedChat: allowedChat, thoughts: map[int]capturedThought{}}
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
		{Command: "nudge", Description: "Send yourself a sample nudge (tests the proactive path)"},
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

	// Lock-down: until TELEGRAM_CHAT_ID is configured, or for any other chat,
	// only reveal the id so the owner can set it. Never process thoughts.
	if !b.allowed(chat) {
		b.reply(ctx, chat, fmt.Sprintf("This bot is private. Chat id: %d\nSet TELEGRAM_CHAT_ID to this value on the server to enable it.", chat))
		if b.allowedChat != 0 {
			b.log.Warn("ignored message from unknown chat", "chat", chat)
		}
		return
	}

	switch {
	case strings.HasPrefix(text, "/start"):
		b.reply(ctx, chat, "Hi. Send me anything on your mind: something you want to do, try, learn or look into. I'll remember it and, later, help you act on it.\n\nNo need to organise it. Just type it.")
	case strings.HasPrefix(text, "/id"):
		b.reply(ctx, chat, fmt.Sprintf("Chat id: %d", chat))
	case strings.HasPrefix(text, "/nudge"):
		b.SendSampleNudge(ctx)
	case strings.HasPrefix(text, "/"):
		b.reply(ctx, chat, "I don't know that command. Just send me a thought.")
	default:
		b.captureThought(ctx, chat, text)
	}
}

// captureThought follows product principle 1: save and acknowledge instantly,
// then let the interpretation arrive when it is ready.
func (b *Bot) captureThought(ctx context.Context, chat int64, raw string) {
	b.mu.Lock()
	b.nextID++
	id := b.nextID
	b.thoughts[id] = capturedThought{Raw: raw, CapturedAt: time.Now()}
	b.mu.Unlock()

	b.reply(ctx, chat, "Got it.")

	go func() {
		ictx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		interp, info, err := b.ai.Interpret(ictx, raw, time.Now())
		if err != nil {
			b.log.Error("interpret", "err", err, "stop", info.StopReason)
			b.reply(ictx, chat, "Saved, but I couldn't make sense of it just now. I'll keep the original wording.")
			return
		}
		b.mu.Lock()
		t := b.thoughts[id]
		t.Interpretation = &interp
		b.thoughts[id] = t
		b.mu.Unlock()
		b.log.Info("interpreted", "id", id, "type", interp.Type, "confidence", interp.Confidence,
			"in", info.InputTokens, "out", info.OutputTokens, "ms", info.Latency.Milliseconds())

		b.sendInterpretation(ictx, chat, id, interp)
	}()
}

func (b *Bot) sendInterpretation(ctx context.Context, chat int64, id int, i ai.Interpretation) {
	var sb strings.Builder
	fmt.Fprintf(&sb, "%s\n%s · confidence %.0f%%\n", i.Title, humanType(i.Type), i.Confidence*100)
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
		{Text: "Looks right", CallbackData: fmt.Sprintf("ok:%d", id)},
		{Text: "Not quite", CallbackData: fmt.Sprintf("wrong:%d", id)},
	}}}
	if _, err := b.client.SendMessage(ctx, chat, sb.String(), kb); err != nil {
		b.log.Error("send interpretation", "err", err)
	}
}

// SendSampleNudge proves the proactive path: the server messages the user
// unprompted, with the action buttons the real decision engine will use.
func (b *Bot) SendSampleNudge(ctx context.Context) {
	if b.allowedChat == 0 {
		return
	}
	b.mu.Lock()
	var latest *capturedThought
	var latestID int
	for id, t := range b.thoughts {
		if t.Interpretation != nil && (latest == nil || id > latestID) {
			tt := t
			latest, latestID = &tt, id
		}
	}
	b.mu.Unlock()

	text := "Sample nudge: this is what a proactive check-in will look like. Buttons work; nothing is stored yet."
	if latest != nil {
		step := "a small first step"
		if len(latest.Interpretation.NextSteps) > 0 {
			step = latest.Interpretation.NextSteps[0].Label
		}
		text = fmt.Sprintf("Still thinking about \"%s\"? If you have ten minutes, you could: %s.", latest.Interpretation.Title, step)
	}
	kb := &InlineKeyboardMarkup{InlineKeyboard: [][]InlineKeyboardButton{
		{{Text: "Let's do it", CallbackData: fmt.Sprintf("do_it:%d", latestID)}, {Text: "Show next steps", CallbackData: fmt.Sprintf("next_steps:%d", latestID)}},
		{{Text: "Later", CallbackData: fmt.Sprintf("later:%d", latestID)}, {Text: "Not interested", CallbackData: fmt.Sprintf("not_interested:%d", latestID)}},
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
	action, _, _ := strings.Cut(q.Data, ":")
	toast := map[string]string{
		"ok":             "Thanks, noted.",
		"wrong":          "Noted. You'll be able to correct it in the next version.",
		"do_it":          "Nice. Let's go.",
		"next_steps":     "Steps are in the message above.",
		"later":          "No problem. I'll bring it back another time.",
		"not_interested": "Understood. I won't bring this one up again.",
	}[action]
	if toast == "" {
		toast = "Noted."
	}
	b.log.Info("callback", "action", action, "data", q.Data)
	_ = b.client.AnswerCallbackQuery(ctx, q.ID, toast)
	_ = b.client.ClearKeyboard(ctx, q.Message.Chat.ID, q.Message.MessageID)
	if action == "do_it" {
		b.reply(ctx, q.Message.Chat.ID, "This is where the per-thought conversation will start (Phase 4). For now: pick the first step and go.")
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
