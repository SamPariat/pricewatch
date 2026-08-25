package v1

import (
	"time"

	"github.com/SamPariat/pricewatch/internal/domain"
)

type PriceSample struct {
	Date        string `json:"date"` // YYYY-MM-DD
	MinMinor    int64  `json:"min_minor"`
	MedianMinor int64  `json:"median_minor"`
	MaxMinor    int64  `json:"max_minor"`
	NQuotes     int    `json:"n_quotes"`
}

func PriceSampleOf(s domain.PriceSample) PriceSample {
	return PriceSample{
		Date:        s.SampleDate.Format("2006-01-02"),
		MinMinor:    s.MinMinor,
		MedianMinor: s.MedianMinor,
		MaxMinor:    s.MaxMinor,
		NQuotes:     s.NQuotes,
	}
}

func PriceSamplesOf(samples []domain.PriceSample) []PriceSample {
	out := make([]PriceSample, len(samples))
	for i, s := range samples {
		out[i] = PriceSampleOf(s)
	}
	return out
}

// History is the full response for GET /api/watches/:id/history — the
// samples plus the same freshness fields the watch list shows, so the
// detail page's staleness badge always agrees with the list's.
type History struct {
	Samples       []PriceSample `json:"samples"`
	LastUpdatedAt *time.Time    `json:"last_updated_at,omitempty"`
	NextRun       *time.Time    `json:"next_run,omitempty"`
}
