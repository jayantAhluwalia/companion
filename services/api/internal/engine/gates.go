// Package engine decides when to nudge. It is the "backend decides" half of
// the product rule: gates and scores are plain code, facts come from the
// signals package, and the model is only asked to choose and word a nudge
// once the code has established that now is acceptable.
package engine

import (
	"fmt"
	"time"

	"companion/api/internal/ai"
	"companion/api/internal/store"
)

const (
	// sleepBufferMin widens the sleep window on both sides: no nudge in the
	// half hour before bed or after waking.
	sleepBufferMin = 30
	lunchStart     = store.Clock(13 * 60)
	lunchEnd       = store.Clock(14 * 60)
)

// isoWeekday converts Go's Sunday=0 to ISO's Monday=1..Sunday=7.
func isoWeekday(t time.Time) int {
	if wd := int(t.Weekday()); wd != 0 {
		return wd
	}
	return 7
}

func shift(c store.Clock, minutes int) store.Clock {
	return store.Clock(((int(c)+minutes)%1440 + 1440) % 1440)
}

// AtWork reports whether now falls inside the user's stated working hours.
func AtWork(now time.Time, p store.Prefs) bool {
	return p.HasWorkHours() && p.IsWorkDay(isoWeekday(now)) && store.ClockOf(now).Between(p.WorkStart, p.WorkEnd)
}

// hardGate answers "may the app message this person at all right now?".
// It returns "" to pass or a short reason. These are rules the model cannot
// bend and no lane is exempt from. When the person is in the middle of
// messaging the app (present = true) sleep and work hours do not apply:
// they are demonstrably awake and looking at the chat.
func hardGate(now time.Time, p store.Prefs, present bool) string {
	if !p.NotificationsEnabled {
		return "paused"
	}
	// Without a city there is no timezone, and every rule about sleep, work
	// and "today" would be evaluated in the wrong one. Stay silent until
	// the person has said where they are.
	if !p.HasHome() {
		return "not_set_up"
	}
	if present {
		return ""
	}
	clock := store.ClockOf(now)
	if clock.Between(shift(p.QuietStart, -sleepBufferMin), shift(p.QuietEnd, sleepBufferMin)) {
		return "quiet_hours"
	}
	if AtWork(now, p) {
		switch p.NudgeDuringWork {
		case "no":
			return "work_hours"
		case "lunch":
			if !clock.Between(lunchStart, lunchEnd) {
				return "work_hours_until_lunch"
			}
		}
	}
	return ""
}

const (
	// obligationsPerDay caps deadline and requested reminders in one day.
	obligationsPerDay = 2
	// obligationGap is the shortest pause before an obligation may follow
	// another message. Shorter than the ordinary interval: a deadline
	// should not wait four hours because something else went out first.
	obligationGap = time.Hour
	// quickPerWeek is the share of the weekly budget that reminders with no
	// outside trigger may use, so they cannot crowd out real opportunities.
	quickPerWeek = 3
)

// Permit is what may be sent right now, and why not when it may not.
//
// Nudges fall into two groups. Obligations (a deadline, or a time the user
// picked with "later") go out whatever else was sent, because a reminder
// with no reason must never block one with a date on it. Everything else is
// budgeted: at most one a day, a few a week, and only in the day's window.
type Permit struct {
	Obligation bool   `json:"obligation"`
	Budgeted   bool   `json:"budgeted"`
	Quick      bool   `json:"quick"`
	Reason     string `json:"reason,omitempty"` // why budgeted nudges are closed
	Window     string `json:"window,omitempty"` // the window now is in, if any
	Chosen     string `json:"chosen,omitempty"` // the window picked for today
	Budget     int    `json:"budget"`           // this week's budget after back-off
	Used       int    `json:"used"`             // of which already used
}

// effectiveBudget shrinks the weekly budget when recent nudges went
// unanswered: two in a row halve it, four reduce it to one a week. One
// response restores it, because the run is counted from the newest nudge.
func effectiveBudget(base, ignoredRun int) int {
	switch {
	case ignoredRun >= 4:
		return 1
	case ignoredRun >= 2:
		return max(1, base/2)
	default:
		return max(1, base)
	}
}

func permit(now time.Time, p store.Prefs, l store.Load, current, chosen string, present bool) Permit {
	pm := Permit{Window: current, Chosen: chosen, Budget: effectiveBudget(p.WeeklyBudget, l.IgnoredRun), Used: l.BudgetedWeek + l.ReviewsWeek}
	since := func(d time.Duration) bool { return l.LastSent == nil || now.Sub(*l.LastSent) >= d }

	pm.Obligation = l.ObligationToday < obligationsPerDay && since(obligationGap)

	switch {
	case l.BudgetedToday >= p.DailyLimit:
		pm.Reason = "daily_cap"
	case pm.Used >= pm.Budget:
		pm.Reason = "weekly_budget"
	case !since(time.Duration(p.MinIntervalMinutes) * time.Minute):
		pm.Reason = "min_interval"
	case !present && (current == "" || current != chosen):
		pm.Reason = "outside_window"
	default:
		pm.Budgeted = true
		pm.Quick = l.QuickWeek < quickPerWeek
	}
	return pm
}

// cooldownDays is how long a thought rests after a nudge: three days,
// doubling for every nudge in a row that went unanswered, capped at 30.
func cooldownDays(ignoredStreak int) int {
	days := 3
	for i := 0; i < ignoredStreak && days < 30; i++ {
		days *= 2
	}
	return min(days, 30)
}

// onCooldown returns a human-readable hold reason, or "".
func onCooldown(t store.Thought, now time.Time) string {
	if t.LastNudgedAt == nil {
		return ""
	}
	until := t.LastNudgedAt.Add(time.Duration(cooldownDays(t.IgnoredStreak)) * 24 * time.Hour)
	if now.Before(until) {
		return fmt.Sprintf("resting until %s", until.In(now.Location()).Format("Mon 2 Jan"))
	}
	return ""
}

// freshFor is how long a thought is left alone after the user last touched
// it, unless something makes it timely sooner.
const freshFor = 48 * time.Hour

// tooFresh holds back a thought the user captured or acted on in the last
// two days. A fired hook overrides it: if the ride was mentioned yesterday
// and the weekend is dry, that is exactly when to speak. Free time does not
// count: it comes round every evening, so it is not a moment worth
// interrupting a rest for (found in the 13 Nov review, when "the workday is
// over" surfaced a one-day-old idea).
func tooFresh(t store.Thought, hooks []HookState, now time.Time) string {
	until := t.LastTouchedAt.Add(freshFor)
	if !now.Before(until) {
		return ""
	}
	for _, h := range hooks {
		if h.State == StateFired && h.Kind != ai.HookFreeTime {
			return ""
		}
	}
	return fmt.Sprintf("you touched this recently; resting until %s", until.In(now.Location()).Format("Mon 2 Jan, 15:04"))
}
