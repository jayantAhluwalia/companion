package engine

import (
	"testing"
	"time"

	"companion/api/internal/ai"
	"companion/api/internal/store"
)

var ist = time.FixedZone("IST", 5*3600+1800)

// A typical routine: sleep 23–07, work 10–19 Mon–Fri, nudges at lunch only.
func prefs() store.Prefs {
	return store.Prefs{
		QuietStart: 23 * 60, QuietEnd: 7 * 60, WorkStart: 10 * 60, WorkEnd: 19 * 60,
		WorkDays: []int{1, 2, 3, 4, 5}, DaysOff: []int{6, 7}, NudgeDuringWork: "lunch",
		DailyLimit: 1, MinIntervalMinutes: 240, NotificationsEnabled: true,
	}
}

// 13 Nov 2026 is a Friday.
func at(dayOfMonth, hour, minute int) time.Time {
	return time.Date(2026, 11, dayOfMonth, hour, minute, 0, 0, ist)
}

func TestGate(t *testing.T) {
	recent := at(13, 6, 0)
	cases := []struct {
		name      string
		now       time.Time
		edit      func(*store.Prefs)
		sentToday int
		lastSent  *time.Time
		want      string
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
		{name: "daily cap", now: at(13, 20, 0), sentToday: 1, want: "daily_cap"},
		{name: "minimum interval", now: at(13, 8, 30), edit: func(p *store.Prefs) { p.DailyLimit = 3 }, sentToday: 1, lastSent: &recent, want: "min_interval"},
		{name: "paused", now: at(13, 8, 30), edit: func(p *store.Prefs) { p.NotificationsEnabled = false }, want: "paused"},
	}
	for _, c := range cases {
		p := prefs()
		if c.edit != nil {
			c.edit(&p)
		}
		if got := Gate(c.now, p, c.sentToday, c.lastSent); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
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
	st = e.weatherState(t.Context(), ai.Hook{Kind: ai.HookWeather}, prefs(), monday)
	if st.State != StateUnknown {
		t.Fatalf("want unknown without a location, got %s", st.State)
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
