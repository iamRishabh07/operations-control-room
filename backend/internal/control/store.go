package control

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaSQL string

//go:embed ranking.sql
var rankingSQL string

// LoadSeed parses a committed seed JSON file.
func LoadSeed(path string) (Seed, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Seed{}, fmt.Errorf("read seed: %w", err)
	}
	var seed Seed
	if err := json.Unmarshal(data, &seed); err != nil {
		return Seed{}, fmt.Errorf("decode seed: %w", err)
	}
	if err := validateSeed(seed); err != nil {
		return Seed{}, fmt.Errorf("invalid seed: %w", err)
	}
	return seed, nil
}

func validateSeed(s Seed) error {
	if len(s.Areas) != 77 {
		return fmt.Errorf("want 77 areas, got %d", len(s.Areas))
	}
	seen := map[int]bool{}
	for _, a := range s.Areas {
		if a.AreaID < 1 || a.AreaID > 77 {
			return fmt.Errorf("area_id %d out of range", a.AreaID)
		}
		if seen[a.AreaID] {
			return fmt.Errorf("duplicate area_id %d", a.AreaID)
		}
		seen[a.AreaID] = true
		if a.Name == "" {
			return fmt.Errorf("area %d has empty name", a.AreaID)
		}
		if a.Lat < 41.6 || a.Lat > 42.1 || a.Lon < -88.0 || a.Lon > -87.4 {
			return fmt.Errorf("area %d coords out of Chicago bounds", a.AreaID)
		}
	}
	const wantBuckets = 77 * 7 * 24
	if len(s.Buckets) != wantBuckets {
		return fmt.Errorf("want %d buckets, got %d", wantBuckets, len(s.Buckets))
	}
	bucketSeen := map[[3]int]bool{}
	sum := 0
	for _, b := range s.Buckets {
		k := [3]int{b.AreaID, b.Dow, b.Hour}
		if bucketSeen[k] {
			return fmt.Errorf("duplicate bucket %v", k)
		}
		if b.AreaID < 1 || b.AreaID > 77 || b.Dow < 0 || b.Dow > 6 || b.Hour < 0 || b.Hour > 23 {
			return fmt.Errorf("bucket out of range: %v", k)
		}
		if b.TotalTrips < 0 {
			return fmt.Errorf("negative trips at %v", k)
		}
		bucketSeen[k] = true
		sum += b.TotalTrips
	}
	if s.History.TripsIncluded > 0 && sum != s.History.TripsIncluded {
		return fmt.Errorf("sum %d != history.trips_included %d", sum, s.History.TripsIncluded)
	}
	return nil
}

// OpenDB opens a SQLite file and initializes it from the seed if empty.
// A non-empty database is reused only when its import metadata matches the seed.
func OpenDB(ctx context.Context, dbPath string, seed Seed) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dbPath+"?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if err := initIfEmpty(ctx, db, seed); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func initIfEmpty(ctx context.Context, db *sql.DB, seed Seed) error {
	var count int
	err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='areas'`).Scan(&count)
	if err != nil {
		return fmt.Errorf("probe schema: %w", err)
	}
	if count == 0 {
		return initFresh(ctx, db, seed)
	}
	return verifyProvenance(ctx, db, seed)
}

func initFresh(ctx context.Context, db *sql.DB, seed Seed) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, stmt := range splitSQL(schemaSQL) {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("schema stmt failed: %w", err)
		}
	}
	areaStmt, err := tx.PrepareContext(ctx, `INSERT INTO areas (area_id, name, lat, lon, coord_source) VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer areaStmt.Close()
	for _, a := range seed.Areas {
		if _, err := areaStmt.ExecContext(ctx, a.AreaID, a.Name, a.Lat, a.Lon, a.CoordSource); err != nil {
			return fmt.Errorf("insert area %d: %w", a.AreaID, err)
		}
	}
	bucketStmt, err := tx.PrepareContext(ctx, `INSERT INTO pickup_baseline (area_id, dow, hour, total_trips) VALUES (?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer bucketStmt.Close()
	for _, b := range seed.Buckets {
		if _, err := bucketStmt.ExecContext(ctx, b.AreaID, b.Dow, b.Hour, b.TotalTrips); err != nil {
			return fmt.Errorf("insert bucket %d/%d/%d: %w", b.AreaID, b.Dow, b.Hour, err)
		}
	}
	h := seed.History
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO dataset_imports (dataset_id, period_start, period_end, retrieved_at, query_text, rows_loaded, trips_included, trips_excluded_invalid_area) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		h.DatasetID, h.PeriodStart, h.PeriodEnd, h.RetrievedAt, h.QueryText, h.RowsLoaded, h.TripsIncluded, h.TripsExcludedInvalidArea,
	); err != nil {
		return fmt.Errorf("insert import record: %w", err)
	}
	return tx.Commit()
}

func verifyProvenance(ctx context.Context, db *sql.DB, seed Seed) error {
	var h ImportRecord
	err := db.QueryRowContext(ctx, `SELECT dataset_id, period_start, period_end, retrieved_at, rows_loaded, trips_included, trips_excluded_invalid_area FROM dataset_imports ORDER BY id DESC LIMIT 1`).Scan(
		&h.DatasetID, &h.PeriodStart, &h.PeriodEnd, &h.RetrievedAt, &h.RowsLoaded, &h.TripsIncluded, &h.TripsExcludedInvalidArea,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errors.New("existing DB has schema but no import metadata; choose a new DB path or remove the file")
		}
		return fmt.Errorf("read import metadata: %w", err)
	}
	s := seed.History
	if h.DatasetID != s.DatasetID || h.PeriodStart != s.PeriodStart || h.PeriodEnd != s.PeriodEnd || h.TripsIncluded != s.TripsIncluded || h.RowsLoaded != s.RowsLoaded {
		return fmt.Errorf("existing DB provenance mismatches seed (dataset=%s/%s period=%s/%s); choose a new DB path or remove the file",
			h.DatasetID, s.DatasetID, h.PeriodStart, s.PeriodStart)
	}
	return nil
}

func splitSQL(blob string) []string {
	parts := strings.Split(blob, ";")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			out = append(out, p)
		}
	}
	return out
}

// RankCandidates returns up to 5 historically positive areas for (dow, hour),
// ordered by total pickups desc then area_id asc.
func RankCandidates(ctx context.Context, db *sql.DB, history ImportRecord, dow, hour int) ([]Candidate, error) {
	rows, err := db.QueryContext(ctx, rankingSQL,
		sql.Named("period_start", history.PeriodStart),
		sql.Named("period_end", history.PeriodEnd),
		sql.Named("dow", dow),
		sql.Named("hour", hour),
	)
	if err != nil {
		return nil, fmt.Errorf("ranking query: %w", err)
	}
	defer rows.Close()
	var out []Candidate
	rank := 1
	for rows.Next() {
		var c Candidate
		if err := rows.Scan(&c.AreaID, &c.Name, &c.Lat, &c.Lon, &c.TotalTrips, &c.SampleDays, &c.MeanPickupsPerDay); err != nil {
			return nil, fmt.Errorf("scan candidate: %w", err)
		}
		c.Rank = rank
		rank++
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate candidates: %w", err)
	}
	return out, nil
}
