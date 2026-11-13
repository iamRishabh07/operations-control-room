package control

import (
	"fmt"
	"time"
)

// Decision is the outcome of the rank-prefix rule. Winner is set only for
// action, ValidUntil for action and no_action, ReasonCode only for unavailable.
type Decision struct {
	Status     string
	ReasonCode string
	Reason     string
	Winner     *Candidate
	ValidUntil *time.Time
}

func unavailable(code, reason string) Decision {
	return Decision{Status: StatusUnavailable, ReasonCode: code, Reason: reason}
}

// Decide applies the rank-prefix rule to candidates in historical rank order
// (see docs/decisions.md):
//
//   - No candidates → unavailable (no_history).
//   - A reading that failed validation → unavailable (weather_invalid).
//   - No usable reading → unavailable (weather_unavailable). Absence of
//     evidence is never read as dry.
//   - The first valid reading at or above the trigger → action. Candidates
//     ranked below the winner are not examined.
//   - Every candidate valid and dry → no_action.
//
// now is the instant used for expiry math; nextHourStart is the end of the
// current local hour and caps validity. Decide performs no I/O.
func Decide(now, nextHourStart time.Time, cands []Candidate) Decision {
	if len(cands) == 0 {
		return unavailable(ReasonNoHistory, "no historical pickups for this weekday and hour")
	}
	// A stale hour boundary must not yield a valid_until in the past.
	if nextHourStart.Before(now) {
		nextHourStart = now.Add(time.Hour)
	}
	prefixExpiry := minTime(now.Add(AssessmentValidity), nextHourStart)
	for i := range cands {
		ev := cands[i].Weather
		if ev.State == WeatherInvalid {
			return unavailable(ReasonWeatherInvalid,
				fmt.Sprintf("the reading for %s failed validation (%s)", cands[i].Name, ev.Reason))
		}
		if ev.State != WeatherValid || ev.PrecipitationMM == nil {
			return unavailable(ReasonWeatherUnavailable, "the weather source returned no usable reading")
		}
		if ev.ValidAt != nil {
			prefixExpiry = minTime(prefixExpiry, ev.ValidAt.Add(WeatherFreshness))
		}
		if *ev.PrecipitationMM >= PrecipitationTriggerMM {
			w := cands[i]
			return Decision{
				Status:     StatusAction,
				Reason:     "modeled precipitation at or above the trigger",
				Winner:     &w,
				ValidUntil: &prefixExpiry,
			}
		}
	}
	return Decision{
		Status:     StatusNoAction,
		Reason:     "no evaluated area meets the precipitation trigger",
		ValidUntil: &prefixExpiry,
	}
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}
