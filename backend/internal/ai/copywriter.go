package ai

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/sampariat/prices-reminder/internal/domain"
	"github.com/sampariat/prices-reminder/internal/logging"
	"github.com/sampariat/prices-reminder/internal/render"
)

// completeTimeout bounds how long one watch's digest run will wait on an
// LLM call. This runs inline in the pipeline's per-watch loop (see
// internal/pipeline), so a hung upstream must not stall the whole digest —
// the guarantee PLAN.md § AI features requires ("the digest must still
// send when the LLM is down") only holds if a slow LLM can't block it
// either.
const completeTimeout = 8 * time.Second

// Copywriter is the digest-copywriting feature from PLAN.md § AI features:
// "Go computes every number; the LLM turns a stats struct into a punchy
// group message." It never generates the numeric lines itself — those
// come from render.Digest unchanged — it only appends one short reaction
// sentence, and only if that sentence contains no digits (see enhance
// below). A nil LLM (no API key configured) makes Enhance a no-op, so the
// deterministic template is always what ships by default.
type Copywriter struct {
	LLM domain.LLM
}

// Enhance appends an LLM-written reaction line to template (the output of
// render.Digest) when c.LLM is configured and produces a usable line.
// Every failure mode — nil LLM, timeout, upstream error, or a response
// that fails the numberless check — returns template completely unchanged,
// so a caller never needs its own fallback branch.
func (c *Copywriter) Enhance(ctx context.Context, w domain.Watch, a render.Analysis, template string) string {
	if c == nil || c.LLM == nil {
		return template
	}

	cctx, cancel := context.WithTimeout(ctx, completeTimeout)
	defer cancel()

	raw, err := c.LLM.Complete(cctx, buildPrompt(w, a))
	if err != nil {
		logging.From(ctx).Warn("ai: digest copywriting failed, using template", "error", err)
		return template
	}

	line, ok := sanitize(raw)
	if !ok {
		logging.From(ctx).Warn("ai: rejected LLM output for digest (contained a number or was empty)")
		return template
	}

	return template + "\n<i>" + line + "</i>"
}

func buildPrompt(w domain.Watch, a render.Analysis) domain.Prompt {
	direction := "unchanged"
	if a.Delta.OK {
		if a.Delta.Pct > 0 {
			direction = "up"
		} else if a.Delta.Pct < 0 {
			direction = "down"
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Route or place: %s\n", watchLabel(w))
	fmt.Fprintf(&b, "Price moved: %s since yesterday\n", direction)
	if a.PercentileOK {
		fmt.Fprintf(&b, "Rank vs the last 90 days: %s\n", percentileBucket(a.Percentile))
	}
	if a.AllTimeLow.OK && a.AllTimeLow.PriceMinor >= a.PriceMinor {
		b.WriteString("This is at or below the all-time low.\n")
	}

	return domain.Prompt{
		System: "You write one short, punchy reaction sentence for a friends' group chat about a travel price update. " +
			"Maximum 12 words. No emoji spam (zero or one is fine). Never include any digit, price, percentage, or number of " +
			"any kind — those are shown separately in the message already. Do not use quotation marks. Reply with only the sentence.",
		User: b.String(),
	}
}

// watchLabel deliberately does not reuse render's unexported title() — the
// LLM only needs a plain label, not render's HTML-formatted variant.
func watchLabel(w domain.Watch) string {
	if w.Name != "" {
		return w.Name
	}
	return string(w.Kind)
}

func percentileBucket(pct float64) string {
	switch {
	case pct >= 90:
		return "one of the cheapest days in months"
	case pct >= 70:
		return "cheaper than most recent days"
	case pct <= 10:
		return "one of the priciest days in months"
	case pct <= 30:
		return "pricier than most recent days"
	default:
		return "about average for recent days"
	}
}

// sanitize enforces the guardrail in PLAN.md § AI features: "Never let the
// model emit prices or numbers. This rule must not bend." Rather than try
// to strip digits out of freeform text (which risks mangling a sentence
// into something that reads broken), any digit anywhere rejects the whole
// line — the caller falls back to the plain template instead.
func sanitize(raw string) (string, bool) {
	line := strings.TrimSpace(raw)
	line = strings.Trim(line, "\"'")
	line = strings.SplitN(line, "\n", 2)[0] // one line only, however much the model wrote

	if line == "" || len(line) > 200 {
		return "", false
	}
	for _, r := range line {
		if r >= '0' && r <= '9' {
			return "", false
		}
	}
	return line, true
}
