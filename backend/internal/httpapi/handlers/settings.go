package handlers

import (
	"github.com/gofiber/fiber/v3"

	"github.com/SamPariat/pricewatch/internal/httpapi/envelope"
	v1 "github.com/SamPariat/pricewatch/internal/httpapi/presenter/v1"
	"github.com/SamPariat/pricewatch/internal/i18n"
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
	loc := i18n.From(c)
	s, err := a.Settings.Get(c)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, i18n.T(loc, "settings.get_failed"))
	}
	return envelope.Ok(c, fiber.StatusOK, v1.SettingsOf(s), i18n.T(loc, "settings.get_ok"))
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
	loc := i18n.From(c)
	var req v1.Settings
	if err := c.Bind().Body(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, i18n.T(loc, "common.invalid_body"))
	}
	saved, err := a.Settings.Update(c, req.ToDomain())
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, i18n.T(loc, "settings.update_failed"))
	}
	return envelope.Ok(c, fiber.StatusOK, v1.SettingsOf(saved), i18n.T(loc, "settings.update_ok"))
}

// GetMeta godoc
// @Summary      Supported API version
// @Description  Unauthenticated — the one route (besides /auth/login) a client needs to work before it has a session.
// @Tags         meta
// @Produce      json
// @Success      200  {object}  v1.Meta
// @Router       /meta [get]
func (a *API) GetMeta(c fiber.Ctx) error {
	return envelope.Ok(c, fiber.StatusOK, v1.Meta{CurrentVersion: APIVersion, SupportedVersions: []string{APIVersion}}, i18n.T(i18n.From(c), "meta.ok"))
}
