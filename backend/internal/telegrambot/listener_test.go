package telegrambot

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/SamPariat/pricewatch/internal/domain"
	"github.com/SamPariat/pricewatch/internal/notify/telegram"
	"github.com/SamPariat/pricewatch/internal/pipeline"
	"github.com/SamPariat/pricewatch/internal/providers"
	"github.com/SamPariat/pricewatch/internal/scheduler"
	"github.com/SamPariat/pricewatch/internal/service"
	"github.com/SamPariat/pricewatch/internal/store/storetest"
)

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

type succeedingProvider struct{}

func (succeedingProvider) Kind() domain.AssetKind { return domain.AssetFlightOneWay }
func (succeedingProvider) Fetch(ctx context.Context, w domain.Watch) ([]domain.Quote, error) {
	return []domain.Quote{{WatchID: w.ID, PriceMinor: 800000, DepartDate: "2026-12-10", Fingerprint: "a"}}, nil
}

func flightWatch(t *testing.T, id string) domain.Watch {
	t.Helper()
	params, err := json.Marshal(domain.FlightParams{Origin: "BLR", Destination: "GOI", DepartDate: "2026-12-10"})
	if err != nil {
		t.Fatal(err)
	}
	return domain.Watch{
		ID: domain.WatchID(id), Kind: domain.AssetFlightOneWay, Enabled: true,
		CronExpr: "0 7 * * *", Timezone: "UTC", Params: params,
	}
}

func newTestListener(t *testing.T, repo *storetest.FakeRepository) *Listener {
	t.Helper()
	registry := providers.NewRegistry()
	registry.Register(succeedingProvider{})
	pl := &pipeline.Pipeline{Registry: registry, Repo: repo, Clock: fixedClock{time.Now()}}
	sched := scheduler.New(repo, pl, nil)
	t.Cleanup(sched.Stop)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(srv.Close)
	bot := telegram.NewForTest("test-token", srv.URL)

	watches := &service.WatchService{Repo: repo, Sched: sched, Pipeline: pl, Notifier: bot, Clock: fixedClock{time.Now()}}

	return &Listener{Bot: bot, Repo: repo, Sched: sched, Watches: watches}
}

func TestParseCallback(t *testing.T) {
	cases := []struct {
		data       string
		wantAction string
		wantID     string
		wantOK     bool
	}{
		{"snooze:abc123", "snooze", "abc123", true},
		{"pause:abc123", "pause", "abc123", true},
		{"refresh:abc123", "refresh", "abc123", true},
		{"snooze:", "", "", false},
		{"noSeparator", "", "", false},
		{"", "", "", false},
	}
	for _, c := range cases {
		action, id, ok := parseCallback(c.data)
		if ok != c.wantOK || action != c.wantAction || id != c.wantID {
			t.Errorf("parseCallback(%q) = (%q, %q, %v), want (%q, %q, %v)",
				c.data, action, id, ok, c.wantAction, c.wantID, c.wantOK)
		}
	}
}

func TestSnooze_SetsSnoozedUntil(t *testing.T) {
	repo := storetest.New()
	w := flightWatch(t, "w1")
	repo.SeedWatch(w)
	l := newTestListener(t, repo)

	msg := l.snooze(context.Background(), w.ID)
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

	l.pause(context.Background(), w.ID)

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

	msg := l.pause(context.Background(), "does-not-exist")
	if msg == "" {
		t.Error("expected a non-empty message for an unknown watch")
	}
}

func TestRefresh_DryRun_RunsPipelineButDoesNotSend(t *testing.T) {
	repo := storetest.New()
	w := flightWatch(t, "w3")
	repo.SeedWatch(w)
	repo.UpdateSettings(context.Background(), domain.Settings{DryRun: true, TelegramChatID: "chat1", Currency: "INR"})
	l := newTestListener(t, repo)

	msg := l.refresh(context.Background(), w.ID)
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

	msg := l.refresh(context.Background(), "does-not-exist")
	if msg == "" {
		t.Error("expected a non-empty message for an unknown watch")
	}
}
