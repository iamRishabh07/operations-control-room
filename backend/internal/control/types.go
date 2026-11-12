// Package control holds the recommendation service: seed loading, SQL ranking,
// Open-Meteo weather adapter, and the pure rank-prefix decision function.
package control

import "time"

// Seed is the committed historical extract loaded at startup.
type Seed struct {
	Areas   []Area       `json:"areas"`
	Buckets []Bucket     `json:"buckets"`
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

// Bucket is one (area, day-of-week, hour-of-day) historical pickup count.
type Bucket struct {
	AreaID     int `json:"area_id"`
	Dow        int `json:"dow"`
	Hour       int `json:"hour"`
	TotalTrips int `json:"total_trips"`
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

// Candidate is one historically eligible area plus the modeled weather at its point.
type Candidate struct {
	Rank              int             `json:"rank"`
	AreaID            int             `json:"area_id"`
	Name              string          `json:"name"`
	Lat               float64         `json:"lat"`
	Lon               float64         `json:"lon"`
	TotalTrips        int             `json:"total_trips"`
	SampleDays        int             `json:"sample_days"`
	MeanPickupsPerDay float64         `json:"mean_pickups_per_day"`
	Weather           WeatherEvidence `json:"weather"`
}

// WeatherEvidence captures either a validated current precipitation reading
// or an explicit invalidity reason. Precipitation is null when invalid.
type WeatherEvidence struct {
	State           string     `json:"state"`
	Reason          string     `json:"reason,omitempty"`
	PrecipitationMM *float64   `json:"precipitation_mm"`
	IntervalSeconds *int       `json:"interval_seconds,omitempty"`
	Units           string     `json:"units,omitempty"`
	ValidAt         *time.Time `json:"valid_at,omitempty"`
}

// Assessment is the single API response: action, no_action or unavailable.
type Assessment struct {
	Status      string          `json:"status"`
	Reason      string          `json:"reason"`
	AssessedAt  time.Time       `json:"assessed_at"`
	ValidUntil  *time.Time      `json:"valid_until"`
	Area        *AreaRef        `json:"area"`
	Candidates  []Candidate     `json:"candidates"`
	History     *ImportRecord   `json:"history"`
	Weather     WeatherSource   `json:"weather"`
	Limitations []string        `json:"limitations"`
}

// AreaRef is the selected area shown with an action status.
type AreaRef struct {
	AreaID int    `json:"area_id"`
	Name   string `json:"name"`
}

// WeatherSource documents the upstream provider and fetch time.
type WeatherSource struct {
	Source    string     `json:"source"`
	FetchedAt *time.Time `json:"fetched_at"`
}

const (
	StatusAction      = "action"
	StatusNoAction    = "no_action"
	StatusUnavailable = "unavailable"

	WeatherValid   = "valid"
	WeatherInvalid = "invalid"

	// PrecipitationTriggerMM is the modeled-precipitation threshold that puts
	// an eligible area on watch. 0.2 mm/15 min is a conservative light-rain
	// level consistent with WMO/NWS "light rain" bands; documented in README.
	PrecipitationTriggerMM = 0.2

	// AssessmentValidity is how long any non-unavailable response may be used
	// after it was assessed, before the client must refresh.
	AssessmentValidity = 5 * time.Minute

	// WeatherFreshness is the maximum age of a modeled reading we accept.
	WeatherFreshness = 30 * time.Minute

	// WeatherLead is the maximum future lead we accept for a modeled reading.
	WeatherLead = 5 * time.Minute

	// ExpectedIntervalSeconds is the Open-Meteo "current" bucket length.
	ExpectedIntervalSeconds = 900

	// ExpectedUnits is the Open-Meteo precipitation unit for the current block.
	ExpectedUnits = "mm"
)
