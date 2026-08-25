package ai

import (
	"context"
	"testing"
	"time"

	"github.com/SamPariat/pricewatch/internal/domain"
)

// fakeCache is a minimal in-memory domain.Cache — good enough to prove
// WithCache calls through on a miss and skips the wrapped LLM on a hit;
// it ignores ttl entirely since no test here needs expiry.
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

func TestWithCache_RepeatedIdenticalPrompt_CallsUnderlyingOnce(t *testing.T) {
	inner := &fakeLLM{text: "a punchy reaction"}
	llm := WithCache(inner, newFakeCache())

	prompt := domain.Prompt{System: "sys", User: "user text"}
	for range 3 {
		got, err := llm.Complete(context.Background(), prompt)
		if err != nil {
			t.Fatalf("Complete: %v", err)
		}
		if got != "a punchy reaction" {
			t.Errorf("got %q, want %q", got, "a punchy reaction")
		}
	}
	if inner.calls != 1 {
		t.Errorf("inner LLM called %d times, want 1", inner.calls)
	}
}

func TestWithCache_DifferentPrompts_CallUnderlyingSeparately(t *testing.T) {
	inner := &fakeLLM{text: "a punchy reaction"}
	llm := WithCache(inner, newFakeCache())

	llm.Complete(context.Background(), domain.Prompt{User: "one"})
	llm.Complete(context.Background(), domain.Prompt{User: "two"})

	if inner.calls != 2 {
		t.Errorf("inner LLM called %d times, want 2", inner.calls)
	}
}
