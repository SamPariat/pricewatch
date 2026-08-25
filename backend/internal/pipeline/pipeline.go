// Package pipeline orchestrates one watch's run: fetch → normalize →
// persist → analyze → render. Sending the result is the scheduler's job,
// not this package's — RunWatch returns a rendered section and lets the
// caller decide how many watches' sections to batch into one Notifier.Send
// call (see PLAN.md § Telegram, "one digest message for all watches").
package pipeline

import (
	"context"
	"fmt"
	"time"

	"github.com/SamPariat/pricewatch/internal/ai"
	"github.com/SamPariat/pricewatch/internal/analytics"
	"github.com/SamPariat/pricewatch/internal/domain"
	"github.com/SamPariat/pricewatch/internal/logging"
	"github.com/SamPariat/pricewatch/internal/providers"
	"github.com/SamPariat/pricewatch/internal/render"
)

type Pipeline struct {
	Registry *providers.Registry
	Repo     domain.Repository
	Clock    domain.Clock

	// Copywriter is optional — see PLAN.md § AI features. A nil Copywriter
	// (or one wrapping a nil domain.LLM) leaves render.Digest's output
	// completely unchanged.
	Copywriter *ai.Copywriter
}

// RunWatch runs the full pipeline for one watch. runID is minted by the
// caller (the scheduler, per PLAN.md Phase 4) and threaded through ctx by
// the caller too, so every log line and every run_events row this method
// writes already carries it via internal/logging.
//
// It always writes a digest_runs row and updates watch_state — success or
// failure — so "why didn't the digest arrive" is answerable from data,
// even when this returns an error.
func (p *Pipeline) RunWatch(ctx context.Context, runID domain.RunID, w domain.Watch) (string, error) {
	started := p.Clock.Now()
	if err := p.Repo.CreateDigestRun(ctx, domain.DigestRun{
		RunID: runID, WatchID: w.ID, StartedAt: started, Status: domain.RunPending,
	}); err != nil {
		return "", fmt.Errorf("pipeline: create digest run: %w", err)
	}

	provider, err := p.Registry.For(w.Kind)
	if err != nil {
		return "", p.fail(ctx, runID, w, "fetch", err)
	}

	p.emit(ctx, runID, "fetch", domain.LevelInfo, "starting fetch", nil)
	quotes, err := provider.Fetch(ctx, w)
	if err != nil {
		return "", p.fail(ctx, runID, w, "fetch", err)
	}
	p.emit(ctx, runID, "fetch", domain.LevelInfo, fmt.Sprintf("fetched %d quotes", len(quotes)), nil)

	if err := p.Repo.InsertQuotes(ctx, quotes); err != nil {
		return "", p.fail(ctx, runID, w, "persist", err)
	}

	sample, match, ok := rollupForWatch(w, quotes, started)
	if !ok {
		return "", p.fail(ctx, runID, w, "normalize",
			fmt.Errorf("no quote in this fetch was within %d days of the watch's configured dates", maxDateDriftDays))
	}
	if !match.Exact {
		p.emit(ctx, runID, "normalize", domain.LevelWarn,
			fmt.Sprintf("no exact date match — using nearest available fare (%s to %s instead of %s to %s)",
				match.MatchedDepart, match.MatchedReturn, match.ConfiguredDepart, match.ConfiguredReturn), nil)
	}
	if err := p.Repo.UpsertPriceSample(ctx, sample); err != nil {
		return "", p.fail(ctx, runID, w, "persist", err)
	}
	p.emit(ctx, runID, "normalize", domain.LevelInfo, "price sample upserted", nil)

	samples, err := p.Repo.ListPriceSamples(ctx, w.ID, started.AddDate(0, 0, -95))
	if err != nil {
		return "", p.fail(ctx, runID, w, "analyze", err)
	}

	pct, pctOK := analytics.PercentileRank(samples, sample.MinMinor, 90, started)
	analysis := render.Analysis{
		PriceMinor:   sample.MinMinor,
		Currency:     firstQuoteCurrency(quotes),
		Delta:        analytics.DeltaVsYesterday(samples, started),
		Percentile:   pct,
		PercentileOK: pctOK,
		AllTimeLow:   analytics.AllTimeLowOf(samples),
		FetchedAt:    started.Format(time.RFC3339),
	}
	if !match.Exact {
		analysis.NearestMatch = render.DateRange{Depart: match.MatchedDepart, Return: match.MatchedReturn}
	}
	p.emit(ctx, runID, "analyze", domain.LevelInfo, "analysis complete", nil)

	text := render.Digest(w, analysis)
	text = p.Copywriter.Enhance(ctx, w, analysis, text)
	p.emit(ctx, runID, "render", domain.LevelInfo, "section rendered", nil)

	now := p.Clock.Now()
	if err := p.Repo.UpsertWatchState(ctx, domain.WatchState{
		WatchID: w.ID, LastSuccessAt: &now, LastAttemptAt: &now, LastError: "", ConsecutiveFailures: 0,
	}); err != nil {
		logging.From(ctx).Error().Err(err).Msg("pipeline: upsert watch state after success")
	}
	if err := p.Repo.FinishDigestRun(ctx, domain.DigestRun{
		RunID: runID, WatchID: w.ID, FinishedAt: &now, Status: domain.RunSuccess, MessageBody: text,
	}); err != nil {
		logging.From(ctx).Error().Err(err).Msg("pipeline: finish digest run after success")
	}

	return text, nil
}

// fail records a failed run — a bad watch_state and digest_runs row, not
// a panic or a swallowed error — and returns the error so the caller
// (the scheduler) knows not to include this watch's section in the batch.
func (p *Pipeline) fail(ctx context.Context, runID domain.RunID, w domain.Watch, stage string, cause error) error {
	now := p.Clock.Now()
	p.emit(ctx, runID, stage, domain.LevelError, cause.Error(), nil)

	state, err := p.Repo.GetWatchState(ctx, w.ID)
	if err != nil {
		state = domain.WatchState{WatchID: w.ID}
	}
	state.WatchID = w.ID
	state.LastAttemptAt = &now
	state.LastError = cause.Error()
	state.ConsecutiveFailures++
	if err := p.Repo.UpsertWatchState(ctx, state); err != nil {
		logging.From(ctx).Error().Err(err).Msg("pipeline: upsert watch state after failure")
	}

	if err := p.Repo.FinishDigestRun(ctx, domain.DigestRun{
		RunID: runID, WatchID: w.ID, FinishedAt: &now, Status: domain.RunFailed, Error: cause.Error(),
	}); err != nil {
		logging.From(ctx).Error().Err(err).Msg("pipeline: finish digest run after failure")
	}

	return fmt.Errorf("pipeline: watch %s: stage %s: %w", w.ID, stage, cause)
}

func (p *Pipeline) emit(ctx context.Context, runID domain.RunID, stage string, level domain.LogLevel, msg string, fields []byte) {
	if err := p.Repo.InsertRunEvent(ctx, domain.RunEvent{
		RunID: runID, At: p.Clock.Now(), Stage: stage, Level: level, Msg: msg, Fields: fields,
	}); err != nil {
		logging.From(ctx).Error().Err(err).Msg("pipeline: insert run event")
	}

	log := logging.From(ctx)
	switch level {
	case domain.LevelError:
		log.Error().Str("stage", stage).Msg(msg)
	case domain.LevelWarn:
		log.Warn().Str("stage", stage).Msg(msg)
	case domain.LevelDebug:
		log.Debug().Str("stage", stage).Msg(msg)
	default:
		log.Info().Str("stage", stage).Msg(msg)
	}
}

// maxDateDriftDays caps how far a "nearest available" match (see
// rollupForWatch) is allowed to drift from the watch's configured dates,
// using whichever leg (depart or return) is further off. Set from a real
// observed case: a DEL→GAU watch configured for Oct 31 → Nov 7 whose only
// real fare that month was Oct 17 → Nov 18 — a 14-day depart drift and an
// 11-day return drift, both legitimate. 21 comfortably covers that with
// room, while still rejecting a match from a genuinely different travel
// window (e.g. a fare three months off) that isn't really the same trip
// anymore.
const maxDateDriftDays = 21

// dateMatch records which quote dates actually backed a PriceSample —
// almost always identical to the watch's configured dates, but not always
// (see rollupForWatch). Exact is false only when the nearest-available
// fallback fired, which render.Digest uses to decide whether to say so.
type dateMatch struct {
	ConfiguredDepart, ConfiguredReturn string
	MatchedDepart, MatchedReturn       string
	Exact                              bool
}

// rollupForWatch extracts, from one fetch's quotes, the quote(s) that back
// this watch's tracked price, and produces exactly one price_samples row
// for `asOf` (the calendar day the fetch ran) — not one row per travel
// date in the response.
//
// It prefers an exact match on the watch's configured dates. Failing
// that, it falls back to the closest real fare within maxDateDriftDays —
// Travelpayouts' calendar endpoint returns whichever specific
// departure/return combo happened to be cheapest that month, not a free
// choice of both dates (PLAN.md § Known gaps: "prices are cached, not
// live-shopped"), so for many routes an exact match is the exception, not
// the rule. dateMatch.Exact tells the caller which happened, so the
// digest can say so rather than silently presenting a shifted-date price
// as if it were the exact trip configured.
//
// price_samples is a day-over-day history of THIS watch's tracked price:
// that's what the digest delta, rolling stats, percentile rank, and chart
// all need. The full month a flight fetch returns (many travel dates) is
// retained in the quotes table (fetched_at-keyed) for a future
// flexible-date heatmap — see PLAN.md § Further suggestions — not rolled
// up here.
func rollupForWatch(w domain.Watch, quotes []domain.Quote, asOf time.Time) (domain.PriceSample, dateMatch, bool) {
	depart, ret, err := targetDates(w)
	if err != nil {
		return domain.PriceSample{}, dateMatch{}, false
	}

	matchOn := func(d, r string) []int64 {
		var prices []int64
		for _, q := range quotes {
			if q.DepartDate == d && q.ReturnDate == r {
				prices = append(prices, q.PriceMinor)
			}
		}
		return prices
	}

	matched := matchOn(depart, ret)
	match := dateMatch{ConfiguredDepart: depart, ConfiguredReturn: ret, MatchedDepart: depart, MatchedReturn: ret, Exact: true}

	if len(matched) == 0 {
		bestDist := -1
		for _, q := range quotes {
			d, ok := dateDistance(depart, ret, q.DepartDate, q.ReturnDate)
			if !ok {
				continue
			}
			if bestDist == -1 || d < bestDist {
				bestDist = d
				match.MatchedDepart, match.MatchedReturn = q.DepartDate, q.ReturnDate
			}
		}
		if bestDist == -1 || bestDist > maxDateDriftDays {
			return domain.PriceSample{}, dateMatch{}, false
		}
		match.Exact = false
		matched = matchOn(match.MatchedDepart, match.MatchedReturn)
	}

	y, m, d := asOf.Date()
	return domain.PriceSample{
		WatchID:     w.ID,
		SampleDate:  time.Date(y, m, d, 0, 0, 0, 0, time.UTC),
		MinMinor:    analytics.MinInt64(matched),
		MedianMinor: analytics.MedianInt64(matched),
		MaxMinor:    analytics.MaxInt64(matched),
		NQuotes:     len(matched),
		UpdatedAt:   asOf,
	}, match, true
}

// dateDistance is the larger of the two legs' day differences, not the
// sum — so a fare that's exact on depart but wildly off on return still
// scores as "wildly off" instead of averaging out to something that looks
// close. A missing return date (one-way) is skipped entirely, since
// there's nothing to compare on that leg.
func dateDistance(wantDepart, wantReturn, gotDepart, gotReturn string) (int, bool) {
	dd, ok := dayDiff(wantDepart, gotDepart)
	if !ok {
		return 0, false
	}
	if wantReturn == "" && gotReturn == "" {
		return dd, true
	}
	rd, ok := dayDiff(wantReturn, gotReturn)
	if !ok {
		return 0, false
	}
	if rd > dd {
		return rd, true
	}
	return dd, true
}

func dayDiff(a, b string) (int, bool) {
	ta, err := time.Parse("2006-01-02", a)
	if err != nil {
		return 0, false
	}
	tb, err := time.Parse("2006-01-02", b)
	if err != nil {
		return 0, false
	}
	days := int(tb.Sub(ta).Hours() / 24)
	if days < 0 {
		days = -days
	}
	return days, true
}

func targetDates(w domain.Watch) (depart, ret string, err error) {
	switch {
	case w.Kind.IsFlight():
		p, err := w.DecodeFlightParams()
		if err != nil {
			return "", "", err
		}
		r := ""
		if p.ReturnDate != nil {
			r = *p.ReturnDate
		}
		return p.DepartDate, r, nil
	case w.Kind == domain.AssetHotel || w.Kind == domain.AssetRental:
		p, err := w.DecodeHotelParams()
		if err != nil {
			return "", "", err
		}
		return p.CheckIn, p.CheckOut, nil
	default:
		return "", "", fmt.Errorf("pipeline: unknown asset kind %q", w.Kind)
	}
}

func firstQuoteCurrency(quotes []domain.Quote) string {
	if len(quotes) == 0 {
		return ""
	}
	return quotes[0].Currency
}
