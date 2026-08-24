package handlers

import (
	"github.com/gofiber/fiber/v3"

	v1 "github.com/sampariat/prices-reminder/internal/httpapi/presenter/v1"
)

func (a *API) GetSettings(c fiber.Ctx) error {
	s, err := a.Repo.GetSettings(c)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "get settings")
	}
	return c.JSON(v1.SettingsOf(s))
}

// UpdateSettings replaces every field — the panel's settings form always
// submits the full object, so there's no partial-merge ambiguity to
// resolve here despite the route being PATCH.
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

func (a *API) GetMeta(c fiber.Ctx) error {
	return c.JSON(v1.Meta{MinVersion: a.MinVersion, MaxVersion: a.MaxVersion})
}
