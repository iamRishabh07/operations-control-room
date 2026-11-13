// Package control holds the recommendation service: seed loading, SQL ranking,
// Open-Meteo weather adapter, and the pure rank-prefix decision function.
package control

import "time"

// Seed is the committed historical extract loaded at startup.
type Seed struct {
	Areas   []Area       `json:"areas"`
	Hours   []HourCounts `json:"hours"`
	History ImportRecord `json:"history"`
}

// Area is a Chicago community area with a representative point for weather lookup.
type Area struct {
	AreaID      int     `json:"area_id"`
	Name        string  `json:"name"`
	Lat         float64 `json:"lat"`
	Lon         float64 `json:"lon"`
	CoordSource string  `json:"coord_source"`
}

// HourCounts holds the pickups of every community area for one clock hour of
// one day. Trips[i] belongs to area i+1.
type HourCounts struct {
	Date  string `json:"date"`
	Hour  int    `json:"hour"`
	Trips []int  `json:"trips"`
}

// ImportRecord is the provenance of the committed seed.
type ImportRecord struct {
	DatasetID                string `json:"dataset_id"`
	PeriodStart              string `json:"period_start"`
	PeriodEnd                string `json:"period_end"`
	RetrievedAt              string `json:"retrieved_at"`
	QueryText                string `json:"query_text"`
	RowsLoaded               int    `json:"rows_loaded"`
	TripsIncluded            int    `json:"trips_included"`
	TripsExcludedInvalidArea int    `json:"trips_excluded_invalid_area"`
}

// Candidate is one area's historical pickups for the current weekday-hour
// plus the modeled weather at its point.
type Candidate struct {
	Rank       int             `json:"rank"`
	AreaID     int             `json:"area_id"`
	Name       string          `json:"name"`
	Lat        float64         `json:"lat"`
	Lon        float64         `json:"lon"`
	SampleDays int             `json:"sample_days"`
	MeanTrips  float64         `json:"mean_trips"`
	MinTrips   int             `json:"min_trips"`
	MaxTrips   int             `json:"max_trips"`
	CityShare  float64         `json:"city_share"`
	Weather    WeatherEvidence `json:"weather"`
}

// WeatherEvidence is one area's reading. State is valid, invalid (with a
// reason) or missing; precipitation is null unless the state is valid.
type WeatherEvidence struct {
	State           string     `json:"state"`
	Reason          string     `json:"reason,omitempty"`
	PrecipitationMM *float64   `json:"precipitation_mm"`
	IntervalSeconds *int       `json:"interval_seconds,omitempty"`
	Units           string     `json:"units,omitempty"`
	ValidAt         *time.Time `json:"valid_at,omitempty"`
}

// Assessment is the single API response: action, no_action or unavailable.
// SlotStart is the start of the Chicago clock hour being evaluated. AssessedAt
// is when the assessment was made; ValidUntil counts from it.
type Assessment struct {
	Status      string        `json:"status"`
	ReasonCode  string        `json:"reason_code,omitempty"` // set only when unavailable
	Reason      string        `json:"reason"`
	SlotStart   time.Time     `json:"slot_start"`
	AssessedAt  time.Time     `json:"assessed_at"`
	ValidUntil  *time.Time    `json:"valid_until"`
	TriggerMM   float64       `json:"trigger_mm"`
	Area        *AreaRef      `json:"area"`
	Candidates  []Candidate   `json:"candidates"`
	History     *ImportRecord `json:"history"`
	Weather     WeatherSource `json:"weather"`
	Limitations []string      `json:"limitations"`
}

// AreaRef is the selected area shown with an action status.
type AreaRef struct {
	AreaID int    `json:"area_id"`
	Name   string `json:"name"`
}

// WeatherSource documents the endpoint the readings came from. Simulated is
// set whenever that endpoint is not the official Open-Meteo forecast API.
type WeatherSource struct {
	Source    string     `json:"source"`
	Simulated bool       `json:"simulated"`
	FetchedAt *time.Time `json:"fetched_at"`
}

const (
	StatusAction      = "action"
	StatusNoAction    = "no_action"
	StatusUnavailable = "unavailable"

	// Evidence states. A candidate starts as missing and stays so when the
	// weather request fails; only a valid reading may be read as dry or wet.
	WeatherValid   = "valid"
	WeatherInvalid = "invalid"
	WeatherMissing = "missing"

	// Why an assessment is unavailable.
	ReasonNoHistory          = "no_history"          // no pickups in the baseline for this weekday-hour
	ReasonWeatherUnavailable = "weather_unavailable" // the weather source gave no usable reading
	ReasonWeatherInvalid     = "weather_invalid"     // a needed reading failed validation

	// PrecipitationTriggerMM is the modeled precipitation per 15-minute
	// interval at or above which an area is put on watch, roughly light rain
	// (0.8 mm/h). It is a heuristic, not a validated threshold.
	PrecipitationTriggerMM = 0.2

	// AssessmentValidity is how long an action or no_action response may be
	// used after it was assessed, before the client must refresh.
	AssessmentValidity = 5 * time.Minute

	// WeatherFreshness is the maximum age of an accepted reading.
	WeatherFreshness = 30 * time.Minute

	// WeatherLead is how far in the future a reading may be stamped. Open-Meteo
	// stamps `current` with the start of a 15-minute interval, which our clock
	// may not have reached yet; one interval plus a minute covers that skew.
	WeatherLead = 16 * time.Minute

	// ExpectedIntervalSeconds is the Open-Meteo "current" bucket length.
	ExpectedIntervalSeconds = 900

	// ExpectedUnits is the Open-Meteo precipitation unit for the current block.
	ExpectedUnits = "mm"
)
