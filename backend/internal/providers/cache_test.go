package providers

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/SamPariat/pricewatch/internal/domain"
)

// fakeCache is a minimal in-memory domain.Cache that also counts misses,
// so a test can assert on how many times the wrapped provider was
// actually called through to.
type fakeCache struct {
	store map[string][]byte
}

func newFakeCache() *fakeCache { return &fakeCache{store: make(map[string][]byte)} }

func (c *fakeCache) GetOrLoad(ctx context.Context, key string, ttl time.Duration, load domain.Loader) ([]byte, error) {
	if v, ok := c.store[key]; ok {
		return v, nil
	}
	v, err := load()
	if err != nil {
		return nil, err
	}
	c.store[key] = v
	return v, nil
}

func (c *fakeCache) Invalidate(ctx context.Context, key string) error {
	delete(c.store, key)
	return nil
}

type countingProvider struct {
	kind   domain.AssetKind
	calls  int
	quotes []domain.Quote
}

func (p *countingProvider) Kind() domain.AssetKind { return p.kind }
func (p *countingProvider) Fetch(ctx context.Context, w domain.Watch) ([]domain.Quote, error) {
	p.calls++
	return p.quotes, nil
}

func flatTTL(d time.Duration) func(domain.WatchID) time.Duration {
	return func(domain.WatchID) time.Duration { return d }
}

func TestWithCache_SecondCallSameWatch_ServedFromCache(t *testing.T) {
	inner := &countingProvider{kind: domain.AssetFlightOneWay, quotes: []domain.Quote{{PriceMinor: 100}}}
	p := WithCache(inner, newFakeCache(), flatTTL(time.Hour))
	w := domain.Watch{ID: "w1"}

	if _, err := p.Fetch(context.Background(), w); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Fetch(context.Background(), w); err != nil {
		t.Fatal(err)
	}
	if inner.calls != 1 {
		t.Errorf("inner provider called %d times, want 1 (second call should hit cache)", inner.calls)
	}
}

// TestWithCache_SkipCache_BypassesAStaleOrEmptyCachedResult locks down the
// fix for a real bug: a manual "run now" (handlers.API.runNow,
// discordbot.Listener.refresh) is supposed to always fetch fresh — but
// before SkipCache existed, it went through the exact same cached
// provider as a scheduled run, so a bad or empty result from one fetch
// (e.g. a watch with backwards depart/return dates) would keep getting
// replayed by every subsequent "run now" click until the TTL — up to 6
// hours — expired on its own.
func TestWithCache_SkipCache_BypassesAStaleOrEmptyCachedResult(t *testing.T) {
	inner := &countingProvider{kind: domain.AssetFlightOneWay, quotes: nil}
	p := WithCache(inner, newFakeCache(), flatTTL(time.Hour))
	w := domain.Watch{ID: "w1"}

	if _, err := p.Fetch(context.Background(), w); err != nil {
		t.Fatal(err)
	}
	if inner.calls != 1 {
		t.Fatalf("setup: inner provider called %d times, want 1", inner.calls)
	}

	// Now the upstream would return real data — SkipCache must still
	// reach it instead of replaying the empty result cached above.
	inner.quotes = []domain.Quote{{PriceMinor: 500}}
	quotes, err := p.Fetch(SkipCache(context.Background()), w)
	if err != nil {
		t.Fatal(err)
	}
	if inner.calls != 2 {
		t.Errorf("inner provider called %d times after SkipCache, want 2 (it must not have been served from cache)", inner.calls)
	}
	if len(quotes) != 1 || quotes[0].PriceMinor != 500 {
		t.Errorf("got %+v, want the fresh quote", quotes)
	}
}

// TestWithCache_SkipCache_RepopulatesCacheForLaterScheduledRuns ensures a
// forced refresh doesn't just bypass the cache for itself — it must leave
// the cache holding the fresh value, so the *next* scheduled fire (which
// does not set SkipCache) reads the new result instead of whatever was
// cached before the manual refresh.
func TestWithCache_SkipCache_RepopulatesCacheForLaterScheduledRuns(t *testing.T) {
	inner := &countingProvider{kind: domain.AssetFlightOneWay, quotes: nil}
	p := WithCache(inner, newFakeCache(), flatTTL(time.Hour))
	w := domain.Watch{ID: "w1"}

	p.Fetch(context.Background(), w) // seeds the cache with an empty result

	inner.quotes = []domain.Quote{{PriceMinor: 500}}
	p.Fetch(SkipCache(context.Background()), w) // forced refresh, calls=2

	quotes, err := p.Fetch(context.Background(), w) // normal call — must read the repopulated cache, not call through again
	if err != nil {
		t.Fatal(err)
	}
	if inner.calls != 2 {
		t.Errorf("inner provider called %d times, want 2 (the third Fetch should have hit the repopulated cache)", inner.calls)
	}
	if len(quotes) != 1 || quotes[0].PriceMinor != 500 {
		t.Errorf("got %+v, want the fresh quote that SkipCache wrote back", quotes)
	}
}

func TestWithCache_DifferentWatches_SeparateCacheEntries(t *testing.T) {
	inner := &countingProvider{kind: domain.AssetFlightOneWay, quotes: []domain.Quote{{PriceMinor: 100}}}
	p := WithCache(inner, newFakeCache(), flatTTL(time.Hour))

	p.Fetch(context.Background(), domain.Watch{ID: "w1"})
	p.Fetch(context.Background(), domain.Watch{ID: "w2"})

	if inner.calls != 2 {
		t.Errorf("inner provider called %d times, want 2 (different watches must not share a cache entry)", inner.calls)
	}
}

func TestWithCache_ResultRoundTripsThroughJSON(t *testing.T) {
	want := []domain.Quote{{WatchID: "w1", PriceMinor: 12345, Currency: "INR", DepartDate: "2026-12-01"}}
	inner := &countingProvider{kind: domain.AssetFlightOneWay, quotes: want}
	p := WithCache(inner, newFakeCache(), flatTTL(time.Hour))

	got, err := p.Fetch(context.Background(), domain.Watch{ID: "w1"})
	if err != nil {
		t.Fatal(err)
	}
	wantJSON, _ := json.Marshal(want)
	gotJSON, _ := json.Marshal(got)
	if string(wantJSON) != string(gotJSON) {
		t.Errorf("got %s, want %s", gotJSON, wantJSON)
	}
}
