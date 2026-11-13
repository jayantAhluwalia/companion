package signals

import (
	"reflect"
	"testing"
)

func TestGeocodeAttemptsTakeAnAddressApart(t *testing.T) {
	got := geocodeAttempts("Narela, Delhi, 110040")
	want := []geocodeAttempt{
		{name: "Narela", context: []string{"delhi"}},
		{name: "Delhi", context: []string{"narela"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
	// No comma: the whole string first, then words peeled off as context.
	got = geocodeAttempts("Narela Delhi")
	if len(got) != 2 || got[0].name != "Narela Delhi" || got[1].name != "Narela" || got[1].context[0] != "delhi" {
		t.Fatalf("got %+v", got)
	}
	if got := geocodeAttempts("110040"); len(got) != 0 {
		t.Fatalf("a bare PIN code is not a place name: %+v", got)
	}
}

func TestPickPlaceUsesTheRestOfTheAddress(t *testing.T) {
	results := []geoResult{
		{Name: "Narela", Admin1: "Madhya Pradesh", Country: "India"},
		{Name: "Narela", Admin1: "National Capital Territory of Delhi", Country: "India"},
	}
	if got, score := pickPlace(results, []string{"delhi"}); got.Admin1 != "National Capital Territory of Delhi" || score != 1 {
		t.Errorf("want the Delhi one with one supporting part, got %s (%d)", got.Admin1, score)
	}
	if got, score := pickPlace(results, nil); got.Admin1 != "Madhya Pradesh" || score != 0 {
		t.Errorf("with no context the service's first result stands, got %s (%d)", got.Admin1, score)
	}
}

func TestLabel(t *testing.T) {
	p := Place{Name: "Narela", Region: "National Capital Territory of Delhi", Country: "India"}
	if got := p.Label(); got != "Narela, Delhi, India" {
		t.Errorf("got %q", got)
	}
	if got := (Place{Name: "Delhi", Region: "National Capital Territory of Delhi", Country: "India"}).Label(); got != "Delhi, India" {
		t.Errorf("got %q", got)
	}
}
