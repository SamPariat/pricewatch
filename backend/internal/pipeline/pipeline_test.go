package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/sampariat/prices-reminder/internal/domain"
	"github.com/sampariat/prices-reminder/internal/providers"
	"github.com/sampariat/prices-reminder/internal/store/storetest"
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

	text, err := p.RunWatch(context.Background(), "run1", w)
	if err != nil {
		t.Fatalf("RunWatch: %v", err)
	}
	if text == "" {
		t.Fatal("expected non-empty rendered text")
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

func TestRunWatch_NoMatchingQuote_Fails(t *testing.T) {
	now := time.Date(2026, 8, 24, 7, 0, 0, 0, time.UTC)
	w := flightWatch(t)

	registry := providers.NewRegistry()
	registry.Register(&fakeProvider{
		kind: domain.AssetFlightReturn,
		quotes: []domain.Quote{
			{WatchID: w.ID, PriceMinor: 100, DepartDate: "2026-12-25", ReturnDate: "2026-12-31", Fingerprint: "x"},
		},
	})

	repo := storetest.New()
	p := &Pipeline{Registry: registry, Repo: repo, Clock: fixedClock{now}}

	_, err := p.RunWatch(context.Background(), "run6", w)
	if err == nil {
		t.Fatal("expected an error when no fetched quote matches the watch's configured date")
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

func TestTargetDates_Hotel(t *testing.T) {
	params, _ := json.Marshal(domain.HotelParams{Location: "Goa", CheckIn: "2026-12-10", CheckOut: "2026-12-15"})
	w := domain.Watch{Kind: domain.AssetHotel, Params: params}
	depart, ret, err := targetDates(w)
	if err != nil {
		t.Fatal(err)
	}
	if depart != "2026-12-10" || ret != "2026-12-15" {
		t.Errorf("targetDates = %q, %q, want check-in/out dates", depart, ret)
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
	sample, ok := rollupForWatch(w, quotes, now)
	if !ok {
		t.Fatal("expected rollupForWatch to find matching quotes")
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
	_, ok := rollupForWatch(w, []domain.Quote{{DepartDate: "2099-01-01"}}, time.Now())
	if ok {
		t.Fatal("expected ok=false when nothing matches the watch's configured date")
	}
}
