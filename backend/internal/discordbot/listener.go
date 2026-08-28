// Package discordbot turns a pressed button (Snooze 7d / Pause / Refresh
// now) into a domain action. It's a driving adapter — same category as
// internal/httpapi/handlers, just triggered by a Discord Gateway
// interaction event instead of an HTTP request — which is why it depends
// on concrete types (domain.Repository, *scheduler.Scheduler,
// *service.WatchService, *discordgo.Session) directly rather than only
// on ports: only internal/domain itself has to stay pure. See PLAN.md §
// Architecture patterns.
//
// Unlike the Telegram long-poll listener this replaced, there's no
// manual polling loop: discordgo.Session.Open establishes the Gateway
// WebSocket connection and dispatches events (including button-click
// interactions) to registered handlers on its own goroutines. Run's job
// is just opening that connection, registering the handler, and closing
// it when ctx is done.
package discordbot

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/SamPariat/pricewatch/internal/domain"
	"github.com/SamPariat/pricewatch/internal/i18n"
	"github.com/SamPariat/pricewatch/internal/logging"
	"github.com/SamPariat/pricewatch/internal/scheduler"
	"github.com/SamPariat/pricewatch/internal/service"
)

const snoozeDuration = 7 * 24 * time.Hour

type Listener struct {
	Session  *discordgo.Session
	Repo     domain.Repository
	Sched    *scheduler.Scheduler
	Watches  *service.WatchService
	Trips    *service.TripService
	Requests *service.RequestService
	// GuildID is where slash commands register — guild-scoped (not
	// global) so they appear instantly, matching this app's single-server
	// scope.
	GuildID string
}

// Run opens the Gateway connection and blocks until ctx is canceled,
// meant to run as its own goroutine from the composition root — the same
// lifecycle shape the old Telegram long-poll loop had, even though the
// mechanics underneath are entirely different (an event subscription,
// not a poll loop). Once open, discordgo's Session handles reconnection
// on its own for any later network blip — the retry loop here only
// covers the initial Open() failing (bad token, Discord unreachable at
// startup, ...), which would otherwise permanently disable every button
// for the rest of the process's life. Same 5s backoff the old Telegram
// listener used for its own getUpdates failures.
func (l *Listener) Run(ctx context.Context) {
	remove := l.Session.AddHandler(l.handleInteraction)
	defer remove()

	for {
		if ctx.Err() != nil {
			return
		}
		if err := l.Session.Open(); err != nil {
			logging.From(ctx).Warn().Err(err).Msg("discordbot: open gateway connection failed, retrying")
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
			}
			continue
		}
		break
	}
	defer func() {
		if err := l.Session.Close(); err != nil {
			logging.From(ctx).Warn().Err(err).Msg("discordbot: close gateway connection failed")
		}
	}()

	// Registered here, not before Open() — Session.State.User.ID (the
	// application ID ApplicationCommandBulkOverwrite needs) is only
	// populated once the Gateway connection is live. A registration
	// failure is logged, not fatal — buttons still work without it.
	if l.Session.State != nil && l.Session.State.User != nil {
		if err := RegisterCommands(l.Session, l.Session.State.User.ID, l.GuildID); err != nil {
			logging.From(ctx).Warn().Err(err).Msg("discordbot: register slash commands failed")
		}
	}

	<-ctx.Done()
}

func (l *Listener) handleInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	switch i.Type {
	case discordgo.InteractionMessageComponent:
		l.handleComponentInteraction(s, i)
	case discordgo.InteractionApplicationCommand:
		l.handleSlashCommand(s, i)
	}
}

func (l *Listener) handleComponentInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data, ok := i.Data.(discordgo.MessageComponentInteractionData)
	if !ok {
		return
	}
	action, watchID, ok := parseCallback(data.CustomID)
	if !ok {
		return
	}

	// discordgo dispatches interaction events on its own goroutine with
	// no request-scoped context to inherit — same reasoning as the
	// Telegram listener this replaced.
	ctx := context.Background()
	loc := l.locale(ctx)

	var confirmText string
	switch action {
	case "snooze":
		confirmText = l.snooze(ctx, domain.WatchID(watchID), loc)
	case "pause":
		confirmText = l.pause(ctx, domain.WatchID(watchID), loc)
	case "refresh":
		confirmText = l.refresh(ctx, domain.WatchID(watchID), loc)
	default:
		return
	}

	l.respondEphemeral(s, i, confirmText)
}

// locale resolves the locale for a Discord interaction — there's no
// Accept-Language header on one, so Settings.Language is the only
// source, same as the digest itself.
func (l *Listener) locale(ctx context.Context) i18n.Locale {
	loc := i18n.EN
	if settings, err := l.Repo.GetSettings(ctx); err == nil && i18n.Valid(settings.Language) {
		loc = i18n.Locale(settings.Language)
	}
	return loc
}

// respondEphemeral replies to any interaction (button or slash command)
// visible only to whoever triggered it — closest equivalent to
// Telegram's answerCallbackQuery toast, so a confirmation doesn't
// clutter the channel for everyone else.
func (l *Listener) respondEphemeral(s *discordgo.Session, i *discordgo.InteractionCreate, content string) {
	resp := &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: content,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	}
	if err := s.InteractionRespond(i.Interaction, resp); err != nil {
		logging.From(context.Background()).Warn().Err(err).Msg("discordbot: interactionRespond failed")
	}
}

// parseCallback splits "action:watchID" — the format scheduler.go's
// snoozeButtons produces into each button's CustomID. Anything else
// (including a bare "action" with no watch ID) is rejected rather than
// guessed at.
func parseCallback(data string) (action, watchID string, ok bool) {
	before, after, found := strings.Cut(data, ":")
	if !found || after == "" {
		return "", "", false
	}
	return before, after, true
}

func (l *Listener) snooze(ctx context.Context, id domain.WatchID, loc i18n.Locale) string {
	until := time.Now().Add(snoozeDuration)
	if err := l.Repo.SetSnooze(ctx, id, &until); err != nil {
		logging.From(ctx).Error().Str("watch_id", string(id)).Err(err).Msg("discordbot: snooze failed")
		return i18n.T(loc, "discord.snooze_failed")
	}
	return i18n.T(loc, "discord.snoozed")
}

func (l *Listener) pause(ctx context.Context, id domain.WatchID, loc i18n.Locale) string {
	w, err := l.Repo.GetWatch(ctx, id)
	if err != nil {
		return i18n.T(loc, "discord.watch_not_found")
	}
	w.Enabled = false
	if _, err := l.Repo.UpdateWatch(ctx, w); err != nil {
		logging.From(ctx).Error().Str("watch_id", string(id)).Err(err).Msg("discordbot: pause failed")
		return i18n.T(loc, "discord.pause_failed")
	}
	if err := l.Sched.Reload(ctx); err != nil {
		logging.From(ctx).Error().Err(err).Msg("discordbot: reload after pause")
	}
	return i18n.T(loc, "discord.paused")
}

// refresh delegates entirely to WatchService.RunNow — the same method
// the HTTP "Run now" button calls (internal/httpapi/handlers.RunWatchNow)
// — so this button and that one can never drift in behavior.
func (l *Listener) refresh(ctx context.Context, id domain.WatchID, loc i18n.Locale) string {
	if _, err := l.Watches.RunNow(ctx, id, true); err != nil {
		if errors.Is(err, service.ErrNotFound) {
			return i18n.T(loc, "discord.watch_not_found")
		}
		return i18n.T(loc, "discord.refresh_failed")
	}
	return i18n.T(loc, "discord.refreshed")
}
