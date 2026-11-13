package control

import (
	"database/sql"
	"encoding/json"
	"fmt"
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
		now := s.Now().UTC()
		local := now.In(s.Loc)
		// Chicago's UTC offset is always a whole number of hours, so truncating
		// the instant gives the local hour start, even in the repeated DST hour.
		hourStart := now.Truncate(time.Hour)

		candidates, err := RankCandidates(r.Context(), s.DB, int(local.Weekday()), local.Hour())
		if err != nil {
			log.Printf("ranking failed: %v", err)
			writeServerError(w)
			return
		}

		// On an upstream failure every candidate keeps its missing weather,
		// which Decide reports as unavailable.
		var fetchedAt *time.Time
		if len(candidates) > 0 {
			evidence, err := s.Weather.Fetch(r.Context(), now, candidates)
			if err != nil {
				log.Printf("weather upstream failure: %v", err)
			} else {
				for i := range evidence {
					candidates[i].Weather = evidence[i]
				}
				at := s.Now().UTC()
				fetchedAt = &at
			}
		}

		d := Decide(now, hourStart.Add(time.Hour), candidates)

		simulated := s.Weather.endpoint() != openMeteoForecastURL
		assessment := Assessment{
			Status:      d.Status,
			ReasonCode:  d.ReasonCode,
			Reason:      d.Reason,
			SlotStart:   hourStart,
			AssessedAt:  now,
			ValidUntil:  d.ValidUntil,
			TriggerMM:   PrecipitationTriggerMM,
			Candidates:  candidates,
			History:     &s.Seed.History,
			Weather:     WeatherSource{Source: s.Weather.endpoint(), Simulated: simulated, FetchedAt: fetchedAt},
			Limitations: s.limitations(simulated, d.ValidUntil != nil),
		}
		if d.Winner != nil {
			assessment.Area = &AreaRef{AreaID: d.Winner.AreaID, Name: d.Winner.Name}
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		if err := json.NewEncoder(w).Encode(assessment); err != nil {
			log.Printf("encode response: %v", err)
		}
	})
}

// limitations lists what the operator should keep in mind. expires says
// whether the assessment carries a valid_until for the last line to refer to.
func (s *Service) limitations(simulated, expires bool) []string {
	h := s.Seed.History
	excluded := 0.0
	if total := h.TripsIncluded + h.TripsExcludedInvalidArea; total > 0 {
		excluded = 100 * float64(h.TripsExcludedInvalidArea) / float64(total)
	}
	weather := "Precipitation is modeled by Open-Meteo for the current 15 minutes at each area's bounding-box centre, not observed. It can include snow."
	if simulated {
		weather = "Precipitation on this page comes from a local simulator for demonstration, not from Open-Meteo."
	}
	out := []string{
		fmt.Sprintf("Pickup counts are historical (Chicago TNP trips, %s to %s), not live demand. The app sees no current rides, drivers, wait times or cancellations.", h.PeriodStart, h.PeriodEnd),
		"Each weekday-hour mean rests on the matching days in that period; the range shows the spread across them.",
		fmt.Sprintf("%.1f%% of trips in the period have no community area (outside Chicago or suppressed for privacy) and are excluded. The publisher rounds start times to 15 minutes.", excluded),
		weather,
		fmt.Sprintf("The %.1f mm trigger is a heuristic, not a validated threshold. Rain is not proof of a change in demand or supply.", PrecipitationTriggerMM),
	}
	if expires {
		out = append(out, "The one-hour watch is a suggested review window. This assessment expires at the time shown.")
	}
	return out
}

func writeServerError(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusServiceUnavailable)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error":   "assessment_unavailable",
		"message": "The assessment could not be loaded.",
	})
}
