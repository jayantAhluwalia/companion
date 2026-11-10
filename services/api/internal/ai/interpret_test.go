package ai

import "testing"

func TestValidateDowngradesLowConfidenceToUnclear(t *testing.T) {
	i := Interpretation{Type: TypeActionable, Title: "Sell helmet", Confidence: 0.3}
	if err := validate(&i); err != nil {
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
		if err := validate(&c); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

func TestValidateCapsNextSteps(t *testing.T) {
	steps := make([]NextStep, 8)
	i := Interpretation{Type: TypeActionable, Title: "x", Confidence: 0.9, NextSteps: steps}
	if err := validate(&i); err != nil {
		t.Fatal(err)
	}
	if len(i.NextSteps) != 5 {
		t.Fatalf("want 5 steps, got %d", len(i.NextSteps))
	}
}
