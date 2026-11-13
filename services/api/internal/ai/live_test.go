//go:build live

// Live tests call the real API and cost money (a few cents per run).
// Run them on purpose:  make live   (needs ANTHROPIC_API_KEY in .env)
package ai

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"
)

func liveClient(t *testing.T) *Client {
	t.Helper()
	key := os.Getenv("ANTHROPIC_API_KEY")
	if key == "" {
		t.Skip("ANTHROPIC_API_KEY not set")
	}
	model := os.Getenv("ANTHROPIC_MODEL")
	if model == "" {
		model = "claude-opus-5-5"
	}
	return New(key, model)
}

func TestLiveScout(t *testing.T) {
	c := liveClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	f, info, err := c.Scout(ctx, ScoutRequest{
		Question: "Are any major sales on men's casual clothing running or announced on Myntra, Ajio or Amazon India?",
		GoodWhen: "a sale is live or starts within a week",
		Today:    time.Now(), City: "Delhi", Country: "IN", Timezone: "Asia/Kolkata",
	})
	t.Logf("tokens in=%d out=%d searches=%d cost=$%.4f latency=%s stop=%s",
		info.InputTokens, info.OutputTokens, info.WebSearches, info.CostUSD(), info.Latency.Round(time.Millisecond), info.StopReason)
	if err != nil {
		t.Fatal(err)
	}
	out, _ := json.MarshalIndent(f, "", "  ")
	t.Logf("%s", out)
}

// TestLiveFixtures checks the interpreter against thoughts with a known
// type and known hook kinds. Run it before shipping any prompt change.
func TestLiveFixtures(t *testing.T) {
	c := liveClient(t)
	file, err := os.Open("prompts/fixtures/thoughts.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	type fixture struct {
		Thought string   `json:"thought"`
		Type    string   `json:"type"`
		Hooks   []string `json:"hooks"`    // kinds that must be present
		NoHooks bool     `json:"no_hooks"` // true = must have none
	}
	today := time.Now()
	var total, typeOK, hookOK int
	var cost float64
	sc := bufio.NewScanner(file)
	for sc.Scan() {
		if len(sc.Bytes()) == 0 {
			continue
		}
		var f fixture
		if err := json.Unmarshal(sc.Bytes(), &f); err != nil {
			t.Fatalf("bad fixture line: %v", err)
		}
		total++
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		got, info, err := c.Interpret(ctx, f.Thought, InterpretContext{Today: today, Home: "Delhi, India"})
		cancel()
		cost += info.CostUSD()
		if err != nil {
			t.Errorf("%q: %v", f.Thought, err)
			continue
		}
		kinds := map[string]bool{}
		for _, h := range got.Hooks {
			kinds[h.Kind] = true
		}
		if string(got.Type) == f.Type {
			typeOK++
		} else {
			t.Logf("TYPE  %q: want %s, got %s (%.2f)", f.Thought, f.Type, got.Type, got.Confidence)
		}
		ok := true
		for _, want := range f.Hooks {
			ok = ok && kinds[want]
		}
		if f.NoHooks && len(got.Hooks) > 0 {
			ok = false
		}
		if ok {
			hookOK++
		} else {
			raw, _ := json.Marshal(got.Hooks)
			t.Logf("HOOKS %q: want %v (none=%v), got %s", f.Thought, f.Hooks, f.NoHooks, raw)
		}
	}
	t.Logf("types %d/%d, hooks %d/%d, cost $%.3f", typeOK, total, hookOK, total, cost)
	// Classification of spontaneous thoughts is fuzzy at the edges; the bar
	// is a regression alarm, not a demand for perfection.
	if float64(typeOK) < 0.85*float64(total) {
		t.Errorf("type accuracy %d/%d is below 85%%", typeOK, total)
	}
	if float64(hookOK) < 0.80*float64(total) {
		t.Errorf("hook accuracy %d/%d is below 80%%", hookOK, total)
	}
}
