package domain

import (
	"encoding/json"
	"fmt"
	"time"
)

type WatchID string

// Watch is a flight or lodging leg — it has no schedule of its own. The
// schedule (cron_expr/timezone) lives on its Trip; see domain.Trip.
type Watch struct {
	ID WatchID
	// TripID is required — every watch is a leg of exactly one trip.
	TripID TripID
	Name   string // optional per-leg label, e.g. "Outbound flight"
	Kind   AssetKind
	// Enabled pauses just this leg — the trip keeps firing for its other
	// legs. Distinct from Trip.Enabled, which pauses the whole trip.
	Enabled      bool
	Params       json.RawMessage
	ThresholdPct float64
	CreatedAt    time.Time
}

// FlightParams backs AssetFlightOneWay and AssetFlightReturn watches.
// ReturnDate is nil for one-way watches.
type FlightParams struct {
	Origin      string  `json:"origin"`
	Destination string  `json:"destination"`
	DepartDate  string  `json:"depart_date"` // YYYY-MM-DD
	ReturnDate  *string `json:"return_date,omitempty"`
}

// DecodeFlightParams decodes w.Params for a flight watch. Callers should
// switch on w.Kind before calling; this returns an error rather than
// panicking if that check was skipped.
func (w Watch) DecodeFlightParams() (FlightParams, error) {
	if !w.Kind.IsFlight() {
		return FlightParams{}, fmt.Errorf("watch %s: kind %q is not a flight kind", w.ID, w.Kind)
	}
	var p FlightParams
	if err := json.Unmarshal(w.Params, &p); err != nil {
		return FlightParams{}, fmt.Errorf("watch %s: decode flight params: %w", w.ID, err)
	}
	return p, nil
}

// LodgingParams backs AssetLodgingAirbnb watches. URL must be a specific
// Airbnb listing page (not a search-results URL) — the scraper loads it
// directly and reads the rendered price for CheckIn/CheckOut.
type LodgingParams struct {
	URL      string `json:"url"`
	CheckIn  string `json:"check_in"`  // YYYY-MM-DD
	CheckOut string `json:"check_out"` // YYYY-MM-DD
	Guests   int    `json:"guests,omitempty"`
}

// DecodeLodgingParams decodes w.Params for a lodging watch. Callers
// should switch on w.Kind before calling; this returns an error rather
// than panicking if that check was skipped.
//
// Note json.Unmarshal silently ignores fields it doesn't recognize, so
// decoding flight-shaped params ({"origin":...}) into LodgingParams
// succeeds with every field empty rather than erroring — decoding alone
// doesn't prove the params actually matched the kind. Callers that need
// that guarantee (validateWatch) must check the required fields landed.
func (w Watch) DecodeLodgingParams() (LodgingParams, error) {
	if !w.Kind.IsLodging() {
		return LodgingParams{}, fmt.Errorf("watch %s: kind %q is not a lodging kind", w.ID, w.Kind)
	}
	var p LodgingParams
	if err := json.Unmarshal(w.Params, &p); err != nil {
		return LodgingParams{}, fmt.Errorf("watch %s: decode lodging params: %w", w.ID, err)
	}
	return p, nil
}
