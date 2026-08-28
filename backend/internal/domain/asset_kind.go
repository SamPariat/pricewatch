package domain

// AssetKind discriminates what a Watch tracks. Watch.Params is JSONB in the
// database and decodes into a different Go struct depending on this value —
// see Watch.DecodeFlightParams.
//
// Flights only: hotel and rental kinds (Hotellook, and an Airbnb stub)
// existed in an earlier version of this app but were removed — neither
// Travelpayouts nor any other option left had a free, working API for
// them (Hotellook's cache endpoint stopped returning usable data, and
// Airbnb has never had an official one).
type AssetKind string

const (
	AssetFlightOneWay AssetKind = "flight_one_way"
	AssetFlightReturn AssetKind = "flight_return"
)

func (k AssetKind) Valid() bool {
	switch k {
	case AssetFlightOneWay, AssetFlightReturn:
		return true
	}
	return false
}

func (k AssetKind) IsFlight() bool {
	return k == AssetFlightOneWay || k == AssetFlightReturn
}
