package ai

import (
	"context"
	_ "embed"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go/v2"
)

//go:embed prompts/interpret_v3.md
var interpretPrompt string

const InterpretPromptVersion = "interpret_v3"

// PromptVersionsWithoutHooks are the versions whose stored readings need a
// re-read to gain hooks. Later versions only refine; they do not trigger one.
var PromptVersionsWithoutHooks = []string{"interpret_v1"}

// ThoughtType is the closed set of classifications from the discovery doc,
// plus "unclear" for low-confidence results that should not be nudged.
type ThoughtType string

const (
	TypeActionable  ThoughtType = "actionable_intention"
	TypeExperience  ThoughtType = "experience_interest"
	TypeIdea        ThoughtType = "idea_possibility"
	TypeDevelopment ThoughtType = "personal_development"
	TypeUnclear     ThoughtType = "unclear"
)

var validTypes = map[ThoughtType]bool{
	TypeActionable: true, TypeExperience: true, TypeIdea: true, TypeDevelopment: true, TypeUnclear: true,
}

type NextStep struct {
	Label            string `json:"label"`
	EstimatedMinutes int    `json:"estimated_minutes"`
}

// Hook kinds: what in the outside world makes a thought timely.
const (
	HookWeather  = "weather"
	HookWeb      = "web"
	HookDate     = "date"
	HookFreeTime = "free_time"
)

// Hook is one condition the nudge engine watches for a thought. It is a flat
// struct: every field is always present and the ones that do not belong to
// the kind are zero. That keeps the JSON schema simple and strict.
type Hook struct {
	Kind string `json:"kind"`
	Why  string `json:"why"`
	// weather
	Activity   string  `json:"activity"`
	Place      string  `json:"place"` // "" = around home
	Lat        float64 `json:"lat"`
	Lon        float64 `json:"lon"`
	DaysNeeded int     `json:"days_needed"`
	// web
	Question string `json:"question"`
	GoodWhen string `json:"good_when"`
	// date
	Date     string `json:"date"` // YYYY-MM-DD
	LeadDays int    `json:"lead_days"`
	// free_time
	Minutes int `json:"minutes"`
}

// HasPlace reports whether a weather hook names its own destination.
func (h Hook) HasPlace() bool { return h.Lat != 0 || h.Lon != 0 }

// Interpretation mirrors the JSON schema below. Keep the two in sync; the
// schema is what the API enforces, this struct is what the code trusts.
type Interpretation struct {
	Type                  ThoughtType `json:"type"`
	Title                 string      `json:"title"`
	Description           string      `json:"description"`
	DesiredOutcome        string      `json:"desired_outcome"`
	NextSteps             []NextStep  `json:"next_steps"`
	Confidence            float64     `json:"confidence"`
	ClarificationNeeded   bool        `json:"clarification_needed"`
	ClarificationQuestion string      `json:"clarification_question"`
	Assumptions           []string    `json:"assumptions"`
	Timeframe             string      `json:"timeframe"`
	Hooks                 []Hook      `json:"hooks"`
	// NotAThought is true when the message was talk aimed at the bot (a
	// greeting, thanks, a question) rather than something to remember. It is
	// acted on at capture and not stored.
	NotAThought bool `json:"not_a_thought"`
}

func str() map[string]any { return map[string]any{"type": "string"} }
func num() map[string]any { return map[string]any{"type": "number"} }
func integer() map[string]any {
	return map[string]any{"type": "integer"}
}

// object builds a strict object schema in which every property is required.
func object(props map[string]any) map[string]any {
	required := make([]string, 0, len(props))
	for k := range props {
		required = append(required, k)
	}
	sort.Strings(required) // map order is random; a stable schema keeps server-side caches warm
	return map[string]any{"type": "object", "additionalProperties": false, "required": required, "properties": props}
}

func array(items map[string]any) map[string]any {
	return map[string]any{"type": "array", "items": items}
}

func enum(values ...string) map[string]any {
	return map[string]any{"type": "string", "enum": values}
}

// interpretSchema is hand-written rather than generated so that it is
// reviewable and stable across SDK upgrades.
var interpretSchema = object(map[string]any{
	"type": enum(string(TypeActionable), string(TypeExperience), string(TypeIdea),
		string(TypeDevelopment), string(TypeUnclear)),
	"title":           str(),
	"description":     str(),
	"desired_outcome": str(),
	"next_steps": array(object(map[string]any{
		"label": str(), "estimated_minutes": integer(),
	})),
	"confidence":             num(),
	"clarification_needed":   map[string]any{"type": "boolean"},
	"clarification_question": str(),
	"assumptions":            array(str()),
	"timeframe":              str(),
	"not_a_thought":          map[string]any{"type": "boolean"},
	"hooks": array(object(map[string]any{
		"kind":     enum(HookWeather, HookWeb, HookDate, HookFreeTime),
		"why":      str(),
		"activity": str(), "place": str(), "lat": num(), "lon": num(), "days_needed": integer(),
		"question": str(), "good_when": str(),
		"date": str(), "lead_days": integer(),
		"minutes": integer(),
	})),
})

// InterpretContext is what the model is told besides the thought itself.
type InterpretContext struct {
	Today         time.Time
	Home          string // "Delhi, India"; empty if unknown
	Clarification string // what the person said when the first reading was wrong
}

// Interpret turns a raw thought into a validated Interpretation.
func (c *Client) Interpret(ctx context.Context, rawThought string, ic InterpretContext) (Interpretation, CallInfo, error) {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Today is %s.\n", ic.Today.Format("Monday 2 January 2006"))
	if ic.Home != "" {
		fmt.Fprintf(&sb, "The person lives in %s.\n", ic.Home)
	}
	fmt.Fprintf(&sb, "\nThe thought:\n%s", rawThought)
	if ic.Clarification != "" {
		fmt.Fprintf(&sb, "\n\nAn earlier interpretation was wrong. The person explained what they meant:\n%s", ic.Clarification)
	}

	var out Interpretation
	info, _, err := c.structured(ctx, spec{
		promptVersion: InterpretPromptVersion,
		system:        interpretPrompt,
		user:          sb.String(),
		schema:        interpretSchema,
		effort:        anthropic.OutputConfigEffortLow,
		maxTokens:     4096,
	}, &out)
	if err != nil {
		return Interpretation{}, info, err
	}
	if err := validate(&out, ic.Today); err != nil {
		return Interpretation{}, info, fmt.Errorf("invalid interpretation: %w", err)
	}
	return out, info, nil
}

// validate re-checks what the schema already enforces (defence in depth) and
// applies the product rules the schema cannot express.
func validate(i *Interpretation, today time.Time) error {
	if !validTypes[i.Type] {
		return fmt.Errorf("unknown type %q", i.Type)
	}
	if i.Title == "" {
		return fmt.Errorf("empty title")
	}
	if i.Confidence < 0 || i.Confidence > 1 {
		return fmt.Errorf("confidence %v out of range", i.Confidence)
	}
	if len(i.NextSteps) > 5 {
		i.NextSteps = i.NextSteps[:5]
	}
	if i.ClarificationNeeded && i.ClarificationQuestion == "" {
		return fmt.Errorf("clarification requested without a question")
	}
	// Product rule 6.1: low confidence is treated as unclear so it is never nudged.
	if i.Confidence < 0.5 && i.Type != TypeUnclear {
		i.Type = TypeUnclear
	}
	i.Hooks = cleanHooks(i.Hooks, today)
	return nil
}

// cleanHooks drops hooks the engine could not act on instead of failing the
// whole interpretation: a bad hook costs a smart nudge, not the thought.
func cleanHooks(in []Hook, today time.Time) []Hook {
	out := make([]Hook, 0, len(in))
	for _, h := range in {
		switch h.Kind {
		case HookWeather:
			if h.Lat < -90 || h.Lat > 90 || h.Lon < -180 || h.Lon > 180 {
				h.Lat, h.Lon = 0, 0
			}
			h.DaysNeeded = min(max(h.DaysNeeded, 1), 14)
		case HookWeb:
			if strings.TrimSpace(h.Question) == "" {
				continue
			}
		case HookDate:
			d, err := time.Parse("2006-01-02", h.Date)
			if err != nil || d.Before(today.AddDate(0, 0, -1)) {
				continue
			}
			h.LeadDays = min(max(h.LeadDays, 0), 30)
		case HookFreeTime:
			if h.Minutes <= 0 {
				h.Minutes = 60
			}
		default:
			continue
		}
		out = append(out, h)
		if len(out) == 3 {
			break
		}
	}
	return out
}
