package handlers

import "github.com/gofiber/fiber/v3"

// GetChannelStatus backs the panel's "Telegram connected" indicator —
// calls the real Telegram getMe endpoint via the Notifier port, so a
// revoked or wrong bot token shows up immediately rather than only being
// discovered when a digest silently fails to send.
func (a *API) GetChannelStatus(c fiber.Ctx) error {
	status, err := a.Notifier.Status(c)
	if err != nil {
		return fiber.NewError(fiber.StatusBadGateway, "check channel status")
	}
	return c.JSON(fiber.Map{"status": string(status)})
}
