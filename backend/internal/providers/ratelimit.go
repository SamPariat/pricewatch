package providers

import (
	"context"

	"golang.org/x/time/rate"

	"github.com/SamPariat/pricewatch/internal/domain"
)

type rateLimitProvider struct {
	next    domain.Provider
	limiter *rate.Limiter
}

// WithRateLimit blocks until a token is available rather than rejecting —
// a watch's fetch should wait a beat under load, not fail, since this is
// a background cron job with no human waiting on the response.
func WithRateLimit(next domain.Provider, requestsPerMinute int) domain.Provider {
	limit := rate.Limit(float64(requestsPerMinute) / 60.0)
	return &rateLimitProvider{next: next, limiter: rate.NewLimiter(limit, 1)}
}

func (r *rateLimitProvider) Kind() domain.AssetKind { return r.next.Kind() }

func (r *rateLimitProvider) Fetch(ctx context.Context, w domain.Watch) ([]domain.Quote, error) {
	if err := r.limiter.Wait(ctx); err != nil {
		return nil, err
	}
	return r.next.Fetch(ctx, w)
}
