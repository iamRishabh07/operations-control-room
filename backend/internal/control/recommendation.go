package control

import "time"

// Decide applies the rank-prefix rule over already-validated candidates and
// returns the assessment status, the action winner (if any) and valid_until.
//
// Rules (see docs/decisions.md):
//   - No candidates → unavailable (insufficient history).
//   - whole-response weather failure (indicated by all candidates having
//     empty State) → unavailable with upstream reason.
//   - Walk candidates in historical rank order:
//       invalid predecessor → unavailable (higher-ranked evidence missing).
//       first valid wet (≥ trigger) → action; validity bounded by prefix.
//   - All valid dry → no_action; validity bounded by every candidate.
//
// Candidates are expected to already carry their weather evidence from
// WeatherClient.Fetch. Decide does not call any I/O.
func Decide(assessedAt time.Time, nextHourStart time.Time, cands []Candidate, upstreamReason string) (status, reason string, winner *Candidate, validUntil *time.Time) {
	if len(cands) == 0 {
		return StatusUnavailable, "no historical candidates", nil, nil
	}
	if upstreamReason != "" {
		return StatusUnavailable, upstreamReason, nil, nil
	}
	prefixExpiry := minTime(assessedAt.Add(AssessmentValidity), nextHourStart)
	for i := range cands {
		ev := cands[i].Weather
		if ev.State == WeatherInvalid {
			return StatusUnavailable, "higher-priority evidence invalid: " + ev.Reason, nil, nil
		}
		if ev.ValidAt != nil {
			prefixExpiry = minTime(prefixExpiry, ev.ValidAt.Add(WeatherFreshness))
		}
		if ev.PrecipitationMM != nil && *ev.PrecipitationMM >= PrecipitationTriggerMM {
			w := cands[i]
			return StatusAction, "modeled precipitation at trigger threshold", &w, &prefixExpiry
		}
	}
	return StatusNoAction, "no evaluated area meets the precipitation trigger", nil, &prefixExpiry
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}
