package providers

import (
	"context"
	"time"

	"github.com/sampariat/prices-reminder/internal/domain"
)

type retryProvider struct {
	next       domain.Provider
	maxRetries int
	baseDelay  time.Duration
}

// WithRetry retries a failed Fetch up to maxRetries times with exponential
// backoff, respecting ctx cancellation between attempts.
func WithRetry(next domain.Provider, maxRetries int, baseDelay time.Duration) domain.Provider {
	return &retryProvider{next: next, maxRetries: maxRetries, baseDelay: baseDelay}
}

func (r *retryProvider) Kind() domain.AssetKind { return r.next.Kind() }

func (r *retryProvider) Fetch(ctx context.Context, w domain.Watch) ([]domain.Quote, error) {
	var lastErr error
	for attempt := 0; attempt <= r.maxRetries; attempt++ {
		if attempt > 0 {
			delay := r.baseDelay * time.Duration(1<<uint(attempt-1))
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}
		quotes, err := r.next.Fetch(ctx, w)
		if err == nil {
			return quotes, nil
		}
		lastErr = err
	}
	return nil, lastErr
}
