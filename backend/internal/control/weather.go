package control

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	maxWeatherBytes      = 1 << 20 // 1 MiB
	openMeteoForecastURL = "https://api.open-meteo.com/v1/forecast"
)

// WeatherClient fetches current modeled precipitation for a list of coordinates.
type WeatherClient struct {
	HTTP    *http.Client
	BaseURL string // empty means Open-Meteo; tests and the demo simulator set it
}

// endpoint is the URL the readings come from: BaseURL when set, else Open-Meteo.
func (w *WeatherClient) endpoint() string {
	if w.BaseURL != "" {
		return w.BaseURL
	}
	return openMeteoForecastURL
}

// Fetch returns one WeatherEvidence per input candidate, in the same order.
// A whole-response failure produces an error; per-candidate problems are
// returned as evidence with state=invalid so callers can still display them.
func (w *WeatherClient) Fetch(ctx context.Context, assessedAt time.Time, candidates []Candidate) ([]WeatherEvidence, error) {
	if len(candidates) == 0 {
		return nil, nil
	}
	base := w.endpoint()
	lats := make([]string, len(candidates))
	lons := make([]string, len(candidates))
	for i, c := range candidates {
		lats[i] = strconv.FormatFloat(c.Lat, 'f', 4, 64)
		lons[i] = strconv.FormatFloat(c.Lon, 'f', 4, 64)
	}
	q := fmt.Sprintf("latitude=%s&longitude=%s&current=precipitation&timezone=UTC&timeformat=unixtime",
		strings.Join(lats, ","), strings.Join(lons, ","))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"?"+q, nil)
	if err != nil {
		return nil, err
	}
	resp, err := w.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("weather request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("weather status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxWeatherBytes+1))
	if err != nil {
		return nil, fmt.Errorf("weather read: %w", err)
	}
	if len(body) > maxWeatherBytes {
		return nil, errors.New("weather body exceeds 1 MiB cap")
	}
	parsed, err := parseWeather(body, len(candidates))
	if err != nil {
		return nil, err
	}
	out := make([]WeatherEvidence, len(candidates))
	for i, raw := range parsed {
		out[i] = classify(raw, assessedAt)
	}
	return out, nil
}

type rawCurrent struct {
	Time          *int64   `json:"time"`
	Interval      *int     `json:"interval"`
	Precipitation *float64 `json:"precipitation"`
}

type rawPoint struct {
	Current      rawCurrent        `json:"current"`
	CurrentUnits map[string]string `json:"current_units"`
}

func parseWeather(body []byte, want int) ([]rawPoint, error) {
	trimmed := []byte(strings.TrimSpace(string(body)))
	if len(trimmed) == 0 {
		return nil, errors.New("empty weather body")
	}
	// Open-Meteo answers a multi-location request with an array and a
	// single location with a bare object.
	if trimmed[0] == '[' {
		var multi []rawPoint
		if err := json.Unmarshal(body, &multi); err != nil {
			return nil, fmt.Errorf("weather json (array): %w", err)
		}
		if len(multi) != want {
			return nil, fmt.Errorf("weather returned %d points, want %d", len(multi), want)
		}
		return multi, nil
	}
	var single rawPoint
	if err := json.Unmarshal(body, &single); err != nil {
		return nil, fmt.Errorf("weather json (object): %w", err)
	}
	if want != 1 {
		return nil, fmt.Errorf("weather returned 1 point, want %d", want)
	}
	return []rawPoint{single}, nil
}

func classify(p rawPoint, assessedAt time.Time) WeatherEvidence {
	units := p.CurrentUnits["precipitation"]
	if p.Current.Time == nil || p.Current.Interval == nil || p.Current.Precipitation == nil {
		return WeatherEvidence{State: WeatherInvalid, Reason: "missing precipitation evidence"}
	}
	validAt := time.Unix(*p.Current.Time, 0).UTC()
	age := assessedAt.Sub(validAt)
	ev := WeatherEvidence{
		Units:           units,
		IntervalSeconds: p.Current.Interval,
		ValidAt:         &validAt,
	}
	if units != ExpectedUnits {
		ev.State = WeatherInvalid
		// The unit string comes from the upstream response: quote and cap it.
		ev.Reason = fmt.Sprintf("unexpected units: %.16q", units)
		return ev
	}
	if *p.Current.Interval != ExpectedIntervalSeconds {
		ev.State = WeatherInvalid
		ev.Reason = fmt.Sprintf("unexpected interval: %d", *p.Current.Interval)
		return ev
	}
	if age > WeatherFreshness {
		ev.State = WeatherInvalid
		ev.Reason = fmt.Sprintf("reading older than %d minutes", int(WeatherFreshness.Minutes()))
		return ev
	}
	if age < -WeatherLead {
		ev.State = WeatherInvalid
		ev.Reason = "reading dated in the future"
		return ev
	}
	v := *p.Current.Precipitation
	if v < 0 {
		ev.State = WeatherInvalid
		ev.Reason = "negative precipitation"
		return ev
	}
	ev.State = WeatherValid
	ev.PrecipitationMM = &v
	return ev
}
