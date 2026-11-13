package telegram

import (
	"regexp"
	"strings"
	"testing"

	"companion/api/internal/ai"
	"companion/api/internal/engine"
	"companion/api/internal/store"
)

var tagRe = regexp.MustCompile(`</?([a-z]+)[^>]*>`)

// balanced checks that every HTML tag Telegram would parse is closed, since
// one stray tag makes Telegram reject the whole message.
func balanced(t *testing.T, name, html string) {
	t.Helper()
	var stack []string
	for _, m := range tagRe.FindAllStringSubmatch(html, -1) {
		if strings.HasPrefix(m[0], "</") {
			if len(stack) == 0 || stack[len(stack)-1] != m[1] {
				t.Fatalf("%s: unbalanced </%s> in:\n%s", name, m[1], html)
			}
			stack = stack[:len(stack)-1]
		} else {
			stack = append(stack, m[1])
		}
	}
	if len(stack) != 0 {
		t.Fatalf("%s: unclosed %v in:\n%s", name, stack, html)
	}
}

func TestMessagesAreValidHTML(t *testing.T) {
	nudge := formatNudge(engine.Outgoing{
		Type: ai.TypeExperience, Title: "Bir Billing <motorcycle> trip",
		Message:  "Saturday & Sunday look dry at Bir. If you leave early, the road is clear.",
		Step:     "Check tyre pressure & fuel up tonight.",
		Footnote: "Bir, Himachal Pradesh: Sat 21 Nov clear 17°/9° rain 0% · Sun 22 Nov clear 16°/7° rain 0%",
		Links:    []ai.Source{{Title: "Forecast <source>", URL: "https://example.com/a?b=1&c=2"}},
	})
	balanced(t, "nudge", nudge)
	if strings.Contains(nudge, "<motorcycle>") || strings.Contains(nudge, "Saturday & Sunday") {
		t.Errorf("dynamic text must be escaped:\n%s", nudge)
	}
	t.Logf("\n%s", nudge)

	lat, lon := 28.65, 77.23
	routine := formatRoutine(store.Prefs{QuietStart: 23 * 60, QuietEnd: 7 * 60, WorkStart: 10 * 60, WorkEnd: 19 * 60,
		WorkDays: []int{1, 2, 3, 4, 5}, DaysOff: []int{6, 7}, NudgeDuringWork: "lunch", DailyLimit: 1,
		HomeCity: "Delhi, India", HomeLat: &lat, HomeLon: &lon})
	balanced(t, "routine", routine)
	t.Logf("\n%s", routine)

	interp := formatInterpretation(4, ai.Interpretation{
		Type: ai.TypeActionable, Title: "Buy lowers for winter", Confidence: 0.9,
		NextSteps: []ai.NextStep{{Label: "Shortlist two pairs.", EstimatedMinutes: 10}},
		Hooks: []ai.Hook{
			{Kind: ai.HookWeb, GoodWhen: "A sale is live or starts within a week."},
			{Kind: ai.HookWeather, Place: "Bir, Himachal Pradesh", DaysNeeded: 3},
			{Kind: ai.HookDate, Date: "2026-12-05", LeadDays: 3},
			{Kind: ai.HookFreeTime, Minutes: 90},
		},
		Assumptions: []string{"Lowers means track pants."},
	})
	balanced(t, "interpretation", interp)
	t.Logf("\n%s", interp)
}
