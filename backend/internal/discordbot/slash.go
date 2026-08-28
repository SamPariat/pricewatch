package discordbot

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"

	"github.com/SamPariat/pricewatch/internal/domain"
	"github.com/SamPariat/pricewatch/internal/i18n"
	"github.com/SamPariat/pricewatch/internal/logging"
)

func (l *Listener) handleSlashCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	ctx := context.Background()
	loc := l.locale(ctx)
	// Requests submitted from here go through validation inside
	// WatchService/TripService via RequestService.Approve, which reads
	// locale off ctx (i18n.From) — without this, a validation error on
	// approval would always come back in English regardless of the
	// panel's configured language. The existing button-interaction
	// handlers (snooze/pause/refresh) don't do this — a known latent
	// gap there, left alone; new code should get it right.
	ctx = i18n.With(ctx, loc)

	data := i.ApplicationCommandData()
	if len(data.Options) == 0 {
		return
	}
	sub := data.Options[0]

	var reply string
	switch data.Name {
	case "trip":
		reply = l.handleTripCommand(ctx, sub, loc)
	case "track":
		reply = l.handleTrackCommand(ctx, sub, loc)
	default:
		return
	}
	l.respondEphemeral(s, i, reply)
}

func (l *Listener) handleTripCommand(ctx context.Context, sub *discordgo.ApplicationCommandInteractionDataOption, loc i18n.Locale) string {
	switch sub.Name {
	case "create":
		opts := optionMap(sub.Options)
		payload := domain.CreateTripPayload{
			Name:     stringOpt(opts, "name"),
			CronExpr: stringOpt(opts, "cron"),
			Timezone: stringOptOr(opts, "timezone", "UTC"),
		}
		title := payload.Name
		note := fmt.Sprintf("New trip · daily at %s", payload.CronExpr)
		if _, err := l.Requests.Submit(ctx, domain.RequestCreateTrip, payload, title, note); err != nil {
			logging.From(ctx).Error().Err(err).Msg("discordbot: submit create_trip request failed")
			return i18n.T(loc, "discord.request_failed")
		}
		return i18n.T(loc, "discord.trip_submitted")
	case "list":
		trips, err := l.Trips.List(ctx)
		if err != nil || len(trips) == 0 {
			return i18n.T(loc, "discord.trip_list_empty")
		}
		var b strings.Builder
		for _, t := range trips {
			fmt.Fprintf(&b, "**%s** — `%s` (%s)\n", t.Name, t.CronExpr, t.ID)
		}
		return b.String()
	default:
		return i18n.T(loc, "discord.request_failed")
	}
}

func (l *Listener) handleTrackCommand(ctx context.Context, sub *discordgo.ApplicationCommandInteractionDataOption, loc i18n.Locale) string {
	switch sub.Name {
	case "add":
		opts := optionMap(sub.Options)
		tripName := stringOpt(opts, "trip")
		trip, ok := l.findTripByName(ctx, tripName)
		if !ok {
			return i18n.T(loc, "discord.trip_not_found")
		}

		params := domain.LodgingParams{
			URL:      stringOpt(opts, "url"),
			CheckIn:  stringOpt(opts, "checkin"),
			CheckOut: stringOpt(opts, "checkout"),
			Guests:   intOpt(opts, "guests"),
		}
		raw, err := json.Marshal(params)
		if err != nil {
			return i18n.T(loc, "discord.request_failed")
		}
		payload := domain.AddLegPayload{TripID: trip.ID, Kind: domain.AssetLodgingAirbnb, Params: raw}
		title := fmt.Sprintf("%s, %s → %s", trip.Name, params.CheckIn, params.CheckOut)
		if _, err := l.Requests.Submit(ctx, domain.RequestAddLeg, payload, title, params.URL); err != nil {
			logging.From(ctx).Error().Err(err).Msg("discordbot: submit add_leg request failed")
			return i18n.T(loc, "discord.request_failed")
		}
		return i18n.T(loc, "discord.track_submitted")
	case "remove":
		opts := optionMap(sub.Options)
		id := stringOpt(opts, "id")
		title := id
		note := ""
		if detail, err := l.Watches.Get(ctx, domain.WatchID(id)); err == nil {
			title = detail.Watch.Name
			if title == "" {
				title = string(detail.Watch.Kind)
			}
			if params, err := detail.Watch.DecodeLodgingParams(); err == nil {
				note = params.URL
			}
		}
		if _, err := l.Requests.Submit(ctx, domain.RequestRemoveLeg, id, "Remove: "+title, note); err != nil {
			logging.From(ctx).Error().Err(err).Msg("discordbot: submit remove_leg request failed")
			return i18n.T(loc, "discord.request_failed")
		}
		return i18n.T(loc, "discord.track_remove_submitted")
	case "list":
		details, err := l.Watches.List(ctx)
		if err != nil {
			return i18n.T(loc, "discord.request_failed")
		}
		var b strings.Builder
		for _, d := range details {
			if !d.Watch.Kind.IsLodging() {
				continue
			}
			name := d.Watch.Name
			if name == "" {
				name = string(d.Watch.Kind)
			}
			fmt.Fprintf(&b, "**%s** — `%s`\n", name, d.Watch.ID)
		}
		if b.Len() == 0 {
			return i18n.T(loc, "discord.track_list_empty")
		}
		return b.String()
	default:
		return i18n.T(loc, "discord.request_failed")
	}
}

func (l *Listener) findTripByName(ctx context.Context, name string) (domain.Trip, bool) {
	trips, err := l.Trips.List(ctx)
	if err != nil {
		return domain.Trip{}, false
	}
	for _, t := range trips {
		if strings.EqualFold(t.Name, name) {
			return t, true
		}
	}
	return domain.Trip{}, false
}

func optionMap(opts []*discordgo.ApplicationCommandInteractionDataOption) map[string]*discordgo.ApplicationCommandInteractionDataOption {
	m := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(opts))
	for _, o := range opts {
		m[o.Name] = o
	}
	return m
}

func stringOpt(opts map[string]*discordgo.ApplicationCommandInteractionDataOption, name string) string {
	if o, ok := opts[name]; ok {
		return o.StringValue()
	}
	return ""
}

func stringOptOr(opts map[string]*discordgo.ApplicationCommandInteractionDataOption, name, fallback string) string {
	if v := stringOpt(opts, name); v != "" {
		return v
	}
	return fallback
}

func intOpt(opts map[string]*discordgo.ApplicationCommandInteractionDataOption, name string) int {
	if o, ok := opts[name]; ok {
		return int(o.IntValue())
	}
	return 0
}
