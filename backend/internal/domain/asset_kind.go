package domain

// AssetKind discriminates what a Watch tracks. Watch.Params is JSONB in the
// database and decodes into a different Go struct depending on this value —
// see Watch.DecodeFlightParams / DecodeHotelParams.
type AssetKind string

const (
	AssetFlightOneWay AssetKind = "flight_one_way"
	AssetFlightReturn AssetKind = "flight_return"
	AssetHotel        AssetKind = "hotel"
	AssetRental       AssetKind = "rental"
)

func (k AssetKind) Valid() bool {
	switch k {
	case AssetFlightOneWay, AssetFlightReturn, AssetHotel, AssetRental:
		return true
	}
	return false
}

func (k AssetKind) IsFlight() bool {
	return k == AssetFlightOneWay || k == AssetFlightReturn
}
