package serverless

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/iamRishabh07/operations-control-room/backend/internal/control"
)

// dryWeather answers like Open-Meteo with 0 mm at every requested point.
func dryWeather(w http.ResponseWriter, r *http.Request) {
	reading := fmt.Sprintf(`{"current":{"time":%d,"interval":900,"precipitation":0},"current_units":{"precipitation":"mm"}}`, time.Now().Unix()-60)
	n := strings.Count(r.URL.Query().Get("latitude"), ",") + 1
	fmt.Fprint(w, "["+strings.TrimSuffix(strings.Repeat(reading+",", n), ",")+"]")
}

func assess(t *testing.T, h http.Handler) control.Assessment {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/recommendation", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var a control.Assessment
	if err := json.Unmarshal(rec.Body.Bytes(), &a); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return a
}

// The serverless entry point must give the same kind of answer as the local
// server, from the seed compiled into the binary and nothing else.
func TestHandlerAnswersFromTheEmbeddedSeed(t *testing.T) {
	weather := httptest.NewServer(http.HandlerFunc(dryWeather))
	t.Cleanup(weather.Close)

	h, err := newHandler(filepath.Join(t.TempDir(), "control.db"), weather.URL)
	if err != nil {
		t.Fatal(err)
	}
	a := assess(t, h)

	if a.Status != control.StatusNoAction || len(a.Candidates) != 5 || a.ValidUntil == nil {
		t.Fatalf("want no_action over five candidates with an expiry, got %s, %d candidates, until %v", a.Status, len(a.Candidates), a.ValidUntil)
	}
	if a.History == nil || a.History.DatasetID != "m6dm-c72p" || a.History.RowsLoaded != 51744 {
		t.Errorf("want the committed seed's provenance, got %+v", a.History)
	}
	if !a.Weather.Simulated {
		t.Error("a weather source other than Open-Meteo must be labelled simulated")
	}
}

// A start that fails must answer 503 and must not be remembered: the same
// instance has to work once the cause is gone.
func TestHandlerRecoversAfterAFailedStart(t *testing.T) {
	weather := httptest.NewServer(http.HandlerFunc(dryWeather))
	t.Cleanup(weather.Close)
	savedPath, savedURL := dbPath, weatherURL
	t.Cleanup(func() { dbPath, weatherURL, handler = savedPath, savedURL, nil })
	weatherURL = weather.URL

	dbPath = filepath.Join(t.TempDir(), "no-such-directory", "control.db")
	rec := httptest.NewRecorder()
	Handler(rec, httptest.NewRequest(http.MethodGet, "/api/recommendation", nil))
	if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), "assessment_unavailable") {
		t.Fatalf("want 503 assessment_unavailable while the database cannot be created, got %d: %s", rec.Code, rec.Body.String())
	}

	dbPath = filepath.Join(t.TempDir(), "control.db")
	if a := assess(t, http.HandlerFunc(Handler)); a.Status != control.StatusNoAction {
		t.Fatalf("want no_action once the database can be created, got %s (%s)", a.Status, a.Reason)
	}
}

// An instance stopped mid-start can leave a broken database file behind. It is
// only a copy of the seed, so the next start replaces it instead of failing.
func TestHandlerReplacesABrokenDatabaseFile(t *testing.T) {
	weather := httptest.NewServer(http.HandlerFunc(dryWeather))
	t.Cleanup(weather.Close)
	path := filepath.Join(t.TempDir(), "control.db")
	if err := os.WriteFile(path, []byte("not a database"), 0o600); err != nil {
		t.Fatal(err)
	}

	h, err := newHandler(path, weather.URL)
	if err != nil {
		t.Fatal(err)
	}
	if a := assess(t, h); a.Status != control.StatusNoAction {
		t.Fatalf("want no_action, got %s (%s)", a.Status, a.Reason)
	}
}
