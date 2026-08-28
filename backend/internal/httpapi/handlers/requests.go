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

// ListRequests godoc
// @Summary  List Discord-submitted change requests
// @Tags     requests
// @Security CookieAuth
// @Produce  json
// @Param    status  query  string  false  "pending|approved|rejected — omit for all"
// @Success  200  {array}  v1.Request
// @Router   /requests [get]
func (a *API) ListRequests(c fiber.Ctx) error {
	loc := i18n.From(c)
	status := domain.RequestStatus(c.Query("status"))
	reqs, err := a.Requests.List(c, status)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, i18n.T(loc, "requests.list_failed"))
	}
	out := make([]v1.Request, len(reqs))
	for i, r := range reqs {
		out[i] = v1.RequestOf(r)
	}
	return envelope.Ok(c, fiber.StatusOK, out, i18n.T(loc, "requests.listed"))
}

// ApproveRequest godoc
// @Summary  Approve a pending request — applies it via the normal WatchService/TripService methods
// @Tags     requests
// @Security CookieAuth
// @Param    id  path  string  true  "Request ID"
// @Success  204  "no content"
// @Failure  400  {string}  string  "the underlying action failed validation — the request stays pending"
// @Failure  404  {string}  string  "request not found"
// @Router   /requests/{id}/approve [post]
func (a *API) ApproveRequest(c fiber.Ctx) error {
	loc := i18n.From(c)
	id := domain.RequestID(c.Params("id"))
	if err := a.Requests.Approve(c, id); err != nil {
		switch {
		case errors.Is(err, service.ErrNotFound):
			return fiber.NewError(fiber.StatusNotFound, i18n.T(loc, "requests.not_found"))
		default:
			var ve service.ValidationError
			if errors.As(err, &ve) {
				return fiber.NewError(fiber.StatusBadRequest, ve.Error())
			}
			// err.Error() stays untranslated by design — same reasoning
			// as RunWatchNow: it's the underlying service call's own
			// failure cause, not app-authored prose.
			return fiber.NewError(fiber.StatusBadRequest, i18n.T(loc, "requests.approve_failed")+": "+err.Error())
		}
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// RejectRequest godoc
// @Summary  Reject a pending request — no side effect beyond marking it rejected
// @Tags     requests
// @Security CookieAuth
// @Param    id  path  string  true  "Request ID"
// @Success  204  "no content"
// @Failure  404  {string}  string  "request not found"
// @Router   /requests/{id}/reject [post]
func (a *API) RejectRequest(c fiber.Ctx) error {
	id := domain.RequestID(c.Params("id"))
	if err := a.Requests.Reject(c, id); err != nil {
		return fiber.NewError(fiber.StatusNotFound, i18n.T(i18n.From(c), "requests.not_found"))
	}
	return c.SendStatus(fiber.StatusNoContent)
}
