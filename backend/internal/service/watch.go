package service

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/SamPariat/pricewatch/internal/analytics"
	"github.com/SamPariat/pricewatch/internal/domain"
	v1 "github.com/SamPariat/pricewatch/internal/httpapi/presenter/v1"
	"github.com/SamPariat/pricewatch/internal/i18n"
	"github.com/SamPariat/pricewatch/internal/logging"
	"github.com/SamPariat/pricewatch/internal/pipeline"
	"github.com/SamPariat/pricewatch/internal/providers"
	"github.com/SamPariat/pricewatch/internal/render"
	"github.com/SamPariat/pricewatch/internal/scheduler"
)

// priceHistoryWindow covers the 90-day percentile-rank window (PLAN.md's
// "cheaper than N% of the last 90 days") plus a few days of slack so a
// sample right at the boundary isn't dropped by a same-day rounding edge.
const priceHistoryWindow = 95 * 24 * time.Hour

// WatchService owns every use case around watches: CRUD, the freshness/
// staleness aggregation the panel and API both need, and running a watch
// on demand. RunNow in particular is used identically by the HTTP "Run
// now" button (internal/httpapi/handlers) and the Discord "Refresh now"
// button (internal/discordbot.Listener) — the whole reason this exists
// as a shared service instead of staying duplicated in two driving
// adapters, which it was until this layer existed.
type WatchService struct {
	Repo     domain.Repository
	Sched    *scheduler.Scheduler
	Pipeline *pipeline.Pipeline
	Notifier domain.Notifier
	Clock    domain.Clock
}

// WatchInput is the create/update payload, decoded from the controller's
// own wire-format request type — kept separate so the service never
// depends on how a request happened to arrive over HTTP. A watch has no
// schedule of its own (see domain.Watch) — TripID is required; the
// schedule lives on that trip.
type WatchInput struct {
	Name         string
	Kind         string
	Enabled      *bool
	Params       []byte
	ThresholdPct float64
	TripID       string
}

// Detail is one watch enriched with everything the panel/API need beyond
// the raw row — run-health state, schedule timing, and a price summary —
// assembled from three ports in one place instead of scattered across
// handler code. PriceSummary is presenter/v1's type reused as a plain
// data carrier (it has no version-specific behavior) rather than
// duplicating an identical service-level struct for a presenter layer
// that, today, has exactly one version.
type Detail struct {
	Watch       domain.Watch
	State       domain.WatchState
	NextRun     time.Time
	Interval    time.Duration
	LastUpdated *time.Time
	Price       *v1.PriceSummary
}

// History is the price-history view for one watch.
type History struct {
	Samples     []domain.PriceSample
	LastUpdated *time.Time
	NextRun     time.Time
}

func (s *WatchService) List(ctx context.Context) ([]Detail, error) {
	watches, err := s.Repo.ListWatches(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Detail, len(watches))
	for i, w := range watches {
		out[i] = s.detail(ctx, w)
	}
	return out, nil
}

func (s *WatchService) Get(ctx context.Context, id domain.WatchID) (Detail, error) {
	w, err := s.Repo.GetWatch(ctx, id)
	if err != nil {
		return Detail{}, ErrNotFound
	}
	return s.detail(ctx, w), nil
}

func (s *WatchService) Create(ctx context.Context, in WatchInput) (Detail, error) {
	w := domain.Watch{
		Name: in.Name, Kind: domain.AssetKind(in.Kind), Enabled: true,
		Params: in.Params, ThresholdPct: in.ThresholdPct, TripID: domain.TripID(in.TripID),
	}
	if in.Enabled != nil {
		w.Enabled = *in.Enabled
	}
	if err := s.validateWatch(ctx, w); err != nil {
		return Detail{}, ValidationError{err}
	}

	created, err := s.Repo.CreateWatch(ctx, w)
	if err != nil {
		return Detail{}, err
	}

	if err := s.Sched.Reload(ctx); err != nil {
		logging.From(ctx).Error().Err(err).Msg("reload after create")
	}

	// Backfill so the chart isn't empty for a week (PLAN.md § Data
	// model). Detached from the request context — the fetch should
	// finish even after the response has already gone back.
	go func() {
		bgCtx := context.Background()
		if _, err := s.RunNow(bgCtx, created.ID, false); err != nil {
			logging.From(bgCtx).Error().Str("watch_id", string(created.ID)).Err(err).Msg("backfill after create failed")
		}
	}()

	return s.detail(ctx, created), nil
}

// Update replaces every field — the panel's watch-builder form always
// submits the full object, so there's no partial-merge ambiguity here
// despite the HTTP route being PATCH.
func (s *WatchService) Update(ctx context.Context, id domain.WatchID, in WatchInput) (Detail, error) {
	existing, err := s.Repo.GetWatch(ctx, id)
	if err != nil {
		return Detail{}, ErrNotFound
	}

	updated := existing
	updated.Name = in.Name
	updated.Kind = domain.AssetKind(in.Kind)
	updated.Params = in.Params
	updated.ThresholdPct = in.ThresholdPct
	updated.TripID = domain.TripID(in.TripID)
	if in.Enabled != nil {
		updated.Enabled = *in.Enabled
	}
	if err := s.validateWatch(ctx, updated); err != nil {
		return Detail{}, ValidationError{err}
	}

	saved, err := s.Repo.UpdateWatch(ctx, updated)
	if err != nil {
		return Detail{}, err
	}

	if err := s.Sched.Reload(ctx); err != nil {
		logging.From(ctx).Error().Err(err).Msg("reload after update")
	}

	return s.detail(ctx, saved), nil
}

func (s *WatchService) Delete(ctx context.Context, id domain.WatchID) error {
	if err := s.Repo.DeleteWatch(ctx, id); err != nil {
		return ErrNotFound
	}
	if err := s.Sched.Reload(ctx); err != nil {
		logging.From(ctx).Error().Err(err).Msg("reload after delete")
	}
	return nil
}

// RunNow runs the pipeline immediately, outside the normal schedule,
// bypassing the provider cache (providers.SkipCache) so it always
// fetches fresh — that's the point of asking for a run right now.
// Honours the global dry-run setting for whether the result actually
// gets sent via the Notifier. This is the single implementation used by
// both the HTTP "Run now" button and the Discord "Refresh now" button.
func (s *WatchService) RunNow(ctx context.Context, id domain.WatchID, send bool) (string, error) {
	w, err := s.Repo.GetWatch(ctx, id)
	if err != nil {
		return "", ErrNotFound
	}

	// The rendered embed is shared between the API response and (when
	// send is true) the actual Discord message, so it can only be in
	// one language — Settings.Language wins over whatever locale the
	// caller's own context carries (an HTTP "Run now" click forwards the
	// admin's panel language via Accept-Language), since this is the
	// real digest content the Discord channel sees, not UI chrome. The
	// Discord "Refresh now" button has no locale on its context at all,
	// so it needs this regardless.
	settings, serr := s.Repo.GetSettings(ctx)
	loc := i18n.From(ctx)
	if serr == nil && i18n.Valid(settings.Language) {
		loc = i18n.Locale(settings.Language)
	}

	runID := domain.RunID(ulid.Make().String())
	rctx := i18n.With(logging.With(ctx, "run_id", string(runID), "watch_id", string(id)), loc)
	rctx = providers.SkipCache(rctx)
	res, err := s.Pipeline.RunWatch(rctx, runID, w)
	if err != nil {
		return "", RunError{err}
	}

	if send && serr == nil && !settings.DryRun && settings.DiscordChannelID != "" {
		tripName := ""
		if trip, terr := s.Repo.GetTrip(ctx, w.TripID); terr == nil {
			tripName = trip.Name
		}
		msg := render.CombineDigest([]domain.Embed{res.Embed}, tripName, loc)
		msg.ImagePNG = res.ImagePNG
		if err := s.Notifier.Send(ctx, domain.Target{ChannelID: settings.DiscordChannelID}, msg); err != nil {
			logging.From(rctx).Error().Err(err).Msg("run-now: send failed")
		}
	}
	return pipeline.PlainText(res.Embed), nil
}

// History returns the price-history view for one watch, days back from
// now. Uses wall-clock time directly rather than domain.Clock — matches
// this app's existing behavior at this specific call site (unlike the
// pipeline's own analytics, which are Clock-driven for testability).
func (s *WatchService) History(ctx context.Context, id domain.WatchID, days int) (History, error) {
	since := time.Now().AddDate(0, 0, -days)
	samples, err := s.Repo.ListPriceSamples(ctx, id, since)
	if err != nil {
		return History{}, err
	}

	nextRun := s.Sched.NextRun(id)
	var lastUpdated *time.Time
	if len(samples) > 0 {
		latest := samples[0]
		for _, sm := range samples[1:] {
			if sm.UpdatedAt.After(latest.UpdatedAt) {
				latest = sm
			}
		}
		t := latest.UpdatedAt
		lastUpdated = &t
	}
	return History{Samples: samples, LastUpdated: lastUpdated, NextRun: nextRun}, nil
}

func (s *WatchService) detail(ctx context.Context, w domain.Watch) Detail {
	state, err := s.Repo.GetWatchState(ctx, w.ID)
	if err != nil {
		state = domain.WatchState{WatchID: w.ID}
	}

	settings, err := s.Repo.GetSettings(ctx)
	currency := "INR"
	if err == nil && settings.Currency != "" {
		currency = settings.Currency
	}

	lastUpdated, price := s.priceSummary(ctx, w.ID, currency)
	return Detail{
		Watch: w, State: state,
		NextRun: s.Sched.NextRun(w.ID), Interval: s.Sched.Interval(w.ID),
		LastUpdated: lastUpdated, Price: price,
	}
}

// priceSummary loads this watch's price history once and derives both
// its freshness timestamp and its current-price/delta/percentile summary
// from the same query — one round trip, not two.
func (s *WatchService) priceSummary(ctx context.Context, id domain.WatchID, currency string) (*time.Time, *v1.PriceSummary) {
	samples, err := s.Repo.ListPriceSamples(ctx, id, s.Clock.Now().Add(-priceHistoryWindow))
	if err != nil || len(samples) == 0 {
		return nil, nil
	}

	latest := samples[0]
	for _, sm := range samples[1:] {
		if sm.UpdatedAt.After(latest.UpdatedAt) {
			latest = sm
		}
	}
	lastUpdated := latest.UpdatedAt

	summary := &v1.PriceSummary{PriceMinor: latest.MinMinor, Currency: currency}
	if d := analytics.DeltaVsYesterday(samples, s.Clock.Now()); d.OK {
		pct := d.Pct
		summary.DeltaPct = &pct
	}
	if pct, ok := analytics.PercentileRank(samples, latest.MinMinor, 90, s.Clock.Now()); ok {
		summary.Percentile = &pct
	}
	return &lastUpdated, summary
}

// validateWatch's messages are localized via i18n.From(ctx). It's a
// method (not a free function) because it needs Repo to confirm TripID
// actually names an existing trip — a watch that decoded fine but
// pointed at a deleted/typo'd trip would otherwise fail confusingly
// later, in the scheduler, instead of at create/update time.
func (s *WatchService) validateWatch(ctx context.Context, w domain.Watch) error {
	loc := i18n.From(ctx)
	if !w.Kind.Valid() {
		return fmt.Errorf("%s", i18n.T(loc, "validation.invalid_kind", "Kind", w.Kind))
	}
	if w.TripID == "" {
		return fmt.Errorf("%s", i18n.T(loc, "validation.missing_trip"))
	}
	if _, err := s.Repo.GetTrip(ctx, w.TripID); err != nil {
		return fmt.Errorf("%s", i18n.T(loc, "validation.trip_not_found", "TripID", w.TripID))
	}
	switch {
	case w.Kind.IsFlight():
		p, err := w.DecodeFlightParams()
		if err != nil {
			return fmt.Errorf("%s", i18n.T(loc, "validation.invalid_params", "Kind", w.Kind, "Error", err.Error()))
		}
		if p.Origin == "" || p.Destination == "" || p.DepartDate == "" {
			return fmt.Errorf("%s", i18n.T(loc, "validation.missing_flight_fields", "Kind", w.Kind))
		}
		if w.Kind == domain.AssetFlightReturn && (p.ReturnDate == nil || *p.ReturnDate == "") {
			return fmt.Errorf("%s", i18n.T(loc, "validation.missing_return_date", "Kind", w.Kind))
		}
	case w.Kind.IsLodging():
		p, err := w.DecodeLodgingParams()
		if err != nil {
			return fmt.Errorf("%s", i18n.T(loc, "validation.invalid_params", "Kind", w.Kind, "Error", err.Error()))
		}
		// json.Unmarshal silently ignores fields it doesn't recognize —
		// flight-shaped params decode into a LodgingParams{} with every
		// field empty without erroring, so decoding alone doesn't prove
		// the params actually matched the kind. Checking the required
		// fields landed does.
		if p.URL == "" || p.CheckIn == "" || p.CheckOut == "" {
			return fmt.Errorf("%s", i18n.T(loc, "validation.missing_lodging_fields", "Kind", w.Kind))
		}
		if !isAirbnbListingURL(p.URL) {
			return fmt.Errorf("%s", i18n.T(loc, "validation.invalid_lodging_url", "Kind", w.Kind))
		}
	}
	return nil
}

// isAirbnbListingURL is a permissive sanity check, not a strict
// validator — it just catches "that's obviously not an Airbnb listing"
// (wrong site, a search-results URL) before it reaches the scraper.
func isAirbnbListingURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return false
	}
	host := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
	if !strings.HasPrefix(host, "airbnb.") {
		return false
	}
	return strings.HasPrefix(u.Path, "/rooms/")
}
