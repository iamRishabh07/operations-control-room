package control

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// These tests drive the real handler, the real SQLite ranking and the real
// weather adapter; only the weather endpoint is a local stub.

func mustParse(t *testing.T, value string) time.Time {
	t.Helper()
	at, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatal(err)
	}
	return at
}

// serviceSeed has four areas with pickups on Thursday 12:00; area 76 leads,
// then 32, 28 and 8.
func serviceSeed(t *testing.T) Seed {
	seed := testSeed(1)
	for _, id := range []int{32, 8, 28, 76} {
		setTrips(t, &seed, "2022-09-15", 12, id, 100+id)
	}
	return seed
}

// weatherStub answers like Open-Meteo: one reading per requested coordinate,
// stamped a minute before *now. precip[i] is the i-th location's value.
func weatherStub(now *time.Time, precip ...float64) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		readings := make([]string, strings.Count(r.URL.Query().Get("latitude"), ",")+1)
		for i := range readings {
			mm := 0.0
			if i < len(precip) {
				mm = precip[i]
			}
			readings[i] = fmt.Sprintf(
				`{"current":{"time":%d,"interval":900,"precipitation":%g},"current_units":{"precipitation":"mm"}}`,
				now.Unix()-60, mm)
		}
		fmt.Fprint(w, "["+strings.Join(readings, ",")+"]")
	}
}

func newService(t *testing.T, seed Seed, now *time.Time, weather http.HandlerFunc) *Service {
	t.Helper()
	loc, err := time.LoadLocation("America/Chicago")
	if err != nil {
		t.Fatal(err)
	}
	upstream := httptest.NewServer(weather)
	t.Cleanup(upstream.Close)
	return &Service{
		DB:      openSeed(t, seed),
		Seed:    seed,
		Weather: &WeatherClient{HTTP: upstream.Client(), BaseURL: upstream.URL},
		Now:     func() time.Time { return *now },
		Loc:     loc,
	}
}

func assess(t *testing.T, svc *Service) Assessment {
	t.Helper()
	rec := httptest.NewRecorder()
	svc.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/recommendation", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var a Assessment
	if err := json.Unmarshal(rec.Body.Bytes(), &a); err != nil {
		t.Fatalf("decode: %v body=%s", err, rec.Body.String())
	}
	return a
}

func TestService_HoldWhenEveryCandidateIsDry(t *testing.T) {
	now := mustParse(t, "2026-11-12T18:00:00Z") // Thursday 12:00 in Chicago
	svc := newService(t, serviceSeed(t), &now, weatherStub(&now))

	a := assess(t, svc)

	if a.Status != StatusNoAction || a.Area != nil {
		t.Fatalf("want no_action without an area, got %s %+v (%s)", a.Status, a.Area, a.Reason)
	}
	if a.ValidUntil == nil || !a.ValidUntil.After(now) {
		t.Errorf("valid_until must be in the future, got %v", a.ValidUntil)
	}
	if got := areaIDs(a.Candidates); len(got) != 4 || got[0] != 76 {
		t.Errorf("want 4 candidates led by area 76, got %v", got)
	}
	for _, c := range a.Candidates {
		if c.Weather.State != WeatherValid {
			t.Errorf("area %d weather: want valid, got %q", c.AreaID, c.Weather.State)
		}
	}
	if n := len(a.Limitations); n == 0 || !strings.Contains(a.Limitations[n-1], "expires") {
		t.Errorf("an assessment with an expiry must say so in its limitations, got %q", a.Limitations)
	}
	if a.TriggerMM != PrecipitationTriggerMM {
		t.Errorf("trigger_mm: want %v, got %v", PrecipitationTriggerMM, a.TriggerMM)
	}
	// The adapter is pointed at a stub, and the response has to say so.
	if !a.Weather.Simulated || a.Weather.Source != svc.Weather.BaseURL {
		t.Errorf("want simulated weather from %s, got %+v", svc.Weather.BaseURL, a.Weather)
	}
}

func TestService_ActionNamesTheBusiestWetArea(t *testing.T) {
	now := mustParse(t, "2026-11-12T18:00:00Z")
	// Rank 1 (area 76) is dry, rank 2 (area 32) is above the trigger.
	svc := newService(t, serviceSeed(t), &now, weatherStub(&now, 0, 0.6))

	a := assess(t, svc)

	if a.Status != StatusAction || a.Area == nil || a.Area.AreaID != 32 {
		t.Fatalf("want action on area 32, got %s %+v (%s)", a.Status, a.Area, a.Reason)
	}
	if a.ValidUntil == nil || !a.ValidUntil.After(now) {
		t.Errorf("valid_until must be in the future, got %v", a.ValidUntil)
	}
}

func TestService_UpstreamFailureIsReportedOnce(t *testing.T) {
	now := mustParse(t, "2026-11-12T18:00:00Z")
	svc := newService(t, serviceSeed(t), &now, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	a := assess(t, svc)

	if a.Status != StatusUnavailable || a.ReasonCode != ReasonWeatherUnavailable || a.ValidUntil != nil {
		t.Fatalf("want unavailable/weather_unavailable with no expiry, got %s/%s %v", a.Status, a.ReasonCode, a.ValidUntil)
	}
	if a.Weather.FetchedAt != nil {
		t.Errorf("fetched_at must be null when the fetch failed, got %v", a.Weather.FetchedAt)
	}
	for _, c := range a.Candidates {
		// The failure is reported once, in the assessment, not on each candidate.
		if c.Weather.State != WeatherMissing || c.Weather.PrecipitationMM != nil || c.Weather.Reason != "" {
			t.Errorf("area %d: weather must be missing with no value or reason, got %+v", c.AreaID, c.Weather)
		}
	}
	// Nothing on the page shows an expiry in this state, so no text may refer to one.
	for _, l := range a.Limitations {
		if strings.Contains(l, "expires") {
			t.Errorf("limitation refers to an expiry that does not exist: %q", l)
		}
	}
}

// A reading in the wrong unit fails validation; the answer names the area and
// does not guess.
func TestService_InvalidReadingIsUnavailable(t *testing.T) {
	now := mustParse(t, "2026-11-12T18:00:00Z")
	svc := newService(t, serviceSeed(t), &now, func(w http.ResponseWriter, r *http.Request) {
		reading := fmt.Sprintf(`{"current":{"time":%d,"interval":900,"precipitation":0.5},"current_units":{"precipitation":"inch"}}`, now.Unix()-60)
		n := strings.Count(r.URL.Query().Get("latitude"), ",") + 1
		fmt.Fprint(w, "["+strings.TrimSuffix(strings.Repeat(reading+",", n), ",")+"]")
	})

	a := assess(t, svc)

	if a.Status != StatusUnavailable || a.ReasonCode != ReasonWeatherInvalid || a.Area != nil || a.ValidUntil != nil {
		t.Fatalf("want unavailable/weather_invalid, got %s/%s area=%+v until=%v", a.Status, a.ReasonCode, a.Area, a.ValidUntil)
	}
	if top := a.Candidates[0]; top.Weather.State != WeatherInvalid || !strings.Contains(a.Reason, top.Name) {
		t.Errorf("reason %q must name the top area %q, whose reading is %q", a.Reason, top.Name, top.Weather.State)
	}
}

// An hour with no pickups in the baseline has nothing to rank: the answer is
// unavailable and the weather source is not asked.
func TestService_NoHistoryIsUnavailable(t *testing.T) {
	now := mustParse(t, "2026-11-12T09:00:00Z") // Thursday 03:00 in Chicago: no trips in the fixture
	asked := false
	svc := newService(t, serviceSeed(t), &now, func(w http.ResponseWriter, r *http.Request) { asked = true })

	a := assess(t, svc)

	if a.Status != StatusUnavailable || a.ReasonCode != ReasonNoHistory || len(a.Candidates) != 0 {
		t.Fatalf("want unavailable/no_history with no candidates, got %s/%s %d", a.Status, a.ReasonCode, len(a.Candidates))
	}
	if asked {
		t.Error("the weather source must not be called when there are no candidates")
	}
}

// When daylight saving ends, 01:00-01:59 happens twice in Chicago. Each pass
// is its own clock hour and must not be mistaken for the other.
func TestService_HourStartInRepeatedDSTHour(t *testing.T) {
	seed := testSeed(1)
	setTrips(t, &seed, "2022-09-18", 1, 8, 10) // Sunday 01:00
	var now time.Time
	svc := newService(t, seed, &now, weatherStub(&now))

	passes := map[string]string{
		"2026-11-01T06:30:00Z": "2026-11-01T06:00:00Z", // 01:30 CDT, first pass
		"2026-11-01T07:30:00Z": "2026-11-01T07:00:00Z", // 01:30 CST, second pass
	}
	for instant, wantStart := range passes {
		now = mustParse(t, instant)
		a := assess(t, svc)
		if !a.SlotStart.Equal(mustParse(t, wantStart)) {
			t.Errorf("at %s: hour start %s, want %s", instant, a.SlotStart.Format(time.RFC3339), wantStart)
		}
		if !a.AssessedAt.Equal(now) {
			t.Errorf("at %s: assessed_at %s must be the moment of assessment", instant, a.AssessedAt.Format(time.RFC3339))
		}
		if a.ValidUntil == nil || !a.ValidUntil.After(now) {
			t.Errorf("at %s: valid_until %v must be after now", instant, a.ValidUntil)
		}
	}
}
