package domain

// Settings is the single-row config table — there is one admin and one
// Telegram destination, so this is a singleton, not a table of rows.
type Settings struct {
	TelegramChatID  string
	QuietHoursStart string // "22:00", 24h local time
	QuietHoursEnd   string // "06:30"
	Currency        string
	DryRun          bool
}
