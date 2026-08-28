package handlers

import (
	"github.com/gofiber/fiber/v3"

	"github.com/SamPariat/pricewatch/internal/httpapi/envelope"
	"github.com/SamPariat/pricewatch/internal/i18n"
)

type channelStatusResponse struct {
	// Status is domain.NotifierStatus — "linked" or "disconnected".
	Status string `json:"status" example:"linked"`
}

// GetChannelStatus godoc
// @Summary      Notifier channel status
// @Description  Backs the panel's "Discord connected" indicator — checks whether the Gateway connection is authenticated via the Notifier port, so a revoked or wrong bot token shows up immediately rather than only being discovered when a digest silently fails to send.
// @Tags         channel
// @Security     CookieAuth
// @Produce      json
// @Success      200  {object}  channelStatusResponse
// @Failure      502  {string}  string  "check channel status"
// @Router       /channel/status [get]
func (a *API) GetChannelStatus(c fiber.Ctx) error {
	loc := i18n.From(c)
	status, err := a.Channel.Status(c)
	if err != nil {
		return fiber.NewError(fiber.StatusBadGateway, i18n.T(loc, "channel.check_failed"))
	}
	return envelope.Ok(c, fiber.StatusOK, channelStatusResponse{Status: string(status)}, i18n.T(loc, "channel.status_ok"))
}
