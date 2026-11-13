package ai

import (
	"context"
	_ "embed"
	"fmt"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go/v2"
)

//go:embed prompts/scout_v1.md
var scoutPrompt string

const ScoutPromptVersion = "scout_v1"

// maxSearches caps the searches in one lookup. Search results are the bulk
// of a lookup's input tokens, so this is the main cost lever.
const maxSearches = 2

type ScoutRequest struct {
	Question string
	GoodWhen string
	Today    time.Time
	City     string
	Country  string // ISO 3166-1 alpha-2
	Timezone string
}

type Source struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}

// Finding is what one web lookup concluded about one hook.
type Finding struct {
	Timely     bool     `json:"timely"`
	Headline   string   `json:"headline"`
	Detail     string   `json:"detail"`
	StartsOn   string   `json:"starts_on"`
	EndsOn     string   `json:"ends_on"`
	RecheckOn  string   `json:"recheck_on"` // when looking again would pay off
	Sources    []Source `json:"sources"`
	Confidence float64  `json:"confidence"`
}

var scoutSchema = object(map[string]any{
	"timely":     map[string]any{"type": "boolean"},
	"headline":   str(),
	"detail":     str(),
	"starts_on":  str(),
	"ends_on":    str(),
	"recheck_on": str(),
	"sources":    array(object(map[string]any{"title": str(), "url": str()})),
	"confidence": num(),
})

// Scout answers one web hook using Claude's server-side web search. Only
// the question is sent: no thought text, no name, just a city for locality.
func (c *Client) Scout(ctx context.Context, r ScoutRequest) (Finding, CallInfo, error) {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Question: %s\n", r.Question)
	fmt.Fprintf(&sb, "A good moment means: %s\n", r.GoodWhen)
	fmt.Fprintf(&sb, "Today is %s.\n", r.Today.Format("Monday 2 January 2006"))
	if r.City != "" {
		fmt.Fprintf(&sb, "The person lives in %s (%s).\n", r.City, r.Country)
	}

	search := anthropic.WebSearchTool20260209Param{MaxUses: anthropic.Int(maxSearches)}
	if r.City != "" || r.Country != "" {
		loc := anthropic.UserLocationParam{}
		if r.City != "" {
			loc.City = anthropic.String(r.City)
		}
		if r.Country != "" {
			loc.Country = anthropic.String(r.Country)
		}
		if r.Timezone != "" {
			loc.Timezone = anthropic.String(r.Timezone)
		}
		search.UserLocation = loc
	}

	var out Finding
	info, blocks, err := c.structured(ctx, spec{
		promptVersion: ScoutPromptVersion,
		system:        scoutPrompt,
		user:          sb.String(),
		schema:        scoutSchema,
		effort:        anthropic.OutputConfigEffortLow,
		maxTokens:     8192,
		tools:         []anthropic.ToolUnionParam{{OfWebSearchTool20260209: &search}},
	}, &out)
	if err != nil {
		return Finding{}, info, err
	}

	// Keep only sources that really came back from the search, so a link in
	// a nudge is never one the model made up.
	seen := map[string]bool{}
	for _, b := range blocks {
		if res, ok := b.AsAny().(anthropic.WebSearchToolResultBlock); ok {
			for _, hit := range res.Content.OfWebSearchResultBlockArray {
				seen[hit.URL] = true
			}
		}
	}
	verified := out.Sources[:0]
	for _, s := range out.Sources {
		if seen[s.URL] {
			verified = append(verified, s)
		}
	}
	out.Sources = verified
	if out.Confidence < 0 || out.Confidence > 1 {
		out.Confidence = 0
	}
	// A timely claim with no surviving source is not trusted.
	if out.Timely && len(out.Sources) == 0 {
		out.Timely = false
		out.Detail = strings.TrimSpace(out.Detail + " (unverified: no source)")
	}
	return out, info, nil
}
