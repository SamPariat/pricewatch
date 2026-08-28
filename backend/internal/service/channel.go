package service

import (
	"context"

	"github.com/SamPariat/pricewatch/internal/domain"
)

type ChannelService struct {
	Notifier domain.Notifier
}

// Status backs the panel's "Discord connected" indicator — checks
// whether the Gateway connection is authenticated via the Notifier port
// (see internal/notify/discord.Notifier.Status), so a revoked or wrong
// bot token shows up immediately rather than only being discovered when
// a digest silently fails to send.
func (s *ChannelService) Status(ctx context.Context) (domain.NotifierStatus, error) {
	return s.Notifier.Status(ctx)
}
