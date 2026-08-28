package domain

// AssetKind discriminates what a Watch tracks. Watch.Params is JSONB in the
// database and decodes into a different Go struct depending on this value —
// see Watch.DecodeFlightParams / Watch.DecodeLodgingParams.
//
// flight_* kinds use the Travelpayouts REST API (internal/providers/aviasales).
// lodging_airbnb has no REST API to call — Airbnb has never had an
// official one — so it's backed by a headless-browser scraper instead
// (internal/providers/airbnb). A hotel/rental kind existed in an earlier
// version of this app and was removed for exactly that "no API" reason;
// the scraper is what makes it viable again.
type AssetKind string

const (
	AssetFlightOneWay  AssetKind = "flight_one_way"
	AssetFlightReturn  AssetKind = "flight_return"
	AssetLodgingAirbnb AssetKind = "lodging_airbnb"
)

func (k AssetKind) Valid() bool {
	switch k {
	case AssetFlightOneWay, AssetFlightReturn, AssetLodgingAirbnb:
		return true
	}
	return false
}

func (k AssetKind) IsFlight() bool {
	return k == AssetFlightOneWay || k == AssetFlightReturn
}

// IsLodging is a family predicate like IsFlight — currently one kind, but
// keeping it a family check (not "== AssetLodgingAirbnb" at every call
// site) means a future lodging_booking kind slots in without touching
// every place that asks "is this a lodging watch".
func (k AssetKind) IsLodging() bool {
	return k == AssetLodgingAirbnb
}
