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
	status, _, winner, until := Decide(now, next, cands, "")
	if status != StatusAction || winner == nil || winner.AreaID != 32 {
		t.Fatalf("want action on area 32, got status=%s winner=%+v", status, winner)
	}
	if until == nil || until.After(next) {
		t.Errorf("valid_until %v should be bounded by next hour %v", until, next)
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
	status, _, winner, _ := Decide(now, next, cands, "")
	if status != StatusAction || winner == nil || winner.AreaID != 8 {
		t.Fatalf("want action on area 8, got status=%s winner=%+v", status, winner)
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
	status, _, _, until := Decide(now, next, cands, "")
	if status != StatusNoAction {
		t.Fatalf("want no_action, got %s", status)
	}
	if until == nil {
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
	status, _, winner, _ := Decide(now, next, cands, "")
	if status != StatusUnavailable || winner != nil {
		t.Fatalf("invalid first should block, got status=%s winner=%+v", status, winner)
	}
}

func TestDecide_NoCandidates(t *testing.T) {
	now := mustTime(t, "2026-11-12T12:00:00Z")
	next := now.Add(time.Hour)
	status, _, _, until := Decide(now, next, nil, "")
	if status != StatusUnavailable || until != nil {
		t.Fatalf("empty list should be unavailable with null expiry, got %s/%v", status, until)
	}
}

func TestDecide_UpstreamFailure(t *testing.T) {
	now := mustTime(t, "2026-11-12T12:00:00Z")
	next := now.Add(time.Hour)
	v := now.Add(-time.Minute)
	cands := []Candidate{mkCand(32, ptr(1.0), WeatherValid, &v)}
	status, reason, _, _ := Decide(now, next, cands, "modeled weather unavailable")
	if status != StatusUnavailable || reason == "" {
		t.Fatalf("upstream failure should be unavailable with reason, got %s/%q", status, reason)
	}
}
