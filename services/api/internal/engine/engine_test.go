package engine

import (
	"testing"
	"time"

	"companion/api/internal/ai"
	"companion/api/internal/store"
)

var ist = time.FixedZone("IST", 5*3600+1800)

// A typical routine: sleep 23–07, work 10–19 Mon–Fri, nudges at lunch only.
var homeLat, homeLon = 28.65, 77.23

func prefs() store.Prefs {
	return store.Prefs{
		HomeLat: &homeLat, HomeLon: &homeLon, HomeCity: "Delhi, India", MonthlyBudgetUSD: 4,
		QuietStart: 23 * 60, QuietEnd: 7 * 60, WorkStart: 10 * 60, WorkEnd: 19 * 60,
		WorkDays: []int{1, 2, 3, 4, 5}, DaysOff: []int{6, 7}, NudgeDuringWork: "lunch",
		DailyLimit: 1, MinIntervalMinutes: 240, NotificationsEnabled: true, WeeklyBudget: 5,
	}
}

// 13 Nov 2026 is a Friday.
func at(dayOfMonth, hour, minute int) time.Time {
	return time.Date(2026, 11, dayOfMonth, hour, minute, 0, 0, ist)
}

func TestHardGate(t *testing.T) {
	cases := []struct {
		name    string
		now     time.Time
		edit    func(*store.Prefs)
		present bool
		want    string
	}{
		{name: "friday morning before work", now: at(13, 8, 30), want: ""},
		{name: "asleep", now: at(13, 2, 0), want: "quiet_hours"},
		{name: "half hour after waking is still quiet", now: at(13, 7, 15), want: "quiet_hours"},
		{name: "half hour before bed is quiet", now: at(13, 22, 45), want: "quiet_hours"},
		{name: "at work, lunch only", now: at(13, 11, 0), want: "work_hours_until_lunch"},
		{name: "lunch hour passes", now: at(13, 13, 30), want: ""},
		{name: "at work, asked for none", now: at(13, 13, 30), edit: func(p *store.Prefs) { p.NudgeDuringWork = "no" }, want: "work_hours"},
		{name: "at work, allowed", now: at(13, 11, 0), edit: func(p *store.Prefs) { p.NudgeDuringWork = "yes" }, want: ""},
		{name: "saturday is not a work day", now: at(14, 11, 0), want: ""},
		{name: "paused", now: at(13, 8, 30), edit: func(p *store.Prefs) { p.NotificationsEnabled = false }, want: "paused"},
		{name: "chatting late at night is not sleep", now: at(13, 23, 30), present: true, want: ""},
		{name: "chatting at work is fine", now: at(13, 11, 0), present: true, want: ""},
		{name: "paused wins even when chatting", now: at(13, 11, 0), present: true, edit: func(p *store.Prefs) { p.NotificationsEnabled = false }, want: "paused"},
		{name: "no city means no timezone, so nothing proactive", now: at(13, 13, 30), edit: func(p *store.Prefs) { p.HomeLat, p.HomeLon = nil, nil }, want: "not_set_up"},
		{name: "not even while chatting", now: at(13, 13, 30), present: true, edit: func(p *store.Prefs) { p.HomeLat, p.HomeLon = nil, nil }, want: "not_set_up"},
	}
	for _, c := range cases {
		p := prefs()
		if c.edit != nil {
			c.edit(&p)
		}
		if got := hardGate(c.now, p, c.present); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

// The permit is the heart of the strategy: obligations always have a way
// out, everything else shares one budget and one window a day.
func TestPermit(t *testing.T) {
	p := prefs() // weekly budget 5, daily limit 1
	now := at(13, 19, 30)
	morning := at(13, 9, 0)
	justNow := at(13, 19, 10)
	cases := []struct {
		name           string
		load           store.Load
		current        string
		chosen         string
		present        bool
		wantBudgeted   bool
		wantObligation bool
		wantReason     string
	}{
		{name: "in today's window with budget left", current: WindowEvening, chosen: WindowEvening, wantBudgeted: true, wantObligation: true},
		{name: "wrong window", current: WindowEvening, chosen: WindowLunch, wantObligation: true, wantReason: "outside_window"},
		{name: "no window right now", current: "", chosen: WindowEvening, wantObligation: true, wantReason: "outside_window"},
		{name: "chatting ignores the window", current: "", chosen: WindowEvening, present: true, wantBudgeted: true, wantObligation: true},
		{name: "today's nudge already sent, a deadline may still go",
			load: store.Load{BudgetedToday: 1, BudgetedWeek: 1, LastSent: &morning}, current: WindowEvening, chosen: WindowEvening,
			wantObligation: true, wantReason: "daily_cap"},
		{name: "weekly budget used, a deadline may still go",
			load: store.Load{BudgetedWeek: 4, ReviewsWeek: 1, LastSent: &morning}, current: WindowEvening, chosen: WindowEvening,
			wantObligation: true, wantReason: "weekly_budget"},
		{name: "two obligations today is the limit",
			load: store.Load{ObligationToday: 2, LastSent: &morning}, current: WindowEvening, chosen: WindowEvening,
			wantBudgeted: true, wantObligation: false},
		{name: "nothing follows a message within the hour, even a deadline",
			load: store.Load{LastSent: &justNow}, current: WindowEvening, chosen: WindowEvening, wantReason: "min_interval"},
	}
	for _, c := range cases {
		pm := permit(now, p, c.load, c.current, c.chosen, c.present)
		if pm.Budgeted != c.wantBudgeted || pm.Obligation != c.wantObligation || pm.Reason != c.wantReason {
			t.Errorf("%s: budgeted=%v obligation=%v reason=%q, want %v %v %q",
				c.name, pm.Budgeted, pm.Obligation, pm.Reason, c.wantBudgeted, c.wantObligation, c.wantReason)
		}
	}
	// Quick reminders may use only part of the budget.
	pm := permit(now, p, store.Load{BudgetedWeek: 3, QuickWeek: 3, LastSent: &morning}, WindowEvening, WindowEvening, false)
	if !pm.Budgeted || pm.Quick {
		t.Errorf("three quick reminders this week: budgeted=%v quick=%v, want true false", pm.Budgeted, pm.Quick)
	}
}

func TestBudgetBacksOffWhenIgnored(t *testing.T) {
	for run, want := range map[int]int{0: 5, 1: 5, 2: 2, 3: 2, 4: 1, 9: 1} {
		if got := effectiveBudget(5, run); got != want {
			t.Errorf("%d ignored in a row: budget %d, want %d", run, got, want)
		}
	}
	if got := effectiveBudget(1, 2); got != 1 {
		t.Errorf("the budget never drops to zero, got %d", got)
	}
}

func TestWindows(t *testing.T) {
	p := prefs() // sleep 23–07, work 10–19, lunch only
	keys := func(ws []Window) (out []string) {
		for _, w := range ws {
			out = append(out, w.Key)
		}
		return
	}
	// A workday has lunch and the evening; nothing before work.
	fri := Windows(at(13, 9, 0), p)
	if got := keys(fri); len(got) != 2 || got[0] != WindowLunch || got[1] != WindowEvening {
		t.Fatalf("workday windows: %v", got)
	}
	if fri[1].Start != 19*60+30 || fri[1].End != 21*60+30 {
		t.Errorf("evening should run from half an hour after work to 90 minutes before sleep, got %s–%s", fri[1].Start, fri[1].End)
	}
	if currentWindow(fri, at(13, 8, 30)) != "" || currentWindow(fri, at(13, 19, 0)) != "" {
		t.Error("early morning and the minute work ends are not send windows")
	}
	if currentWindow(fri, at(13, 13, 30)) != WindowLunch || currentWindow(fri, at(13, 20, 0)) != WindowEvening {
		t.Error("13:30 is lunch and 20:00 is evening")
	}
	// "No nudges at work" removes lunch.
	p.NudgeDuringWork = "no"
	if got := keys(Windows(at(13, 9, 0), p)); len(got) != 1 || got[0] != WindowEvening {
		t.Errorf("no-work-nudges workday: %v", got)
	}
	// A day off starts late morning, never at wake-up.
	sat := Windows(at(14, 9, 0), prefs())
	if got := keys(sat); len(got) != 3 || got[0] != WindowOffMorning {
		t.Fatalf("day-off windows: %v", got)
	}
	if sat[0].Start != 10*60 {
		t.Errorf("late morning should start at 10:00, got %s", sat[0].Start)
	}
	// A late sleeper's late morning moves with them.
	late := prefs()
	late.QuietStart, late.QuietEnd = 1*60, 10*60
	if ws := Windows(at(14, 9, 0), late); ws[0].Start != 12*60 {
		t.Errorf("waking at 10:00 should push late morning to 12:00, got %s", ws[0].Start)
	}
}

func TestChooseWindow(t *testing.T) {
	ws := []Window{{Key: WindowLunch}, {Key: WindowEvening}}
	count := func(tallies map[string]store.Tally) map[string]int {
		got := map[string]int{}
		for seed := int64(0); seed < 2000; seed++ {
			got[chooseWindow(ws, tallies, seed)]++
		}
		return got
	}
	// The same day always gives the same answer.
	if chooseWindow(ws, nil, 42) != chooseWindow(ws, nil, 42) {
		t.Error("the choice must be stable for a given day")
	}
	// With no history the split is roughly even.
	even := count(nil)
	if even[WindowLunch] < 800 || even[WindowEvening] < 800 {
		t.Errorf("no history should be near 50/50, got %v", even)
	}
	// A clear record shifts the choice, but the loser is never abandoned.
	skewed := count(map[string]store.Tally{WindowLunch: {Acted: 1, Ignored: 9}, WindowEvening: {Acted: 9, Ignored: 1}})
	if skewed[WindowEvening] < 1500 {
		t.Errorf("a 9/10 window should win most days, got %v", skewed)
	}
	if skewed[WindowLunch] < 150 {
		t.Errorf("a losing window must still be tried sometimes, got %v", skewed)
	}
	// Two observations are not a record: the split stays close.
	thin := count(map[string]store.Tally{WindowEvening: {Acted: 2}})
	if thin[WindowLunch] < 500 {
		t.Errorf("two successes should not settle it, got %v", thin)
	}
	if chooseWindow(nil, nil, 1) != "" {
		t.Error("no windows, no choice")
	}
}

func TestCooldownDoublesAndCaps(t *testing.T) {
	for streak, want := range map[int]int{0: 3, 1: 6, 2: 12, 3: 24, 4: 30, 9: 30} {
		if got := cooldownDays(streak); got != want {
			t.Errorf("streak %d: got %d days, want %d", streak, got, want)
		}
	}
}

func TestFreeBlock(t *testing.T) {
	p := prefs()
	// Friday, a two-day plan: the weekend starts tomorrow.
	start, length, lead, ok := freeBlock(at(13, 8, 0), p, 1, 2)
	if !ok || lead != 1 || length != 2 || start.Day() != 14 {
		t.Errorf("friday: start=%v length=%d lead=%d ok=%v", start, length, lead, ok)
	}
	// Saturday afternoon, a one-day plan may use Sunday.
	if start, _, lead, _ = freeBlock(at(14, 15, 0), p, 1, 1); lead != 1 || start.Day() != 15 {
		t.Errorf("saturday one-day: start=%v lead=%d", start, lead)
	}
	// Saturday, a two-day plan cannot start mid-weekend: next Saturday.
	if start, _, lead, _ = freeBlock(at(14, 15, 0), p, 1, 2); lead != 7 || start.Day() != 21 {
		t.Errorf("saturday two-day: start=%v lead=%d", start, lead)
	}
}

func TestWeatherWaitsUntilTheForecastIsUseful(t *testing.T) {
	e := &Engine{}
	lat, lon := 28.65, 77.23
	p := prefs()
	p.HomeLat, p.HomeLon, p.HomeCity = &lat, &lon, "Delhi, India"
	// Monday: the weekend is five days out, too far for a two-day plan.
	monday := at(16, 9, 0)
	st := e.weatherState(t.Context(), ai.Hook{Kind: ai.HookWeather, DaysNeeded: 2}, p, monday)
	if st.State != StateWaiting {
		t.Fatalf("want waiting on Monday, got %s (%s)", st.State, st.Summary)
	}
	// No location at all is "unknown", which scores as if there were no hook.
	noHome := prefs()
	noHome.HomeLat, noHome.HomeLon = nil, nil
	st = e.weatherState(t.Context(), ai.Hook{Kind: ai.HookWeather}, noHome, monday)
	if st.State != StateUnknown {
		t.Fatalf("want unknown without a location, got %s", st.State)
	}
}

func TestDaysBetweenSurvivesClockChanges(t *testing.T) {
	ny, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skip("no timezone data")
	}
	// Clocks spring forward in New York on Sunday 14 March 2027.
	sat := time.Date(2027, 3, 13, 9, 0, 0, 0, ny)
	mon := time.Date(2027, 3, 15, 0, 0, 0, 0, ny)
	if got := daysBetween(sat, mon); got != 2 {
		t.Errorf("Saturday to Monday across the change is 2 days, got %d", got)
	}
	// The old arithmetic was a day short here, which is what this guards.
	if naive := int(mon.Sub(day(sat)).Hours() / 24); naive != 1 {
		t.Logf("note: duration arithmetic now gives %d on this date", naive)
	}
	deadline := ai.Hook{Kind: ai.HookDate, Date: "2027-03-15", LeadDays: 1}
	if st := dateState(deadline, sat); st.State != StateWaiting {
		t.Errorf("two days before a lead-1 deadline is still waiting, got %s (%s)", st.State, st.Summary)
	}
	sun := time.Date(2027, 3, 14, 9, 0, 0, 0, ny)
	if st := dateState(deadline, sun); st.State != StateFired || st.DueIn != 1 {
		t.Errorf("the day before: want fired, due in 1; got %s due in %d", st.State, st.DueIn)
	}
	if got := daysBetween(at(13, 23, 30), at(14, 0, 15)); got != 1 {
		t.Errorf("45 minutes across midnight is the next day, got %d", got)
	}
}

func TestDateState(t *testing.T) {
	now := at(13, 9, 0)
	for date, want := range map[string]string{
		"2026-11-14": StateFired,   // tomorrow, inside a 3-day lead
		"2026-11-16": StateFired,   // exactly 3 days
		"2026-11-30": StateWaiting, // far off
		"2026-11-10": StateUnknown, // passed
	} {
		if got := dateState(ai.Hook{Kind: ai.HookDate, Date: date, LeadDays: 3}, now); got.State != want {
			t.Errorf("%s: got %s, want %s", date, got.State, want)
		}
	}
}

func TestDateHookMarksWhatIsCritical(t *testing.T) {
	now := at(14, 15, 0)
	tomorrow := dateState(ai.Hook{Kind: ai.HookDate, Date: "2026-11-15", LeadDays: 1}, now)
	today := dateState(ai.Hook{Kind: ai.HookDate, Date: "2026-11-14", LeadDays: 1}, now)
	later := dateState(ai.Hook{Kind: ai.HookDate, Date: "2026-11-17", LeadDays: 5}, now)
	if !tomorrow.Critical || tomorrow.DueIn != 1 || !today.Critical || today.DueIn != 0 {
		t.Errorf("tomorrow: %+v, today: %+v", tomorrow, today)
	}
	if later.Critical || later.State != StateFired {
		t.Errorf("three days out has fired but is not critical: %+v", later)
	}
	// Lead 0 means "only on the day": no heads-up the evening before.
	arrives := ai.Hook{Kind: ai.HookDate, Date: "2026-11-15", LeadDays: 0}
	if got := dateState(arrives, now); got.State != StateWaiting {
		t.Errorf("a lead-0 date should wait until the day itself, got %+v", got)
	}
	if got := dateState(arrives, at(15, 9, 0)); got.State != StateFired || !got.Critical || got.DueIn != 0 {
		t.Errorf("on the day a lead-0 date fires as critical, got %+v", got)
	}
}

func thoughtOf(typ ai.ThoughtType, urgency string, quickStep bool, touchedDaysAgo int, now time.Time) store.Thought {
	steps := []ai.NextStep{{Label: "plan it", EstimatedMinutes: 60}}
	if quickStep {
		steps = []ai.NextStep{{Label: "order it", EstimatedMinutes: 5}}
	}
	return store.Thought{
		ID: 1, CreatedAt: now.AddDate(0, 0, -touchedDaysAgo).Add(-time.Hour), LastTouchedAt: now.AddDate(0, 0, -touchedDaysAgo),
		Interpretation: &store.Interpretation{Interpretation: ai.Interpretation{Type: typ, Title: "x", NextSteps: steps, Urgency: urgency}},
	}
}

func TestLaneOf(t *testing.T) {
	now := at(14, 10, 30)
	past := now.Add(-time.Minute)
	future := now.Add(time.Hour)
	fired := func(kind string) []HookState { return []HookState{{Kind: kind, State: StateFired}} }
	critical := []HookState{{Kind: ai.HookDate, State: StateFired, Critical: true, DueIn: 1}}

	cases := []struct {
		name    string
		thought store.Thought
		hooks   []HookState
		remind  *time.Time
		want    string
	}{
		{"a time the user picked has come", thoughtOf(ai.TypeIdea, "", false, 5, now), nil, &past, LaneRequested},
		{"a time the user picked is still ahead", thoughtOf(ai.TypeIdea, "", false, 5, now), nil, &future, LaneReview},
		{"due tomorrow", thoughtOf(ai.TypeActionable, "", true, 0, now), critical, nil, LaneDeadline},
		{"dry weekend for a ride", thoughtOf(ai.TypeExperience, "", false, 5, now), fired(ai.HookWeather), nil, LaneOpportunity},
		{"a sale on what they wanted", thoughtOf(ai.TypeActionable, "", true, 5, now), fired(ai.HookWeb), nil, LaneOpportunity},
		{"a five-minute errand", thoughtOf(ai.TypeActionable, "", true, 5, now), nil, nil, LaneQuick},
		{"an urgent errand with no short step", thoughtOf(ai.TypeActionable, ai.UrgencyNow, false, 0, now), nil, nil, LaneQuick},
		{"free time for a course", thoughtOf(ai.TypeDevelopment, "", false, 5, now), fired(ai.HookFreeTime), nil, LaneQuick},
		{"an idea with nothing to do now", thoughtOf(ai.TypeIdea, "", true, 9, now), nil, nil, LaneReview},
		{"a someday trip with no trigger", thoughtOf(ai.TypeExperience, "", false, 9, now), nil, nil, LaneReview},
		{"a big errand with no small step", thoughtOf(ai.TypeActionable, "", false, 9, now), nil, nil, LaneReview},
	}
	for _, c := range cases {
		c.thought.RemindAt = c.remind
		if got := laneOf(c.thought, Scored{Hooks: c.hooks}, now); got != c.want {
			t.Errorf("%s: lane %q, want %q", c.name, got, c.want)
		}
	}
}

func TestHoldReason(t *testing.T) {
	p := prefs()
	now := at(14, 10, 30) // Saturday, in the late-morning window
	ws := Windows(now, p)
	open := Permit{Obligation: true, Budgeted: true, Quick: true, Window: WindowOffMorning, Chosen: WindowOffMorning, Budget: 5}
	closed := Permit{Obligation: true, Reason: "daily_cap", Chosen: WindowOffMorning, Budget: 5}
	evening := Permit{Obligation: true, Reason: "outside_window", Chosen: WindowEvening, Budget: 5}

	scored := func(th store.Thought, hooks []HookState) Scored {
		sc := score(th, store.Stats{Confirmed: true}, hooks, p, now)
		sc.Lane = laneOf(th, sc, now)
		return sc
	}
	held := func(th store.Thought, hooks []HookState, touch store.DeadlineTouch, pm Permit) bool {
		return holdReason(th, scored(th, hooks), touch, pm, ws, now) != ""
	}
	errand := thoughtOf(ai.TypeActionable, "", true, 4, now)
	freshErrand := thoughtOf(ai.TypeActionable, "", true, 0, now)
	urgentErrand := thoughtOf(ai.TypeActionable, ai.UrgencyNow, true, 0, now)
	dueTomorrow := []HookState{{Kind: ai.HookDate, State: StateFired, Critical: true, DueIn: 1}}
	dueToday := []HookState{{Kind: ai.HookDate, State: StateFired, Critical: true, DueIn: 0}}
	waiting := []HookState{{Kind: ai.HookWeb, State: StateWaiting}}
	weather := []HookState{{Kind: ai.HookWeather, State: StateFired}}
	yesterday := now.AddDate(0, 0, -1)

	if held(errand, nil, store.DeadlineTouch{}, open) {
		t.Error("a rested errand in the open window should go")
	}
	if !held(freshErrand, nil, store.DeadlineTouch{}, open) {
		t.Error("an errand captured today rests")
	}
	if held(urgentErrand, nil, store.DeadlineTouch{}, open) {
		t.Error("an urgent errand does not rest")
	}
	if !held(errand, waiting, store.DeadlineTouch{}, open) {
		t.Error("an errand waiting on a sale is held for its moment")
	}
	if !held(errand, nil, store.DeadlineTouch{}, closed) {
		t.Error("budgeted nudges stop once today's has been sent")
	}
	if held(errand, dueToday, store.DeadlineTouch{}, closed) {
		t.Error("a deadline goes out even when today's nudge was sent")
	}
	if held(thoughtOf(ai.TypeExperience, "", false, 0, now), weather, store.DeadlineTouch{}, open) {
		t.Error("a fired trigger sends even a fresh thought")
	}
	ignoredTwice := errand
	ignoredTwice.IgnoredStreak = 2
	if !held(ignoredTwice, weather, store.DeadlineTouch{}, open) {
		t.Error("two unanswered nudges move a thought to the review, trigger or not")
	}
	quickUsed := open
	quickUsed.Quick = false
	if !held(errand, nil, store.DeadlineTouch{}, quickUsed) || held(errand, weather, store.DeadlineTouch{}, quickUsed) {
		t.Error("when the quick share is used, errands wait but opportunities still go")
	}
	// Deadlines: two touches, one a day, and the heads-up waits for the window.
	if !held(errand, dueTomorrow, store.DeadlineTouch{}, evening) {
		t.Error("a heads-up for tomorrow waits for today's window instead of going out mid-morning")
	}
	if held(errand, dueTomorrow, store.DeadlineTouch{}, open) {
		t.Error("in the window, the heads-up goes")
	}
	if !held(errand, dueToday, store.DeadlineTouch{Count: 1, Last: now.Add(-2 * time.Hour)}, open) {
		t.Error("one reminder per deadline per day")
	}
	if held(errand, dueToday, store.DeadlineTouch{Count: 1, Last: yesterday}, open) {
		t.Error("the day-of reminder follows yesterday's heads-up")
	}
	if !held(errand, dueToday, store.DeadlineTouch{Count: 2, Last: yesterday}, open) {
		t.Error("never more than two reminders for one deadline")
	}
	// Whatever the lane, not within minutes of being told.
	justSaid := thoughtOf(ai.TypeActionable, ai.UrgencyNow, true, 0, now)
	justSaid.CreatedAt = now.Add(-5 * time.Minute)
	if !held(justSaid, dueToday, store.DeadlineTouch{}, open) {
		t.Error("nothing is sent about a thought captured five minutes ago")
	}
	if !held(thoughtOf(ai.TypeIdea, "", false, 9, now), nil, store.DeadlineTouch{}, open) {
		t.Error("an idea with no trigger is never pushed")
	}
}

func TestShortlistPutsObligationsFirst(t *testing.T) {
	eligible := []Scored{
		{ThoughtID: 1, Lane: LaneQuick, Score: 0.9},
		{ThoughtID: 2, Lane: LaneOpportunity, Score: 0.6},
		{ThoughtID: 3, Lane: LaneDeadline, Score: 0.5},
	}
	short, obligations := shortlist(eligible, false)
	if !obligations || len(short) != 1 || short[0].ThoughtID != 3 {
		t.Fatalf("a ready deadline is shown alone: %+v (obligations=%v)", short, obligations)
	}
	short, obligations = shortlist(eligible[:2], false)
	if obligations || len(short) != 2 || short[0].ThoughtID != 2 {
		t.Fatalf("an opportunity ranks above a quick reminder whatever the score: %+v", short)
	}
	if short, _ := shortlist(nil, false); len(short) != 0 {
		t.Fatal("nothing eligible, nothing shown")
	}
	// /nudge may pick anything, by score.
	all := append(eligible, Scored{ThoughtID: 4, Lane: LaneReview, Score: 0.95})
	if short, _ := shortlist(all, true); len(short) != 3 || short[0].ThoughtID != 4 {
		t.Fatalf("forced: top three by score, any lane: %+v", short)
	}
}

func TestReviewDue(t *testing.T) {
	p := prefs()
	p.ReviewDay, p.ReviewTime = 7, 19*60
	sunday := at(15, 19, 30)
	lastWeek := store.Review{SentAt: sunday.AddDate(0, 0, -7), ItemCount: 4, TapCount: 2}
	untouched := store.Review{SentAt: sunday.AddDate(0, 0, -7), ItemCount: 4}
	untouched2 := store.Review{SentAt: sunday.AddDate(0, 0, -14), ItemCount: 4}

	if r := reviewDue(sunday, p, nil); r != "" {
		t.Errorf("first review on Sunday evening should be due: %s", r)
	}
	if r := reviewDue(sunday, p, []store.Review{lastWeek}); r != "" {
		t.Errorf("a week after the last one it is due again: %s", r)
	}
	if reviewDue(at(15, 18, 30), p, nil) == "" || reviewDue(at(15, 22, 30), p, nil) == "" {
		t.Error("only within three hours after the set time")
	}
	if reviewDue(at(14, 19, 30), p, nil) == "" {
		t.Error("Saturday is not the review day")
	}
	if reviewDue(sunday, p, []store.Review{{SentAt: sunday.Add(-time.Hour), ItemCount: 3}}) == "" {
		t.Error("not twice in one evening")
	}
	if reviewDue(sunday, p, []store.Review{untouched, untouched2}) == "" {
		t.Error("two untouched reviews pause it for a month")
	}
	old := store.Review{SentAt: sunday.AddDate(0, 0, -28), ItemCount: 4}
	older := store.Review{SentAt: sunday.AddDate(0, 0, -35), ItemCount: 4}
	if r := reviewDue(sunday, p, []store.Review{old, older}); r != "" {
		t.Errorf("after a month it tries again: %s", r)
	}
	p.ReviewDay = 0
	if reviewDue(sunday, p, nil) == "" {
		t.Error("turned off means off")
	}
}

func TestFreeTimeState(t *testing.T) {
	p := prefs()
	h := ai.Hook{Kind: ai.HookFreeTime, Minutes: 90}
	if got := freeTimeState(h, p, at(14, 10, 0)); got.State != StateFired {
		t.Errorf("saturday morning: got %s", got.State)
	}
	if got := freeTimeState(h, p, at(13, 20, 0)); got.State != StateFired {
		t.Errorf("friday after work: got %s", got.State)
	}
	if got := freeTimeState(h, p, at(13, 8, 30)); got.State != StateWaiting {
		t.Errorf("friday before work: got %s", got.State)
	}
}

// These pin the behaviour the weights were chosen for. If a weight changes,
// the expectations below say what must still be true.
func TestScoreShape(t *testing.T) {
	p := prefs()
	now := at(13, 8, 30) // Friday
	thought := func(typ ai.ThoughtType, touchedDaysAgo int, quickStep bool) store.Thought {
		steps := []ai.NextStep{{Label: "plan it", EstimatedMinutes: 60}}
		if quickStep {
			steps = []ai.NextStep{{Label: "order it", EstimatedMinutes: 5}}
		}
		return store.Thought{
			ID: 1, LastTouchedAt: now.AddDate(0, 0, -touchedDaysAgo),
			Interpretation: &store.Interpretation{Interpretation: ai.Interpretation{Type: typ, Title: "x", NextSteps: steps}},
		}
	}
	confirmed := store.Stats{Confirmed: true}
	fired := []HookState{{Kind: ai.HookWeather, State: StateFired}}
	waiting := []HookState{{Kind: ai.HookWeb, State: StateWaiting}}
	negative := []HookState{{Kind: ai.HookWeather, State: StateNegative}}

	cases := []struct {
		name  string
		got   Scored
		above bool
	}{
		{"an errand resurfaces after a few days", score(thought(ai.TypeActionable, 3, true), confirmed, nil, p, now), true},
		{"a fired hook wins even on a fresh thought", score(thought(ai.TypeExperience, 0, false), confirmed, fired, p, now), true},
		{"a waiting hook holds a week-old thought back", score(thought(ai.TypeActionable, 7, true), confirmed, waiting, p, now), false},
		{"but not forever", score(thought(ai.TypeActionable, 14, true), confirmed, waiting, p, now), true},
		{"bad weather holds a ride back", score(thought(ai.TypeExperience, 7, false), confirmed, negative, p, now), false},
	}
	for _, c := range cases {
		if (c.got.Score >= Threshold) != c.above {
			t.Errorf("%s: score %.2f (T%.1f R%.2f I%.2f A%.1f O%.1f), want above bar = %v",
				c.name, c.got.Score, c.got.T, c.got.R, c.got.I, c.got.A, c.got.O, c.above)
		}
	}
}

func TestTimelinessIgnoresUnknownHooks(t *testing.T) {
	waiting, unknown, fired := HookState{State: StateWaiting}, HookState{State: StateUnknown}, HookState{State: StateFired}
	if got := timeliness(nil); got != neutralT {
		t.Errorf("no hooks: got %.1f", got)
	}
	if got := timeliness([]HookState{unknown}); got != neutralT {
		t.Errorf("only unknown: got %.1f", got)
	}
	if got := timeliness([]HookState{waiting, unknown}); got != 0.1 {
		t.Errorf("waiting + unknown should wait: got %.1f", got)
	}
	if got := timeliness([]HookState{waiting, fired}); got != 1.0 {
		t.Errorf("one fired hook is enough: got %.1f", got)
	}
}

func TestTooFresh(t *testing.T) {
	now := at(13, 8, 30)
	fresh := store.Thought{LastTouchedAt: now.Add(-20 * time.Hour)}
	old := store.Thought{LastTouchedAt: now.Add(-72 * time.Hour)}
	if tooFresh(fresh, nil, now) == "" {
		t.Error("a thought captured yesterday should rest")
	}
	if tooFresh(fresh, []HookState{{State: StateWaiting}}, now) == "" {
		t.Error("a waiting hook does not override the rest")
	}
	if tooFresh(fresh, []HookState{{State: StateFired}}, now) != "" {
		t.Error("a fired hook should override the rest")
	}
	if tooFresh(fresh, []HookState{{Kind: ai.HookFreeTime, State: StateFired}}, now) == "" {
		t.Error("free time comes round daily and must not override the rest")
	}
	if tooFresh(old, nil, now) != "" {
		t.Error("a three-day-old thought is no longer fresh")
	}
}

func TestRecheckAt(t *testing.T) {
	now := at(13, 9, 0)
	// A miss with a suggested date is rechecked that morning.
	got := recheckAt(ai.Finding{RecheckOn: "2026-11-22"}, now)
	if got.Day() != 22 {
		t.Errorf("recheck_on honoured: got %v", got)
	}
	// Never sooner than two days, never later than two weeks.
	if got := recheckAt(ai.Finding{RecheckOn: "2026-11-13"}, now); got.Sub(now) < 48*time.Hour {
		t.Errorf("too soon: %v", got)
	}
	if got := recheckAt(ai.Finding{RecheckOn: "2027-03-01"}, now); got.Sub(now) > 14*24*time.Hour {
		t.Errorf("too late: %v", got)
	}
	// A live finding is re-verified within two days.
	if got := recheckAt(ai.Finding{Timely: true, EndsOn: "2026-12-01"}, now); got.Sub(now) > 48*time.Hour {
		t.Errorf("live finding trusted too long: %v", got)
	}
}
