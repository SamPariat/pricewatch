// Package scheduler owns the cron entries derived from the watches table
// and fires the pipeline, batching watches that share an exact schedule
// into one digest send. See PLAN.md § Caching for why this package also
// owns TTL(): "data only changes when the cron fires" — it's the one
// place that actually knows when that is.
package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/robfig/cron/v3"

	"github.com/sampariat/prices-reminder/internal/domain"
	"github.com/sampariat/prices-reminder/internal/logging"
	"github.com/sampariat/prices-reminder/internal/pipeline"
	"github.com/sampariat/prices-reminder/internal/render"
)

const (
	minTTL = 30 * time.Second
	maxTTL = 6 * time.Hour
)

type Scheduler struct {
	repo     domain.Repository
	pipeline *pipeline.Pipeline
	notifier domain.Notifier

	mu       sync.RWMutex
	c        *cron.Cron
	nextRun  map[domain.WatchID]time.Time
	interval map[domain.WatchID]time.Duration
}

func New(repo domain.Repository, p *pipeline.Pipeline, notifier domain.Notifier) *Scheduler {
	return &Scheduler{
		repo: repo, pipeline: p, notifier: notifier,
		nextRun:  make(map[domain.WatchID]time.Time),
		interval: make(map[domain.WatchID]time.Duration),
	}
}

// Reload rebuilds every cron entry from the currently enabled watches,
// grouping watches that share an exact (timezone, cron_expr) so they fire
// — and send — together as one digest message (PLAN.md § Telegram: "one
// digest message for all watches, not one per watch"). Call it after any
// watch create/update/delete/enable/disable so cron reflects the database
// without a process restart.
func (s *Scheduler) Reload(ctx context.Context) error {
	watches, err := s.repo.ListWatches(ctx)
	if err != nil {
		return fmt.Errorf("scheduler: list watches: %w", err)
	}

	groups := make(map[string][]domain.Watch)
	for _, w := range watches {
		if !w.Enabled {
			continue
		}
		// robfig/cron's standard parser strips a leading CRON_TZ= prefix
		// and applies it as that entry's own location — this is how one
		// Cron instance supports per-watch timezones without needing a
		// separate instance per timezone.
		spec := w.CronExpr
		if w.Timezone != "" {
			spec = "CRON_TZ=" + w.Timezone + " " + w.CronExpr
		}
		groups[spec] = append(groups[spec], w)
	}

	c := cron.New(cron.WithChain(cron.Recover(cronLogger{}), cron.SkipIfStillRunning(cronLogger{})))
	nextRun := make(map[domain.WatchID]time.Time)
	interval := make(map[domain.WatchID]time.Duration)

	for spec, ws := range groups {
		// cron.Entry.Next is only populated by the Cron instance's own
		// internal goroutine once Start() has run — reading it here,
		// before Start(), would always see the zero value. Parsing the
		// schedule ourselves and calling Next() directly sidesteps that
		// entirely and lets Reload report accurate NextRun/TTL values
		// immediately, without waiting on the scheduler goroutine.
		schedule, perr := cron.ParseStandard(spec)
		if perr != nil {
			return fmt.Errorf("scheduler: bad schedule %q: %w", spec, perr)
		}
		if _, err = c.AddFunc(spec, func() {
			s.fireGroup(context.Background(), ws)
			s.mu.Lock()
			next := schedule.Next(time.Now())
			for _, w := range ws {
				s.nextRun[w.ID] = next
			}
			s.mu.Unlock()
		}); err != nil {
			return fmt.Errorf("scheduler: bad schedule %q: %w", spec, err)
		}
		next1 := schedule.Next(time.Now())
		// The gap between the next two fires — a plain reading of
		// "the schedule's interval" for a regular cadence (daily,
		// every N minutes), and a reasonable approximation for an
		// irregular one. Used by the API layer to flag a watch as
		// stale once it's gone more than 2x this long without a
		// successful fetch (PLAN.md § Freshness).
		gap := schedule.Next(next1).Sub(next1)
		for _, w := range ws {
			nextRun[w.ID] = next1
			interval[w.ID] = gap
		}
	}

	s.mu.Lock()
	old := s.c
	s.c = c
	s.nextRun = nextRun
	s.interval = interval
	s.mu.Unlock()

	if old != nil {
		old.Stop()
	}
	c.Start()
	return nil
}

func (s *Scheduler) Stop() {
	s.mu.RLock()
	c := s.c
	s.mu.RUnlock()
	if c != nil {
		c.Stop()
	}
}

// NextRun is the next scheduled fire time for a watch, or the zero time
// if it isn't currently scheduled (disabled, or created since the last
// Reload).
func (s *Scheduler) NextRun(watchID domain.WatchID) time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.nextRun[watchID]
}

// Interval is the gap between this watch's scheduled fires, or zero if
// it isn't currently scheduled. The staleness check in PLAN.md §
// Freshness — "older than 2x the watch's cron interval" — is this value
// times two.
func (s *Scheduler) Interval(watchID domain.WatchID) time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.interval[watchID]
}

// TTL is how long a cache entry for this watch should live: exactly until
// its next cron fire, clamped so a misconfigured or unscheduled watch
// can't produce a zero or unbounded TTL. This is the single source of
// truth every cache layer in PLAN.md § Caching derives from.
func (s *Scheduler) TTL(watchID domain.WatchID) time.Duration {
	next := s.NextRun(watchID)
	if next.IsZero() {
		return minTTL
	}
	d := time.Until(next)
	if d < minTTL {
		return minTTL
	}
	if d > maxTTL {
		return maxTTL
	}
	return d
}

func (s *Scheduler) fireGroup(ctx context.Context, ws []domain.Watch) {
	settings, err := s.repo.GetSettings(ctx)
	if err != nil {
		logging.From(ctx).Error("scheduler: get settings", "error", err)
		return
	}

	var sections []string
	for _, w := range ws {
		runID := domain.RunID(ulid.Make().String())
		rctx := logging.With(ctx, "run_id", string(runID), "watch_id", string(w.ID))
		text, err := s.pipeline.RunWatch(rctx, runID, w)
		if err != nil {
			logging.From(rctx).Error("scheduler: run failed", "error", err)
			continue
		}
		sections = append(sections, text)
	}
	if len(sections) == 0 {
		return
	}

	if settings.DryRun {
		logging.From(ctx).Info("scheduler: dry-run, digest not sent", "sections", len(sections))
		return
	}
	if settings.TelegramChatID == "" {
		logging.From(ctx).Warn("scheduler: no telegram chat configured, digest not sent")
		return
	}

	msg := domain.Message{Text: render.CombineDigest(sections)}
	if err := s.notifier.Send(ctx, domain.Target{ChatID: settings.TelegramChatID}, msg); err != nil {
		logging.From(ctx).Error("scheduler: send failed", "error", err)
	}
}

// cronLogger adapts internal/logging to robfig/cron's Logger interface,
// used for panic recovery and skip-if-still-running log lines.
type cronLogger struct{}

func (cronLogger) Info(msg string, keysAndValues ...any) {
	logging.From(context.Background()).Info(msg, keysAndValues...)
}

func (cronLogger) Error(err error, msg string, keysAndValues ...any) {
	logging.From(context.Background()).Error(msg, append(keysAndValues, "error", err)...)
}
