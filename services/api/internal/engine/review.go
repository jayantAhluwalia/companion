package engine

import (
	"context"
	"fmt"
	"time"

	"companion/api/internal/store"
)

const (
	reviewItems    = 5
	reviewCheckIns = 3
	// reviewOpenFor is how long after the set time a review may still go
	// out, so a missed tick or a sleeping user does not shift it to 3 am.
	reviewOpenFor = 3 * time.Hour
)

// reviewDue returns "" when the weekly review should go out now, otherwise
// the reason it should not.
func reviewDue(now time.Time, p store.Prefs, last []store.Review) string {
	if p.ReviewDay == 0 {
		return "the weekly review is turned off"
	}
	if isoWeekday(now) != p.ReviewDay {
		return "not the review day"
	}
	// Built from the clock time, not by adding minutes to midnight, so it is
	// still 19:00 on a day when the clocks change.
	start := time.Date(now.Year(), now.Month(), now.Day(), int(p.ReviewTime)/60, int(p.ReviewTime)%60, 0, 0, now.Location())
	if now.Before(start) || !now.Before(start.Add(reviewOpenFor)) {
		return "not the review time"
	}
	if len(last) > 0 && now.Sub(last[0].SentAt) < 6*24*time.Hour {
		return "this week's review has been sent"
	}
	// Nothing shows people read weekly digests, so an unread one shrinks:
	// after two reviews with no taps it goes monthly until one is used.
	if len(last) >= 2 && last[0].TapCount == 0 && last[1].TapCount == 0 && now.Sub(last[0].SentAt) < 27*24*time.Hour {
		return "the last two reviews went untouched, so the next one waits a month"
	}
	return ""
}

// Review sends the weekly review if it is due, or unconditionally when the
// user asked for it with /review. It reports whether one was sent and, if
// not, why.
func (e *Engine) Review(ctx context.Context, user store.User, onRequest bool) (bool, string, error) {
	defer e.lockUser(user.ID)()
	if e.sender == nil {
		return false, "no delivery channel configured", nil
	}
	loc, err := time.LoadLocation(user.Timezone)
	if err != nil {
		loc = time.UTC
	}
	now := time.Now().In(loc)
	prefs, err := e.db.GetPrefs(ctx, user.ID)
	if err != nil {
		return false, "", err
	}
	if !onRequest {
		if reason := hardGate(now, prefs, false); reason != "" {
			return false, reason, nil
		}
		last, err := e.db.LastReviews(ctx, user.ID, 2)
		if err != nil {
			return false, "", err
		}
		if reason := reviewDue(now, prefs, last); reason != "" {
			return false, reason, nil
		}
	}

	items, err := e.db.ReviewCandidates(ctx, user.ID, reviewItems)
	if err != nil {
		return false, "", err
	}
	checkIns, err := e.db.ReviewCheckIns(ctx, user.ID, reviewCheckIns)
	if err != nil {
		return false, "", err
	}
	if len(items)+len(checkIns) == 0 {
		return false, "nothing needs reviewing this week", nil
	}
	week, err := e.db.WeekSummary(ctx, user.ID)
	if err != nil {
		return false, "", err
	}
	trigger := "scheduled"
	if onRequest {
		trigger = "manual" // asked for with /review: free, and it does not delay the weekly one
	}
	id, err := e.db.CreateReview(ctx, user.ID, len(items)+len(checkIns), trigger)
	if err != nil {
		return false, "", err
	}
	msg := ReviewMessage{ChatID: user.TelegramChatID, ReviewID: id, Week: week, Items: items, CheckIns: checkIns}
	if err := e.sender.SendReview(ctx, msg); err != nil {
		_ = e.db.DeleteReview(ctx, id) // not delivered, so it must not count against the budget
		return false, "", fmt.Errorf("deliver review: %w", err)
	}
	return true, "", nil
}

// Status is the engine's current view of one user, for /why.
type Status struct {
	Gate    string                 // why nothing at all may be sent now, or ""
	Permit  Permit                 // what may be sent
	Windows []Window               // today's windows
	Tallies map[string]store.Tally // how each window has gone
	Load    store.Load
	Closed  string // in words, why budgeted nudges are closed; "" when open
}

func (e *Engine) Status(ctx context.Context, user store.User) (Status, error) {
	loc, err := time.LoadLocation(user.Timezone)
	if err != nil {
		loc = time.UTC
	}
	now := time.Now().In(loc)
	prefs, err := e.db.GetPrefs(ctx, user.ID)
	if err != nil {
		return Status{}, err
	}
	load, err := e.db.NudgeLoad(ctx, user.ID, day(now), now)
	if err != nil {
		return Status{}, err
	}
	tallies, err := e.db.WindowTallies(ctx, user.ID, now)
	if err != nil {
		return Status{}, err
	}
	frozen, err := e.db.WindowTallies(ctx, user.ID, day(now))
	if err != nil {
		return Status{}, err
	}
	ws := Windows(now, prefs)
	st := Status{Gate: hardGate(now, prefs, false), Windows: ws, Tallies: tallies, Load: load}
	st.Permit = permit(now, prefs, load, currentWindow(ws, now), chooseWindow(ws, frozen, daySeed(user.ID, now)), false)
	if !st.Permit.Budgeted {
		st.Closed = closedReason(st.Permit, ws)
	}
	return st, nil
}
