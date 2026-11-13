// Command server runs the Operations Control Room HTTP API and serves the
// built React frontend from ../../frontend/dist.
//
// Startup loads a committed seed, initializes or verifies a local SQLite DB,
// then listens on the configured address. All historical data comes from the
// seed; no Chicago data portal traffic happens on the request path.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	_ "time/tzdata"

	"github.com/iamRishabh07/operations-control-room/backend/internal/control"
)

func main() {
	var (
		addr       = flag.String("addr", "127.0.0.1:8080", "listen address")
		dbPath     = flag.String("db", "data/control.db", "SQLite file path")
		seedPath   = flag.String("seed", "data/seed.json", "committed seed JSON path")
		staticDir  = flag.String("static", "frontend/dist", "built frontend directory; empty disables static serving")
		initOnly   = flag.Bool("init-only", false, "load seed + initialize DB and exit; used by `make setup`")
		weatherURL = flag.String("weather-url", "", "read weather from this endpoint instead of Open-Meteo (demo only; responses are labelled simulated and show this URL, so put no credentials in it)")
	)
	flag.Parse()

	if err := run(*addr, *dbPath, *seedPath, *staticDir, *weatherURL, *initOnly); err != nil {
		log.Fatal(err)
	}
}

func run(addr, dbPath, seedPath, staticDir, weatherURL string, initOnly bool) error {
	loc, err := time.LoadLocation("America/Chicago")
	if err != nil {
		return fmt.Errorf("load timezone: %w", err)
	}

	seed, err := control.LoadSeed(seedPath)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db, err := control.OpenDB(ctx, dbPath, seed)
	if err != nil {
		return err
	}
	if initOnly {
		return db.Close()
	}
	defer db.Close()

	svc := &control.Service{
		DB:   db,
		Seed: seed,
		Weather: &control.WeatherClient{
			HTTP:    &http.Client{Timeout: 3 * time.Second},
			BaseURL: weatherURL,
		},
		Now: func() time.Time { return time.Now().UTC() },
		Loc: loc,
	}

	mux := http.NewServeMux()
	mux.Handle("/api/recommendation", svc.Handler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})
	if staticDir != "" {
		if abs, err := filepath.Abs(staticDir); err == nil {
			if st, err := os.Stat(abs); err == nil && st.IsDir() {
				files := http.FileServer(http.Dir(abs))
				mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
					// Asset names carry a content hash, but the page that names
					// them does not: make the browser revalidate it, or a
					// rebuilt frontend stays hidden behind a cached copy.
					if r.URL.Path == "/" {
						w.Header().Set("Cache-Control", "no-cache")
					}
					files.ServeHTTP(w, r)
				})
			} else {
				log.Printf("static dir %s not present; API-only mode", abs)
			}
		}
	}

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Printf("listening on http://%s", addr)
		errCh <- srv.ListenAndServe()
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	select {
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			return err
		}
	case <-stop:
		log.Printf("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
	return nil
}
