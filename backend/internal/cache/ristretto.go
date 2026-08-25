// Package cache implements domain.Cache with an in-process Ristretto
// store. PLAN.md is explicit that this system runs as one process on one
// VM, so an in-process cache is strictly faster and simpler than Redis —
// Redis would only earn its keep if this scaled to multiple app
// containers, which it doesn't.
package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/dgraph-io/ristretto/v2"

	"github.com/SamPariat/pricewatch/internal/domain"
)

type Ristretto struct {
	c *ristretto.Cache[string, []byte]
}

func NewRistretto() (*Ristretto, error) {
	c, err := ristretto.NewCache(&ristretto.Config[string, []byte]{
		NumCounters: 10_000,
		MaxCost:     32 << 20, // 32MB — this is a cache for JSON quote payloads, not chart images
		BufferItems: 64,
	})
	if err != nil {
		return nil, fmt.Errorf("cache: create ristretto cache: %w", err)
	}
	return &Ristretto{c: c}, nil
}

// GetOrLoad implements domain.Cache. It does not itself deduplicate
// concurrent misses on the same key — that's providers.WithSingleflight's
// job, layered outside this cache in the decorator stack (see
// PLAN.md § Caching) — so this stays a plain cache, not a second
// singleflight implementation.
func (r *Ristretto) GetOrLoad(ctx context.Context, key string, ttl time.Duration, load domain.Loader) ([]byte, error) {
	if v, ok := r.c.Get(key); ok {
		return v, nil
	}
	v, err := load()
	if err != nil {
		return nil, err
	}
	r.c.SetWithTTL(key, v, int64(len(v)), ttl)
	return v, nil
}

// Invalidate implements domain.Cache. Ristretto's Del is a best-effort,
// eventually-applied op (it queues onto the same internal ring buffer as
// Set) — fine here, since the only caller is a manual "run now" that's
// about to overwrite this key with a fresh GetOrLoad anyway.
func (r *Ristretto) Invalidate(ctx context.Context, key string) error {
	r.c.Del(key)
	return nil
}
