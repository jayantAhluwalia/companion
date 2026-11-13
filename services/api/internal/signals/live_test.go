//go:build live

package signals

import (
	"context"
	"testing"
	"time"
)

// Free to run: Open-Meteo needs no key.  go test -tags live ./internal/signals
func TestLiveGeocode(t *testing.T) {
	w := NewWeather()
	for input, want := range map[string]string{
		"Narela, Delhi, 110040":    "Narela, Delhi, India",
		"Narela Delhi":             "Narela, Delhi, India",
		"Narela":                   "Narela, Delhi, India",
		"New Delhi":                "New Delhi, Delhi, India",
		"Bengaluru, Karnataka":     "Bengaluru, Karnataka, India",
		"Sector 62, Noida, 201309": "Noida, Uttar Pradesh, India",
		"Delhi, India":             "Delhi, India",
		"Koramangala, Bangalore":   "Koramangala, Karnataka, India", // the neighbourhood itself is the better answer
		"Mumbai":                   "Mumbai, Maharashtra, India",
		"Pune 411001":              "Pune, Maharashtra, India",
	} {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		p, err := w.Geocode(ctx, input)
		cancel()
		if err != nil {
			t.Errorf("%q: %v", input, err)
			continue
		}
		if p.Label() != want {
			t.Errorf("%q: got %q (%s, %.2f,%.2f), want %q", input, p.Label(), p.Timezone, p.Lat, p.Lon, want)
		} else {
			t.Logf("%q -> %s (%.2f, %.2f, %s)", input, p.Label(), p.Lat, p.Lon, p.Timezone)
		}
	}
}
