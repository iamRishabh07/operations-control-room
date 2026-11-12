package control

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func mkClient(url string) *WeatherClient {
	return &WeatherClient{HTTP: &http.Client{Timeout: 2 * time.Second}, BaseURL: url}
}

func TestWeather_ArrayResponseValid(t *testing.T) {
	assessedAt := time.Unix(1_763_000_000, 0).UTC()
	ts := assessedAt.Unix() - 300 // 5 minutes old
	body := `[{"current":{"time":` + itoa64(ts) + `,"interval":900,"precipitation":0.4},"current_units":{"precipitation":"mm"}},
	          {"current":{"time":` + itoa64(ts) + `,"interval":900,"precipitation":0.0},"current_units":{"precipitation":"mm"}}]`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(body))
	}))
	defer srv.Close()
	wc := mkClient(srv.URL)
	cands := []Candidate{{AreaID: 32, Lat: 41.88, Lon: -87.63}, {AreaID: 8, Lat: 41.90, Lon: -87.63}}
	evs, err := wc.Fetch(context.Background(), assessedAt, cands)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(evs) != 2 || evs[0].State != WeatherValid || evs[1].State != WeatherValid {
		t.Fatalf("want 2 valid, got %+v", evs)
	}
	if *evs[0].PrecipitationMM != 0.4 {
		t.Errorf("value mismatch: %v", evs[0].PrecipitationMM)
	}
}

func TestWeather_SingleObjectResponse(t *testing.T) {
	assessedAt := time.Unix(1_763_000_000, 0).UTC()
	ts := assessedAt.Unix() - 60
	body := `{"current":{"time":` + itoa64(ts) + `,"interval":900,"precipitation":0.1},"current_units":{"precipitation":"mm"}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(body)) }))
	defer srv.Close()
	wc := mkClient(srv.URL)
	cands := []Candidate{{AreaID: 32, Lat: 41.88, Lon: -87.63}}
	evs, err := wc.Fetch(context.Background(), assessedAt, cands)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(evs) != 1 || evs[0].State != WeatherValid {
		t.Fatalf("want 1 valid, got %+v", evs)
	}
}

func TestWeather_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	wc := mkClient(srv.URL)
	cands := []Candidate{{AreaID: 1, Lat: 41.8, Lon: -87.7}}
	_, err := wc.Fetch(context.Background(), time.Now().UTC(), cands)
	if err == nil {
		t.Fatal("want error on 500")
	}
}

func TestWeather_StaleReadingInvalid(t *testing.T) {
	assessedAt := time.Unix(1_763_000_000, 0).UTC()
	ts := assessedAt.Unix() - 3600 // 1 hour old, past 30m freshness
	body := `{"current":{"time":` + itoa64(ts) + `,"interval":900,"precipitation":0.0},"current_units":{"precipitation":"mm"}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(body)) }))
	defer srv.Close()
	wc := mkClient(srv.URL)
	evs, err := wc.Fetch(context.Background(), assessedAt, []Candidate{{AreaID: 1, Lat: 41.8, Lon: -87.7}})
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if evs[0].State != WeatherInvalid || evs[0].PrecipitationMM != nil {
		t.Fatalf("want invalid stale reading with null precipitation, got %+v", evs[0])
	}
}

func TestWeather_WrongUnitsInvalid(t *testing.T) {
	assessedAt := time.Unix(1_763_000_000, 0).UTC()
	ts := assessedAt.Unix() - 60
	body := `{"current":{"time":` + itoa64(ts) + `,"interval":900,"precipitation":0.5},"current_units":{"precipitation":"inch"}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(body)) }))
	defer srv.Close()
	wc := mkClient(srv.URL)
	evs, err := wc.Fetch(context.Background(), assessedAt, []Candidate{{AreaID: 1, Lat: 41.8, Lon: -87.7}})
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if evs[0].State != WeatherInvalid {
		t.Fatalf("want invalid for wrong units, got %+v", evs[0])
	}
}

func TestWeather_CountMismatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[{"current":{"time":1,"interval":900,"precipitation":0},"current_units":{"precipitation":"mm"}}]`))
	}))
	defer srv.Close()
	wc := mkClient(srv.URL)
	_, err := wc.Fetch(context.Background(), time.Now().UTC(), []Candidate{{AreaID: 1, Lat: 41.8, Lon: -87.7}, {AreaID: 2, Lat: 41.8, Lon: -87.7}})
	if err == nil {
		t.Fatal("want error on count mismatch")
	}
}

func itoa64(i int64) string {
	buf := make([]byte, 0, 20)
	if i < 0 {
		return "-" + itoa64(-i)
	}
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
