// Package noop is a Null Object Notifier — dry-run mode is "swap the
// adapter", not an if-branch scattered through the scheduler and pipeline.
// See PLAN.md § Architecture patterns.
package noop

import (
	"context"
	"log/slog"

	"github.com/sampariat/prices-reminder/internal/domain"
)

type Notifier struct{}

func New() *Notifier { return &Notifier{} }

func (n *Notifier) Send(ctx context.Context, t domain.Target, m domain.Message) error {
	slog.InfoContext(ctx, "noop notifier: message not sent", "chars", len(m.Text), "buttons", len(m.Buttons))
	return nil
}

func (n *Notifier) Status(ctx context.Context) (domain.NotifierStatus, error) {
	return domain.NotifierDisconnected, nil
}
