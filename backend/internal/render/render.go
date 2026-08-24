// Package render turns a watch's analysis into a message section. Digest
// is the deterministic template every digest falls back to — the AI
// features phase (PLAN.md § AI features) wraps this, it never replaces it:
// the send path must not depend on an LLM being reachable, and an LLM must
// never be the source of a number, only of phrasing around numbers this
// package already computed.
//
// Formatting here targets Telegram HTML parse mode (bold/code only, no
// MarkdownV2 — see PLAN.md § Traps: MarkdownV2 needs ~18 characters
// escaped and is a reliable source of runtime formatting bugs).
package render

import (
	"fmt"
	"strings"

	"github.com/sampariat/prices-reminder/internal/analytics"
	"github.com/sampariat/prices-reminder/internal/domain"
)

// Analysis carries everything one watch's digest section needs — the
// current price plus every stat internal/analytics can compute for it.
// Fields ending in OK follow the same "false means don't render this
// part" convention as the analytics package itself.
type Analysis struct {
	PriceMinor   int64
	Currency     string
	Delta        analytics.Delta
	Percentile   float64
	PercentileOK bool
	AllTimeLow   analytics.AllTimeLow
	FetchedAt    string // RFC3339 — see PLAN.md § Freshness: always the real fetch time, never "now"
}

// Digest renders one watch's section. It never returns an error: every
// input is already validated upstream, and a template must not be a new
// failure point in the pipeline.
func Digest(w domain.Watch, a Analysis) string {
	var b strings.Builder

	fmt.Fprintf(&b, "<b>%s</b>\n", title(w))
	fmt.Fprintf(&b, "<code>%s</code>", formatPrice(a.PriceMinor, a.Currency))

	if a.Delta.OK {
		arrow, word := "▲", "up"
		if a.Delta.Pct < 0 {
			arrow, word = "▼", "down"
		}
		fmt.Fprintf(&b, "  %s %.1f%% (%s vs yesterday)", arrow, absF(a.Delta.Pct), word)
	}
	b.WriteString("\n")

	if a.PercentileOK {
		fmt.Fprintf(&b, "Cheaper than %.0f%% of the last 90 days\n", a.Percentile)
	}
	if a.AllTimeLow.OK && a.AllTimeLow.PriceMinor < a.PriceMinor {
		fmt.Fprintf(&b, "All-time low: <code>%s</code> on %s\n", formatPrice(a.AllTimeLow.PriceMinor, a.Currency), a.AllTimeLow.Date.Format("Jan 2"))
	}

	return strings.TrimRight(b.String(), "\n")
}

// CombineDigest joins per-watch sections into one message — PLAN.md
// § Telegram: "One digest message for all watches, not one per watch."
// It does not yet split across Telegram's 4096-char limit; that belongs
// with the real Telegram adapter (PLAN.md Phase 6), which is also where
// the truncation would need to become multiple Send calls rather than a
// silently-cut message.
func CombineDigest(sections []string) string {
	header := fmt.Sprintf("<b>Daily digest</b> — %d watch", len(sections))
	if len(sections) != 1 {
		header += "es"
	}
	parts := append([]string{header}, sections...)
	return strings.Join(parts, "\n\n")
}

func title(w domain.Watch) string {
	if w.Name != "" {
		return w.Name
	}
	switch {
	case w.Kind.IsFlight():
		if p, err := w.DecodeFlightParams(); err == nil {
			if p.ReturnDate != nil {
				return fmt.Sprintf("%s → %s · return", p.Origin, p.Destination)
			}
			return fmt.Sprintf("%s → %s · one-way", p.Origin, p.Destination)
		}
	case w.Kind == domain.AssetHotel, w.Kind == domain.AssetRental:
		if p, err := w.DecodeHotelParams(); err == nil {
			return p.Location
		}
	}
	return string(w.Kind)
}

func formatPrice(minor int64, currency string) string {
	whole := minor / 100
	return fmt.Sprintf("%s %s", currency, groupThousands(whole))
}

func groupThousands(n int64) string {
	neg := n < 0
	if neg {
		n = -n
	}
	s := fmt.Sprintf("%d", n)
	var out []byte
	for i, c := range []byte(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, c)
	}
	if neg {
		return "-" + string(out)
	}
	return string(out)
}

func absF(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}
