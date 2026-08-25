package handlers

import (
	"github.com/gofiber/fiber/v3"

	v1 "github.com/sampariat/prices-reminder/internal/httpapi/presenter/v1"
)

// GetSettings godoc
// @Summary  Get the singleton settings
// @Tags     settings
// @Security CookieAuth
// @Produce  json
// @Success  200  {object}  v1.Settings
// @Failure  500  {string}  string  "get settings"
// @Router   /settings [get]
func (a *API) GetSettings(c fiber.Ctx) error {
	s, err := a.Repo.GetSettings(c)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "get settings")
	}
	return c.JSON(v1.SettingsOf(s))
}

// UpdateSettings godoc
// @Summary      Replace the singleton settings
// @Description  Full-replace semantics despite the PATCH verb — the panel's settings form always submits every field.
// @Tags         settings
// @Security     CookieAuth
// @Accept       json
// @Produce      json
// @Param        body  body      v1.Settings  true  "Full settings"
// @Success      200   {object}  v1.Settings
// @Failure      400   {string}  string  "invalid request body"
// @Failure      500   {string}  string  "update settings"
// @Router       /settings [patch]
func (a *API) UpdateSettings(c fiber.Ctx) error {
	var req v1.Settings
	if err := c.Bind().Body(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}
	if err := a.Repo.UpdateSettings(c, req.ToDomain()); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "update settings")
	}
	return c.JSON(req)
}

// GetMeta godoc
// @Summary      Supported API version range
// @Description  Unauthenticated — the one route (besides /auth/login) a client needs to work before it has a session.
// @Tags         meta
// @Produce      json
// @Success      200  {object}  v1.Meta
// @Router       /meta [get]
func (a *API) GetMeta(c fiber.Ctx) error {
	return c.JSON(v1.Meta{MinVersion: a.MinVersion, MaxVersion: a.MaxVersion})
}
