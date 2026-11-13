package telegram

import (
	"fmt"
	"strings"
	"time"

	"companion/api/internal/ai"
	"companion/api/internal/engine"
	"companion/api/internal/store"
)

// One icon and one short label per thought type. Icons are visual anchors
// so a glance at the chat tells you what kind of thing each message is.
func typeIcon(t ai.ThoughtType) string {
	switch t {
	case ai.TypeActionable:
		return "✅"
	case ai.TypeExperience:
		return "🧭"
	case ai.TypeIdea:
		return "💡"
	case ai.TypeDevelopment:
		return "🌱"
	default:
		return "❓"
	}
}

func typeLabel(t ai.ThoughtType) string {
	switch t {
	case ai.TypeActionable:
		return "Something to do"
	case ai.TypeExperience:
		return "An experience or interest"
	case ai.TypeIdea:
		return "An idea"
	case ai.TypeDevelopment:
		return "Personal development"
	default:
		return "Not sure yet"
	}
}

func typePlural(t ai.ThoughtType) string {
	switch t {
	case ai.TypeActionable:
		return "Things to do"
	case ai.TypeExperience:
		return "Experiences and interests"
	case ai.TypeIdea:
		return "Ideas"
	case ai.TypeDevelopment:
		return "Personal development"
	default:
		return "Not sure yet"
	}
}

func clean(s string) string { return strings.TrimRight(strings.TrimSpace(s), ".") }

// formatInterpretation renders the reply to a captured thought.
func formatInterpretation(thoughtID int64, i ai.Interpretation) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "%s  <b>%s</b>\n", typeIcon(i.Type), Esc(i.Title))
	fmt.Fprintf(&sb, "<i>%s · %.0f%% sure · #%d</i>\n", typeLabel(i.Type), i.Confidence*100, thoughtID)
	if i.ClarificationNeeded && i.ClarificationQuestion != "" {
		fmt.Fprintf(&sb, "\n❔ %s\n", Esc(i.ClarificationQuestion))
	}
	if len(i.NextSteps) > 0 {
		sb.WriteString("\n<b>Whenever you feel like it</b>\n")
		for n, s := range i.NextSteps {
			fmt.Fprintf(&sb, "%d.  %s  <i>~%d min</i>\n", n+1, Esc(clean(s.Label)), s.EstimatedMinutes)
		}
	}
	if len(i.Hooks) > 0 {
		sb.WriteString("\n<b>I'll bring it up when</b>\n")
		for _, h := range i.Hooks {
			fmt.Fprintf(&sb, "%s  %s\n", hookIcon(h.Kind), Esc(hookLine(h)))
		}
	}
	if len(i.Assumptions) > 0 {
		sb.WriteString("\n<b>I assumed</b>\n")
		for _, a := range i.Assumptions {
			fmt.Fprintf(&sb, "•  %s\n", Esc(clean(a)))
		}
	}
	return strings.TrimRight(sb.String(), "\n")
}

func hookIcon(kind string) string {
	switch kind {
	case ai.HookWeather:
		return "🌤"
	case ai.HookWeb:
		return "🔎"
	case ai.HookDate:
		return "📅"
	case ai.HookFreeTime:
		return "⏳"
	default:
		return "•"
	}
}

// hookLine says in plain words what the engine is watching for.
func hookLine(h ai.Hook) string {
	switch h.Kind {
	case ai.HookWeather:
		where := "near you"
		if h.Place != "" {
			where = "in " + h.Place
		}
		if h.DaysNeeded > 1 {
			return fmt.Sprintf("the weather looks good %s for %d free days", where, h.DaysNeeded)
		}
		return fmt.Sprintf("the weather looks good %s on a day off", where)
	case ai.HookWeb:
		return lowerFirst(clean(h.GoodWhen))
	case ai.HookDate:
		when := h.Date
		if d, err := time.Parse("2006-01-02", h.Date); err == nil {
			when = d.Format("Mon 2 Jan")
		}
		switch {
		case h.LeadDays > 1:
			return fmt.Sprintf("%s is %d days away or closer", when, h.LeadDays)
		default:
			return when + " is a day away"
		}
	case ai.HookFreeTime:
		return fmt.Sprintf("you have about %d free minutes", h.Minutes)
	}
	return h.Why
}

func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
}

func formatSteps(steps []ai.NextStep) string {
	if len(steps) == 0 {
		return "\n<i>No steps noted for this one. Start wherever feels natural.</i>"
	}
	var sb strings.Builder
	for n, s := range steps {
		fmt.Fprintf(&sb, "\n%d.  %s  <i>~%d min</i>", n+1, Esc(clean(s.Label)), s.EstimatedMinutes)
	}
	return sb.String()
}

func formatRoutine(p store.Prefs) string {
	var sb strings.Builder
	sb.WriteString("🗓  <b>Your routine</b>\n\n")
	fmt.Fprintf(&sb, "🌙  Sleep  <b>%s – %s</b>\n", p.QuietStart, p.QuietEnd)
	if p.HasWorkHours() {
		fmt.Fprintf(&sb, "💼  Work  <b>%s – %s</b>  <i>%s</i>\n", p.WorkStart, p.WorkEnd, engine.DayList(p.WorkDays))
		during := map[string]string{"yes": "yes", "lunch": "only at lunch", "no": "no, after work"}[p.NudgeDuringWork]
		fmt.Fprintf(&sb, "🔔  Nudges during work  <b>%s</b>\n", during)
	} else {
		sb.WriteString("💼  Work  <b>no fixed hours</b>\n")
	}
	fmt.Fprintf(&sb, "🏖  Days off  <b>%s</b>\n", engine.DayList(p.DaysOff))
	if p.HomeCity != "" {
		fmt.Fprintf(&sb, "📍  City  <b>%s</b>\n", Esc(p.HomeCity))
	} else {
		sb.WriteString("📍  City  <i>not set, so no weather-aware nudges yet</i>\n")
	}
	limit := "1 nudge a day"
	if p.DailyLimit != 1 {
		limit = fmt.Sprintf("%d nudges a day", p.DailyLimit)
	}
	fmt.Fprintf(&sb, "\n<i>At most %s. Tap to change anything.</i>", limit)
	return sb.String()
}

// formatList renders /list grouped by type, newest first within a group.
func formatList(ts []store.Thought) string {
	groups := map[ai.ThoughtType][]store.Thought{}
	for _, t := range ts {
		typ := ai.TypeUnclear
		if t.Interpretation != nil {
			typ = t.Interpretation.Type
		}
		groups[typ] = append(groups[typ], t)
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "📋  <b>Your thoughts</b>  <i>(%d)</i>\n", len(ts))
	for _, typ := range []ai.ThoughtType{ai.TypeActionable, ai.TypeExperience, ai.TypeIdea, ai.TypeDevelopment, ai.TypeUnclear} {
		items := groups[typ]
		if len(items) == 0 {
			continue
		}
		fmt.Fprintf(&sb, "\n%s  <b>%s</b>\n", typeIcon(typ), typePlural(typ))
		for _, t := range items {
			title := t.RawContent
			if t.Interpretation != nil {
				title = t.Interpretation.Title
			}
			line := fmt.Sprintf("      #%d  %s", t.ID, Esc(clean(title)))
			if t.Status == "snoozed" && t.SnoozedUntil != nil {
				line += fmt.Sprintf("  <i>😴 until %s</i>", t.SnoozedUntil.Format("2 Jan"))
			}
			sb.WriteString(line + "\n")
		}
	}
	sb.WriteString("\n<i>/done 1 · /snooze 1 7 · /drop 1</i>")
	return sb.String()
}

// formatNudge renders a nudge: what it is about, why now, one small step,
// and the fact that triggered it so the claim can be checked at a glance.
func formatNudge(n engine.Outgoing) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "%s  <b>%s</b>\n\n%s", typeIcon(n.Type), Esc(clean(n.Title)), Esc(n.Message))
	if n.Step != "" {
		fmt.Fprintf(&sb, "\n\n→  %s", Esc(clean(n.Step)))
	}
	if n.Footnote != "" {
		fmt.Fprintf(&sb, "\n\n<i>%s</i>", Esc(n.Footnote))
	}
	for _, l := range n.Links {
		if strings.HasPrefix(l.URL, "https://") || strings.HasPrefix(l.URL, "http://") {
			fmt.Fprintf(&sb, "\n<a href=\"%s\">%s</a>", Esc(l.URL), Esc(clip(l.Title, 60)))
		}
	}
	return sb.String()
}
