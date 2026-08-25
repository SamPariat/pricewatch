package ai

import (
	"context"
	"errors"
	"testing"

	"github.com/SamPariat/pricewatch/internal/domain"
)

type fakeLLM struct {
	text  string
	err   error
	calls int
}

func (f *fakeLLM) Complete(ctx context.Context, p domain.Prompt) (string, error) {
	f.calls++
	return f.text, f.err
}

func TestChain_PrimarySucceeds_NeverCallsFallback(t *testing.T) {
	primary := &fakeLLM{text: "from primary"}
	fallback := &fakeLLM{text: "from fallback"}

	got, err := Chain(primary, fallback).Complete(context.Background(), domain.Prompt{})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if got != "from primary" {
		t.Errorf("got %q, want %q", got, "from primary")
	}
	if fallback.calls != 0 {
		t.Errorf("fallback was called %d times, want 0", fallback.calls)
	}
}

func TestChain_PrimaryFails_UsesFallback(t *testing.T) {
	primary := &fakeLLM{err: errors.New("rate limited")}
	fallback := &fakeLLM{text: "from fallback"}

	got, err := Chain(primary, fallback).Complete(context.Background(), domain.Prompt{})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if got != "from fallback" {
		t.Errorf("got %q, want %q", got, "from fallback")
	}
}

func TestChain_BothFail_ReturnsError(t *testing.T) {
	primary := &fakeLLM{err: errors.New("primary down")}
	fallback := &fakeLLM{err: errors.New("fallback down")}

	if _, err := Chain(primary, fallback).Complete(context.Background(), domain.Prompt{}); err == nil {
		t.Fatal("expected an error when both primary and fallback fail")
	}
}

func TestChain_NilFallback_ReturnsPrimaryError(t *testing.T) {
	primary := &fakeLLM{err: errors.New("primary down")}

	if _, err := Chain(primary, nil).Complete(context.Background(), domain.Prompt{}); err == nil {
		t.Fatal("expected an error when primary fails and there is no fallback")
	}
}
