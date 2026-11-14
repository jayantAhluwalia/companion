package engine

import (
	"math"
	"math/rand"
	"time"

	"companion/api/internal/store"
)

// Send windows. Budgeted nudges go out in one window per day instead of at
// the first minute the gates open. The research behind this (reports/Nudge
// timing research evidence.md) found only one consistent clock-time result,
// that early morning is the worst slot, and no proven difference between
// the windows from mid-morning to evening. So early morning is left out by
// construction and the rest is learned from this user's own responses.
const (
	WindowLunch        = "lunch"            // workday, 13:00–14:00
	WindowEvening      = "evening"          // after work, or from 17:00
	WindowOffMorning   = "offday_morning"   // late morning on a free day
	WindowOffAfternoon = "offday_afternoon" // afternoon on a free day
	WindowChat         = "chat"             // sent as a follow-on while the user was messaging
)

type Window struct {
	Key   string      `json:"key"`
	Start store.Clock `json:"start"`
	End   store.Clock `json:"end"`
}

const (
	eveningEndsBeforeSleepMin = 90
	lateMorningAfterWakeMin   = 120
)

// Windows lists today's windows, earliest first.
func Windows(now time.Time, p store.Prefs) []Window {
	var ws []Window
	add := func(key string, start, end store.Clock) {
		if start >= 0 && end > start {
			ws = append(ws, Window{Key: key, Start: start, End: end})
		}
	}
	eveningEnd := shift(p.QuietStart, -eveningEndsBeforeSleepMin)
	if eveningEnd < 12*60 { // sleep starts after midnight: the evening runs to the end of the day
		eveningEnd = 24*60 - 1
	}
	workday := p.HasWorkHours() && p.IsWorkDay(isoWeekday(now))
	if workday {
		if p.NudgeDuringWork != "no" && lunchStart >= p.WorkStart && lunchEnd <= p.WorkEnd {
			add(WindowLunch, lunchStart, lunchEnd)
		}
		add(WindowEvening, store.Clock(int(p.WorkEnd)+30), eveningEnd)
		if len(ws) > 0 {
			return ws
		}
		// Unusual hours left no window (a late shift, say): fall through to
		// the free-day windows, which the work gate will trim as needed.
	}
	lateMorning := max(store.Clock(10*60), store.Clock(int(p.QuietEnd)+lateMorningAfterWakeMin))
	add(WindowOffMorning, lateMorning, 13*60)
	add(WindowOffAfternoon, 13*60, 17*60)
	add(WindowEvening, 17*60, eveningEnd)
	return ws
}

// currentWindow returns the key of the window that contains now, or "".
func currentWindow(ws []Window, now time.Time) string {
	c := store.ClockOf(now)
	for _, w := range ws {
		if c >= w.Start && c < w.End {
			return w.Key
		}
	}
	return ""
}

func windowStart(ws []Window, key string) store.Clock {
	for _, w := range ws {
		if w.Key == key {
			return w.Start
		}
	}
	return 0
}

const (
	// priorStrength is how many imaginary observations each window starts
	// with, at the user's overall success rate. It keeps one early ignored
	// nudge from writing a window off.
	priorStrength = 4.0
	// exploreShare is the chance a day's window is picked at random, so no
	// window is ever abandoned and the tallies keep being tested.
	exploreShare = 0.25
)

// chooseWindow picks today's window by Thompson sampling: each window draws
// a plausible success rate from what has been seen so far, and the highest
// draw wins. Windows with a good record win more often, and with only a few
// observations the draws overlap, so the choice stays close to even until
// the data says otherwise. The seed fixes the choice for the whole day.
func chooseWindow(ws []Window, tallies map[string]store.Tally, seed int64) string {
	if len(ws) == 0 {
		return ""
	}
	rng := rand.New(rand.NewSource(seed))
	if rng.Float64() < exploreShare {
		return ws[rng.Intn(len(ws))].Key
	}
	acted, ignored := 0, 0
	for _, t := range tallies {
		acted += t.Acted
		ignored += t.Ignored
	}
	overall := float64(acted+1) / float64(acted+ignored+2)

	best, bestDraw := ws[0].Key, -1.0
	for _, w := range ws {
		t := tallies[w.Key]
		draw := betaSample(rng, priorStrength*overall+float64(t.Acted), priorStrength*(1-overall)+float64(t.Ignored))
		if draw > bestDraw {
			best, bestDraw = w.Key, draw
		}
	}
	return best
}

// daySeed is stable for one user on one local day.
func daySeed(userID int64, now time.Time) int64 {
	y, m, d := now.Date()
	return userID*1_000_003 + int64(y)*10_000 + int64(m)*100 + int64(d)
}

func betaSample(rng *rand.Rand, a, b float64) float64 {
	x, y := gammaSample(rng, a), gammaSample(rng, b)
	if x+y == 0 {
		return 0.5
	}
	return x / (x + y)
}

// gammaSample draws from Gamma(shape, 1) with the Marsaglia–Tsang method.
func gammaSample(rng *rand.Rand, shape float64) float64 {
	if shape <= 0 {
		return 0
	}
	if shape < 1 {
		// Boost to shape+1 and scale back.
		return gammaSample(rng, shape+1) * math.Pow(rng.Float64(), 1/shape)
	}
	d := shape - 1.0/3
	c := 1 / math.Sqrt(9*d)
	for {
		x := rng.NormFloat64()
		v := 1 + c*x
		if v <= 0 {
			continue
		}
		v = v * v * v
		u := rng.Float64()
		if u < 1-0.0331*x*x*x*x || math.Log(u) < 0.5*x*x+d*(1-v+math.Log(v)) {
			return d * v
		}
	}
}
