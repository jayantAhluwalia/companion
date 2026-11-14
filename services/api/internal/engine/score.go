package engine

import (
	"math"
	"time"

	"companion/api/internal/ai"
	"companion/api/internal/store"
)

// Weights of the score. Timeliness leads because the product's promise is
// the right moment, not the oldest item.
const (
	wT = 0.35 // timeliness: did a hook fire?
	wR = 0.20 // recency: how long since the user last touched it
	wI = 0.15 // importance: how much the user has invested in it
	wA = 0.15 // actionability: is there a small first step?
	wO = 0.10 // opportunity: does this kind of thought fit this kind of day?
	wF = 0.05 // feedback: learned style preference (Phase 3; constant until then)

	// Threshold is the bar a thought must clear before the model is asked.
	Threshold = 0.50
)

// Scored is one candidate with every component kept, so /why can show the
// arithmetic behind a decision.
type Scored struct {
	ThoughtID int64       `json:"thought_id"`
	Number    int         `json:"number,omitempty"` // the number its owner sees
	Title     string      `json:"title"`
	T         float64     `json:"T"`
	R         float64     `json:"R"`
	I         float64     `json:"I"`
	A         float64     `json:"A"`
	O         float64     `json:"O"`
	F         float64     `json:"F"`
	Score     float64     `json:"score"`
	Lane      string      `json:"lane,omitempty"`
	Held      string      `json:"held,omitempty"` // why it was not eligible this time
	Hooks     []HookState `json:"hooks,omitempty"`
}

func round2(x float64) float64 { return math.Round(x*100) / 100 }

func clamp01(x float64) float64 { return math.Min(1, math.Max(0, x)) }

func recency(t store.Thought, now time.Time) float64 {
	days := now.Sub(t.LastTouchedAt).Hours() / 24
	if days < 2 {
		return 0 // a thought just captured or just acted on is left alone
	}
	return clamp01(days / 14)
}

func importance(st store.Stats) float64 {
	v := 0.5
	if st.Confirmed {
		v += 0.15
	}
	if st.UserEdits > 0 {
		v += 0.20
	}
	return clamp01(v - 0.20*float64(st.Laters))
}

func actionability(i ai.Interpretation) float64 {
	if len(i.NextSteps) == 0 {
		return 0.2
	}
	for _, s := range i.NextSteps {
		if s.EstimatedMinutes > 0 && s.EstimatedMinutes <= 30 {
			return 1.0
		}
	}
	return 0.5
}

// opportunity is a prior by thought type and day. Experiences belong to
// days off and the run-up to them; the rest are steady.
func opportunity(typ ai.ThoughtType, p store.Prefs, now time.Time) float64 {
	switch typ {
	case ai.TypeExperience:
		for lead := 0; lead <= 2; lead++ {
			if p.IsDayOff(isoWeekday(now.AddDate(0, 0, lead))) {
				return 1.0
			}
		}
		return 0.3
	case ai.TypeDevelopment:
		return 0.6
	case ai.TypeActionable:
		return 0.5
	default:
		return 0.4
	}
}

func score(t store.Thought, st store.Stats, hooks []HookState, p store.Prefs, now time.Time) Scored {
	s := Scored{
		ThoughtID: t.ID, Number: t.Number, Title: t.Interpretation.Title, Hooks: hooks,
		T: timeliness(hooks),
		R: recency(t, now),
		I: importance(st),
		A: actionability(t.Interpretation.Interpretation),
		O: opportunity(t.Interpretation.Type, p, now),
		F: 0.5,
	}
	s.Score = round2(wT*s.T + wR*s.R + wI*s.I + wA*s.A + wO*s.O + wF*s.F)
	s.R, s.I = round2(s.R), round2(s.I)
	return s
}
