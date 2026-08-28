package domain

// Settings is the single-row config table — there is one admin and one
// Discord destination, so this is a singleton, not a table of rows.
type Settings struct {
	DiscordChannelID string
	QuietHoursStart  string // "22:00", 24h local time
	QuietHoursEnd    string // "06:30"
	Currency         string
	DryRun           bool
	// Language drives the Discord digest/alert text (internal/render,
	// internal/discordbot) — those are cron- and interaction-triggered,
	// not HTTP requests, so there's no Accept-Language header to read;
	// this persisted preference is their only source of locale. See
	// internal/i18n.Valid for what "hi"/"en" mean.
	Language string
}
