package handlers

import (
	"github.com/gofiber/fiber/v3"

	"github.com/sampariat/prices-reminder/internal/analytics"
	"github.com/sampariat/prices-reminder/internal/domain"
	v1 "github.com/sampariat/prices-reminder/internal/httpapi/presenter/v1"
)

// AnalyticsSummary godoc
// @Summary      Dashboard KPI summary
// @Description  Total/enabled/stale watch counts plus the average 7-day price delta across watches with enough history. Scoped to what's actually computable today — no figure is reported for a feature that doesn't exist yet (e.g. threshold alerts).
// @Tags         analytics
// @Security     CookieAuth
// @Produce      json
// @Success      200  {object}  v1.AnalyticsSummary
// @Failure      500  {string}  string  "list watches"
// @Router       /analytics/summary [get]
func (a *API) AnalyticsSummary(c fiber.Ctx) error {
	watches, err := a.Repo.ListWatches(c)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "list watches")
	}

	summary := v1.AnalyticsSummary{TotalWatches: len(watches)}
	var deltaSum float64
	var deltaCount int
	now := a.Clock.Now()

	for _, w := range watches {
		if w.Enabled {
			summary.EnabledWatches++
		}

		state, err := a.Repo.GetWatchState(c, w.ID)
		if err != nil {
			state = domain.WatchState{WatchID: w.ID}
		}
		if v1.IsStale(state, a.Sched.Interval(w.ID)) {
			summary.StaleWatches++
		}

		samples, err := a.Repo.ListPriceSamples(c, w.ID, now.AddDate(0, 0, -8))
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

	return c.JSON(summary)
}
