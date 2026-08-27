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
	"time"

	"github.com/SamPariat/pricewatch/internal/analytics"
	"github.com/SamPariat/pricewatch/internal/domain"
	"github.com/SamPariat/pricewatch/internal/i18n"
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

	// Locale drives every piece of prose Digest renders — labels, the
	// up/down word, the nearest-match note. Zero value ("") is treated
	// as English by i18n.T, so existing callers that never set this
	// still render exactly as before. Sourced from Settings.Language,
	// not a request header — the digest is cron-triggered, not part of
	// an HTTP request.
	Locale i18n.Locale

	// NearestMatch is set only when no fare exactly matched the watch's
	// configured dates and the pipeline fell back to the closest real one
	// (internal/pipeline.rollupForWatch) — Digest must say so rather than
	// silently presenting a shifted-date price as the exact trip
	// configured. Zero value (both fields empty) means an exact match.
	NearestMatch DateRange
}

type DateRange struct {
	Depart, Return string
}

// Digest renders one watch's section. It never returns an error: every
// input is already validated upstream, and a template must not be a new
// failure point in the pipeline.
func Digest(w domain.Watch, a Analysis) string {
	var b strings.Builder

	fmt.Fprintf(&b, "<b>%s</b>\n", title(w, a.Locale))
	fmt.Fprintf(&b, "<code>%s</code>", formatPrice(a.PriceMinor, a.Currency))

	if a.Delta.OK {
		arrow, wordKey := "▲", "digest.up"
		if a.Delta.Pct < 0 {
			arrow, wordKey = "▼", "digest.down"
		}
		word := i18n.T(a.Locale, wordKey)
		pct := fmt.Sprintf("%.1f", absF(a.Delta.Pct))
		fmt.Fprintf(&b, "  %s", i18n.T(a.Locale, "digest.delta", "Arrow", arrow, "Pct", pct, "Word", word))
	}
	b.WriteString("\n")

	if a.PercentileOK {
		pct := fmt.Sprintf("%.0f", a.Percentile)
		fmt.Fprintf(&b, "%s\n", i18n.T(a.Locale, "digest.cheaper_than_pct", "Pct", pct))
	}
	if a.AllTimeLow.OK && a.AllTimeLow.PriceMinor < a.PriceMinor {
		fmt.Fprintf(&b, "%s\n", i18n.T(a.Locale, "digest.all_time_low",
			"Price", formatPrice(a.AllTimeLow.PriceMinor, a.Currency), "Date", a.AllTimeLow.Date.Format("Jan 2")))
	}
	if a.NearestMatch != (DateRange{}) {
		dates := displayDate(a.NearestMatch.Depart)
		if a.NearestMatch.Return != "" {
			dates += " → " + displayDate(a.NearestMatch.Return)
		}
		fmt.Fprintf(&b, "%s\n", i18n.T(a.Locale, "digest.no_exact_match", "Dates", dates))
	}

	return strings.TrimRight(b.String(), "\n")
}

// CombineDigest joins per-watch sections into one message — PLAN.md
// § Telegram: "One digest message for all watches, not one per watch."
// It does not yet split across Telegram's 4096-char limit; that belongs
// with the real Telegram adapter (PLAN.md Phase 6), which is also where
// the truncation would need to become multiple Send calls rather than a
// silently-cut message.
func CombineDigest(sections []string, loc i18n.Locale) string {
	key := "digest.header_plural"
	if len(sections) == 1 {
		key = "digest.header_singular"
	}
	header := fmt.Sprintf("<b>%s</b>", i18n.T(loc, key, "Count", len(sections)))
	parts := append([]string{header}, sections...)
	return strings.Join(parts, "\n\n")
}

func title(w domain.Watch, loc i18n.Locale) string {
	if w.Name != "" {
		return w.Name
	}
	switch {
	case w.Kind.IsFlight():
		if p, err := w.DecodeFlightParams(); err == nil {
			if p.ReturnDate != nil {
				return fmt.Sprintf("%s → %s %s", p.Origin, p.Destination, i18n.T(loc, "digest.flight_return"))
			}
			return fmt.Sprintf("%s → %s %s", p.Origin, p.Destination, i18n.T(loc, "digest.flight_oneway"))
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

// displayDate formats a YYYY-MM-DD date as "Jan 2", matching AllTimeLow's
// date formatting above. Falls back to the raw string on a parse failure
// rather than erroring — Digest never returns an error (see its own doc
// comment) and this is cosmetic, not load-bearing.
func displayDate(s string) string {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return s
	}
	return t.Format("Jan 2")
}

func absF(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}
