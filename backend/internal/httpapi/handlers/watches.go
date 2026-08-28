package handlers

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/SamPariat/pricewatch/internal/domain"
	"github.com/SamPariat/pricewatch/internal/httpapi/envelope"
	v1 "github.com/SamPariat/pricewatch/internal/httpapi/presenter/v1"
	"github.com/SamPariat/pricewatch/internal/i18n"
	"github.com/SamPariat/pricewatch/internal/service"
)

type watchRequest struct {
	Name         string          `json:"name"`
	Kind         string          `json:"kind"`
	Enabled      *bool           `json:"enabled"`
	CronExpr     string          `json:"cron_expr"`
	Timezone     string          `json:"timezone"`
	Params       json.RawMessage `json:"params"`
	ThresholdPct float64         `json:"threshold_pct"`
}

func (r watchRequest) toInput() service.WatchInput {
	return service.WatchInput{
		Name: r.Name, Kind: r.Kind, Enabled: r.Enabled,
		CronExpr: r.CronExpr, Timezone: r.Timezone, Params: r.Params, ThresholdPct: r.ThresholdPct,
	}
}

func detailToDTO(d service.Detail) v1.Watch {
	return v1.WatchOf(d.Watch, d.State, d.NextRun, d.Interval, d.LastUpdated, d.Price)
}

// ListWatches godoc
// @Summary      List watches
// @Description  Every watch, each enriched with its current price, freshness, and staleness state.
// @Tags         watches
// @Security     CookieAuth
// @Produce      json
// @Success      200  {array}   v1.Watch
// @Failure      500  {string}  string  "list watches"
// @Router       /watches [get]
func (a *API) ListWatches(c fiber.Ctx) error {
	loc := i18n.From(c)
	details, err := a.Watches.List(c)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, i18n.T(loc, "watches.list_failed"))
	}
	out := make([]v1.Watch, len(details))
	for i, d := range details {
		out[i] = detailToDTO(d)
	}
	return envelope.Ok(c, fiber.StatusOK, out, i18n.T(loc, "watches.listed"))
}

// GetWatch godoc
// @Summary      Get a watch
// @Tags         watches
// @Security     CookieAuth
// @Produce      json
// @Param        id   path      string  true  "Watch ID"
// @Success      200  {object}  v1.Watch
// @Failure      404  {string}  string  "watch not found"
// @Router       /watches/{id} [get]
func (a *API) GetWatch(c fiber.Ctx) error {
	loc := i18n.From(c)
	id := domain.WatchID(c.Params("id"))
	d, err := a.Watches.Get(c, id)
	if err != nil {
		return fiber.NewError(fiber.StatusNotFound, i18n.T(loc, "watches.not_found"))
	}
	return envelope.Ok(c, fiber.StatusOK, detailToDTO(d), i18n.T(loc, "watches.found"))
}

// CreateWatch godoc
// @Summary      Create a watch
// @Description  Enabled by default. Triggers a background backfill fetch immediately so the chart isn't empty for a week, and reloads the scheduler so it's picked up on the next cron cycle without a restart.
// @Tags         watches
// @Security     CookieAuth
// @Accept       json
// @Produce      json
// @Param        body  body      watchRequest  true  "Watch fields"
// @Success      201   {object}  v1.Watch
// @Failure      400   {string}  string  "invalid request body, or a validation error naming the bad field"
// @Failure      500   {string}  string  "create watch"
// @Router       /watches [post]
func (a *API) CreateWatch(c fiber.Ctx) error {
	loc := i18n.From(c)
	var req watchRequest
	if err := c.Bind().Body(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, i18n.T(loc, "common.invalid_body"))
	}

	d, err := a.Watches.Create(c, req.toInput())
	if err != nil {
		var ve service.ValidationError
		if errors.As(err, &ve) {
			return fiber.NewError(fiber.StatusBadRequest, ve.Error())
		}
		return fiber.NewError(fiber.StatusInternalServerError, i18n.T(loc, "watches.create_failed"))
	}
	return envelope.Ok(c, fiber.StatusCreated, detailToDTO(d), i18n.T(loc, "watches.created"))
}

// UpdateWatch godoc
// @Summary      Replace a watch
// @Description  Full-replace semantics despite the PATCH verb — the panel's watch-builder form always submits every field, so there's no partial-merge case to support.
// @Tags         watches
// @Security     CookieAuth
// @Accept       json
// @Produce      json
// @Param        id    path      string        true  "Watch ID"
// @Param        body  body      watchRequest  true  "Full watch fields"
// @Success      200   {object}  v1.Watch
// @Failure      400   {string}  string  "invalid request body, or a validation error naming the bad field"
// @Failure      404   {string}  string  "watch not found"
// @Failure      500   {string}  string  "update watch"
// @Router       /watches/{id} [patch]
func (a *API) UpdateWatch(c fiber.Ctx) error {
	loc := i18n.From(c)
	id := domain.WatchID(c.Params("id"))
	var req watchRequest
	if err := c.Bind().Body(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, i18n.T(loc, "common.invalid_body"))
	}

	d, err := a.Watches.Update(c, id, req.toInput())
	if err != nil {
		switch {
		case errors.Is(err, service.ErrNotFound):
			return fiber.NewError(fiber.StatusNotFound, i18n.T(loc, "watches.not_found"))
		default:
			var ve service.ValidationError
			if errors.As(err, &ve) {
				return fiber.NewError(fiber.StatusBadRequest, ve.Error())
			}
			return fiber.NewError(fiber.StatusInternalServerError, i18n.T(loc, "watches.update_failed"))
		}
	}
	return envelope.Ok(c, fiber.StatusOK, detailToDTO(d), i18n.T(loc, "watches.updated"))
}

// DeleteWatch godoc
// @Summary  Delete a watch
// @Tags     watches
// @Security CookieAuth
// @Param    id  path  string  true  "Watch ID"
// @Success  204  "no content"
// @Failure  404  {string}  string  "watch not found"
// @Router   /watches/{id} [delete]
func (a *API) DeleteWatch(c fiber.Ctx) error {
	id := domain.WatchID(c.Params("id"))
	if err := a.Watches.Delete(c, id); err != nil {
		return fiber.NewError(fiber.StatusNotFound, i18n.T(i18n.From(c), "watches.not_found"))
	}
	return c.SendStatus(fiber.StatusNoContent)
}

type runNowResponse struct {
	Message string `json:"message"`
}

// RunWatchNow godoc
// @Summary      Run a watch now
// @Description  Runs the pipeline immediately, outside the normal schedule, bypassing the provider cache so it always fetches fresh (see providers.SkipCache). Honours the global dry-run setting for whether the result actually gets sent to Discord, matching the scheduled path's own behavior.
// @Tags         watches
// @Security     CookieAuth
// @Produce      json
// @Param        id   path      string  true  "Watch ID"
// @Success      200  {object}  runNowResponse
// @Failure      404  {string}  string  "watch not found"
// @Failure      502  {string}  string  "run failed: <cause>"
// @Router       /watches/{id}/run [post]
func (a *API) RunWatchNow(c fiber.Ctx) error {
	loc := i18n.From(c)
	id := domain.WatchID(c.Params("id"))
	text, err := a.Watches.RunNow(c, id, true)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrNotFound):
			return fiber.NewError(fiber.StatusNotFound, i18n.T(loc, "watches.not_found"))
		default:
			// err.Error() stays untranslated by design — it's the
			// pipeline's own failure cause (often a wrapped third-party
			// error, e.g. a provider timeout), not app-authored prose.
			return fiber.NewError(fiber.StatusBadGateway, i18n.T(loc, "watches.run_failed")+": "+err.Error())
		}
	}
	return envelope.Ok(c, fiber.StatusOK, runNowResponse{Message: text}, i18n.T(loc, "watches.run_started"))
}

// GetHistory godoc
// @Summary      Get a watch's price history
// @Description  Server-side cached for exactly as long as the watch's own schedule (see internal/httpapi.historyCache) — "data only changes when the cron fires."
// @Tags         watches
// @Security     CookieAuth
// @Produce      json
// @Param        id     path      string  true   "Watch ID"
// @Param        range  query     string  false  "Lookback window, e.g. 30d/90d/365d"  default(90d)
// @Success      200    {object}  v1.History
// @Failure      500    {string}  string  "list price samples"
// @Router       /watches/{id}/history [get]
func (a *API) GetHistory(c fiber.Ctx) error {
	id := domain.WatchID(c.Params("id"))
	days := parseRangeDays(c.Query("range", "90d"))

	h, err := a.Watches.History(c, id, days)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, i18n.T(i18n.From(c), "watches.history_failed"))
	}

	if h.LastUpdated != nil {
		c.Set("X-Data-Fetched-At", h.LastUpdated.Format(time.RFC3339))
	}
	if !h.NextRun.IsZero() {
		c.Set("X-Next-Run", h.NextRun.Format(time.RFC3339))
	}

	resp := v1.History{Samples: v1.PriceSamplesOf(h.Samples), LastUpdatedAt: h.LastUpdated}
	if !h.NextRun.IsZero() {
		resp.NextRun = &h.NextRun
	}
	return envelope.Ok(c, fiber.StatusOK, resp, i18n.T(i18n.From(c), "watches.history"))
}

func parseRangeDays(r string) int {
	switch r {
	case "7d":
		return 7
	case "30d":
		return 30
	case "all":
		return 3650
	default:
		return 90
	}
}
