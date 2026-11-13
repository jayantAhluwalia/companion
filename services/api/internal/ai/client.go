// Package ai wraps every call to the Claude API. Prompts are embedded and
// versioned so a change to wording is a change to prompt_version in the logs.
package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go/v2"
	"github.com/anthropics/anthropic-sdk-go/v2/option"
)

// CallInfo is what gets persisted into the llm_calls table.
type CallInfo struct {
	Model         string
	PromptVersion string
	InputTokens   int64
	OutputTokens  int64
	WebSearches   int64
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

// spec describes one structured call: a fixed system prompt, one user
// message, and a JSON schema the reply must satisfy.
type spec struct {
	promptVersion string
	system        string
	user          string
	schema        map[string]any
	effort        anthropic.OutputConfigEffort
	maxTokens     int64
	tools         []anthropic.ToolUnionParam
}

// maxContinuations bounds the resume loop for server-side tools. The API
// pauses a turn after ten tool iterations; a web lookup should need none.
const maxContinuations = 3

// structured runs the call and decodes the reply into out. Every exit path
// returns a CallInfo so failures are logged with their cost too.
func (c *Client) structured(ctx context.Context, s spec, out any) (CallInfo, []anthropic.ContentBlockUnion, error) {
	info := CallInfo{Model: c.model, PromptVersion: s.promptVersion}
	start := time.Now()
	messages := []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock(s.user))}
	var blocks []anthropic.ContentBlockUnion

	for attempt := 0; ; attempt++ {
		resp, err := c.api.Messages.New(ctx, anthropic.MessageNewParams{
			Model:     c.model,
			MaxTokens: s.maxTokens,
			System:    []anthropic.TextBlockParam{{Text: s.system, CacheControl: anthropic.NewCacheControlEphemeralParam()}},
			Messages:  messages,
			Tools:     s.tools,
			OutputConfig: anthropic.OutputConfigParam{
				Effort: s.effort,
				Format: anthropic.JSONOutputFormatParam{Schema: s.schema},
			},
		})
		info.Latency = time.Since(start)
		if err != nil {
			return info, blocks, fmt.Errorf("anthropic: %w", err)
		}
		u := resp.Usage
		info.InputTokens += u.InputTokens + u.CacheCreationInputTokens + u.CacheReadInputTokens
		info.OutputTokens += u.OutputTokens
		info.WebSearches += u.ServerToolUse.WebSearchRequests
		info.StopReason = string(resp.StopReason)
		blocks = append(blocks, resp.Content...)

		switch resp.StopReason {
		case anthropic.StopReasonRefusal:
			return info, blocks, fmt.Errorf("model refused: %s", resp.StopDetails.Explanation)
		case anthropic.StopReasonMaxTokens:
			return info, blocks, fmt.Errorf("output truncated at max_tokens")
		case anthropic.StopReasonPauseTurn:
			if attempt >= maxContinuations {
				return info, blocks, fmt.Errorf("turn still paused after %d continuations", maxContinuations)
			}
			// Re-send with the partial assistant turn; the server resumes it.
			messages = append(messages[:1], resp.ToParam())
			continue
		}

		var text strings.Builder
		for _, block := range resp.Content {
			if tb, ok := block.AsAny().(anthropic.TextBlock); ok {
				text.WriteString(tb.Text)
			}
		}
		if err := decodeObject(text.String(), out); err != nil {
			return info, blocks, fmt.Errorf("parse structured output: %w (raw: %.200s)", err, text.String())
		}
		return info, blocks, nil
	}
}

// decodeObject parses the reply as JSON. With tools in play the model may
// write a sentence before the object, so on failure it falls back to the
// last top-level JSON object found in the text.
func decodeObject(text string, out any) error {
	text = strings.TrimSpace(text)
	first := json.Unmarshal([]byte(text), out)
	if first == nil {
		return nil
	}
	var last json.RawMessage
	for i := 0; i < len(text); i++ {
		if text[i] != '{' {
			continue
		}
		var raw json.RawMessage
		if json.NewDecoder(strings.NewReader(text[i:])).Decode(&raw) == nil {
			last = raw
			i += len(raw) - 1 // skip past it so nested objects are not candidates
		}
	}
	if last != nil && json.Unmarshal(last, out) == nil {
		return nil
	}
	return first
}
