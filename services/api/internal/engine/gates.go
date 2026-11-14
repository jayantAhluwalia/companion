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

// Gate answers "is now an acceptable moment to message at all?". It returns
// "" to pass or a short reason. Which thought to pick is the score's job;
// whether to interrupt is decided here, by rules the model cannot bend.
func Gate(now time.Time, p store.Prefs, sentToday int, lastSent *time.Time) string {
	clock := store.ClockOf(now)
	switch {
	case !p.NotificationsEnabled:
		return "paused"
	case clock.Between(shift(p.QuietStart, -sleepBufferMin), shift(p.QuietEnd, sleepBufferMin)):
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
	if sentToday >= p.DailyLimit {
		return "daily_cap"
	}
	if lastSent != nil && now.Sub(*lastSent) < time.Duration(p.MinIntervalMinutes)*time.Minute {
		return "min_interval"
	}
	return ""
}

// criticalExtra is how many nudges beyond the daily limit a deadline may
// claim. A reminder with no outside reason must never be able to block one
// with a date on it (14 Nov review: a beard-oil check-in at 09:00 used the
// day's slot and held back "service the motorcycle tomorrow morning").
const criticalExtra = 1

// criticalOnly reports whether the day's ordinary nudge is used but a
// deadline may still go out. The interval between nudges still applies.
func criticalOnly(gateReason string, now time.Time, p store.Prefs, sentToday int, lastSent *time.Time) bool {
	if gateReason != "daily_cap" || sentToday >= p.DailyLimit+criticalExtra {
		return false
	}
	return lastSent == nil || now.Sub(*lastSent) >= time.Duration(p.MinIntervalMinutes)*time.Minute
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
