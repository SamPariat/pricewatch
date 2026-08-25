// Package telegrambot is the long-poll listener that turns a pressed
// inline keyboard button (Snooze 7d / Pause / Refresh now) into a domain
// action. It's a driving adapter — same category as internal/httpapi/
// handlers, just triggered by Telegram's getUpdates instead of an HTTP
// request — which is why it depends on concrete types (Repository,
// *scheduler.Scheduler, *pipeline.Pipeline, *telegram.Notifier) directly
// rather than only on ports: only internal/domain itself has to stay
// pure. See PLAN.md § Architecture patterns.
package telegrambot

import (
	"context"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/sampariat/prices-reminder/internal/domain"
	"github.com/sampariat/prices-reminder/internal/logging"
	"github.com/sampariat/prices-reminder/internal/notify/telegram"
	"github.com/sampariat/prices-reminder/internal/pipeline"
	"github.com/sampariat/prices-reminder/internal/providers"
	"github.com/sampariat/prices-reminder/internal/render"
	"github.com/sampariat/prices-reminder/internal/scheduler"
)

const snoozeDuration = 7 * 24 * time.Hour

type Listener struct {
	Bot      *telegram.Notifier
	Repo     domain.Repository
	Sched    *scheduler.Scheduler
	Pipeline *pipeline.Pipeline
}

// Run polls forever until ctx is canceled, meant to run as its own
// goroutine from the composition root. A getUpdates failure (network
// blip, Telegram hiccup) backs off 5s and retries rather than exiting —
// this loop is expected to run for the process's entire lifetime.
func (l *Listener) Run(ctx context.Context) {
	var offset int64
	for {
		if ctx.Err() != nil {
			return
		}

		updates, err := l.Bot.GetUpdates(ctx, offset, 30)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			logging.From(ctx).Warn("telegrambot: getUpdates failed", "error", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
			}
			continue
		}

		for _, u := range updates {
			offset = u.UpdateID + 1
			if u.CallbackQuery != nil {
				l.handleCallback(ctx, *u.CallbackQuery)
			}
		}
	}
}

func (l *Listener) handleCallback(ctx context.Context, cb telegram.CallbackQuery) {
	action, watchID, ok := parseCallback(cb.Data)
	if !ok {
		return
	}

	var confirmText string
	switch action {
	case "snooze":
		confirmText = l.snooze(ctx, domain.WatchID(watchID))
	case "pause":
		confirmText = l.pause(ctx, domain.WatchID(watchID))
	case "refresh":
		confirmText = l.refresh(ctx, domain.WatchID(watchID))
	default:
		return
	}

	if err := l.Bot.AnswerCallbackQuery(ctx, cb.ID, confirmText); err != nil {
		logging.From(ctx).Warn("telegrambot: answerCallbackQuery failed", "error", err)
	}
}

// parseCallback splits "action:watchID" — the format scheduler.go's
// snoozeButtons produces. Anything else (including a bare "action" with
// no watch ID) is rejected rather than guessed at.
func parseCallback(data string) (action, watchID string, ok bool) {
	before, after, found := strings.Cut(data, ":")
	if !found || after == "" {
		return "", "", false
	}
	return before, after, true
}

func (l *Listener) snooze(ctx context.Context, id domain.WatchID) string {
	until := time.Now().Add(snoozeDuration)
	if err := l.Repo.SetSnooze(ctx, id, &until); err != nil {
		logging.From(ctx).Error("telegrambot: snooze failed", "watch_id", id, "error", err)
		return "Failed to snooze — try again from the panel."
	}
	return "Snoozed for 7 days."
}

func (l *Listener) pause(ctx context.Context, id domain.WatchID) string {
	w, err := l.Repo.GetWatch(ctx, id)
	if err != nil {
		return "Watch not found."
	}
	w.Enabled = false
	if _, err := l.Repo.UpdateWatch(ctx, w); err != nil {
		logging.From(ctx).Error("telegrambot: pause failed", "watch_id", id, "error", err)
		return "Failed to pause — try again from the panel."
	}
	if err := l.Sched.Reload(ctx); err != nil {
		logging.From(ctx).Error("telegrambot: reload after pause", "error", err)
	}
	return "Paused. Re-enable it from the panel when you're ready."
}

// refresh mirrors handlers.RunWatchNow's shape (mint a run ID, run the
// pipeline, send unless dry-run) but isn't the same function — this
// fires from a Telegram callback, not an authenticated HTTP request, and
// the two call sites don't share enough surrounding context to be worth
// merging into one shared helper for ~10 lines.
func (l *Listener) refresh(ctx context.Context, id domain.WatchID) string {
	w, err := l.Repo.GetWatch(ctx, id)
	if err != nil {
		return "Watch not found."
	}

	runID := domain.RunID(ulid.Make().String())
	rctx := logging.With(ctx, "run_id", string(runID), "watch_id", string(id))
	rctx = providers.SkipCache(rctx)
	text, err := l.Pipeline.RunWatch(rctx, runID, w)
	if err != nil {
		return "Refresh failed — check the run log in the panel."
	}

	settings, err := l.Repo.GetSettings(ctx)
	if err == nil && !settings.DryRun && settings.TelegramChatID != "" {
		msg := domain.Message{Text: render.CombineDigest([]string{text})}
		if err := l.Bot.Send(ctx, domain.Target{ChatID: settings.TelegramChatID}, msg); err != nil {
			logging.From(rctx).Warn("telegrambot: refresh send failed", "error", err)
		}
	}
	return "Refreshed."
}
