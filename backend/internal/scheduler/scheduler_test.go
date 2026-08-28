package scheduler

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/SamPariat/pricewatch/internal/domain"
	"github.com/SamPariat/pricewatch/internal/notify/noop"
	"github.com/SamPariat/pricewatch/internal/pipeline"
	"github.com/SamPariat/pricewatch/internal/providers"
	"github.com/SamPariat/pricewatch/internal/store/storetest"
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

// recordingNotifier captures the last Message sent, so tests can assert
// on Buttons without a real Discord client.
type recordingNotifier struct {
	sent []domain.Message
}

func (n *recordingNotifier) Send(ctx context.Context, t domain.Target, m domain.Message) error {
	n.sent = append(n.sent, m)
	return nil
}
func (n *recordingNotifier) Status(ctx context.Context) (domain.NotifierStatus, error) {
	return domain.NotifierLinked, nil
}

type succeedingProvider struct{ kind domain.AssetKind }

func (p *succeedingProvider) Kind() domain.AssetKind { return p.kind }
func (p *succeedingProvider) Fetch(ctx context.Context, w domain.Watch) ([]domain.Quote, error) {
	return []domain.Quote{{WatchID: w.ID, PriceMinor: 800000, DepartDate: "2026-12-10", Fingerprint: "a"}}, nil
}

// priceProvider is succeedingProvider with a caller-chosen price, used by
// the threshold-alert tests to produce a specific-size drop against a
// seeded prior-day sample.
type priceProvider struct {
	kind       domain.AssetKind
	priceMinor int64
}

func (p *priceProvider) Kind() domain.AssetKind { return p.kind }
func (p *priceProvider) Fetch(ctx context.Context, w domain.Watch) ([]domain.Quote, error) {
	return []domain.Quote{{WatchID: w.ID, PriceMinor: p.priceMinor, DepartDate: "2026-12-10", Fingerprint: "a"}}, nil
}

func TestFireGroup_SingleWatch_AttachesSnoozeButtons(t *testing.T) {
	repo := storetest.New()
	w := flightWatch(t, "solo", "0 7 * * *")
	repo.SeedWatch(w)
	repo.UpdateSettings(context.Background(), domain.Settings{DiscordChannelID: "chat1", Currency: "INR"})

	registry := providers.NewRegistry()
	registry.Register(&succeedingProvider{kind: domain.AssetFlightOneWay})
	p := &pipeline.Pipeline{Registry: registry, Repo: repo, Clock: fixedClock{time.Now()}}
	notifier := &recordingNotifier{}
	s := New(repo, p, notifier)

	s.fireGroup(context.Background(), []domain.Watch{w})

	if len(notifier.sent) != 1 {
		t.Fatalf("got %d sends, want 1", len(notifier.sent))
	}
	if len(notifier.sent[0].Buttons) == 0 {
		t.Error("expected snooze/pause/refresh buttons on a single-watch digest")
	}
}

func TestFireGroup_MultipleWatches_NoButtons(t *testing.T) {
	repo := storetest.New()
	w1 := flightWatch(t, "multi1", "0 7 * * *")
	w2 := flightWatch(t, "multi2", "0 7 * * *")
	repo.SeedWatch(w1)
	repo.SeedWatch(w2)
	repo.UpdateSettings(context.Background(), domain.Settings{DiscordChannelID: "chat1", Currency: "INR"})

	registry := providers.NewRegistry()
	registry.Register(&succeedingProvider{kind: domain.AssetFlightOneWay})
	p := &pipeline.Pipeline{Registry: registry, Repo: repo, Clock: fixedClock{time.Now()}}
	notifier := &recordingNotifier{}
	s := New(repo, p, notifier)

	s.fireGroup(context.Background(), []domain.Watch{w1, w2})

	if len(notifier.sent) != 1 {
		t.Fatalf("got %d sends, want 1", len(notifier.sent))
	}
	if len(notifier.sent[0].Buttons) != 0 {
		t.Error("expected no buttons on a batched multi-watch digest — there's no single watch for them to act on")
	}
}

func TestFireGroup_SkipsSnoozedWatch(t *testing.T) {
	repo := storetest.New()
	w := flightWatch(t, "snoozed", "0 7 * * *")
	repo.SeedWatch(w)
	repo.UpdateSettings(context.Background(), domain.Settings{DiscordChannelID: "chat1", Currency: "INR"})
	until := time.Now().Add(7 * 24 * time.Hour)
	if err := repo.SetSnooze(context.Background(), w.ID, &until); err != nil {
		t.Fatal(err)
	}

	registry := providers.NewRegistry()
	registry.Register(&succeedingProvider{kind: domain.AssetFlightOneWay})
	p := &pipeline.Pipeline{Registry: registry, Repo: repo, Clock: fixedClock{time.Now()}}
	notifier := &recordingNotifier{}
	s := New(repo, p, notifier)

	s.fireGroup(context.Background(), []domain.Watch{w})

	if len(notifier.sent) != 0 {
		t.Errorf("expected no send for a snoozed watch, got %d", len(notifier.sent))
	}
}

func TestFireGroup_ExpiredSnooze_StillRuns(t *testing.T) {
	repo := storetest.New()
	w := flightWatch(t, "expired-snooze", "0 7 * * *")
	repo.SeedWatch(w)
	repo.UpdateSettings(context.Background(), domain.Settings{DiscordChannelID: "chat1", Currency: "INR"})
	past := time.Now().Add(-time.Hour) // snooze already lapsed
	if err := repo.SetSnooze(context.Background(), w.ID, &past); err != nil {
		t.Fatal(err)
	}

	registry := providers.NewRegistry()
	registry.Register(&succeedingProvider{kind: domain.AssetFlightOneWay})
	p := &pipeline.Pipeline{Registry: registry, Repo: repo, Clock: fixedClock{time.Now()}}
	notifier := &recordingNotifier{}
	s := New(repo, p, notifier)

	s.fireGroup(context.Background(), []domain.Watch{w})

	if len(notifier.sent) != 1 {
		t.Errorf("expected the watch to run once its snooze has lapsed, got %d sends", len(notifier.sent))
	}
}

// TestFireGroup_ThresholdBreach_SendsExtraAlert seeds yesterday's price so
// today's fetch registers as a real drop, then asserts the breached watch
// gets its own alert message on top of (not instead of) the normal
// digest send — see pipeline.RunResult.ThresholdBreach's doc comment.
func TestFireGroup_ThresholdBreach_SendsExtraAlert(t *testing.T) {
	repo := storetest.New()
	// A fixed UTC time, not time.Now() — rollupForWatch always stamps a
	// fresh sample's SampleDate in time.UTC, and dateOnly() (which the
	// delta comparison uses) preserves whatever Location the seeded
	// sample was given, so a Location mismatch here would make the
	// "yesterday" sample invisible to DeltaVsYesterday on any machine
	// not already running in UTC.
	now := time.Date(2026, 8, 24, 7, 0, 0, 0, time.UTC)
	w := flightWatch(t, "cheap", "0 7 * * *")
	w.ThresholdPct = 10
	repo.SeedWatch(w)
	repo.UpdateSettings(context.Background(), domain.Settings{DiscordChannelID: "chat1", Currency: "INR"})
	if err := repo.UpsertPriceSample(context.Background(), domain.PriceSample{
		WatchID: w.ID, SampleDate: now.AddDate(0, 0, -1), MedianMinor: 1000000, MinMinor: 1000000, MaxMinor: 1000000, NQuotes: 1,
	}); err != nil {
		t.Fatal(err)
	}

	registry := providers.NewRegistry()
	registry.Register(&priceProvider{kind: domain.AssetFlightOneWay, priceMinor: 500000}) // 50% drop
	p := &pipeline.Pipeline{Registry: registry, Repo: repo, Clock: fixedClock{now}}
	notifier := &recordingNotifier{}
	s := New(repo, p, notifier)

	s.fireGroup(context.Background(), []domain.Watch{w})

	if len(notifier.sent) != 2 {
		t.Fatalf("got %d sends, want 2 (threshold alert + digest)", len(notifier.sent))
	}
	if !strings.Contains(notifier.sent[0].Content, "Threshold alert") {
		t.Errorf("expected the first send to be the threshold alert, got: %q", notifier.sent[0].Content)
	}
	if len(notifier.sent[0].Buttons) == 0 {
		t.Error("expected snooze/pause/refresh buttons on the threshold alert")
	}
	if strings.Contains(notifier.sent[1].Content, "Threshold alert") {
		t.Errorf("expected the second send to be the normal digest, got: %q", notifier.sent[1].Content)
	}
}

// TestFireGroup_NoThreshold_SendsOnlyDigest is the control for the test
// above: with no ThresholdPct configured, the same 50% drop must not
// produce an extra alert send.
func TestFireGroup_NoThreshold_SendsOnlyDigest(t *testing.T) {
	repo := storetest.New()
	now := time.Date(2026, 8, 24, 7, 0, 0, 0, time.UTC)
	w := flightWatch(t, "no-threshold", "0 7 * * *") // ThresholdPct left at zero value
	repo.SeedWatch(w)
	repo.UpdateSettings(context.Background(), domain.Settings{DiscordChannelID: "chat1", Currency: "INR"})
	if err := repo.UpsertPriceSample(context.Background(), domain.PriceSample{
		WatchID: w.ID, SampleDate: now.AddDate(0, 0, -1), MedianMinor: 1000000, MinMinor: 1000000, MaxMinor: 1000000, NQuotes: 1,
	}); err != nil {
		t.Fatal(err)
	}

	registry := providers.NewRegistry()
	registry.Register(&priceProvider{kind: domain.AssetFlightOneWay, priceMinor: 500000})
	p := &pipeline.Pipeline{Registry: registry, Repo: repo, Clock: fixedClock{now}}
	notifier := &recordingNotifier{}
	s := New(repo, p, notifier)

	s.fireGroup(context.Background(), []domain.Watch{w})

	if len(notifier.sent) != 1 {
		t.Fatalf("got %d sends, want 1 (digest only — no threshold configured)", len(notifier.sent))
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
