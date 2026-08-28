// Package storetest is an in-memory domain.Repository for tests — the
// httptest pattern: a regular importable package, used only from other
// packages' _test.go files, never from production code. Kept in one place
// because Repository has enough methods (PLAN.md deliberately keeps it to
// one interface, not one per table) that duplicating a fake per test
// package would be real toil, not "three similar lines."
package storetest

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/SamPariat/pricewatch/internal/domain"
)

var ErrNotFound = errors.New("storetest: not found")

type FakeRepository struct {
	mu         sync.Mutex
	nextID     int
	watches    map[domain.WatchID]domain.Watch
	quotes     []domain.Quote
	samples    map[domain.WatchID]map[string]domain.PriceSample // watchID -> sampleDate(YYYY-MM-DD) -> sample
	runs       map[domain.RunID]domain.DigestRun
	events     map[domain.RunID][]domain.RunEvent
	watchState map[domain.WatchID]domain.WatchState
	settings   domain.Settings
	trips      map[domain.TripID]domain.Trip
	requests   map[domain.RequestID]domain.Request
}

func New() *FakeRepository {
	return &FakeRepository{
		watches:    make(map[domain.WatchID]domain.Watch),
		samples:    make(map[domain.WatchID]map[string]domain.PriceSample),
		runs:       make(map[domain.RunID]domain.DigestRun),
		events:     make(map[domain.RunID][]domain.RunEvent),
		watchState: make(map[domain.WatchID]domain.WatchState),
		settings:   domain.Settings{Currency: "INR", Language: "en"},
		trips:      make(map[domain.TripID]domain.Trip),
		requests:   make(map[domain.RequestID]domain.Request),
	}
}

// SeedWatch inserts a watch with a caller-chosen ID, bypassing
// CreateWatch's ID generation — convenient for tests that want a known ID.
func (f *FakeRepository) SeedWatch(w domain.Watch) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.watches[w.ID] = w
}

// SeedTrip inserts a trip with a caller-chosen ID, bypassing CreateTrip's
// ID generation — convenient for tests that want a known ID (e.g. to
// seed a watch with a matching TripID).
func (f *FakeRepository) SeedTrip(t domain.Trip) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.trips[t.ID] = t
}

func (f *FakeRepository) CreateWatch(ctx context.Context, w domain.Watch) (domain.Watch, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextID++
	w.ID = domain.WatchID(strconv.Itoa(f.nextID))
	w.CreatedAt = time.Now()
	f.watches[w.ID] = w
	return w, nil
}

func (f *FakeRepository) GetWatch(ctx context.Context, id domain.WatchID) (domain.Watch, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	w, ok := f.watches[id]
	if !ok {
		return domain.Watch{}, ErrNotFound
	}
	return w, nil
}

func (f *FakeRepository) ListWatches(ctx context.Context) ([]domain.Watch, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]domain.Watch, 0, len(f.watches))
	for _, w := range f.watches {
		out = append(out, w)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (f *FakeRepository) UpdateWatch(ctx context.Context, w domain.Watch) (domain.Watch, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.watches[w.ID]; !ok {
		return domain.Watch{}, ErrNotFound
	}
	f.watches[w.ID] = w
	return w, nil
}

func (f *FakeRepository) DeleteWatch(ctx context.Context, id domain.WatchID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.watches[id]; !ok {
		return ErrNotFound
	}
	delete(f.watches, id)
	return nil
}

func (f *FakeRepository) InsertQuotes(ctx context.Context, qs []domain.Quote) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.quotes = append(f.quotes, qs...)
	return nil
}

func (f *FakeRepository) UpsertPriceSample(ctx context.Context, s domain.PriceSample) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.samples[s.WatchID] == nil {
		f.samples[s.WatchID] = make(map[string]domain.PriceSample)
	}
	f.samples[s.WatchID][s.SampleDate.Format("2006-01-02")] = s
	return nil
}

func (f *FakeRepository) ListPriceSamples(ctx context.Context, watchID domain.WatchID, since time.Time) ([]domain.PriceSample, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []domain.PriceSample
	for _, s := range f.samples[watchID] {
		if !s.SampleDate.Before(since) {
			out = append(out, s)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SampleDate.Before(out[j].SampleDate) })
	return out, nil
}

func (f *FakeRepository) CreateDigestRun(ctx context.Context, r domain.DigestRun) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.runs[r.RunID] = r
	return nil
}

func (f *FakeRepository) FinishDigestRun(ctx context.Context, r domain.DigestRun) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	existing := f.runs[r.RunID]
	existing.FinishedAt = r.FinishedAt
	existing.Status = r.Status
	existing.MessageBody = r.MessageBody
	existing.Error = r.Error
	f.runs[r.RunID] = existing
	return nil
}

func (f *FakeRepository) ListRecentDigestRuns(ctx context.Context, limit int) ([]domain.DigestRun, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]domain.DigestRun, 0, len(f.runs))
	for _, r := range f.runs {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartedAt.After(out[j].StartedAt) })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (f *FakeRepository) InsertRunEvent(ctx context.Context, e domain.RunEvent) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events[e.RunID] = append(f.events[e.RunID], e)
	return nil
}

func (f *FakeRepository) ListRunEvents(ctx context.Context, runID domain.RunID) ([]domain.RunEvent, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]domain.RunEvent(nil), f.events[runID]...), nil
}

func (f *FakeRepository) GetWatchState(ctx context.Context, watchID domain.WatchID) (domain.WatchState, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s, ok := f.watchState[watchID]
	if !ok {
		return domain.WatchState{WatchID: watchID}, nil
	}
	return s, nil
}

// UpsertWatchState writes run-health fields only, preserving whatever
// SnoozedUntil is already stored — matching Postgres.UpsertWatchState,
// which never includes that column in its UPDATE SET clause. A naive
// whole-struct overwrite here would silently clear an active snooze on
// every run, a behavior real Postgres does NOT have — this fake must not
// diverge from it.
func (f *FakeRepository) UpsertWatchState(ctx context.Context, s domain.WatchState) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	s.SnoozedUntil = f.watchState[s.WatchID].SnoozedUntil
	f.watchState[s.WatchID] = s
	return nil
}

func (f *FakeRepository) SetSnooze(ctx context.Context, watchID domain.WatchID, until *time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	s := f.watchState[watchID]
	s.WatchID = watchID
	s.SnoozedUntil = until
	f.watchState[watchID] = s
	return nil
}

func (f *FakeRepository) GetSettings(ctx context.Context) (domain.Settings, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.settings, nil
}

func (f *FakeRepository) UpdateSettings(ctx context.Context, s domain.Settings) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.settings = s
	return nil
}

// Runs returns a snapshot of every DigestRun recorded — for tests to
// assert on after calling something that should have written one.
func (f *FakeRepository) Runs() map[domain.RunID]domain.DigestRun {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make(map[domain.RunID]domain.DigestRun, len(f.runs))
	for k, v := range f.runs {
		out[k] = v
	}
	return out
}

// --- Trips ---

func (f *FakeRepository) CreateTrip(ctx context.Context, t domain.Trip) (domain.Trip, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextID++
	t.ID = domain.TripID(strconv.Itoa(f.nextID))
	t.CreatedAt = time.Now()
	f.trips[t.ID] = t
	return t, nil
}

func (f *FakeRepository) GetTrip(ctx context.Context, id domain.TripID) (domain.Trip, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	t, ok := f.trips[id]
	if !ok {
		return domain.Trip{}, ErrNotFound
	}
	return t, nil
}

func (f *FakeRepository) ListTrips(ctx context.Context) ([]domain.Trip, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]domain.Trip, 0, len(f.trips))
	for _, t := range f.trips {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (f *FakeRepository) UpdateTrip(ctx context.Context, t domain.Trip) (domain.Trip, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.trips[t.ID]; !ok {
		return domain.Trip{}, ErrNotFound
	}
	f.trips[t.ID] = t
	return t, nil
}

// DeleteTrip mirrors the migration's ON DELETE CASCADE — every leg
// belonging to this trip is deleted along with it.
func (f *FakeRepository) DeleteTrip(ctx context.Context, id domain.TripID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.trips[id]; !ok {
		return ErrNotFound
	}
	delete(f.trips, id)
	for wid, w := range f.watches {
		if w.TripID == id {
			delete(f.watches, wid)
		}
	}
	return nil
}

// --- Requests ---

func (f *FakeRepository) CreateRequest(ctx context.Context, r domain.Request) (domain.Request, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextID++
	r.ID = domain.RequestID(strconv.Itoa(f.nextID))
	r.CreatedAt = time.Now()
	f.requests[r.ID] = r
	return r, nil
}

func (f *FakeRepository) GetRequest(ctx context.Context, id domain.RequestID) (domain.Request, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.requests[id]
	if !ok {
		return domain.Request{}, ErrNotFound
	}
	return r, nil
}

func (f *FakeRepository) ListRequests(ctx context.Context, status domain.RequestStatus) ([]domain.Request, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]domain.Request, 0, len(f.requests))
	for _, r := range f.requests {
		if status == "" || r.Status == status {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

func (f *FakeRepository) ResolveRequest(ctx context.Context, id domain.RequestID, status domain.RequestStatus) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.requests[id]
	if !ok {
		return ErrNotFound
	}
	now := time.Now()
	r.Status = status
	r.ResolvedAt = &now
	f.requests[id] = r
	return nil
}
