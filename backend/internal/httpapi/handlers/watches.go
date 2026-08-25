package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/oklog/ulid/v2"
	"github.com/robfig/cron/v3"

	"github.com/sampariat/prices-reminder/internal/analytics"
	"github.com/sampariat/prices-reminder/internal/domain"
	v1 "github.com/sampariat/prices-reminder/internal/httpapi/presenter/v1"
	"github.com/sampariat/prices-reminder/internal/logging"
	"github.com/sampariat/prices-reminder/internal/providers"
	"github.com/sampariat/prices-reminder/internal/render"
)

// priceHistoryWindow covers the 90-day percentile-rank window (PLAN.md's
// "cheaper than N% of the last 90 days") plus a few days of slack so a
// sample right at the boundary isn't dropped by a same-day rounding edge.
const priceHistoryWindow = 95 * 24 * time.Hour

type watchRequest struct {
	Name         string          `json:"name"`
	Kind         string          `json:"kind"`
	Enabled      *bool           `json:"enabled"`
	CronExpr     string          `json:"cron_expr"`
	Timezone     string          `json:"timezone"`
	Params       json.RawMessage `json:"params"`
	ThresholdPct float64         `json:"threshold_pct"`
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
	watches, err := a.Repo.ListWatches(c)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "list watches")
	}

	out := make([]v1.Watch, len(watches))
	for i, w := range watches {
		out[i] = a.toDTO(c, w)
	}
	return c.JSON(out)
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
	id := domain.WatchID(c.Params("id"))
	w, err := a.Repo.GetWatch(c, id)
	if err != nil {
		return fiber.NewError(fiber.StatusNotFound, "watch not found")
	}
	return c.JSON(a.toDTO(c, w))
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
	var req watchRequest
	if err := c.Bind().Body(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}

	w := domain.Watch{
		Name: req.Name, Kind: domain.AssetKind(req.Kind), Enabled: true,
		CronExpr: req.CronExpr, Timezone: req.Timezone, Params: req.Params, ThresholdPct: req.ThresholdPct,
	}
	if req.Enabled != nil {
		w.Enabled = *req.Enabled
	}
	if err := validateWatch(w); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	created, err := a.Repo.CreateWatch(c, w)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "create watch")
	}

	if err := a.Sched.Reload(c); err != nil {
		logging.From(c).Error("reload after create", "error", err)
	}

	// Backfill so the chart isn't empty for a week (PLAN.md § Data
	// model). Detached from the request context — the fetch should
	// finish even after the response has already gone back.
	go a.runNow(context.Background(), created, false)

	return c.Status(fiber.StatusCreated).JSON(a.toDTO(c, created))
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
	id := domain.WatchID(c.Params("id"))
	existing, err := a.Repo.GetWatch(c, id)
	if err != nil {
		return fiber.NewError(fiber.StatusNotFound, "watch not found")
	}

	var req watchRequest
	if err := c.Bind().Body(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}

	updated := existing
	updated.Name = req.Name
	updated.Kind = domain.AssetKind(req.Kind)
	updated.CronExpr = req.CronExpr
	updated.Timezone = req.Timezone
	updated.Params = req.Params
	updated.ThresholdPct = req.ThresholdPct
	if req.Enabled != nil {
		updated.Enabled = *req.Enabled
	}
	if err := validateWatch(updated); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	saved, err := a.Repo.UpdateWatch(c, updated)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "update watch")
	}

	if err := a.Sched.Reload(c); err != nil {
		logging.From(c).Error("reload after update", "error", err)
	}

	return c.JSON(a.toDTO(c, saved))
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
	if err := a.Repo.DeleteWatch(c, id); err != nil {
		return fiber.NewError(fiber.StatusNotFound, "watch not found")
	}
	if err := a.Sched.Reload(c); err != nil {
		logging.From(c).Error("reload after delete", "error", err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

type runNowResponse struct {
	Message string `json:"message"`
}

// RunWatchNow godoc
// @Summary      Run a watch now
// @Description  Runs the pipeline immediately, outside the normal schedule, bypassing the provider cache so it always fetches fresh (see providers.SkipCache). Honours the global dry-run setting for whether the result actually gets sent to Telegram, matching the scheduled path's own behavior.
// @Tags         watches
// @Security     CookieAuth
// @Produce      json
// @Param        id   path      string  true  "Watch ID"
// @Success      200  {object}  runNowResponse
// @Failure      404  {string}  string  "watch not found"
// @Failure      502  {string}  string  "run failed: <cause>"
// @Router       /watches/{id}/run [post]
func (a *API) RunWatchNow(c fiber.Ctx) error {
	id := domain.WatchID(c.Params("id"))
	w, err := a.Repo.GetWatch(c, id)
	if err != nil {
		return fiber.NewError(fiber.StatusNotFound, "watch not found")
	}

	text, err := a.runNow(c, w, true)
	if err != nil {
		return fiber.NewError(fiber.StatusBadGateway, "run failed: "+err.Error())
	}
	return c.JSON(runNowResponse{Message: text})
}

func (a *API) runNow(ctx context.Context, w domain.Watch, send bool) (string, error) {
	runID := domain.RunID(ulid.Make().String())
	rctx := logging.With(ctx, "run_id", string(runID), "watch_id", string(w.ID))
	rctx = providers.SkipCache(rctx)
	text, err := a.Pipeline.RunWatch(rctx, runID, w)
	if err != nil {
		return "", err
	}
	if send {
		settings, serr := a.Repo.GetSettings(ctx)
		if serr == nil && !settings.DryRun && settings.TelegramChatID != "" {
			msg := domain.Message{Text: render.CombineDigest([]string{text})}
			if err := a.Notifier.Send(ctx, domain.Target{ChatID: settings.TelegramChatID}, msg); err != nil {
				logging.From(rctx).Error("run-now: send failed", "error", err)
			}
		}
	}
	return text, nil
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
	since := time.Now().AddDate(0, 0, -days)

	samples, err := a.Repo.ListPriceSamples(c, id, since)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "list price samples")
	}

	nextRun := a.Sched.NextRun(id)
	var lastUpdated *time.Time
	if len(samples) > 0 {
		latest := samples[0]
		for _, s := range samples[1:] {
			if s.UpdatedAt.After(latest.UpdatedAt) {
				latest = s
			}
		}
		t := latest.UpdatedAt
		lastUpdated = &t
		c.Set("X-Data-Fetched-At", t.Format(time.RFC3339))
	}
	if !nextRun.IsZero() {
		c.Set("X-Next-Run", nextRun.Format(time.RFC3339))
	}

	resp := v1.History{Samples: v1.PriceSamplesOf(samples), LastUpdatedAt: lastUpdated}
	if !nextRun.IsZero() {
		resp.NextRun = &nextRun
	}
	return c.JSON(resp)
}

// toDTO enriches a domain.Watch with the freshness/staleness/schedule/
// price data that only exists outside the watches table — WatchState,
// Scheduler.NextRun/Interval, and a PriceSummary computed from
// price_samples via the same internal/analytics functions the digest
// uses, so the panel and the Telegram message can never disagree about a
// delta or a percentile.
func (a *API) toDTO(c fiber.Ctx, w domain.Watch) v1.Watch {
	state, err := a.Repo.GetWatchState(c, w.ID)
	if err != nil {
		state = domain.WatchState{WatchID: w.ID}
	}

	settings, err := a.Repo.GetSettings(c)
	currency := "INR"
	if err == nil && settings.Currency != "" {
		currency = settings.Currency
	}

	lastUpdated, price := a.priceSummary(c, w.ID, currency)
	return v1.WatchOf(w, state, a.Sched.NextRun(w.ID), a.Sched.Interval(w.ID), lastUpdated, price)
}

// priceSummary loads this watch's price history once and derives both
// its freshness timestamp and its current-price/delta/percentile summary
// from the same query — one round trip, not two.
func (a *API) priceSummary(ctx context.Context, id domain.WatchID, currency string) (*time.Time, *v1.PriceSummary) {
	samples, err := a.Repo.ListPriceSamples(ctx, id, a.Clock.Now().Add(-priceHistoryWindow))
	if err != nil || len(samples) == 0 {
		return nil, nil
	}

	latest := samples[0]
	for _, s := range samples[1:] {
		if s.UpdatedAt.After(latest.UpdatedAt) {
			latest = s
		}
	}
	lastUpdated := latest.UpdatedAt

	summary := &v1.PriceSummary{PriceMinor: latest.MinMinor, Currency: currency}
	if d := analytics.DeltaVsYesterday(samples, a.Clock.Now()); d.OK {
		pct := d.Pct
		summary.DeltaPct = &pct
	}
	if pct, ok := analytics.PercentileRank(samples, latest.MinMinor, 90, a.Clock.Now()); ok {
		summary.Percentile = &pct
	}

	return &lastUpdated, summary
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

func validateWatch(w domain.Watch) error {
	if !w.Kind.Valid() {
		return fmt.Errorf("invalid kind %q", w.Kind)
	}
	if _, err := cron.ParseStandard(w.CronExpr); err != nil {
		return fmt.Errorf("invalid cron_expr: %w", err)
	}
	if w.Timezone != "" {
		if _, err := time.LoadLocation(w.Timezone); err != nil {
			return fmt.Errorf("invalid timezone: %w", err)
		}
	}
	switch {
	case w.Kind.IsFlight():
		p, err := w.DecodeFlightParams()
		if err != nil {
			return fmt.Errorf("invalid params for kind %q: %w", w.Kind, err)
		}
		// json.Unmarshal silently ignores fields it doesn't recognize —
		// hotel-shaped params ({"location":...}) decode into a
		// FlightParams{} with every field empty without erroring, so
		// decoding alone doesn't prove the params actually matched the
		// kind. Checking the required fields landed does.
		if p.Origin == "" || p.Destination == "" || p.DepartDate == "" {
			return fmt.Errorf("invalid params for kind %q: origin, destination, and depart_date are required", w.Kind)
		}
		if w.Kind == domain.AssetFlightReturn && (p.ReturnDate == nil || *p.ReturnDate == "") {
			return fmt.Errorf("invalid params for kind %q: return_date is required", w.Kind)
		}
	case w.Kind == domain.AssetHotel || w.Kind == domain.AssetRental:
		p, err := w.DecodeHotelParams()
		if err != nil {
			return fmt.Errorf("invalid params for kind %q: %w", w.Kind, err)
		}
		if p.Location == "" || p.CheckIn == "" || p.CheckOut == "" {
			return fmt.Errorf("invalid params for kind %q: location, check_in, and check_out are required", w.Kind)
		}
	}
	return nil
}
