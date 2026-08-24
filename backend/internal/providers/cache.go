package providers

import (
	"context"
	"encoding/json"
	"time"

	"github.com/sampariat/prices-reminder/internal/domain"
	"github.com/sampariat/prices-reminder/internal/logging"
)

type cacheProvider struct {
	next   domain.Provider
	cache  domain.Cache
	ttlFor func(domain.WatchID) time.Duration
}

// WithCache serves a watch's Fetch result from cache until the TTL
// supplied by ttlFor expires. ttlFor is a function rather than a fixed
// duration because the real TTL is "time until this watch's next cron
// fire" (Scheduler.TTL, wired in once the scheduler exists) — data only
// changes when the cron runs, so the cache should too.
func WithCache(next domain.Provider, cache domain.Cache, ttlFor func(domain.WatchID) time.Duration) domain.Provider {
	return &cacheProvider{next: next, cache: cache, ttlFor: ttlFor}
}

func (c *cacheProvider) Kind() domain.AssetKind { return c.next.Kind() }

func (c *cacheProvider) Fetch(ctx context.Context, w domain.Watch) ([]domain.Quote, error) {
	key := "provider:" + string(c.next.Kind()) + ":" + string(w.ID)
	ttl := c.ttlFor(w.ID)

	missed := false
	raw, err := c.cache.GetOrLoad(ctx, key, ttl, func() ([]byte, error) {
		missed = true
		quotes, err := c.next.Fetch(ctx, w)
		if err != nil {
			return nil, err
		}
		return json.Marshal(quotes)
	})
	if err != nil {
		return nil, err
	}
	logging.From(ctx).Debug("provider cache", "key", key, "ttl", ttl, "hit", !missed)

	var quotes []domain.Quote
	if err := json.Unmarshal(raw, &quotes); err != nil {
		return nil, err
	}
	return quotes, nil
}
