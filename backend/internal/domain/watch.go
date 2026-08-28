package domain

import (
	"encoding/json"
	"fmt"
	"time"
)

type WatchID string

type Watch struct {
	ID           WatchID
	Name         string
	Kind         AssetKind
	Enabled      bool
	CronExpr     string
	Timezone     string
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
