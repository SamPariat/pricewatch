package ai

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/sampariat/prices-reminder/internal/domain"
)

// cacheTTL is deliberately flat and long rather than schedule-derived like
// providers.WithCache: the cache key already includes the full prompt
// text, and a digest's prompt changes whenever the price it's describing
// changes — so a stale hit is only possible for a genuinely repeated
// prompt (e.g. a retried run within the same fetch), which is exactly the
// case worth collapsing. See PLAN.md § AI features guardrails: "Cache LLM
// responses on the same schedule-derived TTL" — this is a simpler
// approximation of that intent, chosen because domain.LLM.Complete has no
// watch ID to derive a per-watch TTL from the way domain.Provider does.
const cacheTTL = 6 * time.Hour

type cachingLLM struct {
	next  domain.LLM
	cache domain.Cache
}

// WithCache memoizes identical prompts so a retried or duplicated call
// within cacheTTL doesn't spend another request against a rate-limited
// free tier.
func WithCache(next domain.LLM, cache domain.Cache) domain.LLM {
	return &cachingLLM{next: next, cache: cache}
}

func (c *cachingLLM) Complete(ctx context.Context, p domain.Prompt) (string, error) {
	key := "llm:" + promptHash(p)
	raw, err := c.cache.GetOrLoad(ctx, key, cacheTTL, func() ([]byte, error) {
		text, err := c.next.Complete(ctx, p)
		if err != nil {
			return nil, err
		}
		return []byte(text), nil
	})
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func promptHash(p domain.Prompt) string {
	h := sha256.New()
	h.Write([]byte(p.System))
	h.Write([]byte{0})
	h.Write([]byte(p.User))
	return hex.EncodeToString(h.Sum(nil))
}
