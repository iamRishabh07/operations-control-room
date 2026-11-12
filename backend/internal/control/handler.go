package control

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

// Service wires together the DB, weather client and clock for one HTTP handler.
type Service struct {
	DB      *sql.DB
	Seed    Seed
	Weather *WeatherClient
	Now     func() time.Time
	Loc     *time.Location
}

// Handler returns an http.Handler for GET /api/recommendation.
func (s *Service) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		assessedAt := s.Now().UTC()
		local := assessedAt.In(s.Loc)
		hourStart := time.Date(local.Year(), local.Month(), local.Day(), local.Hour(), 0, 0, 0, s.Loc).UTC()
		nextHour := hourStart.Add(time.Hour)
		dow := int(local.Weekday())
		hour := local.Hour()

		candidates, err := RankCandidates(r.Context(), s.DB, s.Seed.History, dow, hour)
		if err != nil {
			log.Printf("ranking failed: %v", err)
			writeServerError(w)
			return
		}

		evidence, weatherErr := s.Weather.Fetch(r.Context(), assessedAt, candidates)
		fetchedAt := s.Now().UTC()

		upstream := ""
		if weatherErr != nil {
			log.Printf("weather upstream failure: %v", weatherErr)
			upstream = "modeled weather unavailable"
			for i := range candidates {
				candidates[i].Weather = WeatherEvidence{State: WeatherInvalid, Reason: upstream}
			}
		} else {
			for i := range evidence {
				candidates[i].Weather = evidence[i]
			}
		}

		status, reason, winner, validUntil := Decide(hourStart, nextHour, candidates, upstream)

		assessment := Assessment{
			Status:      status,
			Reason:      reason,
			AssessedAt:  hourStart,
			ValidUntil:  validUntil,
			Candidates:  candidates,
			History:     &s.Seed.History,
			Weather:     WeatherSource{Source: openMeteoForecastURL},
			Limitations: s.limitations(),
		}
		if weatherErr == nil {
			assessment.Weather.FetchedAt = &fetchedAt
		}
		if winner != nil {
			assessment.Area = &AreaRef{AreaID: winner.AreaID, Name: winner.Name}
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		if err := json.NewEncoder(w).Encode(assessment); err != nil {
			log.Printf("encode response: %v", err)
		}
	})
}

func (s *Service) limitations() []string {
	return []string{
		"Historical baseline reflects " + s.Seed.History.PeriodStart + " to " + s.Seed.History.PeriodEnd + "; areas chosen by raw pickup volume, not current demand.",
		"Modeled precipitation is a forecast, not an observation; it may include snow or mixed precipitation.",
		"Representative coordinates are community-area bounding-box centers; small positional error is expected.",
		"Watch duration (1 hour) is a suggested operator review window; the assessment itself expires per valid_until.",
	}
}

func writeServerError(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusServiceUnavailable)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error":   "assessment_unavailable",
		"message": "The assessment could not be loaded.",
	})
}
