// Package ai wraps every call to the Claude API. Prompts are embedded and
// versioned so a change to wording is a change to prompt_version in the logs.
package ai

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"time"

	"github.com/anthropics/anthropic-sdk-go/v2"
	"github.com/anthropics/anthropic-sdk-go/v2/option"
)

//go:embed prompts/interpret_v1.md
var interpretV1 string

const InterpretPromptVersion = "interpret_v1"

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
}

// interpretSchema is hand-written rather than generated so that it is
// reviewable and stable across SDK upgrades.
var interpretSchema = map[string]any{
	"type":                 "object",
	"additionalProperties": false,
	"required": []string{
		"type", "title", "description", "desired_outcome", "next_steps",
		"confidence", "clarification_needed", "clarification_question", "assumptions",
	},
	"properties": map[string]any{
		"type": map[string]any{
			"type": "string",
			"enum": []string{
				string(TypeActionable), string(TypeExperience), string(TypeIdea),
				string(TypeDevelopment), string(TypeUnclear),
			},
		},
		"title":           map[string]any{"type": "string"},
		"description":     map[string]any{"type": "string"},
		"desired_outcome": map[string]any{"type": "string"},
		"next_steps": map[string]any{
			"type": "array",
			"items": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"required":             []string{"label", "estimated_minutes"},
				"properties": map[string]any{
					"label":             map[string]any{"type": "string"},
					"estimated_minutes": map[string]any{"type": "integer"},
				},
			},
		},
		"confidence":             map[string]any{"type": "number"},
		"clarification_needed":   map[string]any{"type": "boolean"},
		"clarification_question": map[string]any{"type": "string"},
		"assumptions":            map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
	},
}

// CallInfo is what Phase 1 will persist into the llm_calls table.
type CallInfo struct {
	Model         string
	PromptVersion string
	InputTokens   int64
	OutputTokens  int64
	Latency       time.Duration
	StopReason    string
}

type Client struct {
	api   anthropic.Client
	model string
}

func New(apiKey, model string) *Client {
	return &Client{api: anthropic.NewClient(option.WithAPIKey(apiKey)), model: model}
}

// Interpret turns a raw thought into a validated Interpretation.
func (c *Client) Interpret(ctx context.Context, rawThought string, today time.Time) (Interpretation, CallInfo, error) {
	info := CallInfo{Model: c.model, PromptVersion: InterpretPromptVersion}
	start := time.Now()

	userText := fmt.Sprintf("Today is %s.\n\nThe thought:\n%s", today.Format("Monday 2 January 2006"), rawThought)

	resp, err := c.api.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     c.model,
		MaxTokens: 2048,
		System:    []anthropic.TextBlockParam{{Text: interpretV1, CacheControl: anthropic.NewCacheControlEphemeralParam()}},
		Messages:  []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock(userText))},
		OutputConfig: anthropic.OutputConfigParam{
			Effort: anthropic.OutputConfigEffortLow,
			Format: anthropic.JSONOutputFormatParam{Schema: interpretSchema},
		},
	})
	info.Latency = time.Since(start)
	if err != nil {
		return Interpretation{}, info, fmt.Errorf("anthropic: %w", err)
	}
	info.InputTokens = resp.Usage.InputTokens
	info.OutputTokens = resp.Usage.OutputTokens
	info.StopReason = string(resp.StopReason)

	if resp.StopReason == anthropic.StopReasonRefusal {
		return Interpretation{}, info, fmt.Errorf("model refused: %s", resp.StopDetails.Explanation)
	}
	if resp.StopReason == anthropic.StopReasonMaxTokens {
		return Interpretation{}, info, fmt.Errorf("output truncated at max_tokens")
	}

	var text string
	for _, block := range resp.Content {
		if tb, ok := block.AsAny().(anthropic.TextBlock); ok {
			text += tb.Text
		}
	}
	var out Interpretation
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		return Interpretation{}, info, fmt.Errorf("parse structured output: %w (raw: %.200s)", err, text)
	}
	if err := validate(&out); err != nil {
		return Interpretation{}, info, fmt.Errorf("invalid interpretation: %w", err)
	}
	return out, info, nil
}

// validate re-checks what the schema already enforces (defence in depth) and
// applies the product rules the schema cannot express.
func validate(i *Interpretation) error {
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
	return nil
}
