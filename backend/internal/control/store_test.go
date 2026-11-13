package control

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"path/filepath"
	"testing"
	"time"
)

// testSeed returns a dense all-zero seed covering whole weeks from Monday
// 2022-09-12 (so 09-13 is a Tuesday, 09-15 a Thursday, 09-18 a Sunday).
func testSeed(weeks int) Seed {
	areas := make([]Area, areaCount)
	for i := range areas {
		areas[i] = Area{AreaID: i + 1, Name: fmt.Sprintf("Area %d", i+1), Lat: 41.8, Lon: -87.7, CoordSource: "test"}
	}
	start := time.Date(2022, 9, 12, 0, 0, 0, 0, time.UTC)
	hours := make([]HourCounts, 0, weeks*7*24)
	for day := 0; day < weeks*7; day++ {
		date := start.AddDate(0, 0, day).Format(time.DateOnly)
		for hour := 0; hour < 24; hour++ {
			hours = append(hours, HourCounts{Date: date, Hour: hour, Trips: make([]int, areaCount)})
		}
	}
	return Seed{Areas: areas, Hours: hours, History: ImportRecord{
		DatasetID:   "test",
		PeriodStart: "2022-09-12",
		PeriodEnd:   start.AddDate(0, 0, weeks*7-1).Format(time.DateOnly),
		RetrievedAt: "2026-11-13T00:00:00Z",
		QueryText:   "test",
		RowsLoaded:  weeks * 7 * 24 * areaCount,
	}}
}

// setTrips records pickups for one area, date and hour and keeps the seed total in step.
func setTrips(t *testing.T, s *Seed, date string, hour, areaID, trips int) {
	t.Helper()
	for i := range s.Hours {
		if s.Hours[i].Date == date && s.Hours[i].Hour == hour {
			s.History.TripsIncluded += trips - s.Hours[i].Trips[areaID-1]
			s.Hours[i].Trips[areaID-1] = trips
			return
		}
	}
	t.Fatalf("no hourly row for %s hour %d", date, hour)
}

func openSeed(t *testing.T, seed Seed) *sql.DB {
	t.Helper()
	db, err := OpenDB(context.Background(), filepath.Join(t.TempDir(), "test.db"), seed)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func rank(t *testing.T, db *sql.DB, dow, hour int) []Candidate {
	t.Helper()
	got, err := RankCandidates(context.Background(), db, dow, hour)
	if err != nil {
		t.Fatalf("rank dow=%d hour=%d: %v", dow, hour, err)
	}
	return got
}

func areaIDs(cands []Candidate) []int {
	ids := make([]int, len(cands))
	for i, c := range cands {
		ids[i] = c.AreaID
	}
	return ids
}

func TestRankAggregatesMatchingDaysInSQL(t *testing.T) {
	seed := testSeed(2)
	setTrips(t, &seed, "2022-09-12", 8, 8, 100)
	setTrips(t, &seed, "2022-09-19", 8, 8, 300)
	setTrips(t, &seed, "2022-09-12", 8, 32, 50) // the second Monday stays 0 and must count
	setTrips(t, &seed, "2022-09-12", 8, 76, 10)
	setTrips(t, &seed, "2022-09-19", 8, 76, 10)

	got := rank(t, openSeed(t, seed), 1, 8) // weekday 1: 2022-09-12 was a Monday

	want := []Candidate{
		{Rank: 1, AreaID: 8, SampleDays: 2, MeanTrips: 200, MinTrips: 100, MaxTrips: 300, CityShare: 200.0 / 235},
		{Rank: 2, AreaID: 32, SampleDays: 2, MeanTrips: 25, MinTrips: 0, MaxTrips: 50, CityShare: 25.0 / 235},
		{Rank: 3, AreaID: 76, SampleDays: 2, MeanTrips: 10, MinTrips: 10, MaxTrips: 10, CityShare: 10.0 / 235},
	}
	if len(got) != len(want) {
		t.Fatalf("want %d candidates, got %d: %+v", len(want), len(got), got)
	}
	for i, w := range want {
		g := got[i]
		if g.Rank != w.Rank || g.AreaID != w.AreaID || g.SampleDays != w.SampleDays ||
			g.MinTrips != w.MinTrips || g.MaxTrips != w.MaxTrips ||
			math.Abs(g.MeanTrips-w.MeanTrips) > 1e-9 || math.Abs(g.CityShare-w.CityShare) > 1e-9 {
			t.Errorf("candidate %d:\n got %+v\nwant %+v", i+1, g, w)
		}
	}
}

// Ranking is by volume: a ratio would put a 9-trip area above the busiest ones.
func TestRankKeepsLowVolumeSpikeBelowHighVolumeArea(t *testing.T) {
	seed := testSeed(1)
	setTrips(t, &seed, "2022-09-15", 14, 55, 9) // that area's only pickups all Thursday
	for hour := 0; hour < 24; hour++ {
		setTrips(t, &seed, "2022-09-15", hour, 8, 1500) // flat and busy
	}

	got := areaIDs(rank(t, openSeed(t, seed), 4, 14))

	if len(got) != 2 || got[0] != 8 || got[1] != 55 {
		t.Fatalf("want the 1,500-trip area ahead of the 9-trip area, got %v", got)
	}
}

func TestRankFiltersSlotAndBreaksTiesByAreaID(t *testing.T) {
	seed := testSeed(1)
	setTrips(t, &seed, "2022-09-12", 8, 28, 500)
	setTrips(t, &seed, "2022-09-12", 8, 8, 500)  // ties with 28; lower id first
	setTrips(t, &seed, "2022-09-12", 9, 32, 999) // another hour
	setTrips(t, &seed, "2022-09-13", 8, 76, 999) // another weekday
	db := openSeed(t, seed)

	if got := areaIDs(rank(t, db, 1, 8)); len(got) != 2 || got[0] != 8 || got[1] != 28 {
		t.Fatalf("Monday 08: want [8 28], got %v", got)
	}
	if got := areaIDs(rank(t, db, 1, 9)); len(got) != 1 || got[0] != 32 {
		t.Errorf("Monday 09: want [32], got %v", got)
	}
	if got := areaIDs(rank(t, db, 2, 8)); len(got) != 1 || got[0] != 76 {
		t.Errorf("Tuesday 08: want [76], got %v", got)
	}
}

func TestRankKeepsFiveAndSharesCoverEveryArea(t *testing.T) {
	seed := testSeed(1)
	for i, id := range []int{1, 2, 3, 4, 5, 6, 7} {
		setTrips(t, &seed, "2022-09-18", 3, id, 100*(i+1)) // Sunday 03:00; area 7 is busiest
	}

	got := rank(t, openSeed(t, seed), 0, 3) // weekday 0 is Sunday

	if ids := areaIDs(got); fmt.Sprint(ids) != "[7 6 5 4 3]" {
		t.Fatalf("want the five busiest of seven, got %v", ids)
	}
	listed := 0.0
	for _, c := range got {
		listed += c.CityShare
	}
	// Areas 1 and 2 are cut by the limit but still belong in the denominator;
	// the 70 areas with no pickups do not appear at all.
	if want := 2500.0 / 2800; math.Abs(listed-want) > 1e-9 {
		t.Errorf("listed areas hold %.6f of the slot, want %.6f", listed, want)
	}
}

func TestOpenDBRejectsADatabaseThatNoLongerMatchesTheSeed(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "reused.db")
	seed := testSeed(1)
	setTrips(t, &seed, "2022-09-12", 8, 8, 100)

	db, err := OpenDB(ctx, path, seed)
	if err != nil {
		t.Fatalf("first open: %v", err)
	}
	db.Close()
	db, err = OpenDB(ctx, path, seed)
	if err != nil {
		t.Fatalf("reopening an untouched DB must work: %v", err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE pickup_hourly SET trips = trips + 1 WHERE area_id = 8 AND service_date = '2022-09-12' AND hour = 8`); err != nil {
		t.Fatal(err)
	}
	db.Close()

	if _, err := OpenDB(ctx, path, seed); err == nil {
		t.Error("edited rows with unchanged metadata must be rejected")
	}
	other := testSeed(1)
	setTrips(t, &other, "2022-09-12", 8, 8, 999)
	if _, err := OpenDB(ctx, path, other); err == nil {
		t.Error("a DB built from another seed must be rejected")
	}
}

func TestRankReturnsNothingForAnEmptySlot(t *testing.T) {
	got := rank(t, openSeed(t, testSeed(1)), 3, 14)
	if got == nil || len(got) != 0 {
		t.Fatalf("want an empty, non-nil list, got %#v", got)
	}
}

func TestValidateSeedRejectsIncompleteOrInconsistentExtracts(t *testing.T) {
	if err := validateSeed(testSeed(1)); err != nil {
		t.Fatalf("clean seed rejected: %v", err)
	}
	cases := map[string]func(*Seed){
		"negative count":      func(s *Seed) { s.Hours[0].Trips[0] = -1 },
		"short count row":     func(s *Seed) { s.Hours[0].Trips = s.Hours[0].Trips[:76] },
		"duplicate hour":      func(s *Seed) { s.Hours[1] = s.Hours[0] },
		"missing hour":        func(s *Seed) { s.Hours = s.Hours[1:] },
		"partial week":        func(s *Seed) { s.History.PeriodEnd = "2022-09-17" },
		"total mismatch":      func(s *Seed) { s.History.TripsIncluded = 1 },
		"row count mismatch":  func(s *Seed) { s.History.RowsLoaded-- },
		"date outside period": func(s *Seed) { s.Hours[0].Date = "2022-09-11" },
	}
	for name, corrupt := range cases {
		t.Run(name, func(t *testing.T) {
			seed := testSeed(1)
			corrupt(&seed)
			if err := validateSeed(seed); err == nil {
				t.Fatal("want an error")
			}
		})
	}
}
