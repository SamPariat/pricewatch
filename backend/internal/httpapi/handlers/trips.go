package handlers

import (
	"errors"

	"github.com/gofiber/fiber/v3"

	"github.com/SamPariat/pricewatch/internal/domain"
	"github.com/SamPariat/pricewatch/internal/httpapi/envelope"
	v1 "github.com/SamPariat/pricewatch/internal/httpapi/presenter/v1"
	"github.com/SamPariat/pricewatch/internal/i18n"
	"github.com/SamPariat/pricewatch/internal/service"
)

type tripRequest struct {
	Name     string `json:"name"`
	CronExpr string `json:"cron_expr"`
	Timezone string `json:"timezone"`
	Enabled  *bool  `json:"enabled"`
}

func (r tripRequest) enabled() bool {
	if r.Enabled == nil {
		return true
	}
	return *r.Enabled
}

// legsOf fetches every watch and filters to those belonging to id — a
// full list-and-filter rather than a dedicated repository query, since
// the panel needs the same "all watches" data for its own Watches/Stays
// views anyway and this keeps the query surface small.
func (a *API) legsOf(c fiber.Ctx, id domain.TripID) []v1.Watch {
	details, err := a.Watches.List(c)
	if err != nil {
		return nil
	}
	var legs []v1.Watch
	for _, d := range details {
		if d.Watch.TripID == id {
			legs = append(legs, detailToDTO(d))
		}
	}
	return legs
}

// ListTrips godoc
// @Summary  List trips, each with its legs
// @Tags     trips
// @Security CookieAuth
// @Produce  json
// @Success  200  {array}  v1.Trip
// @Router   /trips [get]
func (a *API) ListTrips(c fiber.Ctx) error {
	loc := i18n.From(c)
	trips, err := a.Trips.List(c)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, i18n.T(loc, "trips.list_failed"))
	}
	out := make([]v1.Trip, len(trips))
	for i, t := range trips {
		out[i] = v1.TripOf(t, a.legsOf(c, t.ID))
	}
	return envelope.Ok(c, fiber.StatusOK, out, i18n.T(loc, "trips.listed"))
}

// GetTrip godoc
// @Summary  Get a trip and its legs
// @Tags     trips
// @Security CookieAuth
// @Produce  json
// @Param    id  path  string  true  "Trip ID"
// @Success  200  {object}  v1.Trip
// @Failure  404  {string}  string  "trip not found"
// @Router   /trips/{id} [get]
func (a *API) GetTrip(c fiber.Ctx) error {
	loc := i18n.From(c)
	id := domain.TripID(c.Params("id"))
	t, err := a.Trips.Get(c, id)
	if err != nil {
		return fiber.NewError(fiber.StatusNotFound, i18n.T(loc, "trips.not_found"))
	}
	return envelope.Ok(c, fiber.StatusOK, v1.TripOf(t, a.legsOf(c, id)), i18n.T(loc, "trips.found"))
}

// CreateTrip godoc
// @Summary  Create a trip
// @Tags     trips
// @Security CookieAuth
// @Accept   json
// @Produce  json
// @Param    body  body  tripRequest  true  "Trip fields"
// @Success  201  {object}  v1.Trip
// @Failure  400  {string}  string  "invalid request body, or a validation error naming the bad field"
// @Router   /trips [post]
func (a *API) CreateTrip(c fiber.Ctx) error {
	loc := i18n.From(c)
	var req tripRequest
	if err := c.Bind().Body(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, i18n.T(loc, "common.invalid_body"))
	}
	t, err := a.Trips.Create(c, req.Name, req.CronExpr, req.Timezone, req.enabled())
	if err != nil {
		var ve service.ValidationError
		if errors.As(err, &ve) {
			return fiber.NewError(fiber.StatusBadRequest, ve.Error())
		}
		return fiber.NewError(fiber.StatusInternalServerError, i18n.T(loc, "trips.create_failed"))
	}
	return envelope.Ok(c, fiber.StatusCreated, v1.TripOf(t, nil), i18n.T(loc, "trips.created"))
}

// UpdateTrip godoc
// @Summary  Replace a trip
// @Tags     trips
// @Security CookieAuth
// @Accept   json
// @Produce  json
// @Param    id    path  string       true  "Trip ID"
// @Param    body  body  tripRequest  true  "Full trip fields"
// @Success  200  {object}  v1.Trip
// @Failure  404  {string}  string  "trip not found"
// @Router   /trips/{id} [patch]
func (a *API) UpdateTrip(c fiber.Ctx) error {
	loc := i18n.From(c)
	id := domain.TripID(c.Params("id"))
	var req tripRequest
	if err := c.Bind().Body(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, i18n.T(loc, "common.invalid_body"))
	}
	t, err := a.Trips.Update(c, id, req.Name, req.CronExpr, req.Timezone, req.enabled())
	if err != nil {
		switch {
		case errors.Is(err, service.ErrNotFound):
			return fiber.NewError(fiber.StatusNotFound, i18n.T(loc, "trips.not_found"))
		default:
			var ve service.ValidationError
			if errors.As(err, &ve) {
				return fiber.NewError(fiber.StatusBadRequest, ve.Error())
			}
			return fiber.NewError(fiber.StatusInternalServerError, i18n.T(loc, "trips.update_failed"))
		}
	}
	return envelope.Ok(c, fiber.StatusOK, v1.TripOf(t, a.legsOf(c, id)), i18n.T(loc, "trips.updated"))
}

// DeleteTrip godoc
// @Summary  Delete a trip and every leg in it
// @Tags     trips
// @Security CookieAuth
// @Param    id  path  string  true  "Trip ID"
// @Success  204  "no content"
// @Failure  404  {string}  string  "trip not found"
// @Router   /trips/{id} [delete]
func (a *API) DeleteTrip(c fiber.Ctx) error {
	id := domain.TripID(c.Params("id"))
	if err := a.Trips.Delete(c, id); err != nil {
		return fiber.NewError(fiber.StatusNotFound, i18n.T(i18n.From(c), "trips.not_found"))
	}
	return c.SendStatus(fiber.StatusNoContent)
}
