package handlers

import (
	"github.com/gofiber/fiber/v3"

	"github.com/SamPariat/pricewatch/internal/httpapi/envelope"
	v1 "github.com/SamPariat/pricewatch/internal/httpapi/presenter/v1"
	"github.com/SamPariat/pricewatch/internal/i18n"
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
	loc := i18n.From(c)
	summary, err := a.Analytics.Summary(c)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, i18n.T(loc, "analytics.summary_failed"))
	}
	return envelope.Ok(c, fiber.StatusOK, v1.AnalyticsSummary(summary), i18n.T(loc, "analytics.summary_ok"))
}
