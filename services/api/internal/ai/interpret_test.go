package ai

import (
	"testing"
	"time"
)

func TestValidateDowngradesLowConfidenceToUnclear(t *testing.T) {
	i := Interpretation{Type: TypeActionable, Title: "Sell helmet", Confidence: 0.3}
	if err := validate(&i, time.Now()); err != nil {
		t.Fatal(err)
	}
	if i.Type != TypeUnclear {
		t.Fatalf("want unclear, got %s", i.Type)
	}
}

func TestValidateRejectsBadInput(t *testing.T) {
	cases := map[string]Interpretation{
		"unknown type":            {Type: "task", Title: "x", Confidence: 0.9},
		"empty title":             {Type: TypeIdea, Confidence: 0.9},
		"confidence out of range": {Type: TypeIdea, Title: "x", Confidence: 1.4},
		"question missing":        {Type: TypeIdea, Title: "x", Confidence: 0.9, ClarificationNeeded: true},
	}
	for name, c := range cases {
		if err := validate(&c, time.Now()); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

func TestValidateCapsNextSteps(t *testing.T) {
	steps := make([]NextStep, 8)
	i := Interpretation{Type: TypeActionable, Title: "x", Confidence: 0.9, NextSteps: steps}
	if err := validate(&i, time.Now()); err != nil {
		t.Fatal(err)
	}
	if len(i.NextSteps) != 5 {
		t.Fatalf("want 5 steps, got %d", len(i.NextSteps))
	}
}

func TestCleanHooksDropsWhatTheEngineCannotUse(t *testing.T) {
	today := time.Date(2026, 11, 13, 0, 0, 0, 0, time.UTC)
	got := cleanHooks([]Hook{
		{Kind: HookWeather, Lat: 32.05, Lon: 76.72, DaysNeeded: 0},
		{Kind: HookWeb, Question: "  "},
		{Kind: HookDate, Date: "2026-10-01"},
		{Kind: HookDate, Date: "not a date"},
		{Kind: "mood"},
		{Kind: HookDate, Date: "2026-12-05", LeadDays: 3},
	}, today)
	if len(got) != 2 {
		t.Fatalf("want 2 usable hooks, got %d: %+v", len(got), got)
	}
	if got[0].DaysNeeded != 1 {
		t.Errorf("days_needed should default to 1, got %d", got[0].DaysNeeded)
	}
	if got[1].Date != "2026-12-05" {
		t.Errorf("want the future date hook kept, got %+v", got[1])
	}
}

func TestDecodeObjectToleratesAPreamble(t *testing.T) {
	var out struct {
		Timely bool `json:"timely"`
		Inner  struct {
			N int `json:"n"`
		} `json:"inner"`
	}
	text := "I searched and found this.\n{\"timely\": true, \"inner\": {\"n\": 2}}"
	if err := decodeObject(text, &out); err != nil {
		t.Fatal(err)
	}
	if !out.Timely || out.Inner.N != 2 {
		t.Fatalf("decoded the wrong object: %+v", out)
	}
}
