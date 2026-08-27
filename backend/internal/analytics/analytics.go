// Package analytics computes deltas, rolling stats, and percentile rank
// over domain.PriceSample. Every function here is pure — no I/O, no ports,
// callers pass `asOf` explicitly instead of the functions reading the
// clock themselves, which is what makes them trivial to unit test.
package analytics

import (
	"sort"
	"time"

	"github.com/SamPariat/pricewatch/internal/domain"
)

// Delta is the change between the latest sample and the day before it.
// OK is false when there's no prior-day sample to compare against — e.g.
// a watch's first day, or a gap in fetches.
type Delta struct {
	AbsMinor int64
	Pct      float64
	OK       bool
}

// DeltaVsYesterday compares the latest sample on or before asOf to the
// sample from the day before *that sample's own date* — not asOf itself,
// so a watch whose fetch hasn't landed yet today still gets a correct
// delta against its own last two real data points.
func DeltaVsYesterday(samples []domain.PriceSample, asOf time.Time) Delta {
	latest, ok := latestOnOrBefore(samples, asOf)
	if !ok {
		return Delta{}
	}
	priorDate := dateOnly(latest.SampleDate).AddDate(0, 0, -1)
	prior, ok := sampleOnDate(samples, priorDate)
	if !ok || prior.MedianMinor == 0 {
		return Delta{}
	}
	abs := latest.MedianMinor - prior.MedianMinor
	pct := float64(abs) / float64(prior.MedianMinor) * 100
	return Delta{AbsMinor: abs, Pct: pct, OK: true}
}

// Rolling is the low/median/high across a trailing window of days.
type Rolling struct {
	LowMinor    int64
	MedianMinor int64
	HighMinor   int64
	N           int
	OK          bool
}

// RollingWindow summarizes the `days` days ending at asOf (inclusive).
// MedianMinor is the median of each day's own median — a reasonable
// window-level "typical price" given the data is already a daily rollup,
// not raw quotes.
func RollingWindow(samples []domain.PriceSample, days int, asOf time.Time) Rolling {
	window := inWindow(samples, days, asOf)
	if len(window) == 0 {
		return Rolling{}
	}
	lows := make([]int64, len(window))
	meds := make([]int64, len(window))
	highs := make([]int64, len(window))
	for i, s := range window {
		lows[i] = s.MinMinor
		meds[i] = s.MedianMinor
		highs[i] = s.MaxMinor
	}
	return Rolling{
		LowMinor:    MinInt64(lows),
		MedianMinor: MedianInt64(meds),
		HighMinor:   MaxInt64(highs),
		N:           len(window),
		OK:          true,
	}
}

// PercentileRank returns what fraction of the trailing `days` days (ending
// at asOf) were at least as expensive as priceMinor — the "cheaper than
// 87% of the last 90 days" figure the digest and panel lead with, because
// it tells someone whether to book in a way a raw delta doesn't.
// OK is false when the window has no samples to compare against.
func PercentileRank(samples []domain.PriceSample, priceMinor int64, days int, asOf time.Time) (pct float64, ok bool) {
	window := inWindow(samples, days, asOf)
	if len(window) == 0 {
		return 0, false
	}
	atLeastAsExpensive := 0
	for _, s := range window {
		if s.MinMinor >= priceMinor {
			atLeastAsExpensive++
		}
	}
	return float64(atLeastAsExpensive) / float64(len(window)) * 100, true
}

// AllTimeLow is the lowest MinMinor across every sample given, and the day
// it happened on.
type AllTimeLow struct {
	PriceMinor int64
	Date       time.Time
	OK         bool
}

func AllTimeLowOf(samples []domain.PriceSample) AllTimeLow {
	if len(samples) == 0 {
		return AllTimeLow{}
	}
	best := samples[0]
	for _, s := range samples[1:] {
		if s.MinMinor < best.MinMinor {
			best = s
		}
	}
	return AllTimeLow{PriceMinor: best.MinMinor, Date: best.SampleDate, OK: true}
}

// dateOnly truncates to a calendar day in UTC, regardless of t's own
// Location. This must match price_samples.sample_date's own normalization:
// it's a plain SQL `date` column, so Postgres discards time-of-day and any
// offset entirely, and pgx always reconstructs it as UTC-located on read —
// every domain.PriceSample.SampleDate that has round-tripped through the
// database is UTC-located no matter what Location it was written with.
// asOf, by contrast, is Clock.Now() fresh out of the process, in whatever
// Location the process happens to run in. Preserving t.Location() here
// (the previous behavior) compared those two under different Locations
// whenever the process wasn't already running in UTC, which silently
// broke latestOnOrBefore/sampleOnDate's day-equality checks — every delta,
// rolling stat, and threshold alert would go quietly dark rather than
// erroring, since Delta.OK simply stays false.
func dateOnly(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func inWindow(samples []domain.PriceSample, days int, asOf time.Time) []domain.PriceSample {
	end := dateOnly(asOf)
	start := end.AddDate(0, 0, -(days - 1))
	var out []domain.PriceSample
	for _, s := range samples {
		d := dateOnly(s.SampleDate)
		if !d.Before(start) && !d.After(end) {
			out = append(out, s)
		}
	}
	return out
}

func latestOnOrBefore(samples []domain.PriceSample, asOf time.Time) (domain.PriceSample, bool) {
	cutoff := dateOnly(asOf)
	var best domain.PriceSample
	found := false
	for _, s := range samples {
		d := dateOnly(s.SampleDate)
		if d.After(cutoff) {
			continue
		}
		if !found || d.After(dateOnly(best.SampleDate)) {
			best, found = s, true
		}
	}
	return best, found
}

func sampleOnDate(samples []domain.PriceSample, date time.Time) (domain.PriceSample, bool) {
	target := dateOnly(date)
	for _, s := range samples {
		if dateOnly(s.SampleDate).Equal(target) {
			return s, true
		}
	}
	return domain.PriceSample{}, false
}

func MinInt64(vals []int64) int64 {
	m := vals[0]
	for _, v := range vals[1:] {
		if v < m {
			m = v
		}
	}
	return m
}

func MaxInt64(vals []int64) int64 {
	m := vals[0]
	for _, v := range vals[1:] {
		if v > m {
			m = v
		}
	}
	return m
}

func MedianInt64(vals []int64) int64 {
	sorted := append([]int64(nil), vals...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	n := len(sorted)
	if n%2 == 1 {
		return sorted[n/2]
	}
	return (sorted[n/2-1] + sorted[n/2]) / 2
}
