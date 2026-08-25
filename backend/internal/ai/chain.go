// Package ai composes domain.LLM adapters (gemini, ollama) and wraps them
// around the deterministic digest template in internal/render. Every path
// here degrades to that template — see copywriter.go — because PLAN.md §
// AI features is explicit that AI is optional end to end: "The digest must
// still send when the LLM is down."
package ai

import (
	"context"
	"errors"

	"github.com/SamPariat/pricewatch/internal/domain"
	"github.com/SamPariat/pricewatch/internal/logging"
)

type chain struct {
	primary  domain.LLM
	fallback domain.LLM // may be nil
}

// Chain tries primary first and falls back to fallback on any error —
// PLAN.md § AI features: "Gemini Flash primary, Ollama fallback." fallback
// may be nil, in which case a primary failure just returns the error (the
// caller, Copywriter, already treats any error as "skip the AI touch").
func Chain(primary domain.LLM, fallback domain.LLM) domain.LLM {
	return &chain{primary: primary, fallback: fallback}
}

func (c *chain) Complete(ctx context.Context, p domain.Prompt) (string, error) {
	text, err := c.primary.Complete(ctx, p)
	if err == nil {
		return text, nil
	}
	if c.fallback == nil {
		return "", err
	}
	logging.From(ctx).Warn().Err(err).Msg("ai: primary LLM failed, trying fallback")

	text, ferr := c.fallback.Complete(ctx, p)
	if ferr != nil {
		return "", errors.Join(err, ferr)
	}
	return text, nil
}
