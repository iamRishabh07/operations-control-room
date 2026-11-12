package control

import (
	"context"
	"path/filepath"
	"testing"
)

func testSeed() Seed {
	areas := make([]Area, 77)
	for i := 0; i < 77; i++ {
		areas[i] = Area{
			AreaID:      i + 1,
			Name:        "Area " + itoa(i+1),
			Lat:         41.8,
			Lon:         -87.7,
			CoordSource: "test",
		}
	}
	buckets := make([]Bucket, 0, 77*7*24)
	for a := 1; a <= 77; a++ {
		for d := 0; d < 7; d++ {
			for h := 0; h < 24; h++ {
				buckets = append(buckets, Bucket{AreaID: a, Dow: d, Hour: h, TotalTrips: 0})
			}
		}
	}
	return Seed{
		Areas:   areas,
		Buckets: buckets,
		History: ImportRecord{
			DatasetID:     "test",
			PeriodStart:   "2022-01-01",
			PeriodEnd:     "2022-12-31",
			RetrievedAt:   "2026-11-12T00:00:00Z",
			QueryText:     "test",
			RowsLoaded:    77 * 7 * 24,
			TripsIncluded: 0,
		},
	}
}

func itoa(i int) string {
	buf := make([]byte, 0, 3)
	if i == 0 {
		return "0"
	}
	n := i
	for n > 0 {
		buf = append([]byte{byte('0' + n%10)}, buf...)
		n /= 10
	}
	return string(buf)
}

func setBucket(s *Seed, areaID, dow, hour, trips int) {
	for i := range s.Buckets {
		b := &s.Buckets[i]
		if b.AreaID == areaID && b.Dow == dow && b.Hour == hour {
			b.TotalTrips = trips
			return
		}
	}
}

func openTestDB(t *testing.T, seed Seed) *testing.T {
	t.Helper()
	// no-op placeholder; see withDB.
	return t
}

func withDB(t *testing.T, seed Seed, fn func(ctx context.Context)) {
	t.Helper()
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := OpenDB(ctx, dbPath, seed)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	seed.History.TripsIncluded = sumBuckets(seed.Buckets)
	// Reopen with the real included total.
	db.Close()
	db2, err := OpenDB(ctx, dbPath, seed)
	if err != nil {
		t.Fatalf("reopen db: %v", err)
	}
	defer db2.Close()
	// RankCandidates is called by the fn via a closure capturing db2 — rebind
	// through a package-scoped test helper by reassigning testCtxDB.
	testCtxDB = db2
	defer func() { testCtxDB = nil }()
	fn(ctx)
}

var testCtxDB any // set by withDB; consumed by TestRankCandidates

func sumBuckets(bs []Bucket) int {
	s := 0
	for _, b := range bs {
		s += b.TotalTrips
	}
	return s
}

func TestRankCandidatesBasic(t *testing.T) {
	seed := testSeed()
	// Monday 08:00 — three positive areas with different totals.
	setBucket(&seed, 8, 1, 8, 500)   // Near North (highest)
	setBucket(&seed, 32, 1, 8, 1200) // Loop
	setBucket(&seed, 28, 1, 8, 500)  // Near West — ties with 8, higher ID
	setBucket(&seed, 1, 1, 8, 100)   // small
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "rank.db")
	seed.History.TripsIncluded = sumBuckets(seed.Buckets)
	db, err := OpenDB(ctx, dbPath, seed)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	cands, err := RankCandidates(ctx, db, seed.History, 1, 8)
	if err != nil {
		t.Fatalf("rank: %v", err)
	}
	if len(cands) != 4 {
		t.Fatalf("want 4 candidates, got %d", len(cands))
	}
	want := []int{32, 8, 28, 1} // 32 highest; 8 and 28 tie at 500, lower area_id first
	for i, c := range cands {
		if c.AreaID != want[i] {
			t.Errorf("rank %d: want area %d, got %d", i+1, want[i], c.AreaID)
		}
		if c.Rank != i+1 {
			t.Errorf("rank %d: Rank field wrong: %d", i+1, c.Rank)
		}
	}
	if cands[0].SampleDays <= 0 {
		t.Errorf("sample_days should be positive, got %d", cands[0].SampleDays)
	}
}

func TestRankCandidatesEmpty(t *testing.T) {
	seed := testSeed()
	seed.History.TripsIncluded = 0
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "empty.db")
	db, err := OpenDB(ctx, dbPath, seed)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	cands, err := RankCandidates(ctx, db, seed.History, 3, 14)
	if err != nil {
		t.Fatalf("rank: %v", err)
	}
	if len(cands) != 0 {
		t.Errorf("want 0 candidates for all-zero seed, got %d", len(cands))
	}
}

func TestRankCandidatesWrongHourExcluded(t *testing.T) {
	seed := testSeed()
	setBucket(&seed, 8, 1, 9, 999) // Monday 09:00 — should NOT appear in query for 08:00
	setBucket(&seed, 32, 1, 8, 100)
	seed.History.TripsIncluded = sumBuckets(seed.Buckets)
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "hour.db")
	db, err := OpenDB(ctx, dbPath, seed)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	cands, err := RankCandidates(ctx, db, seed.History, 1, 8)
	if err != nil {
		t.Fatalf("rank: %v", err)
	}
	if len(cands) != 1 || cands[0].AreaID != 32 {
		t.Fatalf("expected only area 32 for Monday 08:00, got %+v", cands)
	}
}

func TestSchemaRejectsNegative(t *testing.T) {
	seed := testSeed()
	seed.Buckets[0].TotalTrips = -1
	err := validateSeed(seed)
	if err == nil {
		t.Fatal("expected validateSeed to reject negative trips")
	}
}
