// Package config loads and validates configuration from the environment.
// This is the only package allowed to read os.Getenv directly — everything
// else receives values through Config so the required set stays visible
// in one place instead of scattered across the codebase.
package config

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/sampariat/prices-reminder/internal/logging"
)

type Config struct {
	Env      string // "development" | "production"
	Port     string
	LogLevel slog.Level

	DatabaseURL logging.Secret

	TelegramBotToken   logging.Secret
	TravelpayoutsToken logging.Secret

	GeminiAPIKey logging.Secret // optional — AI features disabled if empty

	AdminPasswordHash string // bcrypt hash, not a raw secret to redact-guard
	SessionSecret     logging.Secret

	AppDomain string

	APIVersionMin int
	APIVersionMax int

	HTTPTimeout time.Duration
}

// Load reads and validates required env vars, failing fast with a clear
// message naming every missing variable at once rather than one at a time.
func Load() (Config, error) {
	var missing []string
	req := func(key string) string {
		v := os.Getenv(key)
		if v == "" {
			missing = append(missing, key)
		}
		return v
	}

	cfg := Config{
		Env:                envOr("APP_ENV", "development"),
		Port:               envOr("PORT", "8080"),
		DatabaseURL:        logging.Secret(req("DATABASE_URL")),
		TelegramBotToken:   logging.Secret(req("TELEGRAM_BOT_TOKEN")),
		TravelpayoutsToken: logging.Secret(req("TRAVELPAYOUTS_TOKEN")),
		GeminiAPIKey:       logging.Secret(os.Getenv("GEMINI_API_KEY")), // optional
		AdminPasswordHash:  req("ADMIN_PASSWORD_HASH"),
		SessionSecret:      logging.Secret(req("SESSION_SECRET")),
		AppDomain:          envOr("APP_DOMAIN", "localhost"),
		APIVersionMin:      1,
		APIVersionMax:      1,
		HTTPTimeout:        15 * time.Second,
	}

	if len(missing) > 0 {
		return Config{}, fmt.Errorf("missing required env vars: %v", missing)
	}

	cfg.LogLevel = parseLevel(envOr("LOG_LEVEL", "info"))
	return cfg, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func parseLevel(s string) slog.Level {
	switch s {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
