package engine

import (
	"fmt"
	"time"

	"companion/api/internal/ai"
	"companion/api/internal/store"
)

// Lanes. A lane says why a thought might be pushed, and that decides which
// rules apply to it.
const (
	LaneRequested   = "requested"   // the user tapped "later" and picked this time
	LaneDeadline    = "deadline"    // a date that is today or tomorrow
	LaneOpportunity = "opportunity" // an outside trigger fired: weather, a web finding, a date coming into range
	LaneQuick       = "quick"       // a small thing that can be done right now, with no outside trigger
	LaneReview      = "review"      // never pushed on its own; offered in the weekly review
)

func isObligation(lane string) bool { return lane == LaneRequested || lane == LaneDeadline }

const quickStepMinutes = 15

// laneOf places a scored thought in a lane. The rule behind the split: push
// only what can be acted on now, and review the rest on a rhythm. An idea
// with no trigger gives nothing to do in the moment, so a ping about it is
// noise; a five-minute errand is the opposite.
func laneOf(t store.Thought, sc Scored, now time.Time) string {
	if t.RemindAt != nil && !t.RemindAt.After(now) {
		return LaneRequested
	}
	if sc.critical() {
		return LaneDeadline
	}
	freeTime := false
	for _, h := range sc.Hooks {
		if h.State != StateFired {
			continue
		}
		if h.Kind == ai.HookFreeTime {
			freeTime = true
			continue
		}
		return LaneOpportunity
	}
	in := t.Interpretation
	urgent := in.Urgency == ai.UrgencyNow
	if freeTime || (in.Type == ai.TypeActionable && (urgent || hasQuickStep(in.NextSteps))) {
		return LaneQuick
	}
	return LaneReview
}

func hasQuickStep(steps []ai.NextStep) bool {
	for _, s := range steps {
		if s.EstimatedMinutes > 0 && s.EstimatedMinutes <= quickStepMinutes {
			return true
		}
	}
	return false
}

const (
	justCapturedFor = 30 * time.Minute
	// reviewOnlyAfter is how many unanswered nudges in a row move a thought
	// out of the push lanes: after that it is only offered in the review.
	reviewOnlyAfter = 2
	// deadlineTouches is the most reminders one deadline gets: a heads-up
	// the day before and the moment itself. The trials behind this found
	// two reminders beat one and nothing for more.
	deadlineTouches = 2
)

func sameDay(a, b time.Time) bool {
	a = a.In(b.Location())
	return a.Year() == b.Year() && a.YearDay() == b.YearDay()
}

// dueIn is the nearest critical deadline of a thought, in days (0 or 1).
func dueIn(sc Scored) int {
	due := 1
	for _, h := range sc.Hooks {
		if h.Critical && h.DueIn < due {
			due = h.DueIn
		}
	}
	return due
}

// holdReason says why a thought will not be sent in this pass, or "" if it
// may be. The reasons are written for the user: /why shows them as they are.
func holdReason(t store.Thought, sc Scored, touch store.DeadlineTouch, pm Permit, ws []Window, now time.Time) string {
	if now.Sub(t.CreatedAt) < justCapturedFor {
		return "you told me this a moment ago"
	}
	switch sc.Lane {
	case LaneRequested:
		if !pm.Obligation {
			return "waiting for a gap after the last message"
		}
		return ""

	case LaneDeadline:
		switch {
		case touch.Count >= deadlineTouches:
			return "both reminders for this date have been sent"
		case touch.Count > 0 && sameDay(touch.Last, now):
			return "already reminded today"
		case !pm.Obligation:
			return "waiting for a gap after the last message"
		}
		// Due tomorrow: the heads-up goes in today's window, not at dawn.
		// Due today: now.
		if dueIn(sc) == 1 && pm.Chosen != "" {
			if start := windowStart(ws, pm.Chosen); store.ClockOf(now) < start {
				return fmt.Sprintf("due tomorrow; the heads-up goes out at %s today", start)
			}
		}
		return ""

	case LaneOpportunity, LaneQuick:
		if !pm.Budgeted {
			return closedReason(pm, ws)
		}
		if hold := onCooldown(t, now); hold != "" {
			return hold
		}
		if t.IgnoredStreak >= reviewOnlyAfter {
			return "went unanswered twice, so it is kept for the weekly review"
		}
		if sc.Lane == LaneOpportunity {
			return ""
		}
		urgent := t.Interpretation.Urgency == ai.UrgencyNow
		switch {
		case !pm.Quick:
			return "this week's reminders without a trigger are used up"
		case urgent:
			return ""
		}
		if hold := tooFresh(t, sc.Hooks, now); hold != "" {
			return hold
		}
		if sc.Score < Threshold {
			return "waiting for its moment: the trigger I am watching has not fired"
		}
		return ""

	default:
		return "no action to take right now, so it waits for the weekly review"
	}
}

// closedReason explains in words why budgeted nudges cannot go out now.
func closedReason(pm Permit, ws []Window) string {
	switch pm.Reason {
	case "daily_cap":
		return "today's nudge has been sent"
	case "weekly_budget":
		return fmt.Sprintf("this week's %d nudges are used", pm.Budget)
	case "min_interval":
		return "the last message was too recent"
	case "outside_window":
		if pm.Chosen == "" {
			return "no send window today"
		}
		for _, w := range ws {
			if w.Key == pm.Chosen {
				return fmt.Sprintf("today's window is %s to %s", w.Start, w.End)
			}
		}
	}
	return pm.Reason
}
