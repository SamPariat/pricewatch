package v1

import "github.com/SamPariat/pricewatch/internal/domain"

type Settings struct {
	TelegramChatID  string `json:"telegram_chat_id"`
	QuietHoursStart string `json:"quiet_hours_start"`
	QuietHoursEnd   string `json:"quiet_hours_end"`
	Currency        string `json:"currency"`
	DryRun          bool   `json:"dry_run"`
}

func SettingsOf(s domain.Settings) Settings {
	return Settings{
		TelegramChatID: s.TelegramChatID, QuietHoursStart: s.QuietHoursStart,
		QuietHoursEnd: s.QuietHoursEnd, Currency: s.Currency, DryRun: s.DryRun,
	}
}

func (s Settings) ToDomain() domain.Settings {
	return domain.Settings{
		TelegramChatID: s.TelegramChatID, QuietHoursStart: s.QuietHoursStart,
		QuietHoursEnd: s.QuietHoursEnd, Currency: s.Currency, DryRun: s.DryRun,
	}
}

// Meta reports which API version this server currently speaks, at the
// URL prefix (/api/v1/...) rather than the header-negotiated scheme this
// replaced — see PLAN.md § API versioning and internal/httpapi/router.go.
type Meta struct {
	CurrentVersion    string   `json:"current_version"`
	SupportedVersions []string `json:"supported_versions"`
}

// AnalyticsSummary backs GET /api/analytics/summary — the dashboard KPI
// row. Scoped to what's actually computable today: no "alerts sent"
// figure, since threshold alerts (PLAN.md § Further suggestions) aren't
// built yet, and a summary must not report a number for a feature that
// doesn't exist.
type AnalyticsSummary struct {
	TotalWatches   int      `json:"total_watches"`
	EnabledWatches int      `json:"enabled_watches"`
	StaleWatches   int      `json:"stale_watches"`
	AvgDelta7dPct  *float64 `json:"avg_delta_7d_pct,omitempty"`
}
