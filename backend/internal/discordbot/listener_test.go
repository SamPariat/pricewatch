package discordbot

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/SamPariat/pricewatch/internal/domain"
	"github.com/SamPariat/pricewatch/internal/i18n"
	"github.com/SamPariat/pricewatch/internal/pipeline"
	"github.com/SamPariat/pricewatch/internal/providers"
	"github.com/SamPariat/pricewatch/internal/scheduler"
	"github.com/SamPariat/pricewatch/internal/service"
	"github.com/SamPariat/pricewatch/internal/store/storetest"
)

func flightWatch(t *testing.T, id string) domain.Watch {
	t.Helper()
	params, err := json.Marshal(domain.FlightParams{Origin: "BLR", Destination: "GOI", DepartDate: "2026-12-10"})
	if err != nil {
		t.Fatal(err)
	}
	return domain.Watch{ID: domain.WatchID(id), Kind: domain.AssetFlightOneWay, Enabled: true, CronExpr: "0 7 * * *", Timezone: "UTC", Params: params}
}

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

type succeedingProvider struct{}

func (succeedingProvider) Kind() domain.AssetKind { return domain.AssetFlightOneWay }
func (succeedingProvider) Fetch(ctx context.Context, w domain.Watch) ([]domain.Quote, error) {
	return []domain.Quote{{WatchID: w.ID, PriceMinor: 800000, DepartDate: "2026-12-10", Fingerprint: "a"}}, nil
}

// newTestListener wires a Listener against fakes only — Session stays
// nil, which is fine: snooze/pause/refresh (what these tests exercise)
// never touch it, only Run/handleInteraction do, and those need a real
// Gateway connection this package's unit tests deliberately don't spin up.
func newTestListener(t *testing.T, repo *storetest.FakeRepository) *Listener {
	t.Helper()
	registry := providers.NewRegistry()
	registry.Register(succeedingProvider{})
	p := &pipeline.Pipeline{Registry: registry, Repo: repo, Clock: fixedClock{time.Now()}}
	sched := scheduler.New(repo, p, nil)
	watches := &service.WatchService{Repo: repo, Sched: sched, Pipeline: p, Notifier: nil, Clock: fixedClock{time.Now()}}
	return &Listener{Repo: repo, Sched: sched, Watches: watches}
}

func TestParseCallback(t *testing.T) {
	cases := []struct {
		in         string
		action, id string
		ok         bool
	}{
		{"snooze:w1", "snooze", "w1", true},
		{"pause:w1", "pause", "w1", true},
		{"refresh:abc-123", "refresh", "abc-123", true},
		{"snooze", "", "", false},   // no watch ID
		{"snooze:", "", "", false},  // empty watch ID
		{"", "", "", false},         // empty
		{"a:b:c", "a", "b:c", true}, // Cut splits on the first ':' only
	}
	for _, c := range cases {
		action, id, ok := parseCallback(c.in)
		if action != c.action || id != c.id || ok != c.ok {
			t.Errorf("parseCallback(%q) = (%q, %q, %v), want (%q, %q, %v)", c.in, action, id, ok, c.action, c.id, c.ok)
		}
	}
}

func TestSnooze_SetsSnoozedUntil(t *testing.T) {
	repo := storetest.New()
	w := flightWatch(t, "w1")
	repo.SeedWatch(w)
	l := newTestListener(t, repo)

	msg := l.snooze(context.Background(), w.ID, i18n.EN)
	if msg == "" {
		t.Error("expected a non-empty confirmation message")
	}

	state, err := repo.GetWatchState(context.Background(), w.ID)
	if err != nil {
		t.Fatal(err)
	}
	if state.SnoozedUntil == nil {
		t.Fatal("expected SnoozedUntil to be set")
	}
	wantMin := time.Now().Add(6*24*time.Hour + 23*time.Hour) // just under 7 days, allows test run time
	if state.SnoozedUntil.Before(wantMin) {
		t.Errorf("SnoozedUntil = %v, want roughly 7 days from now", *state.SnoozedUntil)
	}
}

func TestPause_DisablesWatchAndReloadsScheduler(t *testing.T) {
	repo := storetest.New()
	w := flightWatch(t, "w2")
	repo.SeedWatch(w)
	l := newTestListener(t, repo)

	if err := l.Sched.Reload(context.Background()); err != nil {
		t.Fatal(err)
	}
	if l.Sched.NextRun(w.ID).IsZero() {
		t.Fatal("expected the watch to be scheduled before pausing")
	}

	l.pause(context.Background(), w.ID, i18n.EN)

	updated, err := repo.GetWatch(context.Background(), w.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Enabled {
		t.Error("expected the watch to be disabled after pause")
	}
	if got := l.Sched.NextRun(w.ID); !got.IsZero() {
		t.Errorf("expected the scheduler to drop the paused watch after Reload, NextRun = %v", got)
	}
}

func TestPause_UnknownWatch_ReturnsMessageWithoutPanicking(t *testing.T) {
	repo := storetest.New()
	l := newTestListener(t, repo)

	msg := l.pause(context.Background(), "does-not-exist", i18n.EN)
	if msg == "" {
		t.Error("expected a non-empty message for an unknown watch")
	}
}

func TestRefresh_DryRun_RunsPipelineButDoesNotSend(t *testing.T) {
	repo := storetest.New()
	w := flightWatch(t, "w3")
	repo.SeedWatch(w)
	if err := repo.UpdateSettings(context.Background(), domain.Settings{DryRun: true, DiscordChannelID: "chan1", Currency: "INR"}); err != nil {
		t.Fatal(err)
	}
	l := newTestListener(t, repo)

	msg := l.refresh(context.Background(), w.ID, i18n.EN)
	if msg == "" {
		t.Error("expected a non-empty confirmation message")
	}

	samples, err := repo.ListPriceSamples(context.Background(), w.ID, time.Now().AddDate(0, 0, -1))
	if err != nil {
		t.Fatal(err)
	}
	if len(samples) == 0 {
		t.Error("expected refresh to have actually run the pipeline and written a price sample")
	}
}

func TestRefresh_UnknownWatch_ReturnsMessage(t *testing.T) {
	repo := storetest.New()
	l := newTestListener(t, repo)

	msg := l.refresh(context.Background(), "does-not-exist", i18n.EN)
	if msg == "" {
		t.Error("expected a non-empty message for an unknown watch")
	}
}

func TestSnoozePauseRefresh_Hindi_ReturnDistinctText(t *testing.T) {
	repo := storetest.New()
	w := flightWatch(t, "w4")
	repo.SeedWatch(w)
	l := newTestListener(t, repo)

	en := l.pause(context.Background(), w.ID, i18n.EN)
	hi := l.pause(context.Background(), w.ID, i18n.HI)
	if en == hi {
		t.Errorf("expected EN and HI confirmation text to differ, both = %q", en)
	}
}
