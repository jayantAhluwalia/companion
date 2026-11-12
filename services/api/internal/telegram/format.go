package telegram

import (
	"fmt"
	"strings"

	"companion/api/internal/ai"
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
	if len(i.Assumptions) > 0 {
		sb.WriteString("\n<b>I assumed</b>\n")
		for _, a := range i.Assumptions {
			fmt.Fprintf(&sb, "•  %s\n", Esc(clean(a)))
		}
	}
	return strings.TrimRight(sb.String(), "\n")
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

func formatNudge(title, step string) string {
	return fmt.Sprintf("👋  Still thinking about <b>%s</b>?\n\nIf you have ten minutes, you could:\n→  %s", Esc(clean(title)), Esc(clean(step)))
}
