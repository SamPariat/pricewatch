package handlers

import "github.com/gofiber/fiber/v3"

type channelStatusResponse struct {
	// Status is domain.NotifierStatus — "linked" or "disconnected".
	Status string `json:"status" example:"linked"`
}

// GetChannelStatus godoc
// @Summary      Notifier channel status
// @Description  Backs the panel's "Telegram connected" indicator — calls the real Telegram getMe endpoint via the Notifier port, so a revoked or wrong bot token shows up immediately rather than only being discovered when a digest silently fails to send.
// @Tags         channel
// @Security     CookieAuth
// @Produce      json
// @Success      200  {object}  channelStatusResponse
// @Failure      502  {string}  string  "check channel status"
// @Router       /channel/status [get]
func (a *API) GetChannelStatus(c fiber.Ctx) error {
	status, err := a.Notifier.Status(c)
	if err != nil {
		return fiber.NewError(fiber.StatusBadGateway, "check channel status")
	}
	return c.JSON(channelStatusResponse{Status: string(status)})
}
