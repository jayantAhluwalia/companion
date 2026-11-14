package engine

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"companion/api/internal/ai"
	"companion/api/internal/signals"
	"companion/api/internal/store"
)

// Hook states. The timeliness score is the best state across a thought's hooks.
const (
	StateFired    = "fired"    // the condition holds: this is the moment
	StateMixed    = "mixed"    // partly holds
	StateWaiting  = "waiting"  // not yet; hold the thought back
	StateNegative = "negative" // the condition is against it
	StateUnknown  = "unknown"  // could not be checked; treated as no hook
)

// HookState is one hook after checking it against the world.
type HookState struct {
	Kind     string      `json:"kind"`
	State    string      `json:"state"`
	Summary  string      `json:"summary"`
	Facts    any         `json:"facts,omitempty"`
	Footnote string      `json:"footnote,omitempty"` // short line shown under a nudge
	Links    []ai.Source `json:"links,omitempty"`
	// Critical marks a deadline that is today or tomorrow. A critical nudge
	// may go out even when the day's ordinary nudge has been used.
	Critical bool `json:"critical,omitempty"`
	DueIn    int  `json:"due_in,omitempty"` // days until a critical deadline: 0 or 1
}

// critical reports whether any hook of a scored thought is date-critical.
func (s Scored) critical() bool {
	for _, h := range s.Hooks {
		if h.Critical {
			return true
		}
	}
	return false
}

// strength maps a state to the timeliness value T. A thought with a hook
// that has not fired scores below a thought with no hook at all: that is
// what makes the engine wait for the right moment instead of nagging.
func (h HookState) strength() float64 {
	switch h.State {
	case StateFired:
		if h.Kind == ai.HookFreeTime {
			return 0.8
		}
		return 1.0
	case StateMixed:
		return 0.4
	case StateWaiting:
		return 0.1
	case StateNegative:
		return 0.0
	default:
		return neutralT
	}
}

const neutralT = 0.5

// timeliness is T for a thought: the best hook that could be checked wins.
// Hooks that could not be checked are ignored, so one unknown never
// outranks a definite "not yet" from another hook.
func timeliness(states []HookState) float64 {
	best, known := 0.0, false
	for _, s := range states {
		if s.State == StateUnknown {
			continue
		}
		known = true
		best = max(best, s.strength())
	}
	if !known {
		return neutralT
	}
	return best
}

// daysBetween counts calendar days from one date to another, whatever the
// clocks did in between. Dividing a duration by 24 hours is a day short
// across a spring-forward change, which would put a deadline reminder on
// the wrong day for anyone living where clocks change.
func daysBetween(from, to time.Time) int {
	fy, fm, fd := from.Date()
	ty, tm, td := to.Date()
	a := time.Date(fy, fm, fd, 0, 0, 0, 0, time.UTC)
	b := time.Date(ty, tm, td, 0, 0, 0, 0, time.UTC)
	return int(b.Sub(a).Hours() / 24)
}

func day(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// freeBlock finds the next run of consecutive days off that starts at
// least minLead days from today (today is lead 0). A multi-day plan must
// start on the first day of a run; a one-day plan may use any day off.
func freeBlock(now time.Time, p store.Prefs, minLead, daysNeeded int) (start time.Time, length, lead int, ok bool) {
	today := day(now)
	for lead = minLead; lead <= 14; lead++ {
		d := today.AddDate(0, 0, lead)
		if !p.IsDayOff(isoWeekday(d)) {
			continue
		}
		midRun := lead > 0 && p.IsDayOff(isoWeekday(d.AddDate(0, 0, -1)))
		if midRun && daysNeeded > 1 {
			continue
		}
		length = 0
		for p.IsDayOff(isoWeekday(d.AddDate(0, 0, length))) && length < 7 {
			length++
		}
		return d, length, lead, true
	}
	return time.Time{}, 0, 0, false
}

// leadWindow is how far ahead of the free days a weather nudge is useful:
// long enough to plan, short enough for the forecast to be trustworthy.
func leadWindow(daysNeeded int, now time.Time) (minLead, maxLead int) {
	switch {
	case daysNeeded <= 1:
		if now.Hour() < 11 {
			return 0, 2 // "today looks good" only works in the morning
		}
		return 1, 2
	case daysNeeded == 2:
		return 1, 3
	default:
		return 2, 5 // a longer trip needs planning, but a forecast past five days is a guess
	}
}

func (e *Engine) weatherState(ctx context.Context, h ai.Hook, p store.Prefs, now time.Time) HookState {
	st := HookState{Kind: ai.HookWeather}
	lat, lon, where := 0.0, 0.0, ""
	switch {
	case h.HasPlace():
		lat, lon, where = h.Lat, h.Lon, h.Place
	case p.HasHome():
		lat, lon, where = *p.HomeLat, *p.HomeLon, p.HomeCity
	default:
		st.State, st.Summary = StateUnknown, "no location known; set your city with /routine to get weather-aware nudges"
		return st
	}
	if where == "" {
		where = "the destination"
	}

	need := max(h.DaysNeeded, 1)
	minLead, maxLead := leadWindow(need, now)
	start, length, lead, ok := freeBlock(now, p, minLead, need)
	if !ok {
		st.State, st.Summary = StateUnknown, "no days off are set"
		return st
	}
	if lead > maxLead {
		checkFrom := start.AddDate(0, 0, -maxLead)
		st.State = StateWaiting
		st.Summary = fmt.Sprintf("next free days start %s; the forecast is worth checking from %s",
			start.Format("Mon 2 Jan"), checkFrom.Format("Mon 2 Jan"))
		return st
	}

	forecast, err := e.weather.Forecast(ctx, lat, lon)
	if err != nil {
		e.log.Warn("forecast", "err", err)
		st.State, st.Summary = StateUnknown, "the forecast could not be fetched"
		return st
	}
	byDate := map[string]signals.Day{}
	for _, d := range forecast {
		byDate[d.Date] = d
	}
	var days []signals.Day
	for i := 0; i < need; i++ {
		d, ok := byDate[start.AddDate(0, 0, i).Format("2006-01-02")]
		if !ok {
			st.State, st.Summary = StateWaiting, "the free days are beyond the forecast range"
			return st
		}
		days = append(days, d)
	}

	good, bad := 0, 0
	parts := make([]string, 0, len(days))
	for _, d := range days {
		switch d.Verdict {
		case "good":
			good++
		case "bad":
			bad++
		}
		t, _ := time.Parse("2006-01-02", d.Date)
		parts = append(parts, fmt.Sprintf("%s %s %.0f°/%.0f° rain %d%%", t.Format("Mon 2 Jan"), d.Sky, d.TMaxC, d.TMinC, d.RainProb))
	}
	switch {
	case bad > 0:
		st.State = StateNegative
	case good == len(days):
		st.State = StateFired
	default:
		st.State = StateMixed
	}
	st.Footnote = fmt.Sprintf("%s: %s", where, strings.Join(parts, " · "))
	st.Summary = fmt.Sprintf("forecast for %s on the next free days (%d day(s) away): %s", where, lead, strings.Join(parts, "; "))
	facts := map[string]any{"place": where, "days_until": lead, "activity": h.Activity, "days": days}
	if need > length {
		facts["note"] = fmt.Sprintf("needs %d days but only %d consecutive day(s) off: would need leave", need, length)
	}
	st.Facts = facts
	return st
}

func dateState(h ai.Hook, now time.Time) HookState {
	st := HookState{Kind: ai.HookDate}
	d, err := time.ParseInLocation("2006-01-02", h.Date, now.Location())
	if err != nil {
		st.State, st.Summary = StateUnknown, "unreadable date"
		return st
	}
	until := daysBetween(now, d)
	// The lead is the interpreter's judgement of how far ahead a reminder
	// helps. Zero is a real answer: "the parcel arrives on the 16th" needs
	// no heads-up the day before, only a word on the day.
	lead := max(h.LeadDays, 0)
	st.Facts = map[string]any{"date": h.Date, "days_until": until}
	switch {
	case until < 0:
		st.State, st.Summary = StateUnknown, fmt.Sprintf("the date %s has passed", d.Format("Mon 2 Jan"))
	case until <= lead:
		st.State = StateFired
		st.Critical = until <= 1
		st.DueIn = until
		st.Summary = fmt.Sprintf("%s is %s", d.Format("Mon 2 Jan"), inDays(until))
		st.Footnote = fmt.Sprintf("%s is %s", d.Format("Mon 2 Jan"), inDays(until))
	default:
		st.State = StateWaiting
		st.Summary = fmt.Sprintf("%s is %s; I will bring it up %s", d.Format("Mon 2 Jan"), inDays(until), leadText(lead))
	}
	return st
}

func leadText(lead int) string {
	switch lead {
	case 0:
		return "on the day"
	case 1:
		return "the day before"
	default:
		return fmt.Sprintf("%d days before", lead)
	}
}

func inDays(n int) string {
	switch n {
	case 0:
		return "today"
	case 1:
		return "tomorrow"
	default:
		return fmt.Sprintf("in %d days", n)
	}
}

func freeTimeState(h ai.Hook, p store.Prefs, now time.Time) HookState {
	st := HookState{Kind: ai.HookFreeTime, Facts: map[string]any{"minutes_needed": h.Minutes}}
	clock := store.ClockOf(now)
	switch {
	case p.IsDayOff(isoWeekday(now)) && now.Hour() < 19:
		st.State, st.Summary = StateFired, fmt.Sprintf("today is a day off; it needs about %d minutes", h.Minutes)
	case p.HasWorkHours() && p.IsWorkDay(isoWeekday(now)) && clock >= p.WorkEnd && h.Minutes <= 120:
		st.State, st.Summary = StateFired, fmt.Sprintf("the workday is over; it needs about %d minutes", h.Minutes)
	case !p.HasWorkHours() && h.Minutes <= 90:
		st.State, st.Summary = StateUnknown, "no working hours are set, so free time is a guess"
	default:
		st.State, st.Summary = StateWaiting, fmt.Sprintf("needs about %d free minutes; better after work or on a day off", h.Minutes)
	}
	return st
}

func hookKey(h ai.Hook) string {
	sum := sha1.Sum([]byte(h.Question + "\x00" + h.GoodWhen))
	return hex.EncodeToString(sum[:8])
}

// webState returns the cached finding for a web hook, or runs a lookup when
// the cache is stale and the lookup budget allows it.
func (e *Engine) webState(ctx context.Context, user store.User, p store.Prefs, t store.Thought, h ai.Hook, now time.Time, budget *scoutBudget) HookState {
	key := hookKey(h)
	cachedSig, err := e.db.GetSignal(ctx, t.ID, key)
	if err != nil {
		e.log.Warn("get signal", "err", err)
	}
	if cachedSig != nil && now.Before(cachedSig.ExpiresAt) {
		return stateFromSignal(*cachedSig)
	}
	if !budget.take() {
		if cachedSig != nil {
			st := stateFromSignal(*cachedSig)
			if st.State == StateFired { // a stale "yes" is not a yes
				st.State = StateWaiting
			}
			st.Summary = "last checked " + cachedSig.FetchedAt.In(now.Location()).Format("Mon 2 Jan") + ": " + st.Summary
			return st
		}
		return HookState{Kind: ai.HookWeb, State: StateWaiting, Summary: "not looked up yet (" + budget.why + ")"}
	}

	city, _, _ := strings.Cut(p.HomeCity, ",")
	sctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	finding, info, err := e.ai.Scout(sctx, ai.ScoutRequest{
		Question: h.Question, GoodWhen: h.GoodWhen, Today: now,
		City: strings.TrimSpace(city), Country: p.HomeCountry, Timezone: user.Timezone,
	})
	cancel()
	_ = e.db.LogLLMCall(ctx, store.LLMCall{
		UserID: &user.ID, ThoughtID: &t.ID, Purpose: "scout", Model: info.Model, PromptVer: info.PromptVersion,
		InputTokens: info.InputTokens, OutputTokens: info.OutputTokens, WebSearches: info.WebSearches,
		CostUSD: info.CostUSD(), Latency: info.Latency, StopReason: info.StopReason, Err: err,
	})
	if err != nil {
		e.log.Error("scout", "err", err, "thought", t.ID)
		// Remember the failure for a few hours. Without this, a lookup that
		// keeps failing would be retried on every run and, being first in
		// line, would use up the day's lookups for everything else.
		failed := store.Signal{ThoughtID: t.ID, HookKey: key, Kind: ai.HookWeb, State: StateUnknown,
			Summary: "the web lookup failed; I will try again later", Facts: []byte("{}"), FetchedAt: now, ExpiresAt: now.Add(6 * time.Hour)}
		if uerr := e.db.UpsertSignal(ctx, failed); uerr != nil {
			e.log.Warn("store failed signal", "err", uerr)
		}
		return stateFromSignal(failed)
	}

	sig := store.Signal{ThoughtID: t.ID, HookKey: key, Kind: ai.HookWeb, State: StateWaiting,
		Summary: finding.Headline, FetchedAt: now, ExpiresAt: recheckAt(finding, now)}
	if finding.Timely && finding.Confidence >= 0.6 {
		sig.State = StateFired
	}
	sig.Facts, _ = json.Marshal(finding)
	if err := e.db.UpsertSignal(ctx, sig); err != nil {
		e.log.Warn("store signal", "err", err)
	}
	e.log.Info("scouted", "thought", t.ID, "state", sig.State, "headline", finding.Headline,
		"searches", info.WebSearches, "cost", info.CostUSD(), "recheck", sig.ExpiresAt.Format("2006-01-02"))
	return stateFromSignal(sig)
}

// recheckAt decides how long a finding stays trusted. A live window is
// re-verified within two days; a miss is rechecked when the lookup itself
// suggested, between two days and two weeks out.
func recheckAt(f ai.Finding, now time.Time) time.Time {
	parse := func(s string) (time.Time, bool) {
		d, err := time.ParseInLocation("2006-01-02", s, now.Location())
		return d, err == nil
	}
	if f.Timely {
		until := now.Add(48 * time.Hour)
		if end, ok := parse(f.EndsOn); ok && end.Add(24*time.Hour).Before(until) {
			until = end.Add(24 * time.Hour)
		}
		return maxTime(until, now.Add(12*time.Hour))
	}
	at := now.Add(5 * 24 * time.Hour)
	if d, ok := parse(f.RecheckOn); ok {
		at = d.Add(7 * time.Hour) // morning of that day
	}
	return minTime(maxTime(at, now.Add(48*time.Hour)), now.Add(14*24*time.Hour))
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

func stateFromSignal(sig store.Signal) HookState {
	st := HookState{Kind: sig.Kind, State: sig.State, Summary: sig.Summary}
	var f ai.Finding
	if json.Unmarshal(sig.Facts, &f) == nil {
		st.Facts = map[string]any{
			"headline": f.Headline, "detail": f.Detail, "starts_on": f.StartsOn, "ends_on": f.EndsOn,
			"confidence": f.Confidence, "checked_on": sig.FetchedAt.Format("2006-01-02"),
		}
		if sig.State == StateFired {
			st.Footnote = f.Headline
			if len(f.Sources) > 0 {
				st.Links = f.Sources[:1]
			}
		}
	}
	return st
}

// scoutBudget rations web lookups, the only expensive signal.
type scoutBudget struct {
	left int
	why  string
}

func (b *scoutBudget) take() bool {
	if b.left <= 0 {
		return false
	}
	b.left--
	if b.left == 0 {
		b.why = "lookup limit for this check reached"
	}
	return true
}
