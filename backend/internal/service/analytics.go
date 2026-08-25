package service

import (
	"context"

	"github.com/SamPariat/pricewatch/internal/analytics"
	"github.com/SamPariat/pricewatch/internal/domain"
	v1 "github.com/SamPariat/pricewatch/internal/httpapi/presenter/v1"
	"github.com/SamPariat/pricewatch/internal/scheduler"
)

type AnalyticsService struct {
	Repo  domain.Repository
	Sched *scheduler.Scheduler
	Clock domain.Clock
}

// Summary is the dashboard KPI row — total/enabled/stale watch counts
// plus the average 7-day price delta across watches with enough history.
// Returns presenter/v1's AnalyticsSummary type reused as a plain data
// carrier, same reasoning as WatchService.Detail.
func (s *AnalyticsService) Summary(ctx context.Context) (v1.AnalyticsSummary, error) {
	watches, err := s.Repo.ListWatches(ctx)
	if err != nil {
		return v1.AnalyticsSummary{}, err
	}

	summary := v1.AnalyticsSummary{TotalWatches: len(watches)}
	var deltaSum float64
	var deltaCount int
	now := s.Clock.Now()

	for _, w := range watches {
		if w.Enabled {
			summary.EnabledWatches++
		}

		state, err := s.Repo.GetWatchState(ctx, w.ID)
		if err != nil {
			state = domain.WatchState{WatchID: w.ID}
		}
		if v1.IsStale(state, s.Sched.Interval(w.ID)) {
			summary.StaleWatches++
		}

		samples, err := s.Repo.ListPriceSamples(ctx, w.ID, now.AddDate(0, 0, -8))
		if err != nil || len(samples) == 0 {
			continue
		}
		if d := analytics.DeltaVsYesterday(samples, now); d.OK {
			deltaSum += d.Pct
			deltaCount++
		}
	}

	if deltaCount > 0 {
		avg := deltaSum / float64(deltaCount)
		summary.AvgDelta7dPct = &avg
	}

	return summary, nil
}
