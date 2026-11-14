package ai

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go/v2"
)

//go:embed prompts/decide_v2.md
var decidePrompt string

const DecidePromptVersion = "decide_v2"

// Nudge types, shared with the nudges table.
var NudgeTypes = []string{"opportunity", "action_prompt", "check_in", "revival", "relevance_check", "follow_up"}

// DecideSignal is one checked hook as the model sees it.
type DecideSignal struct {
	Kind    string `json:"kind"`
	State   string `json:"state"`
	Summary string `json:"summary"`
	Facts   any    `json:"facts,omitempty"`
}

type DecideCandidate struct {
	ThoughtID      int64          `json:"thought_id"`
	OriginalWords  string         `json:"original_words"`
	Type           ThoughtType    `json:"type"`
	Title          string         `json:"title"`
	Description    string         `json:"description,omitempty"`
	DesiredOutcome string         `json:"desired_outcome,omitempty"`
	Timeframe      string         `json:"timeframe,omitempty"`
	NextSteps      []NextStep     `json:"next_steps"`
	AgeDays        int            `json:"age_days"`
	NudgesSent     int            `json:"nudges_sent"`
	IgnoredInARow  int            `json:"ignored_in_a_row"`
	LastResponse   string         `json:"last_response,omitempty"`
	Score          float64        `json:"score"`
	Signals        []DecideSignal `json:"signals"`
}

type DecideInput struct {
	Now        string            `json:"now"`
	Routine    string            `json:"routine"`
	Home       string            `json:"home,omitempty"`
	Forced     bool              `json:"forced"`
	Candidates []DecideCandidate `json:"candidates"`
}

// Decision is the model's proposal. The engine validates it before anything
// is sent: the backend decides, the model only proposes.
type Decision struct {
	Action        string `json:"action"` // nudge | none
	ThoughtID     int64  `json:"thought_id"`
	NudgeType     string `json:"nudge_type"`
	Message       string `json:"message"`
	SuggestedStep string `json:"suggested_step"`
	Reason        string `json:"reason"`
}

var decideSchema = object(map[string]any{
	"action":         enum("nudge", "none"),
	"thought_id":     integer(),
	"nudge_type":     enum(NudgeTypes...),
	"message":        str(),
	"suggested_step": str(),
	"reason":         str(),
})

// MaxNudgeChars is a hard ceiling; the prompt asks for 320.
const MaxNudgeChars = 500

func (c *Client) Decide(ctx context.Context, in DecideInput) (Decision, CallInfo, error) {
	payload, err := json.MarshalIndent(in, "", "  ")
	if err != nil {
		return Decision{}, CallInfo{Model: c.model, PromptVersion: DecidePromptVersion}, err
	}
	var out Decision
	info, _, err := c.structured(ctx, spec{
		promptVersion: DecidePromptVersion,
		system:        decidePrompt,
		user:          string(payload),
		schema:        decideSchema,
		effort:        anthropic.OutputConfigEffortMedium,
		maxTokens:     4096,
	}, &out)
	if err != nil {
		return Decision{}, info, err
	}
	out.Message = strings.TrimSpace(out.Message)
	out.SuggestedStep = strings.TrimSpace(out.SuggestedStep)
	if out.Action != "nudge" {
		return Decision{Action: "none", Reason: out.Reason}, info, nil
	}
	// The model may only pick from what it was shown.
	known := false
	for _, cand := range in.Candidates {
		known = known || cand.ThoughtID == out.ThoughtID
	}
	switch {
	case !known:
		return Decision{}, info, fmt.Errorf("decision names thought %d, which was not a candidate", out.ThoughtID)
	case out.Message == "":
		return Decision{}, info, fmt.Errorf("decision has an empty message")
	case len([]rune(out.Message)) > MaxNudgeChars:
		return Decision{}, info, fmt.Errorf("nudge message is %d characters, over the %d limit", len([]rune(out.Message)), MaxNudgeChars)
	}
	return out, info, nil
}
