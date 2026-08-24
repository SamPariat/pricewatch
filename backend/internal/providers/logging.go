package providers

import (
	"context"
	"time"

	"github.com/sampariat/prices-reminder/internal/domain"
	"github.com/sampariat/prices-reminder/internal/logging"
)

type loggingProvider struct {
	next domain.Provider
}

// WithLogging logs one line per Fetch call carrying whatever run_id/
// watch_id the context already has attached (see internal/logging.With),
// so a provider failure shows up in that run's timeline without the
// adapter itself needing to know about logging at all.
func WithLogging(next domain.Provider) domain.Provider {
	return &loggingProvider{next: next}
}

func (l *loggingProvider) Kind() domain.AssetKind { return l.next.Kind() }

func (l *loggingProvider) Fetch(ctx context.Context, w domain.Watch) ([]domain.Quote, error) {
	start := time.Now()
	quotes, err := l.next.Fetch(ctx, w)
	dur := time.Since(start)

	log := logging.From(ctx)
	if err != nil {
		log.Warn("provider fetch failed",
			"provider", l.next.Kind(), "watch_id", w.ID, "duration_ms", dur.Milliseconds(), "error", err)
		return nil, err
	}
	log.Info("provider fetch",
		"provider", l.next.Kind(), "watch_id", w.ID, "duration_ms", dur.Milliseconds(), "quotes", len(quotes))
	return quotes, nil
}
