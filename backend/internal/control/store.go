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
	"time"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaSQL string

//go:embed ranking.sql
var rankingSQL string

const areaCount = 77

// LoadSeed reads and validates a committed seed JSON file.
func LoadSeed(path string) (Seed, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Seed{}, fmt.Errorf("read seed: %w", err)
	}
	return ParseSeed(data)
}

// ParseSeed decodes and validates seed JSON.
func ParseSeed(data []byte) (Seed, error) {
	var seed Seed
	if err := json.Unmarshal(data, &seed); err != nil {
		return Seed{}, fmt.Errorf("decode seed: %w", err)
	}
	if err := validateSeed(seed); err != nil {
		return Seed{}, fmt.Errorf("invalid seed: %w", err)
	}
	return seed, nil
}

// validateSeed rejects anything but a dense extract: 77 areas and exactly one
// row of 77 counts for every hour of every day in a whole number of weeks.
func validateSeed(s Seed) error {
	if len(s.Areas) != areaCount {
		return fmt.Errorf("want %d areas, got %d", areaCount, len(s.Areas))
	}
	seenArea := map[int]bool{}
	for _, a := range s.Areas {
		if a.AreaID < 1 || a.AreaID > areaCount {
			return fmt.Errorf("area_id %d out of range", a.AreaID)
		}
		if seenArea[a.AreaID] {
			return fmt.Errorf("duplicate area_id %d", a.AreaID)
		}
		seenArea[a.AreaID] = true
		if a.Name == "" {
			return fmt.Errorf("area %d has empty name", a.AreaID)
		}
		if a.Lat < 41.6 || a.Lat > 42.1 || a.Lon < -88.0 || a.Lon > -87.4 {
			return fmt.Errorf("area %d coords out of Chicago bounds", a.AreaID)
		}
	}

	start, errStart := time.Parse(time.DateOnly, s.History.PeriodStart)
	end, errEnd := time.Parse(time.DateOnly, s.History.PeriodEnd)
	if errStart != nil || errEnd != nil {
		return fmt.Errorf("period %q..%q is not a pair of ISO dates", s.History.PeriodStart, s.History.PeriodEnd)
	}
	days := int(end.Sub(start).Hours()/24) + 1
	if days <= 0 || days%7 != 0 {
		return fmt.Errorf("period must cover whole weeks, got %d days", days)
	}
	if len(s.Hours) != days*24 {
		return fmt.Errorf("want %d hourly rows, got %d", days*24, len(s.Hours))
	}
	seenHour := map[string]bool{}
	sum := 0
	for _, h := range s.Hours {
		day, err := time.Parse(time.DateOnly, h.Date)
		if err != nil || day.Before(start) || day.After(end) {
			return fmt.Errorf("hourly row date %q is outside the period", h.Date)
		}
		if h.Hour < 0 || h.Hour > 23 {
			return fmt.Errorf("hourly row %s has hour %d", h.Date, h.Hour)
		}
		key := fmt.Sprintf("%sT%02d", h.Date, h.Hour)
		if seenHour[key] {
			return fmt.Errorf("duplicate hourly row %s", key)
		}
		seenHour[key] = true
		if len(h.Trips) != areaCount {
			return fmt.Errorf("hourly row %s has %d counts, want %d", key, len(h.Trips), areaCount)
		}
		for i, trips := range h.Trips {
			if trips < 0 {
				return fmt.Errorf("negative trips for area %d at %s", i+1, key)
			}
			sum += trips
		}
	}
	if sum != s.History.TripsIncluded {
		return fmt.Errorf("hourly counts sum to %d, history.trips_included is %d", sum, s.History.TripsIncluded)
	}
	if want := days * 24 * areaCount; s.History.RowsLoaded != want {
		return fmt.Errorf("history.rows_loaded is %d, want %d", s.History.RowsLoaded, want)
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
	hourStmt, err := tx.PrepareContext(ctx, `INSERT INTO pickup_hourly (area_id, service_date, hour, trips) VALUES (?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer hourStmt.Close()
	for _, h := range seed.Hours {
		for i, trips := range h.Trips {
			if _, err := hourStmt.ExecContext(ctx, i+1, h.Date, h.Hour, trips); err != nil {
				return fmt.Errorf("insert %s hour %d area %d: %w", h.Date, h.Hour, i+1, err)
			}
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
			return errors.New("existing DB has schema but no import metadata; rerun `make setup` to rebuild it")
		}
		return fmt.Errorf("read import metadata: %w", err)
	}
	s := seed.History
	var diffs []string
	if h.DatasetID != s.DatasetID {
		diffs = append(diffs, fmt.Sprintf("dataset_id: %q vs %q", h.DatasetID, s.DatasetID))
	}
	if h.PeriodStart != s.PeriodStart || h.PeriodEnd != s.PeriodEnd {
		diffs = append(diffs, fmt.Sprintf("period: %s..%s vs %s..%s", h.PeriodStart, h.PeriodEnd, s.PeriodStart, s.PeriodEnd))
	}
	if h.TripsIncluded != s.TripsIncluded {
		diffs = append(diffs, fmt.Sprintf("trips_included: %d vs %d", h.TripsIncluded, s.TripsIncluded))
	}
	if h.RowsLoaded != s.RowsLoaded {
		diffs = append(diffs, fmt.Sprintf("rows_loaded: %d vs %d", h.RowsLoaded, s.RowsLoaded))
	}
	if len(diffs) > 0 {
		return fmt.Errorf(
			"existing DB was built from a different seed (%s); rerun `make setup` to rebuild it",
			strings.Join(diffs, "; "),
		)
	}
	// Matching metadata is not enough: the rows themselves must still add up.
	var rows, trips int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*), COALESCE(SUM(trips), 0) FROM pickup_hourly`).Scan(&rows, &trips); err != nil {
		return fmt.Errorf("read pickup_hourly: %w; rerun `make setup` to rebuild it", err)
	}
	if rows != s.RowsLoaded || trips != s.TripsIncluded {
		return fmt.Errorf(
			"existing DB holds %d rows and %d trips, the seed has %d and %d; rerun `make setup` to rebuild it",
			rows, trips, s.RowsLoaded, s.TripsIncluded,
		)
	}
	return nil
}

// splitSQL splits on every semicolon, so schema.sql must keep them out of
// literals and comments.
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

// RankCandidates returns up to five areas for a weekday (0 = Sunday) and hour,
// ordered by mean historical pickups, then area_id.
func RankCandidates(ctx context.Context, db *sql.DB, dow, hour int) ([]Candidate, error) {
	rows, err := db.QueryContext(ctx, rankingSQL, sql.Named("dow", dow), sql.Named("hour", hour))
	if err != nil {
		return nil, fmt.Errorf("ranking query: %w", err)
	}
	defer rows.Close()
	out := []Candidate{}
	for rows.Next() {
		c := Candidate{Rank: len(out) + 1, Weather: WeatherEvidence{State: WeatherMissing}}
		if err := rows.Scan(&c.AreaID, &c.Name, &c.Lat, &c.Lon, &c.SampleDays, &c.MeanTrips, &c.MinTrips, &c.MaxTrips, &c.CityShare); err != nil {
			return nil, fmt.Errorf("scan candidate: %w", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate candidates: %w", err)
	}
	return out, nil
}
