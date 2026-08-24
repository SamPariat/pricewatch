package domain

import (
	"encoding/json"
	"time"
)

// Quote is one price observation from one provider at one point in time.
// PriceMinor is the smallest currency unit (paise/cents) so it stays an
// exact integer end to end — no floats touch money anywhere in this system.
//
// DepartDate/ReturnDate carry check-in/check-out for hotel and rental
// quotes too, matching the shared quotes table — the naming is
// flight-first because flights were the original case, not because hotels
// are a special case of it.
type Quote struct {
	WatchID     WatchID
	FetchedAt   time.Time
	Provider    string
	PriceMinor  int64
	Currency    string
	DepartDate  string // YYYY-MM-DD
	ReturnDate  string // YYYY-MM-DD, empty for one-way flight quotes
	Stops       int
	Carrier     string
	DeepLink    string
	Fingerprint string
	Raw         json.RawMessage
}

// PriceSample is one day's rollup of Quotes for a watch — the table
// analytics and charts read from; raw Quotes are pruned after ~90 days but
// samples are kept indefinitely.
type PriceSample struct {
	WatchID     WatchID
	SampleDate  time.Time // truncated to day
	MinMinor    int64
	MedianMinor int64
	MaxMinor    int64
	NQuotes     int
	UpdatedAt   time.Time
}
