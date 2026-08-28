// Package noop is a Null Object Notifier — dry-run mode is "swap the
// adapter", not an if-branch scattered through the scheduler and pipeline.
// See PLAN.md § Architecture patterns.
package noop

import (
	"context"

	"github.com/SamPariat/pricewatch/internal/domain"
	"github.com/SamPariat/pricewatch/internal/logging"
)

type Notifier struct{}

func New() *Notifier { return &Notifier{} }

func (n *Notifier) Send(ctx context.Context, t domain.Target, m domain.Message) error {
	logging.From(ctx).Info().
		Int("embeds", len(m.Embeds)).
		Int("buttons", len(m.Buttons)).
		Msg("noop notifier: message not sent")
	return nil
}

func (n *Notifier) Status(ctx context.Context) (domain.NotifierStatus, error) {
	return domain.NotifierDisconnected, nil
}
