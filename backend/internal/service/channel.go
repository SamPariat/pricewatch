package service

import (
	"context"

	"github.com/SamPariat/pricewatch/internal/domain"
)

type ChannelService struct {
	Notifier domain.Notifier
}

// Status backs the panel's "Telegram connected" indicator — calls the
// real Telegram getMe endpoint via the Notifier port, so a revoked or
// wrong bot token shows up immediately rather than only being discovered
// when a digest silently fails to send.
func (s *ChannelService) Status(ctx context.Context) (domain.NotifierStatus, error) {
	return s.Notifier.Status(ctx)
}
