package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/SamPariat/pricewatch/internal/domain"
	"github.com/SamPariat/pricewatch/internal/providers"
	"github.com/SamPariat/pricewatch/internal/store/storetest"
)

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

type fakeProvider struct {
	kind   domain.AssetKind
	quotes []domain.Quote
	err    error
}

func (f *fakeProvider) Kind() domain.AssetKind { return f.kind }

func (f *fakeProvider) Fetch(ctx context.Context, w domain.Watch) ([]domain.Quote, error) {
	return f.quotes, f.err
}

func flightWatch(t *testing.T) domain.Watch {
	t.Helper()
	ret := "2026-12-15"
	params, err := json.Marshal(domain.FlightParams{Origin: "BLR", Destination: "GOI", DepartDate: "2026-12-10", ReturnDate: &ret})
	if err != nil {
		t.Fatal(err)
	}
	return domain.Watch{ID: "w1", Kind: domain.AssetFlightReturn, Enabled: true, Params: params}
}

func TestRunWatch_Success(t *testing.T) {
	now := time.Date(2026, 8, 24, 7, 0, 0, 0, time.UTC)
	w := flightWatch(t)

	registry := providers.NewRegistry()
	registry.Register(&fakeProvider{
		kind: domain.AssetFlightReturn,
		quotes: []domain.Quote{
			{WatchID: w.ID, Provider: "aviasales", PriceMinor: 800000, Currency: "INR", DepartDate: "2026-12-10", ReturnDate: "2026-12-15", Fingerprint: "a"},
			{WatchID: w.ID, Provider: "aviasales", PriceMinor: 850000, Currency: "INR", DepartDate: "2026-12-10", ReturnDate: "2026-12-15", Fingerprint: "b"},
			// A different travel date in the same month — must be
			// excluded from this watch's price_samples rollup.
			{WatchID: w.ID, Provider: "aviasales", PriceMinor: 100, Currency: "INR", DepartDate: "2026-12-11", ReturnDate: "2026-12-16", Fingerprint: "c"},
		},
	})

	repo := storetest.New()
	p := &Pipeline{Registry: registry, Repo: repo, Clock: fixedClock{now}}

	res, err := p.RunWatch(context.Background(), "run1", w)
	if err != nil {
		t.Fatalf("RunWatch: %v", err)
	}
	if res.Embed.Title == "" {
		t.Fatal("expected a non-empty embed title")
	}
	if res.ThresholdBreach {
		t.Error("expected no threshold breach — watch has no ThresholdPct configured")
	}

	samples, err := repo.ListPriceSamples(context.Background(), w.ID, now.AddDate(0, 0, -1))
	if err != nil {
		t.Fatal(err)
	}
	if len(samples) != 1 {
		t.Fatalf("got %d samples, want 1 (only today's rollup for the watch's own date)", len(samples))
	}
	s := samples[0]
	if s.MinMinor != 800000 || s.MaxMinor != 850000 || s.MedianMinor != 825000 {
		t.Errorf("rollup = min:%d median:%d max:%d, want min:800000 median:825000 max:850000",
			s.MinMinor, s.MedianMinor, s.MaxMinor)
	}
	if s.NQuotes != 2 {
		t.Errorf("NQuotes = %d, want 2 (the off-date quote must be excluded)", s.NQuotes)
	}

	runs := repo.Runs()
	run, ok := runs["run1"]
	if !ok {
		t.Fatal("expected a digest_runs row for run1")
	}
	if run.Status != domain.RunSuccess {
		t.Errorf("run.Status = %q, want success", run.Status)
	}

	state, err := repo.GetWatchState(context.Background(), w.ID)
	if err != nil {
		t.Fatal(err)
	}
	if state.ConsecutiveFailures != 0 {
		t.Errorf("ConsecutiveFailures = %d, want 0 after success", state.ConsecutiveFailures)
	}
	if state.LastSuccessAt == nil {
		t.Error("expected LastSuccessAt to be set after success")
	}
}

func TestRunWatch_ProviderError(t *testing.T) {
	now := time.Date(2026, 8, 24, 7, 0, 0, 0, time.UTC)
	w := flightWatch(t)

	registry := providers.NewRegistry()
	registry.Register(&fakeProvider{kind: domain.AssetFlightReturn, err: errors.New("upstream boom")})

	repo := storetest.New()
	p := &Pipeline{Registry: registry, Repo: repo, Clock: fixedClock{now}}

	_, err := p.RunWatch(context.Background(), "run2", w)
	if err == nil {
		t.Fatal("expected an error when the provider fails")
	}

	run := repo.Runs()["run2"]
	if run.Status != domain.RunFailed {
		t.Errorf("run.Status = %q, want failed", run.Status)
	}

	state, _ := repo.GetWatchState(context.Background(), w.ID)
	if state.ConsecutiveFailures != 1 {
		t.Errorf("ConsecutiveFailures = %d, want 1", state.ConsecutiveFailures)
	}
	if state.LastError == "" {
		t.Error("expected LastError to be set")
	}
}

func TestRunWatch_ConsecutiveFailuresAccumulate(t *testing.T) {
	now := time.Date(2026, 8, 24, 7, 0, 0, 0, time.UTC)
	w := flightWatch(t)

	registry := providers.NewRegistry()
	registry.Register(&fakeProvider{kind: domain.AssetFlightReturn, err: errors.New("still down")})

	repo := storetest.New()
	p := &Pipeline{Registry: registry, Repo: repo, Clock: fixedClock{now}}

	p.RunWatch(context.Background(), "run3", w)
	p.RunWatch(context.Background(), "run4", w)
	p.RunWatch(context.Background(), "run5", w)

	state, _ := repo.GetWatchState(context.Background(), w.ID)
	if state.ConsecutiveFailures != 3 {
		t.Errorf("ConsecutiveFailures = %d, want 3 after three failed runs", state.ConsecutiveFailures)
	}
}

// TestRunWatch_PreservesSnooze guards against UpsertWatchState silently
// clearing an active Discord snooze on a normal run — see
// storetest.FakeRepository.UpsertWatchState's doc comment. A regression
// here would mean "Snooze 7d" stops working the moment the very next
// scheduled fetch completes.
func TestRunWatch_PreservesSnooze(t *testing.T) {
	now := time.Date(2026, 8, 24, 7, 0, 0, 0, time.UTC)
	w := flightWatch(t)

	registry := providers.NewRegistry()
	registry.Register(&fakeProvider{
		kind: domain.AssetFlightReturn,
		quotes: []domain.Quote{
			{WatchID: w.ID, PriceMinor: 800000, DepartDate: "2026-12-10", ReturnDate: "2026-12-15", Fingerprint: "a"},
		},
	})

	repo := storetest.New()
	until := now.AddDate(0, 0, 7)
	if err := repo.SetSnooze(context.Background(), w.ID, &until); err != nil {
		t.Fatal(err)
	}

	p := &Pipeline{Registry: registry, Repo: repo, Clock: fixedClock{now}}
	if _, err := p.RunWatch(context.Background(), "run-snooze", w); err != nil {
		t.Fatalf("RunWatch: %v", err)
	}

	state, err := repo.GetWatchState(context.Background(), w.ID)
	if err != nil {
		t.Fatal(err)
	}
	if state.SnoozedUntil == nil || !state.SnoozedUntil.Equal(until) {
		t.Errorf("SnoozedUntil = %v, want %v — a normal run must not clear it", state.SnoozedUntil, until)
	}
}

// TestRunWatch_NoMatchingQuote_Fails uses a date far enough off (about six
// months) that even the nearest-available fallback (rollupForWatch's
// maxDateDriftDays) rejects it — a genuine "nothing here is this trip"
// case, distinct from TestRunWatch_NearestMatch_Succeeds below.
func TestRunWatch_NoMatchingQuote_Fails(t *testing.T) {
	now := time.Date(2026, 8, 24, 7, 0, 0, 0, time.UTC)
	w := flightWatch(t)

	registry := providers.NewRegistry()
	registry.Register(&fakeProvider{
		kind: domain.AssetFlightReturn,
		quotes: []domain.Quote{
			{WatchID: w.ID, PriceMinor: 100, DepartDate: "2027-06-25", ReturnDate: "2027-07-01", Fingerprint: "x"},
		},
	})

	repo := storetest.New()
	p := &Pipeline{Registry: registry, Repo: repo, Clock: fixedClock{now}}

	_, err := p.RunWatch(context.Background(), "run6", w)
	if err == nil {
		t.Fatal("expected an error when nothing fetched is within the drift cap of the watch's configured date")
	}
}

// TestRunWatch_NearestMatch_Succeeds is the full RunWatch path for the
// near-miss case the fallback exists for — a real fare exists but isn't
// exactly the configured dates, and RunWatch must still succeed, writing
// a digest that says so rather than silently presenting the shifted-date
// price as the exact trip configured (see render.Digest's NearestMatch
// note).
func TestRunWatch_NearestMatch_Succeeds(t *testing.T) {
	now := time.Date(2026, 8, 24, 7, 0, 0, 0, time.UTC)
	w := flightWatch(t) // configured for 2026-12-10 -> 2026-12-15

	registry := providers.NewRegistry()
	registry.Register(&fakeProvider{
		kind: domain.AssetFlightReturn,
		quotes: []domain.Quote{
			{WatchID: w.ID, PriceMinor: 100, DepartDate: "2026-12-25", ReturnDate: "2026-12-31", Fingerprint: "x"},
		},
	})

	repo := storetest.New()
	p := &Pipeline{Registry: registry, Repo: repo, Clock: fixedClock{now}}

	res, err := p.RunWatch(context.Background(), "run7", w)
	if err != nil {
		t.Fatalf("expected the nearest-available fallback to succeed, got: %v", err)
	}
	if !strings.Contains(res.Embed.Footer, "No exact match") {
		t.Errorf("expected the embed footer to note the fallback match, got: %q", res.Embed.Footer)
	}
	if !strings.Contains(res.Embed.Footer, "Dec 25") || !strings.Contains(res.Embed.Footer, "Dec 31") {
		t.Errorf("expected the footer to show the actual matched dates, got: %q", res.Embed.Footer)
	}
}

// TestRunWatch_ThresholdBreach_PriceDropMeetsThreshold seeds yesterday's
// price sample directly (bypassing a second RunWatch call, which would
// also need its own quotes) so DeltaVsYesterday has a real prior day to
// compare against — see analytics.DeltaVsYesterday's own doc comment on
// why Delta.OK is false without one.
func TestRunWatch_ThresholdBreach_PriceDropMeetsThreshold(t *testing.T) {
	now := time.Date(2026, 8, 24, 7, 0, 0, 0, time.UTC)
	w := flightWatch(t)
	w.ThresholdPct = 10 // alert on a 10%+ drop

	repo := storetest.New()
	if err := repo.UpsertPriceSample(context.Background(), domain.PriceSample{
		WatchID: w.ID, SampleDate: now.AddDate(0, 0, -1), MedianMinor: 1000000, MinMinor: 1000000, MaxMinor: 1000000, NQuotes: 1,
	}); err != nil {
		t.Fatal(err)
	}

	registry := providers.NewRegistry()
	registry.Register(&fakeProvider{
		kind: domain.AssetFlightReturn,
		quotes: []domain.Quote{
			// A 50% drop from yesterday's 1000000 — comfortably past the 10% threshold.
			{WatchID: w.ID, PriceMinor: 500000, Currency: "INR", DepartDate: "2026-12-10", ReturnDate: "2026-12-15", Fingerprint: "a"},
		},
	})
	p := &Pipeline{Registry: registry, Repo: repo, Clock: fixedClock{now}}

	res, err := p.RunWatch(context.Background(), "run-breach", w)
	if err != nil {
		t.Fatalf("RunWatch: %v", err)
	}
	if !res.ThresholdBreach {
		t.Error("expected ThresholdBreach=true for a 50% drop against a 10% threshold")
	}
}

// TestRunWatch_ThresholdBreach_DropBelowThreshold_NoBreach is the same
// setup with a drop that doesn't clear the configured threshold.
func TestRunWatch_ThresholdBreach_DropBelowThreshold_NoBreach(t *testing.T) {
	now := time.Date(2026, 8, 24, 7, 0, 0, 0, time.UTC)
	w := flightWatch(t)
	w.ThresholdPct = 50

	repo := storetest.New()
	if err := repo.UpsertPriceSample(context.Background(), domain.PriceSample{
		WatchID: w.ID, SampleDate: now.AddDate(0, 0, -1), MedianMinor: 1000000, MinMinor: 1000000, MaxMinor: 1000000, NQuotes: 1,
	}); err != nil {
		t.Fatal(err)
	}

	registry := providers.NewRegistry()
	registry.Register(&fakeProvider{
		kind: domain.AssetFlightReturn,
		quotes: []domain.Quote{
			// A 10% drop — real, but short of the 50% threshold configured.
			{WatchID: w.ID, PriceMinor: 900000, Currency: "INR", DepartDate: "2026-12-10", ReturnDate: "2026-12-15", Fingerprint: "a"},
		},
	})
	p := &Pipeline{Registry: registry, Repo: repo, Clock: fixedClock{now}}

	res, err := p.RunWatch(context.Background(), "run-no-breach", w)
	if err != nil {
		t.Fatalf("RunWatch: %v", err)
	}
	if res.ThresholdBreach {
		t.Error("expected ThresholdBreach=false — a 10% drop should not clear a 50% threshold")
	}
}

// TestRunWatch_ThresholdBreach_PriceRise_NoBreach guards the sign
// convention: a price going up must never count as a breach, even past
// the configured percentage.
func TestRunWatch_ThresholdBreach_PriceRise_NoBreach(t *testing.T) {
	now := time.Date(2026, 8, 24, 7, 0, 0, 0, time.UTC)
	w := flightWatch(t)
	w.ThresholdPct = 10

	repo := storetest.New()
	if err := repo.UpsertPriceSample(context.Background(), domain.PriceSample{
		WatchID: w.ID, SampleDate: now.AddDate(0, 0, -1), MedianMinor: 500000, MinMinor: 500000, MaxMinor: 500000, NQuotes: 1,
	}); err != nil {
		t.Fatal(err)
	}

	registry := providers.NewRegistry()
	registry.Register(&fakeProvider{
		kind: domain.AssetFlightReturn,
		quotes: []domain.Quote{
			// A 100% price rise from yesterday.
			{WatchID: w.ID, PriceMinor: 1000000, Currency: "INR", DepartDate: "2026-12-10", ReturnDate: "2026-12-15", Fingerprint: "a"},
		},
	})
	p := &Pipeline{Registry: registry, Repo: repo, Clock: fixedClock{now}}

	res, err := p.RunWatch(context.Background(), "run-rise", w)
	if err != nil {
		t.Fatalf("RunWatch: %v", err)
	}
	if res.ThresholdBreach {
		t.Error("expected ThresholdBreach=false — a price rise is not a threshold breach")
	}
}

func TestTargetDates_Flight(t *testing.T) {
	w := flightWatch(t)
	depart, ret, err := targetDates(w)
	if err != nil {
		t.Fatal(err)
	}
	if depart != "2026-12-10" || ret != "2026-12-15" {
		t.Errorf("targetDates = %q, %q, want 2026-12-10, 2026-12-15", depart, ret)
	}
}

func TestTargetDates_OneWay_EmptyReturn(t *testing.T) {
	params, _ := json.Marshal(domain.FlightParams{Origin: "BLR", Destination: "GOI", DepartDate: "2026-12-10"})
	w := domain.Watch{Kind: domain.AssetFlightOneWay, Params: params}
	_, ret, err := targetDates(w)
	if err != nil {
		t.Fatal(err)
	}
	if ret != "" {
		t.Errorf("expected empty return date for a one-way watch, got %q", ret)
	}
}

func TestRollupForWatch_ComputesMinMedianMax(t *testing.T) {
	w := flightWatch(t)
	now := time.Date(2026, 8, 24, 7, 0, 0, 0, time.UTC)
	quotes := []domain.Quote{
		{DepartDate: "2026-12-10", ReturnDate: "2026-12-15", PriceMinor: 300},
		{DepartDate: "2026-12-10", ReturnDate: "2026-12-15", PriceMinor: 100},
		{DepartDate: "2026-12-10", ReturnDate: "2026-12-15", PriceMinor: 200},
		{DepartDate: "2026-12-11", ReturnDate: "2026-12-16", PriceMinor: 1}, // wrong date, excluded
	}
	sample, match, ok := rollupForWatch(w, quotes, now)
	if !ok {
		t.Fatal("expected rollupForWatch to find matching quotes")
	}
	if !match.Exact {
		t.Error("expected an exact match when one exists, not the nearest-available fallback")
	}
	if sample.MinMinor != 100 || sample.MedianMinor != 200 || sample.MaxMinor != 300 || sample.NQuotes != 3 {
		t.Errorf("got min:%d median:%d max:%d n:%d, want min:100 median:200 max:300 n:3",
			sample.MinMinor, sample.MedianMinor, sample.MaxMinor, sample.NQuotes)
	}
	if !sample.SampleDate.Equal(time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("SampleDate = %v, want today (the fetch day), not the travel date", sample.SampleDate)
	}
}

func TestRollupForWatch_NoMatch(t *testing.T) {
	w := flightWatch(t)
	_, _, ok := rollupForWatch(w, []domain.Quote{{DepartDate: "2099-01-01"}}, time.Now())
	if ok {
		t.Fatal("expected ok=false when nothing is within the drift cap of the watch's configured date")
	}
}

// TestRollupForWatch_NearestMatch_WithinCap_Succeeds is the real case that
// motivated the fallback: a DEL→GAU watch configured for Oct 31 → Nov 7
// whose only real fare that month was Oct 17 → Nov 18 (14 days off on
// depart, 11 on return) — previously this failed the run outright.
func TestRollupForWatch_NearestMatch_WithinCap_Succeeds(t *testing.T) {
	w := flightWatch(t) // configured for 2026-12-10 -> 2026-12-15
	quotes := []domain.Quote{
		{DepartDate: "2026-12-24", ReturnDate: "2026-12-26", PriceMinor: 500}, // 14/11 days off — within the 21-day cap
	}
	sample, match, ok := rollupForWatch(w, quotes, time.Now())
	if !ok {
		t.Fatal("expected the nearest-available fallback to succeed within the drift cap")
	}
	if match.Exact {
		t.Error("expected Exact=false — this was a fallback match, not the configured dates")
	}
	if match.MatchedDepart != "2026-12-24" || match.MatchedReturn != "2026-12-26" {
		t.Errorf("matched dates = %s/%s, want the fallback fare's own dates", match.MatchedDepart, match.MatchedReturn)
	}
	if match.ConfiguredDepart != "2026-12-10" || match.ConfiguredReturn != "2026-12-15" {
		t.Errorf("configured dates = %s/%s, want the watch's own configured dates preserved", match.ConfiguredDepart, match.ConfiguredReturn)
	}
	if sample.MinMinor != 500 {
		t.Errorf("MinMinor = %d, want 500", sample.MinMinor)
	}
}

func TestRollupForWatch_NearestMatch_BeyondCap_Fails(t *testing.T) {
	w := flightWatch(t) // configured for 2026-12-10 -> 2026-12-15
	quotes := []domain.Quote{
		{DepartDate: "2027-06-01", ReturnDate: "2027-06-08", PriceMinor: 500}, // ~6 months off — not the same trip
	}
	_, _, ok := rollupForWatch(w, quotes, time.Now())
	if ok {
		t.Fatal("expected ok=false when the closest fare is still beyond the drift cap")
	}
}

func TestRollupForWatch_NearestMatch_PicksTheClosestAmongSeveral(t *testing.T) {
	w := flightWatch(t) // configured for 2026-12-10 -> 2026-12-15
	quotes := []domain.Quote{
		{DepartDate: "2026-12-20", ReturnDate: "2026-12-25", PriceMinor: 999}, // 10 days off
		{DepartDate: "2026-12-13", ReturnDate: "2026-12-18", PriceMinor: 700}, // 3 days off — closest
		{DepartDate: "2026-12-24", ReturnDate: "2026-12-26", PriceMinor: 500}, // 14/11 days off
	}
	_, match, ok := rollupForWatch(w, quotes, time.Now())
	if !ok {
		t.Fatal("expected the nearest-available fallback to succeed")
	}
	if match.MatchedDepart != "2026-12-13" || match.MatchedReturn != "2026-12-18" {
		t.Errorf("matched dates = %s/%s, want the closest fare (2026-12-13/2026-12-18)", match.MatchedDepart, match.MatchedReturn)
	}
}

func TestRollupForWatch_OneWay_FallbackIgnoresReturnLeg(t *testing.T) {
	params, _ := json.Marshal(domain.FlightParams{Origin: "BLR", Destination: "GOI", DepartDate: "2026-12-10"})
	w := domain.Watch{Kind: domain.AssetFlightOneWay, Params: params}
	quotes := []domain.Quote{
		{DepartDate: "2026-12-15", ReturnDate: "", PriceMinor: 500}, // 5 days off, no return leg to compare
	}
	_, match, ok := rollupForWatch(w, quotes, time.Now())
	if !ok {
		t.Fatal("expected the nearest-available fallback to succeed for a one-way watch")
	}
	if match.MatchedDepart != "2026-12-15" || match.MatchedReturn != "" {
		t.Errorf("matched dates = %s/%q, want 2026-12-15/\"\"", match.MatchedDepart, match.MatchedReturn)
	}
}
