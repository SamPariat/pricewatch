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

	"github.com/sampariat/prices-reminder/internal/ai"
	"github.com/sampariat/prices-reminder/internal/analytics"
	"github.com/sampariat/prices-reminder/internal/domain"
	"github.com/sampariat/prices-reminder/internal/logging"
	"github.com/sampariat/prices-reminder/internal/providers"
	"github.com/sampariat/prices-reminder/internal/render"
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

	sample, ok := rollupForWatch(w, quotes, started)
	if !ok {
		return "", p.fail(ctx, runID, w, "normalize",
			fmt.Errorf("no quote in this fetch matched the watch's configured date"))
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
	p.emit(ctx, runID, "analyze", domain.LevelInfo, "analysis complete", nil)

	text := render.Digest(w, analysis)
	text = p.Copywriter.Enhance(ctx, w, analysis, text)
	p.emit(ctx, runID, "render", domain.LevelInfo, "section rendered", nil)

	now := p.Clock.Now()
	if err := p.Repo.UpsertWatchState(ctx, domain.WatchState{
		WatchID: w.ID, LastSuccessAt: &now, LastAttemptAt: &now, LastError: "", ConsecutiveFailures: 0,
	}); err != nil {
		logging.From(ctx).Error("pipeline: upsert watch state after success", "error", err)
	}
	if err := p.Repo.FinishDigestRun(ctx, domain.DigestRun{
		RunID: runID, WatchID: w.ID, FinishedAt: &now, Status: domain.RunSuccess, MessageBody: text,
	}); err != nil {
		logging.From(ctx).Error("pipeline: finish digest run after success", "error", err)
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
		logging.From(ctx).Error("pipeline: upsert watch state after failure", "error", err)
	}

	if err := p.Repo.FinishDigestRun(ctx, domain.DigestRun{
		RunID: runID, WatchID: w.ID, FinishedAt: &now, Status: domain.RunFailed, Error: cause.Error(),
	}); err != nil {
		logging.From(ctx).Error("pipeline: finish digest run after failure", "error", err)
	}

	return fmt.Errorf("pipeline: watch %s: stage %s: %w", w.ID, stage, cause)
}

func (p *Pipeline) emit(ctx context.Context, runID domain.RunID, stage string, level domain.LogLevel, msg string, fields []byte) {
	if err := p.Repo.InsertRunEvent(ctx, domain.RunEvent{
		RunID: runID, At: p.Clock.Now(), Stage: stage, Level: level, Msg: msg, Fields: fields,
	}); err != nil {
		logging.From(ctx).Error("pipeline: insert run event", "error", err)
	}

	log := logging.From(ctx)
	switch level {
	case domain.LevelError:
		log.Error(msg, "stage", stage)
	case domain.LevelWarn:
		log.Warn(msg, "stage", stage)
	case domain.LevelDebug:
		log.Debug(msg, "stage", stage)
	default:
		log.Info(msg, "stage", stage)
	}
}

// rollupForWatch extracts, from one fetch's quotes, only the quote(s)
// matching the watch's own configured travel date, and produces exactly
// one price_samples row for `asOf` (the calendar day the fetch ran) — not
// one row per travel date in the response.
//
// price_samples is a day-over-day history of THIS watch's tracked price:
// that's what the digest delta, rolling stats, percentile rank, and chart
// all need. The full month a flight fetch returns (many travel dates) is
// retained in the quotes table (fetched_at-keyed) for a future
// flexible-date heatmap — see PLAN.md § Further suggestions — not rolled
// up here.
func rollupForWatch(w domain.Watch, quotes []domain.Quote, asOf time.Time) (domain.PriceSample, bool) {
	depart, ret, err := targetDates(w)
	if err != nil {
		return domain.PriceSample{}, false
	}

	var matched []int64
	for _, q := range quotes {
		if q.DepartDate == depart && q.ReturnDate == ret {
			matched = append(matched, q.PriceMinor)
		}
	}
	if len(matched) == 0 {
		return domain.PriceSample{}, false
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
	}, true
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
