package scheduler

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/sampariat/prices-reminder/internal/domain"
	"github.com/sampariat/prices-reminder/internal/notify/noop"
	"github.com/sampariat/prices-reminder/internal/pipeline"
	"github.com/sampariat/prices-reminder/internal/providers"
	"github.com/sampariat/prices-reminder/internal/store/storetest"
)

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

func flightWatch(t *testing.T, id, cronExpr string) domain.Watch {
	t.Helper()
	params, err := json.Marshal(domain.FlightParams{Origin: "BLR", Destination: "GOI", DepartDate: "2026-12-10"})
	if err != nil {
		t.Fatal(err)
	}
	return domain.Watch{
		ID: domain.WatchID(id), Kind: domain.AssetFlightOneWay, Enabled: true,
		CronExpr: cronExpr, Timezone: "UTC", Params: params,
	}
}

func TestTTL_ClampsToMinAndMax(t *testing.T) {
	s := New(storetest.New(), nil, nil)

	// Directly set internal state rather than going through Reload/real
	// cron — this test is about the clamping arithmetic in TTL, not
	// about cron parsing.
	s.nextRun["past-due"] = time.Now().Add(-time.Hour) // already overdue
	s.nextRun["far-future"] = time.Now().Add(365 * 24 * time.Hour)
	s.nextRun["normal"] = time.Now().Add(45 * time.Minute)

	if got := s.TTL("past-due"); got != minTTL {
		t.Errorf("TTL(past-due) = %v, want minTTL (%v)", got, minTTL)
	}
	if got := s.TTL("far-future"); got != maxTTL {
		t.Errorf("TTL(far-future) = %v, want maxTTL (%v)", got, maxTTL)
	}
	if got := s.TTL("normal"); got <= minTTL || got > 46*time.Minute {
		t.Errorf("TTL(normal) = %v, want roughly 45m", got)
	}
}

func TestTTL_UnscheduledWatch_ReturnsMinTTL(t *testing.T) {
	s := New(storetest.New(), nil, nil)
	if got := s.TTL("never-scheduled"); got != minTTL {
		t.Errorf("TTL(never-scheduled) = %v, want minTTL (%v)", got, minTTL)
	}
}

func TestReload_WatchesSharingScheduleGetIdenticalNextRun(t *testing.T) {
	repo := storetest.New()
	repo.SeedWatch(flightWatch(t, "a", "*/5 * * * *"))
	repo.SeedWatch(flightWatch(t, "b", "*/5 * * * *")) // same schedule as a
	repo.SeedWatch(flightWatch(t, "c", "0 0 1 1 *"))   // once a year — very different Next

	p := &pipeline.Pipeline{Registry: providers.NewRegistry(), Repo: repo, Clock: fixedClock{time.Now()}}
	s := New(repo, p, noop.New())

	if err := s.Reload(context.Background()); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	defer s.Stop()

	nextA, nextB, nextC := s.NextRun("a"), s.NextRun("b"), s.NextRun("c")
	if nextA.IsZero() || nextB.IsZero() || nextC.IsZero() {
		t.Fatalf("expected all three watches to be scheduled, got a=%v b=%v c=%v", nextA, nextB, nextC)
	}
	if !nextA.Equal(nextB) {
		t.Errorf("watches a and b share a cron_expr and should share one entry (equal NextRun), got a=%v b=%v", nextA, nextB)
	}
	if nextA.Equal(nextC) {
		t.Errorf("watches a and c have different schedules and should not share a NextRun")
	}
}

func TestReload_Interval_MatchesScheduleGap(t *testing.T) {
	repo := storetest.New()
	repo.SeedWatch(flightWatch(t, "a", "*/5 * * * *")) // fires every 5 minutes

	p := &pipeline.Pipeline{Registry: providers.NewRegistry(), Repo: repo, Clock: fixedClock{time.Now()}}
	s := New(repo, p, noop.New())

	if err := s.Reload(context.Background()); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	defer s.Stop()

	got := s.Interval("a")
	if got < 4*time.Minute || got > 6*time.Minute {
		t.Errorf("Interval = %v, want roughly 5m for a */5 schedule", got)
	}
}

func TestReload_DisabledWatchIsNotScheduled(t *testing.T) {
	repo := storetest.New()
	w := flightWatch(t, "disabled", "*/5 * * * *")
	w.Enabled = false
	repo.SeedWatch(w)

	p := &pipeline.Pipeline{Registry: providers.NewRegistry(), Repo: repo, Clock: fixedClock{time.Now()}}
	s := New(repo, p, noop.New())

	if err := s.Reload(context.Background()); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	defer s.Stop()

	if got := s.NextRun("disabled"); !got.IsZero() {
		t.Errorf("expected a disabled watch to have no NextRun, got %v", got)
	}
}
