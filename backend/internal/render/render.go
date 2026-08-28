// Package render turns a watch's analysis into a Discord embed. Digest
// is the deterministic template every digest falls back to — the AI
// features phase (PLAN.md § AI features) wraps this, it never replaces it:
// the send path must not depend on an LLM being reachable, and an LLM must
// never be the source of a number, only of phrasing around numbers this
// package already computed.
//
// Digest returns a domain.Embed, not a discordgo type — this package
// deliberately doesn't import discordgo, so the domain core stays free of
// any one adapter's SDK (PLAN.md § Architecture patterns). internal/notify/
// discord is the only place a domain.Embed becomes a discordgo.MessageEmbed.
package render

import (
	"fmt"
	"time"

	"github.com/SamPariat/pricewatch/internal/analytics"
	"github.com/SamPariat/pricewatch/internal/domain"
	"github.com/SamPariat/pricewatch/internal/i18n"
)

// Embed colors — a plain int (0xRRGGBB) rather than an i18n key, since
// these are visual, not text: down (a lower price than yesterday) reads
// as good news, up as a caution.
const (
	colorNeutral = 0x64748b // slate-500 — no delta to compare against yet
	colorUp      = 0xef4444 // red-500
	colorDown    = 0x22c55e // green-500
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

// Digest renders one watch's section as an embed. It never returns an
// error: every input is already validated upstream, and a template must
// not be a new failure point in the pipeline.
func Digest(w domain.Watch, a Analysis) domain.Embed {
	embed := domain.Embed{
		Title:       title(w, a.Locale),
		Description: fmt.Sprintf("**%s**", formatPrice(a.PriceMinor, a.Currency)),
		Color:       colorNeutral,
	}

	if a.Delta.OK {
		arrow, wordKey, color := "▲", "digest.up", colorUp
		if a.Delta.Pct < 0 {
			arrow, wordKey, color = "▼", "digest.down", colorDown
		}
		embed.Color = color
		word := i18n.T(a.Locale, wordKey)
		pct := fmt.Sprintf("%.1f", absF(a.Delta.Pct))
		embed.Fields = append(embed.Fields, domain.EmbedField{
			Name:   i18n.T(a.Locale, "digest.field_change"),
			Value:  i18n.T(a.Locale, "digest.delta", "Arrow", arrow, "Pct", pct, "Word", word),
			Inline: true,
		})
	}

	if a.PercentileOK {
		pct := fmt.Sprintf("%.0f", a.Percentile)
		embed.Fields = append(embed.Fields, domain.EmbedField{
			Name:   i18n.T(a.Locale, "digest.field_percentile"),
			Value:  i18n.T(a.Locale, "digest.cheaper_than_pct", "Pct", pct),
			Inline: true,
		})
	}

	if a.AllTimeLow.OK && a.AllTimeLow.PriceMinor < a.PriceMinor {
		embed.Fields = append(embed.Fields, domain.EmbedField{
			Name: i18n.T(a.Locale, "digest.field_all_time_low"),
			Value: i18n.T(a.Locale, "digest.all_time_low",
				"Price", formatPrice(a.AllTimeLow.PriceMinor, a.Currency), "Date", a.AllTimeLow.Date.Format("Jan 2")),
			Inline: true,
		})
	}

	if a.NearestMatch != (DateRange{}) {
		dates := displayDate(a.NearestMatch.Depart)
		if a.NearestMatch.Return != "" {
			dates += " → " + displayDate(a.NearestMatch.Return)
		}
		embed.Footer = i18n.T(a.Locale, "digest.no_exact_match", "Dates", dates)
	}

	return embed
}

// CombineDigest joins per-watch embeds into one message — PLAN.md §
// Discord: "one digest message for all watches, not one per watch."
// Discord allows up to 10 embeds in a single message, which is what
// makes this trivial compared to the old Telegram HTML-string
// concatenation this replaced: each watch keeps its own embed instead of
// being flattened into shared text.
func CombineDigest(embeds []domain.Embed, loc i18n.Locale) domain.Message {
	key := "digest.header_plural"
	if len(embeds) == 1 {
		key = "digest.header_singular"
	}
	return domain.Message{
		Content: i18n.T(loc, key, "Count", len(embeds)),
		Embeds:  embeds,
	}
}

func title(w domain.Watch, loc i18n.Locale) string {
	if w.Name != "" {
		return w.Name
	}
	if p, err := w.DecodeFlightParams(); err == nil {
		if p.ReturnDate != nil {
			return fmt.Sprintf("%s → %s %s", p.Origin, p.Destination, i18n.T(loc, "digest.flight_return"))
		}
		return fmt.Sprintf("%s → %s %s", p.Origin, p.Destination, i18n.T(loc, "digest.flight_oneway"))
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
