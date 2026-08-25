package handlers

import (
	"github.com/gofiber/fiber/v3"

	"github.com/SamPariat/pricewatch/internal/domain"
	"github.com/SamPariat/pricewatch/internal/httpapi/envelope"
	v1 "github.com/SamPariat/pricewatch/internal/httpapi/presenter/v1"
)

// ListRuns godoc
// @Summary      List recent digest runs
// @Description  The 50 most recent runs across every watch, newest first — backs the panel's run-history screen.
// @Tags         runs
// @Security     CookieAuth
// @Produce      json
// @Success      200  {array}   v1.DigestRun
// @Failure      500  {string}  string  "list runs"
// @Router       /runs [get]
func (a *API) ListRuns(c fiber.Ctx) error {
	runs, err := a.Runs.List(c, 50)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "list runs")
	}
	return envelope.Ok(c, fiber.StatusOK, v1.DigestRunsOf(runs), "runs listed")
}

// GetRunEvents godoc
// @Summary      Get one run's timeline
// @Description  Per-stage log lines (fetch → normalize → persist → analyze → render → send) for one run — the debugging tool behind "why didn't the digest arrive."
// @Tags         runs
// @Security     CookieAuth
// @Produce      json
// @Param        run_id  path      string  true  "Run ID (ULID)"
// @Success      200     {array}   v1.RunEvent
// @Failure      500     {string}  string  "list run events"
// @Router       /runs/{run_id}/events [get]
func (a *API) GetRunEvents(c fiber.Ctx) error {
	runID := domain.RunID(c.Params("run_id"))
	events, err := a.Runs.Events(c, runID)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "list run events")
	}
	return envelope.Ok(c, fiber.StatusOK, v1.RunEventsOf(events), "run events")
}
