package telegram

import (
	"testing"

	"companion/api/internal/store"
)

func TestParseRange(t *testing.T) {
	good := map[string][2]store.Clock{
		"23:00-07:00":   {23 * 60, 7 * 60},
		"23:30 - 07:15": {23*60 + 30, 7*60 + 15},
		"11pm to 7am":   {23 * 60, 7 * 60},
		"9 - 18":        {9 * 60, 18 * 60},
		"12am-8am":      {0, 8 * 60},
		"9.30 to 6pm":   {9*60 + 30, 18 * 60},
	}
	for in, want := range good {
		s, e, ok := parseRange(in)
		if !ok || s != want[0] || e != want[1] {
			t.Errorf("%q: got %s-%s ok=%v, want %s-%s", in, s, e, ok, want[0], want[1])
		}
	}
	for _, bad := range []string{"", "late", "25:00-07:00", "23:00", "9-9", "1-2-3"} {
		if _, _, ok := parseRange(bad); ok {
			t.Errorf("%q should not parse", bad)
		}
	}
}

func TestIsChatter(t *testing.T) {
	for _, s := range []string{"ok", "OK!", "Thanks 🙏", "thank you.", "👍", "🙂🙂", "...", "Got it", "hmm", "Good morning!", "theek hai", "Okay thats fine", "Okay, that's fine.", "sounds good"} {
		if !isChatter(s) {
			t.Errorf("%q should be treated as chatter", s)
		}
	}
	// Short is not the same as empty: these must reach the interpreter.
	for _, s := range []string{"gym", "ok so I need to buy milk", "thanks giving dinner plan", "call mom", "good shoes for running", "test the new bike", "no sugar for a month"} {
		if isChatter(s) {
			t.Errorf("%q is a thought and must not be dropped", s)
		}
	}
}
