package telegram

import (
	"context"
	"fmt"
	"strings"
	"time"

	"companion/api/internal/ai"
	"companion/api/internal/store"
)

const pendingTTL = 10 * time.Minute

var correctionTypes = map[string]ai.ThoughtType{
	"a": ai.TypeActionable, "e": ai.TypeExperience, "i": ai.TypeIdea, "d": ai.TypeDevelopment,
}

// correction handles the "Not quite" flow. A correction is never an edit:
// it is a new interpretation version authored by the user, so the AI's
// original reading stays in the history.
func (b *Bot) correction(ctx context.Context, user store.User, action string, thoughtID int64, arg string, msgID int64) string {
	chat := user.TelegramChatID
	switch action {
	case "wrong":
		_ = b.db.AddEvent(ctx, thoughtID, nil, "interpretation_flagged", nil)
		_ = b.client.EditKeyboard(ctx, chat, msgID, InlineKeyboardMarkup{InlineKeyboard: [][]InlineKeyboardButton{
			{{Text: "✅ It's a to-do", CallbackData: fmt.Sprintf("ct:%d:a", thoughtID)}, {Text: "🧭 An experience", CallbackData: fmt.Sprintf("ct:%d:e", thoughtID)}},
			{{Text: "💡 Just an idea", CallbackData: fmt.Sprintf("ct:%d:i", thoughtID)}, {Text: "🌱 Self-growth", CallbackData: fmt.Sprintf("ct:%d:d", thoughtID)}},
			{{Text: "✏️ Fix the title", CallbackData: fmt.Sprintf("cn:%d", thoughtID)}, {Text: "💬 Let me explain", CallbackData: fmt.Sprintf("cx:%d", thoughtID)}},
		}})
		return "What should I fix?"

	case "ct":
		typ, ok := correctionTypes[arg]
		t, err := b.db.GetThought(ctx, user.ID, thoughtID)
		if !ok || err != nil || t.Interpretation == nil {
			return "I couldn't find that thought."
		}
		fixed := t.Interpretation.Interpretation
		fixed.Type, fixed.Confidence = typ, 1
		fixed.ClarificationNeeded, fixed.ClarificationQuestion = false, ""
		if _, err := b.db.AddInterpretation(ctx, thoughtID, "user", fixed, "", ""); err != nil {
			b.log.Error("store correction", "err", err)
			return "Couldn't save that."
		}
		_ = b.db.Touch(ctx, thoughtID)
		_ = b.client.EditText(ctx, chat, msgID, formatInterpretation(thoughtID, fixed))
		return "Fixed."

	case "cn":
		_ = b.client.ClearKeyboard(ctx, chat, msgID)
		_ = b.db.SetPending(ctx, user.ID, &store.Pending{Kind: "retitle", ThoughtID: thoughtID, ExpiresAt: time.Now().Add(pendingTTL)})
		b.reply(ctx, chat, "✏️  What would you call it? Send me the title.\n<i>/cancel to leave it as is.</i>")
		return ""

	case "cx":
		_ = b.client.ClearKeyboard(ctx, chat, msgID)
		_ = b.db.SetPending(ctx, user.ID, &store.Pending{Kind: "clarify", ThoughtID: thoughtID, ExpiresAt: time.Now().Add(pendingTTL)})
		b.reply(ctx, chat, "💬  Tell me what you meant, in a sentence or two.\n<i>/cancel to leave it as is.</i>")
		return ""
	}
	return ""
}

// handlePending routes a typed reply to whichever question is waiting for it.
func (b *Bot) handlePending(ctx context.Context, user store.User, p store.Pending, text string) {
	chat := user.TelegramChatID
	switch p.Kind {
	case "retitle":
		t, err := b.db.GetThought(ctx, user.ID, p.ThoughtID)
		if err != nil || t.Interpretation == nil {
			b.reply(ctx, chat, "I couldn't find that thought any more.")
			return
		}
		fixed := t.Interpretation.Interpretation
		fixed.Title = clip(strings.Join(strings.Fields(text), " "), 80)
		fixed.Confidence = 1
		if _, err := b.db.AddInterpretation(ctx, p.ThoughtID, "user", fixed, "", ""); err != nil {
			b.log.Error("store retitle", "err", err)
			b.reply(ctx, chat, "Couldn't save that title.")
			return
		}
		_ = b.db.Touch(ctx, p.ThoughtID)
		b.reply(ctx, chat, formatInterpretation(p.ThoughtID, fixed))

	case "clarify":
		t, err := b.db.GetThought(ctx, user.ID, p.ThoughtID)
		if err != nil {
			b.reply(ctx, chat, "I couldn't find that thought any more.")
			return
		}
		_ = b.db.AddEvent(ctx, p.ThoughtID, nil, "clarified", map[string]any{"text": text})
		_ = b.db.Touch(ctx, p.ThoughtID)
		b.reply(ctx, chat, "👍  Thanks. Reading it again.")
		go b.interpret(user, p.ThoughtID, t.RawContent, text)

	case "city":
		// A full sentence is a thought, not a city: the user moved on.
		if len(strings.Fields(text)) > 4 {
			b.captureThought(ctx, user, text)
			return
		}
		b.onboardingTyped(ctx, user, p, text)

	case "sleep", "work":
		b.onboardingTyped(ctx, user, p, text)
	}
}

func clip(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
