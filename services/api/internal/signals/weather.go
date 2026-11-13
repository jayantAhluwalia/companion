// Package signals fetches facts about the outside world that make a thought
// timely. Everything here is plain data: no model is involved, so a nudge
// that cites the weather is citing a number this package fetched.
package signals

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Day is one day of forecast at one place.
type Day struct {
	Date     string  `json:"date"` // YYYY-MM-DD, local to the place
	Weekday  string  `json:"weekday"`
	Sky      string  `json:"sky"`
	TMaxC    float64 `json:"high_c"`
	TMinC    float64 `json:"low_c"`
	RainProb int     `json:"rain_chance_pct"`
	RainMM   float64 `json:"rain_mm"`
	WindKmh  float64 `json:"max_wind_kmh"`
	Code     int     `json:"-"`
	Verdict  string  `json:"outdoor_verdict"` // good | mixed | bad
}

// Place is a geocoded town.
type Place struct {
	Name        string
	Region      string
	Country     string
	CountryCode string
	Lat, Lon    float64
	Timezone    string
}

// Label is the short human form: "Narela, Delhi, India".
func (p Place) Label() string {
	parts := []string{p.Name}
	region := p.Region
	for _, prefix := range []string{"National Capital Territory of ", "Union Territory of ", "State of "} {
		region = strings.TrimPrefix(region, prefix)
	}
	if region != "" && region != p.Name {
		parts = append(parts, region)
	}
	if p.Country != "" {
		parts = append(parts, p.Country)
	}
	return strings.Join(parts, ", ")
}

// Weather talks to Open-Meteo, which is free, needs no key and takes only
// coordinates. Forecasts are cached in memory because they change slowly.
type Weather struct {
	http  *http.Client
	mu    sync.Mutex
	cache map[string]cached
}

type cached struct {
	days []Day
	at   time.Time
}

const forecastTTL = 3 * time.Hour

func NewWeather() *Weather {
	return &Weather{http: &http.Client{Timeout: 15 * time.Second}, cache: map[string]cached{}}
}

func (w *Weather) get(ctx context.Context, rawURL string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "companion-bot/1.0")
	resp, err := w.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("open-meteo: HTTP %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

type geoResult struct {
	Name        string  `json:"name"`
	Admin1      string  `json:"admin1"`
	Admin2      string  `json:"admin2"`
	Country     string  `json:"country"`
	CountryCode string  `json:"country_code"`
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	Timezone    string  `json:"timezone"`
	Population  int64   `json:"population"`
}

func (w *Weather) search(ctx context.Context, name string) ([]geoResult, error) {
	q := url.Values{"name": {name}, "count": {"10"}, "language": {"en"}, "format": {"json"}}
	var body struct {
		Results []geoResult `json:"results"`
	}
	err := w.get(ctx, "https://geocoding-api.open-meteo.com/v1/search?"+q.Encode(), &body)
	return body.Results, err
}

// Geocode resolves what a person typed for "where do you live" to
// coordinates. The lookup service matches one place name at a time and
// knows nothing of addresses, so "Narela, Delhi, 110040" is taken apart.
// Every part is tried as the place name, and the winner is the reading the
// rest of the address supports best: Narela in Delhi rather than Narela in
// Madhya Pradesh, and Noida rather than a "Sector 62" in another country.
// Between equally supported readings, the larger place wins.
func (w *Weather) Geocode(ctx context.Context, input string) (Place, error) {
	var best geoResult
	bestScore, found := -1, false
	for _, attempt := range geocodeAttempts(input) {
		results, err := w.search(ctx, attempt.name)
		if err != nil {
			return Place{}, err
		}
		if len(results) == 0 {
			continue
		}
		r, score := pickPlace(results, attempt.context)
		if !found || score > bestScore || (score == bestScore && r.Population > best.Population) {
			best, bestScore, found = r, score, true
		}
	}
	if !found {
		return Place{}, ErrNoSuchPlace
	}
	return Place{Name: best.Name, Region: best.Admin1, Country: best.Country, CountryCode: best.CountryCode,
		Lat: best.Latitude, Lon: best.Longitude, Timezone: best.Timezone}, nil
}

type geocodeAttempt struct {
	name    string
	context []string // the rest of what was typed, lower-cased
}

// geocodeAttempts lists the lookups to try, most specific first.
func geocodeAttempts(input string) []geocodeAttempt {
	var parts []string
	for _, p := range strings.FieldsFunc(input, func(r rune) bool { return r == ',' || r == ';' || r == '\n' }) {
		p = strings.TrimSpace(p)
		// Postal codes and house numbers are not place names.
		if p == "" || strings.Trim(p, "0123456789 -") == "" {
			continue
		}
		parts = append(parts, p)
	}
	others := func(skip int, from []string) []string {
		var out []string
		for i, p := range from {
			if i != skip {
				out = append(out, strings.ToLower(p))
			}
		}
		return out
	}
	var attempts []geocodeAttempt
	for i, p := range parts {
		attempts = append(attempts, geocodeAttempt{name: p, context: others(i, parts)})
	}
	// "Narela Delhi" with no comma: peel words off the end and use them as context.
	if len(parts) == 1 {
		words := strings.Fields(parts[0])
		for k := len(words) - 1; k >= 1; k-- {
			var ctxWords []string
			for _, word := range words[k:] {
				ctxWords = append(ctxWords, strings.ToLower(word))
			}
			attempts = append(attempts, geocodeAttempt{name: strings.Join(words[:k], " "), context: ctxWords})
		}
	}
	return attempts
}

// pickPlace chooses among same-named places by how much of the remaining
// text each one's region and country explain, and returns that count. Ties
// keep the service's own order, which favours larger places.
func pickPlace(results []geoResult, context []string) (geoResult, int) {
	best, bestScore := results[0], -1
	for _, r := range results {
		hay := strings.ToLower(r.Admin1 + " | " + r.Admin2 + " | " + r.Country + " | " + r.CountryCode)
		score := 0
		for _, c := range context {
			if c != "" && strings.Contains(hay, c) {
				score++
			}
		}
		if score > bestScore {
			best, bestScore = r, score
		}
	}
	return best, bestScore
}

var ErrNoSuchPlace = fmt.Errorf("no such place")

// Forecast returns up to 14 days starting today at the given coordinates.
func (w *Weather) Forecast(ctx context.Context, lat, lon float64) ([]Day, error) {
	key := fmt.Sprintf("%.2f,%.2f", lat, lon)
	w.mu.Lock()
	hit, ok := w.cache[key]
	w.mu.Unlock()
	if ok && time.Since(hit.at) < forecastTTL {
		return hit.days, nil
	}

	q := url.Values{
		"latitude":      {fmt.Sprintf("%.4f", lat)},
		"longitude":     {fmt.Sprintf("%.4f", lon)},
		"daily":         {"weather_code,temperature_2m_max,temperature_2m_min,precipitation_probability_max,precipitation_sum,wind_speed_10m_max"},
		"timezone":      {"auto"},
		"forecast_days": {"14"},
	}
	var body struct {
		Daily struct {
			Time     []string   `json:"time"`
			Code     []*int     `json:"weather_code"`
			TMax     []*float64 `json:"temperature_2m_max"`
			TMin     []*float64 `json:"temperature_2m_min"`
			RainProb []*int     `json:"precipitation_probability_max"`
			RainMM   []*float64 `json:"precipitation_sum"`
			Wind     []*float64 `json:"wind_speed_10m_max"`
		} `json:"daily"`
	}
	if err := w.get(ctx, "https://api.open-meteo.com/v1/forecast?"+q.Encode(), &body); err != nil {
		return nil, err
	}
	d := body.Daily
	days := make([]Day, 0, len(d.Time))
	for i, date := range d.Time {
		// The far end of a forecast can come back null; stop at the first gap.
		if i >= len(d.Code) || i >= len(d.TMax) || i >= len(d.TMin) || i >= len(d.RainProb) || i >= len(d.RainMM) || i >= len(d.Wind) ||
			d.Code[i] == nil || d.TMax[i] == nil || d.TMin[i] == nil || d.RainMM[i] == nil || d.Wind[i] == nil {
			break
		}
		day := Day{Date: date, Code: *d.Code[i], TMaxC: *d.TMax[i], TMinC: *d.TMin[i], RainMM: *d.RainMM[i], WindKmh: *d.Wind[i]}
		if d.RainProb[i] != nil {
			day.RainProb = *d.RainProb[i]
		}
		if t, err := time.Parse("2006-01-02", date); err == nil {
			day.Weekday = t.Weekday().String()
		}
		day.Sky = skyText(day.Code)
		day.Verdict = Verdict(day)
		days = append(days, day)
	}
	if len(days) == 0 {
		return nil, fmt.Errorf("open-meteo: empty forecast")
	}
	w.mu.Lock()
	w.cache[key] = cached{days: days, at: time.Now()}
	w.mu.Unlock()
	return days, nil
}

// Verdict judges a day for being outdoors on rain, wind and storms only.
// Temperature is left to the nudge writer, because "too hot" depends on
// the activity and the numbers are passed along.
func Verdict(d Day) string {
	storm := d.Code >= 95
	heavySnow := d.Code == 75 || d.Code == 86
	switch {
	case storm || heavySnow || d.RainProb >= 60 || d.RainMM >= 5 || d.WindKmh >= 50:
		return "bad"
	case d.RainProb <= 30 && d.RainMM < 1 && d.WindKmh < 35:
		return "good"
	default:
		return "mixed"
	}
}

// skyText maps WMO weather codes to words.
func skyText(code int) string {
	switch {
	case code == 0:
		return "clear"
	case code == 1:
		return "mostly clear"
	case code == 2:
		return "partly cloudy"
	case code == 3:
		return "overcast"
	case code == 45 || code == 48:
		return "fog"
	case code >= 51 && code <= 57:
		return "drizzle"
	case code >= 61 && code <= 67:
		return "rain"
	case code >= 71 && code <= 77:
		return "snow"
	case code >= 80 && code <= 82:
		return "rain showers"
	case code == 85 || code == 86:
		return "snow showers"
	case code >= 95:
		return "thunderstorm"
	default:
		return "unsettled"
	}
}
