// Package serverless is the entry point for platforms that call a handler per
// request instead of running cmd/server. It serves the same control.Service:
// same SQL, same decision rule, same weather adapter.
package serverless

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "time/tzdata"

	"github.com/iamRishabh07/operations-control-room/backend/internal/control"
	"github.com/iamRishabh07/operations-control-room/data"
)

var (
	mu      sync.Mutex
	handler http.Handler

	// Where Handler keeps its database and reads weather. The temp directory
	// is the only writable place on such platforms; an empty weather URL
	// means Open-Meteo. Tests replace both.
	dbPath     = filepath.Join(os.TempDir(), "control.db")
	weatherURL = ""
)

// Handler answers GET /api/recommendation. The service is built on the first
// request an instance receives and reused for the rest of its life.
func Handler(w http.ResponseWriter, r *http.Request) {
	h, err := ready()
	if err != nil {
		log.Printf("startup failed: %v", err)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprint(w, `{"error":"assessment_unavailable","message":"The assessment could not be loaded."}`)
		return
	}
	h.ServeHTTP(w, r)
}

// ready returns the service, building it if this instance has none yet. A
// failed start is not remembered: the next request tries again.
func ready() (http.Handler, error) {
	mu.Lock()
	defer mu.Unlock()
	if handler == nil {
		h, err := newHandler(dbPath, weatherURL)
		if err != nil {
			return nil, err
		}
		handler = h
	}
	return handler, nil
}

// newHandler builds the service from the embedded seed, with its SQLite file
// at dbPath. The file is rebuilt whenever an instance starts without one.
func newHandler(dbPath, weatherURL string) (http.Handler, error) {
	loc, err := time.LoadLocation("America/Chicago")
	if err != nil {
		return nil, fmt.Errorf("load timezone: %w", err)
	}
	seed, err := control.ParseSeed(data.Seed)
	if err != nil {
		return nil, err
	}
	open := func() (*sql.DB, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		return control.OpenDB(ctx, dbPath, seed)
	}
	db, err := open()
	if err != nil {
		// A file left half-built by an instance that was stopped mid-start is
		// only a copy of the seed: discard it and build again, once.
		for _, suffix := range []string{"", "-wal", "-shm"} {
			os.Remove(dbPath + suffix)
		}
		if db, err = open(); err != nil {
			return nil, err
		}
	}
	svc := &control.Service{
		DB:      db,
		Seed:    seed,
		Weather: &control.WeatherClient{HTTP: &http.Client{Timeout: 3 * time.Second}, BaseURL: weatherURL},
		Now:     func() time.Time { return time.Now().UTC() },
		Loc:     loc,
	}
	return svc.Handler(), nil
}
