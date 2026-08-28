// Package v1 holds the version-1 wire DTOs. A handler builds one domain
// object and calls a presenter for the negotiated version — one handler,
// one domain object, N presenters — never forks the handler itself per
// version (PLAN.md § API versioning: that's how versioned APIs rot).
package v1

import (
	"encoding/json"
	"time"

	"github.com/SamPariat/pricewatch/internal/domain"
)

type Watch struct {
	ID                  string          `json:"id"`
	Name                string          `json:"name"`
	Kind                string          `json:"kind"`
	Enabled             bool            `json:"enabled"`
	CronExpr            string          `json:"cron_expr"`
	Timezone            string          `json:"timezone"`
	Params              json.RawMessage `json:"params"`
	ThresholdPct        float64         `json:"threshold_pct"`
	CreatedAt           time.Time       `json:"created_at"`
	LastUpdatedAt       *time.Time      `json:"last_updated_at,omitempty"`
	LastError           string          `json:"last_error,omitempty"`
	ConsecutiveFailures int             `json:"consecutive_failures"`
	Stale               bool            `json:"stale"`
	NextRun             *time.Time      `json:"next_run,omitempty"`
	Price               *PriceSummary   `json:"price,omitempty"`
}

// PriceSummary is the current-price figure the watch list and detail
// header lead with — computed once server-side from internal/analytics
// (the same functions the digest uses), not reimplemented client-side,
// so the panel and the Discord message can never disagree about a
// delta or a percentile.
type PriceSummary struct {
	PriceMinor int64    `json:"price_minor"`
	Currency   string   `json:"currency"`
	DeltaPct   *float64 `json:"delta_pct,omitempty"`
	Percentile *float64 `json:"percentile,omitempty"`
}

// WatchOf builds the v1 Watch DTO. lastUpdatedAt is the fetched_at of the
// newest data backing this watch (nil if it has never run) — never
// "now," per PLAN.md § Freshness: a value here that isn't genuinely the
// last real fetch time defeats the entire point of showing it. price is
// nil for a watch that has never successfully run.
func WatchOf(w domain.Watch, state domain.WatchState, nextRun time.Time, interval time.Duration, lastUpdatedAt *time.Time, price *PriceSummary) Watch {
	dto := Watch{
		ID:                  string(w.ID),
		Name:                w.Name,
		Kind:                string(w.Kind),
		Enabled:             w.Enabled,
		CronExpr:            w.CronExpr,
		Timezone:            w.Timezone,
		Params:              w.Params,
		ThresholdPct:        w.ThresholdPct,
		CreatedAt:           w.CreatedAt,
		LastUpdatedAt:       lastUpdatedAt,
		LastError:           state.LastError,
		ConsecutiveFailures: state.ConsecutiveFailures,
		Price:               price,
	}
	if !nextRun.IsZero() {
		dto.NextRun = &nextRun
	}
	dto.Stale = IsStale(state, interval)
	return dto
}

// IsStale implements PLAN.md § Freshness's staleness badge: a watch whose
// newest data is older than 2x its own schedule's interval is flagged —
// the failure mode where a silently broken watch looks identical to one
// whose price simply hasn't moved. Exported so the analytics-summary
// handler can count stale watches with the same rule the watch list uses.
func IsStale(state domain.WatchState, interval time.Duration) bool {
	if interval <= 0 {
		return false // not currently scheduled — nothing to compare against
	}
	if state.LastSuccessAt == nil {
		return true // never succeeded at all
	}
	return time.Since(*state.LastSuccessAt) > 2*interval
}
