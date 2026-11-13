package control

import (
	"testing"
	"time"
)

func mustTime(t *testing.T, s string) time.Time {
	t.Helper()
	tm, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatal(err)
	}
	return tm
}

func mkCand(id int, precip *float64, state string, validAt *time.Time) Candidate {
	return Candidate{
		AreaID: id,
		Name:   "Area",
		Weather: WeatherEvidence{
			State:           state,
			PrecipitationMM: precip,
			ValidAt:         validAt,
		},
	}
}

func ptr[T any](v T) *T { return &v }

func TestDecide_FirstWet(t *testing.T) {
	now := mustTime(t, "2026-11-12T12:00:00Z")
	next := now.Add(time.Hour)
	v := now.Add(-2 * time.Minute)
	cands := []Candidate{
		mkCand(32, ptr(0.5), WeatherValid, &v),
		mkCand(8, ptr(0.0), WeatherValid, &v),
	}
	d := Decide(now, next, cands)
	if d.Status != StatusAction || d.Winner == nil || d.Winner.AreaID != 32 {
		t.Fatalf("want action on area 32, got status=%s winner=%+v", d.Status, d.Winner)
	}
	if d.ValidUntil == nil || d.ValidUntil.After(next) {
		t.Errorf("valid_until %v should be bounded by next hour %v", d.ValidUntil, next)
	}
}

func TestDecide_FirstDrySecondWet(t *testing.T) {
	now := mustTime(t, "2026-11-12T12:00:00Z")
	next := now.Add(time.Hour)
	v := now.Add(-time.Minute)
	cands := []Candidate{
		mkCand(32, ptr(0.0), WeatherValid, &v),
		mkCand(8, ptr(0.3), WeatherValid, &v),
	}
	d := Decide(now, next, cands)
	if d.Status != StatusAction || d.Winner == nil || d.Winner.AreaID != 8 {
		t.Fatalf("want action on area 8, got status=%s winner=%+v", d.Status, d.Winner)
	}
}

func TestDecide_AllDryNoAction(t *testing.T) {
	now := mustTime(t, "2026-11-12T12:00:00Z")
	next := now.Add(time.Hour)
	v := now.Add(-time.Minute)
	cands := []Candidate{
		mkCand(32, ptr(0.0), WeatherValid, &v),
		mkCand(8, ptr(0.1), WeatherValid, &v),
	}
	d := Decide(now, next, cands)
	if d.Status != StatusNoAction || d.Winner != nil || d.ReasonCode != "" {
		t.Fatalf("want no_action with no winner and no reason code, got %+v", d)
	}
	if d.ValidUntil == nil {
		t.Fatal("no_action should have valid_until")
	}
}

func TestDecide_InvalidPredecessorBlocks(t *testing.T) {
	now := mustTime(t, "2026-11-12T12:00:00Z")
	next := now.Add(time.Hour)
	v := now.Add(-time.Minute)
	cands := []Candidate{
		mkCand(32, nil, WeatherInvalid, nil),
		mkCand(8, ptr(1.0), WeatherValid, &v),
	}
	d := Decide(now, next, cands)
	if d.Status != StatusUnavailable || d.ReasonCode != ReasonWeatherInvalid || d.Winner != nil || d.ValidUntil != nil {
		t.Fatalf("invalid first should block as weather_invalid, got %+v", d)
	}
}

// A reading below the winner is not needed, so its state must not matter.
func TestDecide_LowerRankedProblemDoesNotBlockAWinner(t *testing.T) {
	now := mustTime(t, "2026-11-12T12:00:00Z")
	v := now.Add(-time.Minute)
	cands := []Candidate{
		mkCand(32, ptr(0.4), WeatherValid, &v),
		mkCand(8, nil, WeatherMissing, nil),
	}
	d := Decide(now, now.Add(time.Hour), cands)
	if d.Status != StatusAction || d.Winner == nil || d.Winner.AreaID != 32 {
		t.Fatalf("want action on area 32, got %+v", d)
	}
}

func TestDecide_NoCandidates(t *testing.T) {
	now := mustTime(t, "2026-11-12T12:00:00Z")
	next := now.Add(time.Hour)
	d := Decide(now, next, nil)
	if d.Status != StatusUnavailable || d.ReasonCode != ReasonNoHistory || d.ValidUntil != nil {
		t.Fatalf("empty list should be unavailable/no_history with null expiry, got %+v", d)
	}
}

// A candidate without a usable reading must block the decision. Absence of
// evidence is never read as "dry".
func TestDecide_MissingReadingIsNeverDry(t *testing.T) {
	now := mustTime(t, "2026-11-12T12:00:00Z")
	next := now.Add(time.Hour)
	v := now.Add(-time.Minute)
	for name, top := range map[string]Candidate{
		"missing":             mkCand(32, nil, WeatherMissing, nil),
		"zero value":          mkCand(32, nil, "", nil),
		"valid without value": mkCand(32, nil, WeatherValid, &v),
	} {
		cands := []Candidate{top, mkCand(8, ptr(0.0), WeatherValid, &v)}
		d := Decide(now, next, cands)
		if d.Status != StatusUnavailable || d.ReasonCode != ReasonWeatherUnavailable || d.ValidUntil != nil {
			t.Errorf("%s: want unavailable/weather_unavailable with no expiry, got %+v", name, d)
		}
	}
}

func TestDecide_StaleHourBoundaryIsClamped(t *testing.T) {
	// A caller can pass a past nextHourStart through clock skew or a race at
	// the hour boundary; valid_until must still be in the future.
	now := mustTime(t, "2026-11-12T12:00:00Z")
	stale := now.Add(-time.Hour)
	v := now.Add(-time.Minute)
	cands := []Candidate{mkCand(32, ptr(0.0), WeatherValid, &v)}
	d := Decide(now, stale, cands)
	if d.ValidUntil == nil || !d.ValidUntil.After(now) {
		t.Fatalf("valid_until must be in the future, got %v (now=%v)", d.ValidUntil, now)
	}
}
