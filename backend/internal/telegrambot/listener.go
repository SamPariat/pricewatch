// Package telegrambot is the long-poll listener that turns a pressed
// inline keyboard button (Snooze 7d / Pause / Refresh now) into a domain
// action. It's a driving adapter — same category as internal/httpapi/
// handlers, just triggered by Telegram's getUpdates instead of an HTTP
// request — which is why it depends on concrete types (Repository,
// *scheduler.Scheduler, *service.WatchService, *telegram.Notifier)
// directly rather than only on ports: only internal/domain itself has to
// stay pure. See PLAN.md § Architecture patterns.
package telegrambot

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/SamPariat/pricewatch/internal/domain"
	"github.com/SamPariat/pricewatch/internal/logging"
	"github.com/SamPariat/pricewatch/internal/notify/telegram"
	"github.com/SamPariat/pricewatch/internal/scheduler"
	"github.com/SamPariat/pricewatch/internal/service"
)

const snoozeDuration = 7 * 24 * time.Hour

type Listener struct {
	Bot     *telegram.Notifier
	Repo    domain.Repository
	Sched   *scheduler.Scheduler
	Watches *service.WatchService
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
			logging.From(ctx).Warn().Err(err).Msg("telegrambot: getUpdates failed")
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
		logging.From(ctx).Warn().Err(err).Msg("telegrambot: answerCallbackQuery failed")
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
		logging.From(ctx).Error().Str("watch_id", string(id)).Err(err).Msg("telegrambot: snooze failed")
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
		logging.From(ctx).Error().Str("watch_id", string(id)).Err(err).Msg("telegrambot: pause failed")
		return "Failed to pause — try again from the panel."
	}
	if err := l.Sched.Reload(ctx); err != nil {
		logging.From(ctx).Error().Err(err).Msg("telegrambot: reload after pause")
	}
	return "Paused. Re-enable it from the panel when you're ready."
}

// refresh delegates entirely to WatchService.RunNow — the same method
// the HTTP "Run now" button calls (internal/httpapi/handlers.RunWatchNow)
// — so this button and that one can never drift in behavior. Before the
// service layer existed, this method duplicated RunNow's mint-runID /
// SkipCache / RunWatch / Send sequence by hand.
func (l *Listener) refresh(ctx context.Context, id domain.WatchID) string {
	if _, err := l.Watches.RunNow(ctx, id, true); err != nil {
		if errors.Is(err, service.ErrNotFound) {
			return "Watch not found."
		}
		return "Refresh failed — check the run log in the panel."
	}
	return "Refreshed."
}
