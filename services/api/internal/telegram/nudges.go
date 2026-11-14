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
		// Park it for three days straight away, so nothing dangles if the
		// follow-up question is never answered. Picking a time replaces this.
		until := time.Now().Add(72 * time.Hour)
		if err := b.db.SetStatus(ctx, thoughtID, "snoozed", &until); err != nil {
			b.log.Error("snooze", "err", err)
		}
		return "No problem. When should I bring it back?"
	case "not_interested":
		if err := b.db.SetStatus(ctx, thoughtID, "archived", nil); err != nil {
			b.log.Error("archive", "err", err)
		}
		return "Understood. I won't bring this one up again."
	}
	return "Noted."
}

// laterKeyboard asks when to bring a thought back. The answer is the most
// direct evidence there is of when this person expects to be able to act.
func laterKeyboard(thoughtID int64) InlineKeyboardMarkup {
	d := func(opt string) string { return fmt.Sprintf("lt:%d:%s", thoughtID, opt) }
	return InlineKeyboardMarkup{InlineKeyboard: [][]InlineKeyboardButton{
		{btn("⏳ In a few hours", d("h")), btn("🌅 Tomorrow morning", d("t"))},
		{btn("🏖 Next day off", d("w")), btn("📆 Next week", d("n"))},
	}}
}

// laterTime turns a "later" choice into a moment in the user's own day.
func laterTime(opt string, now time.Time, p store.Prefs) time.Time {
	at := func(d time.Time, c store.Clock) time.Time {
		return time.Date(d.Year(), d.Month(), d.Day(), int(c)/60, int(c)%60, 0, 0, now.Location())
	}
	morning := store.Clock(int(p.QuietEnd) + 60)
	tomorrowMorning := at(now.AddDate(0, 0, 1), morning)
	switch opt {
	case "h":
		t := now.Add(3 * time.Hour)
		// Three hours from now may be bedtime; then it is tomorrow's job.
		if store.ClockOf(t).Between(store.Clock((int(p.QuietStart)-60+1440)%1440), morning) {
			return tomorrowMorning
		}
		return t
	case "w":
		for lead := 1; lead <= 7; lead++ {
			d := now.AddDate(0, 0, lead)
			wd := int(d.Weekday())
			if wd == 0 {
				wd = 7
			}
			if p.IsDayOff(wd) {
				return at(d, max(store.Clock(10*60+30), store.Clock(int(p.QuietEnd)+120)))
			}
		}
		return now.AddDate(0, 0, 7)
	case "n":
		// The same time next week: they were reachable at this hour today.
		return now.AddDate(0, 0, 7)
	default:
		return tomorrowMorning
	}
}

func (b *Bot) laterChoice(ctx context.Context, user store.User, thoughtID int64, opt string) string {
	t, err := b.db.GetThought(ctx, user.ID, thoughtID)
	if err != nil {
		return "I couldn't find that thought."
	}
	prefs, err := b.db.GetPrefs(ctx, user.ID)
	if err != nil {
		return "Couldn't load your routine."
	}
	now := userNow(user)
	when := laterTime(opt, now, prefs)
	if err := b.db.SnoozeWithReminder(ctx, t.ID, when); err != nil {
		b.log.Error("snooze with reminder", "err", err)
		return "Couldn't save that."
	}
	_ = b.db.AddEvent(ctx, t.ID, nil, "later_chosen", map[string]any{"option": opt, "at": when.Format(time.RFC3339)})
	if when.YearDay() == now.YearDay() {
		return "I'll bring it back around " + when.Format("15:04") + "."
	}
	return "I'll bring it back " + when.Format("Mon 2 Jan") + " around " + when.Format("15:04") + "."
}

// reviewNow is /review: go through the backlog now instead of waiting for
// the weekly slot.
func (b *Bot) reviewNow(ctx context.Context, user store.User) {
	go func() {
		rctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		sent, why, err := b.eng.Review(rctx, user, true)
		switch {
		case err != nil:
			b.log.Error("review", "err", err)
			b.reply(rctx, user.TelegramChatID, "Something went wrong putting the review together.")
		case !sent:
			b.reply(rctx, user.TelegramChatID, "✨  "+Esc(why)+".")
		}
	}()
}

// SendReview implements engine.Sender for the weekly review. Each item gets
// its own row of buttons, and each tap is recorded as a decision: the
// evidence for periodic reviews is that they help when outcomes are logged.
func (b *Bot) SendReview(ctx context.Context, r engine.ReviewMessage) error {
	d := func(thoughtID int64, answer string) string {
		return fmt.Sprintf("rv:%d:%d:%s", r.ReviewID, thoughtID, answer)
	}
	var rows [][]InlineKeyboardButton
	for _, it := range r.CheckIns {
		rows = append(rows, []InlineKeyboardButton{
			btn(fmt.Sprintf("#%d  ✅ Done", it.ThoughtID), d(it.ThoughtID, "y")),
			btn(fmt.Sprintf("#%d  Not yet", it.ThoughtID), d(it.ThoughtID, "n")),
		})
	}
	for _, it := range r.Items {
		rows = append(rows, []InlineKeyboardButton{
			btn(fmt.Sprintf("#%d  Keep", it.ThoughtID), d(it.ThoughtID, "k")),
			btn("😴 A month", d(it.ThoughtID, "m")),
			btn("🗑 Drop", d(it.ThoughtID, "d")),
		})
	}
	_, err := b.client.SendMessage(ctx, r.ChatID, formatReview(r), &InlineKeyboardMarkup{InlineKeyboard: rows})
	return err
}

// reviewAnswer records one decision from a review and removes that row.
func (b *Bot) reviewAnswer(ctx context.Context, user store.User, reviewID, thoughtID int64, answer string, m *Message) string {
	t, err := b.db.GetThought(ctx, user.ID, thoughtID)
	if err != nil {
		return "I couldn't find that thought."
	}
	toast := "Noted."
	switch answer {
	case "y":
		err = b.db.SetStatus(ctx, t.ID, "done", nil)
		_ = b.db.AddEvent(ctx, t.ID, nil, "review_done", nil)
		toast = "🎉 Marked done."
	case "n":
		_ = b.db.AddEvent(ctx, t.ID, nil, "review_not_yet", nil)
		err = b.db.Touch(ctx, t.ID)
		toast = "No problem. Still on the list."
	case "k":
		_ = b.db.AddEvent(ctx, t.ID, nil, "review_keep", nil)
		err = b.db.Touch(ctx, t.ID)
		toast = "Kept."
	case "m":
		until := time.Now().AddDate(0, 0, 30)
		err = b.db.SetStatus(ctx, t.ID, "snoozed", &until)
		_ = b.db.AddEvent(ctx, t.ID, nil, "review_snooze", nil)
		toast = "Out of sight for a month."
	case "d":
		err = b.db.SetStatus(ctx, t.ID, "archived", nil)
		_ = b.db.AddEvent(ctx, t.ID, nil, "review_drop", nil)
		toast = "Let go."
	}
	if err != nil {
		b.log.Error("review answer", "err", err, "thought", t.ID)
		return "Couldn't save that."
	}
	_ = b.db.CountReviewTap(ctx, reviewID)

	// Take the answered row away and leave the others.
	if m != nil && m.ReplyMarkup != nil {
		marker := fmt.Sprintf(":%d:", thoughtID)
		var rest [][]InlineKeyboardButton
		for _, row := range m.ReplyMarkup.InlineKeyboard {
			if len(row) > 0 && strings.Contains(row[0].CallbackData, marker) && strings.HasPrefix(row[0].CallbackData, "rv:") {
				continue
			}
			rest = append(rest, row)
		}
		if rest == nil {
			rest = [][]InlineKeyboardButton{}
			toast += " That's all of them."
		}
		_ = b.client.EditKeyboard(ctx, m.Chat.ID, m.MessageID, InlineKeyboardMarkup{InlineKeyboard: rest})
	}
	return toast
}

var gateReasons = map[string]string{
	"paused":                 "nudges are paused (/resume turns them on)",
	"quiet_hours":            "it is inside your sleep window",
	"work_hours":             "you are at work and asked not to be nudged there",
	"work_hours_until_lunch": "you are at work; I only nudge at lunch (13:00–14:00)",
}

var laneLabels = map[string]string{
	engine.LaneRequested: "you asked for this time", engine.LaneDeadline: "deadline", engine.LaneOpportunity: "opportunity",
	engine.LaneQuick: "quick action", engine.LaneReview: "weekly review",
}

var windowLabels = map[string]string{
	engine.WindowLunch: "lunch", engine.WindowEvening: "evening", engine.WindowOffMorning: "late morning",
	engine.WindowOffAfternoon: "afternoon", engine.WindowChat: "while chatting",
}

// why answers "why did or didn't you nudge me?" from the engine's current
// state and its decision log.
func (b *Bot) why(ctx context.Context, user store.User) {
	chat := user.TelegramChatID
	st, err := b.eng.Status(ctx, user)
	if err != nil {
		b.log.Error("status", "err", err)
		b.reply(ctx, chat, "Couldn't load the engine's state just now.")
		return
	}
	now := userNow(user)
	var sb strings.Builder
	sb.WriteString("🔍  <b>Why, or why not</b>\n\n")

	switch {
	case st.Gate != "":
		fmt.Fprintf(&sb, "<b>Right now</b>  I would stay quiet: %s.\n", gateReasons[st.Gate])
	case st.Closed != "":
		fmt.Fprintf(&sb, "<b>Right now</b>  Only a deadline, or a time you picked, would go out: %s.\n", Esc(st.Closed))
	default:
		sb.WriteString("<b>Right now</b>  I am free to nudge if something can be acted on.\n")
	}
	fmt.Fprintf(&sb, "<b>This week</b>  %d of %d nudges used", st.Permit.Used, st.Permit.Budget)
	if st.Load.IgnoredRun >= 2 {
		fmt.Fprintf(&sb, " <i>(reduced: the last %d went unanswered)</i>", st.Load.IgnoredRun)
	}
	sb.WriteString("\n")
	for _, w := range st.Windows {
		if w.Key == st.Permit.Chosen {
			fmt.Fprintf(&sb, "<b>Today's window</b>  %s, %s to %s\n", windowLabels[w.Key], w.Start, w.End)
		}
	}
	var record []string
	for _, key := range []string{engine.WindowLunch, engine.WindowEvening, engine.WindowOffMorning, engine.WindowOffAfternoon, engine.WindowChat} {
		if t, ok := st.Tallies[key]; ok && t.Acted+t.Ignored > 0 {
			record = append(record, fmt.Sprintf("%s %d/%d", windowLabels[key], t.Acted, t.Acted+t.Ignored))
		}
	}
	if len(record) > 0 {
		fmt.Fprintf(&sb, "<b>Acted on, by window</b>  %s\n", strings.Join(record, " · "))
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
		sb.WriteString("\n<b>Your thoughts at that moment</b>\n")
	}
	for i, s := range scores {
		if i == 7 {
			fmt.Fprintf(&sb, "<i>…and %d more</i>\n", len(scores)-i)
			break
		}
		lane := laneLabels[s.Lane]
		if lane != "" {
			lane = "  <i>· " + lane + "</i>"
		}
		fmt.Fprintf(&sb, "\n#%d  %s%s\n", s.ThoughtID, Esc(clean(s.Title)), lane)
		if s.Held != "" {
			fmt.Fprintf(&sb, "      ⏸ <i>%s</i>\n", Esc(s.Held))
		} else {
			sb.WriteString("      ▶️ <i>ready to send</i>\n")
		}
		for _, h := range s.Hooks {
			fmt.Fprintf(&sb, "      %s <i>%s:</i> %s\n", hookIcon(h.Kind), h.State, Esc(clip(h.Summary, 150)))
		}
	}
	b.reply(ctx, chat, sb.String())
}
