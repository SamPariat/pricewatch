// Package config loads and validates configuration from the environment.
// This is the only package allowed to read os.Getenv directly — everything
// else receives values through Config so the required set stays visible
// in one place instead of scattered across the codebase.
package config

import (
	"fmt"
	"os"
	"time"

	"github.com/rs/zerolog"

	"github.com/SamPariat/pricewatch/internal/logging"
)

type Config struct {
	Env      string // "development" | "production"
	Port     string
	LogLevel zerolog.Level

	DatabaseURL logging.Secret

	DiscordBotToken    logging.Secret
	DiscordGuildID     string // required — slash commands register guild-scoped, not global
	TravelpayoutsToken logging.Secret

	GeminiAPIKey logging.Secret // optional — AI features disabled if empty
	GeminiModel  string         // optional override, defaults inside the gemini adapter
	OllamaURL    string         // optional fallback LLM — empty disables it
	OllamaModel  string         // optional override, defaults inside the ollama adapter

	AdminPasswordHash string // bcrypt hash, not a raw secret to redact-guard
	SessionSecret     logging.Secret

	AppDomain string

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
		DiscordBotToken:    logging.Secret(req("DISCORD_BOT_TOKEN")),
		DiscordGuildID:     req("DISCORD_GUILD_ID"),
		TravelpayoutsToken: logging.Secret(req("TRAVELPAYOUTS_TOKEN")),
		GeminiAPIKey:       logging.Secret(os.Getenv("GEMINI_API_KEY")), // optional
		GeminiModel:        os.Getenv("GEMINI_MODEL"),                   // optional
		OllamaURL:          os.Getenv("OLLAMA_URL"),                     // optional
		OllamaModel:        os.Getenv("OLLAMA_MODEL"),                   // optional
		AdminPasswordHash:  req("ADMIN_PASSWORD_HASH"),
		SessionSecret:      logging.Secret(req("SESSION_SECRET")),
		AppDomain:          envOr("APP_DOMAIN", "localhost"),
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

func parseLevel(s string) zerolog.Level {
	switch s {
	case "debug":
		return zerolog.DebugLevel
	case "warn":
		return zerolog.WarnLevel
	case "error":
		return zerolog.ErrorLevel
	default:
		return zerolog.InfoLevel
	}
}
