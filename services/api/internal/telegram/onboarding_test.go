package telegram

import (
	"testing"
	"time"

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

func TestLaterTime(t *testing.T) {
	ist := time.FixedZone("IST", 5*3600+1800)
	p := store.Prefs{QuietStart: 22 * 60, QuietEnd: 8 * 60, DaysOff: []int{6, 7}}
	fri := func(h, m int) time.Time { return time.Date(2026, 11, 13, h, m, 0, 0, ist) } // a Friday

	if got := laterTime("h", fri(13, 0), p); got.Hour() != 16 || got.Day() != 13 {
		t.Errorf("a few hours after 13:00 is 16:00 the same day, got %v", got)
	}
	if got := laterTime("h", fri(19, 30), p); got.Day() != 14 || got.Hour() != 9 {
		t.Errorf("a few hours after 19:30 runs into bedtime, so tomorrow 09:00; got %v", got)
	}
	if got := laterTime("t", fri(13, 0), p); got.Day() != 14 || got.Hour() != 9 {
		t.Errorf("tomorrow morning is an hour after waking, got %v", got)
	}
	if got := laterTime("w", fri(13, 0), p); got.Weekday() != time.Saturday || got.Hour() != 10 || got.Minute() != 30 {
		t.Errorf("next day off from Friday is Saturday 10:30, got %v", got)
	}
	sat := time.Date(2026, 11, 14, 11, 0, 0, 0, ist)
	if got := laterTime("w", sat, p); got.Weekday() != time.Sunday {
		t.Errorf("next day off from Saturday is Sunday, got %v", got)
	}
	if got := laterTime("n", fri(13, 0), p); got.Day() != 20 || got.Hour() != 13 {
		t.Errorf("next week is the same time seven days on, got %v", got)
	}
}
