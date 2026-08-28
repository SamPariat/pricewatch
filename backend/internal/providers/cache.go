package providers

import (
	"context"
	"encoding/json"
	"time"

	"github.com/SamPariat/pricewatch/internal/domain"
	"github.com/SamPariat/pricewatch/internal/logging"
)

type skipCacheKey struct{}

// SkipCache marks ctx so a WithCache-wrapped provider treats this one
// Fetch as a forced refresh: the existing cache entry (if any) is
// invalidated first, so GetOrLoad genuinely misses and calls the real
// upstream instead of replaying whatever is already cached — including a
// previously empty or wrong result, which a plain TTL-respecting read
// would otherwise keep serving until it naturally expires.
//
// Used by exactly the two "the user explicitly asked for this right now"
// paths — handlers.API.runNow and discordbot.Listener.refresh — never by
// the scheduler's routine fires, which are supposed to respect the cache
// (PLAN.md § Caching: "data only changes when the cron fires").
func SkipCache(ctx context.Context) context.Context {
	return context.WithValue(ctx, skipCacheKey{}, true)
}

func skipCache(ctx context.Context) bool {
	v, _ := ctx.Value(skipCacheKey{}).(bool)
	return v
}

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

	if skipCache(ctx) {
		if err := c.cache.Invalidate(ctx, key); err != nil {
			logging.From(ctx).Warn().Str("key", key).Err(err).Msg("provider cache: invalidate for forced refresh")
		}
	}

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
	logging.From(ctx).Debug().Str("key", key).Dur("ttl", ttl).Bool("hit", !missed).Msg("provider cache")

	var quotes []domain.Quote
	if err := json.Unmarshal(raw, &quotes); err != nil {
		return nil, err
	}
	return quotes, nil
}
