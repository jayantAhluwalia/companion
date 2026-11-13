package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"companion/api/internal/engine"
	"companion/api/internal/store"
)

// SendNudge implements engine.Sender. The four buttons carry the same action
// ids the iOS notification category uses, so the engine never knows which
// channel delivered a nudge.
func (b *Bot) SendNudge(ctx context.Context, n engine.Outgoing) (string, error) {
	d := func(action string) string { return fmt.Sprintf("%s:%d:%d", action, n.ThoughtID, n.NudgeID) }
	kb := &InlineKeyboardMarkup{InlineKeyboard: [][]InlineKeyboardButton{
		{btn("🚀 Let's do it", d("do_it")), btn("📝 Next steps", d("next_steps"))},
		{btn("⏰ Later", d("later")), btn("🙅 Not interested", d("not_interested"))},
	}}
	m, err := b.client.SendMessage(ctx, n.ChatID, formatNudge(n), kb)
	if err != nil {
		return "", err
	}
	return fmt.Sprint(m.MessageID), nil
}

// nudgeNow is /nudge: the user asks for the best thing to bring up now, so
// gates, cooldowns and the score bar are skipped.
func (b *Bot) nudgeNow(ctx context.Context, user store.User) {
	b.reply(ctx, user.TelegramChatID, "🔎  Looking for the best thing to bring up…")
	go func() {
		ectx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		res, err := b.eng.Evaluate(ectx, user, engine.Options{Trigger: "manual", Force: true})
		switch {
		case err != nil:
			b.log.Error("manual evaluate", "err", err)
			b.reply(ectx, user.TelegramChatID, "Something went wrong while I was thinking. /why has the details.")
		case res.Outcome != "nudged":
			b.reply(ectx, user.TelegramChatID, "🤫  Nothing to bring up: "+Esc(res.Reason))
		}
	}()
}

// EvaluateNow runs the engine for the one configured user; the admin
// endpoints call it.
func (b *Bot) EvaluateNow(ctx context.Context, opt engine.Options) (engine.Result, error) {
	if b.allowedChat == 0 {
		return engine.Result{}, engine.ErrNoUser
	}
	user, err := b.db.GetOrCreateUser(ctx, b.allowedChat, "")
	if err != nil {
		return engine.Result{}, err
	}
	return b.eng.Evaluate(ctx, user, opt)
}

// nudgeResponse records a button press on a nudge and returns the toast.
func (b *Bot) nudgeResponse(ctx context.Context, user store.User, action string, thoughtID, nudgeID int64) string {
	chat := user.TelegramChatID
	if nudgeID != 0 {
		if err := b.db.RespondNudge(ctx, nudgeID, action); err != nil {
			b.log.Warn("respond nudge", "err", err, "nudge", nudgeID)
		}
	}
	var nid *int64
	if nudgeID != 0 {
		nid = &nudgeID
	}
	_ = b.db.AddEvent(ctx, thoughtID, nid, "nudge_"+action, nil)
	_ = b.db.Touch(ctx, thoughtID)

	switch action {
	case "do_it", "next_steps":
		t, err := b.db.GetThought(ctx, user.ID, thoughtID)
		if err != nil || t.Interpretation == nil {
			return "Go for it."
		}
		head := "📝  <b>" + Esc(clean(t.Interpretation.Title)) + "</b>"
		if action == "do_it" {
			head = "🚀  <b>Nice.</b> Here is the path for <b>" + Esc(clean(t.Interpretation.Title)) + "</b>"
		}
		b.reply(ctx, chat, head+"\n"+formatSteps(t.Interpretation.NextSteps)+fmt.Sprintf("\n\n<i>When it's finished: /done %d</i>", thoughtID))
		if action == "do_it" {
			return "Nice. Let's go."
		}
		return "Here they are."
	case "later":
		until := time.Now().Add(72 * time.Hour)
		if err := b.db.SetStatus(ctx, thoughtID, "snoozed", &until); err != nil {
			b.log.Error("snooze", "err", err)
		}
		return "No problem. Quiet for three days."
	case "not_interested":
		if err := b.db.SetStatus(ctx, thoughtID, "archived", nil); err != nil {
			b.log.Error("archive", "err", err)
		}
		return "Understood. I won't bring this one up again."
	}
	return "Noted."
}

var gateReasons = map[string]string{
	"paused":                 "nudges are paused (/resume turns them on)",
	"quiet_hours":            "it is inside your sleep window",
	"work_hours":             "you are at work and asked not to be nudged there",
	"work_hours_until_lunch": "you are at work; I only nudge at lunch (13:00–14:00)",
	"daily_cap":              "today's nudge has already been sent",
	"min_interval":           "the last nudge was too recent",
}

// why answers "why did or didn't you nudge me?" from the decision log.
func (b *Bot) why(ctx context.Context, user store.User) {
	chat := user.TelegramChatID
	prefs, err := b.db.GetPrefs(ctx, user.ID)
	if err != nil {
		b.reply(ctx, chat, "Couldn't load your settings just now.")
		return
	}
	now := userNow(user)
	var sb strings.Builder
	sb.WriteString("🔍  <b>Why, or why not</b>\n\n")

	sentToday, lastSent, _ := b.db.NudgeStats(ctx, user.ID, time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()))
	if reason := engine.Gate(now, prefs, sentToday, lastSent); reason != "" {
		fmt.Fprintf(&sb, "<b>Right now</b>  I would stay quiet: %s.\n", gateReasons[reason])
	} else {
		sb.WriteString("<b>Right now</b>  I am free to nudge if something is timely.\n")
	}

	runs, err := b.db.LastRuns(ctx, user.ID, 1, true)
	if err != nil || len(runs) == 0 {
		sb.WriteString("\nI have not made a decision yet. I check every half hour.")
		b.reply(ctx, chat, sb.String())
		return
	}
	run := runs[0]
	loc := now.Location()
	fmt.Fprintf(&sb, "\n<b>Last decision</b>  <i>%s · %s</i>\n", run.StartedAt.In(loc).Format("Mon 2 Jan, 15:04"), run.Trigger)
	outcome := map[string]string{"nudged": "✅ I nudged you", "none": "🤫 I stayed quiet", "dry_run": "🧪 Dry run",
		"error": "⚠️ Something failed"}[run.Outcome]
	if outcome == "" {
		outcome = run.Outcome
	}
	fmt.Fprintf(&sb, "%s\n<i>%s</i>\n", outcome, Esc(clip(run.Reason, 300)))

	var scores []engine.Scored
	_ = json.Unmarshal(run.Scores, &scores)
	if len(scores) > 0 {
		sb.WriteString("\n<b>How your thoughts ranked</b>\n")
	}
	for i, s := range scores {
		if i == 6 {
			fmt.Fprintf(&sb, "<i>…and %d more</i>\n", len(scores)-i)
			break
		}
		if s.Held != "" {
			fmt.Fprintf(&sb, "\n#%d  %s\n      <i>%s</i>\n", s.ThoughtID, Esc(clean(s.Title)), Esc(s.Held))
			for _, h := range s.Hooks {
				fmt.Fprintf(&sb, "      %s <i>%s:</i> %s\n", hookIcon(h.Kind), h.State, Esc(clip(h.Summary, 160)))
			}
			continue
		}
		mark := ""
		if s.Score >= engine.Threshold {
			mark = "  ✓"
		}
		fmt.Fprintf(&sb, "\n#%d  %s  <b>%.2f</b>%s\n", s.ThoughtID, Esc(clean(s.Title)), s.Score, mark)
		fmt.Fprintf(&sb, "      <i>timely %.1f · rested %.1f · matters %.1f · doable %.1f · fits today %.1f</i>\n", s.T, s.R, s.I, s.A, s.O)
		for _, h := range s.Hooks {
			fmt.Fprintf(&sb, "      %s <i>%s:</i> %s\n", hookIcon(h.Kind), h.State, Esc(clip(h.Summary, 160)))
		}
	}
	fmt.Fprintf(&sb, "\n<i>A thought needs %.2f to be considered. “Timely” is high when its moment has come and low while I am waiting for it.</i>", engine.Threshold)
	b.reply(ctx, chat, sb.String())
}
