package handlers

import (
	"github.com/gofiber/fiber/v3"

	"github.com/sampariat/prices-reminder/internal/domain"
	v1 "github.com/sampariat/prices-reminder/internal/httpapi/presenter/v1"
)

func (a *API) ListRuns(c fiber.Ctx) error {
	limit := 50
	runs, err := a.Repo.ListRecentDigestRuns(c, limit)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "list runs")
	}
	return c.JSON(v1.DigestRunsOf(runs))
}

func (a *API) GetRunEvents(c fiber.Ctx) error {
	runID := domain.RunID(c.Params("run_id"))
	events, err := a.Repo.ListRunEvents(c, runID)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "list run events")
	}
	return c.JSON(v1.RunEventsOf(events))
}
