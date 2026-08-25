// Package di is the composition root: the only place adapters are chosen
// and wired together, moved out of cmd/server/main.go into its own file
// and given a name. This is the same manual, one-function wiring the app
// has always used — no reflection-based container, no codegen — just
// isolated from server startup so main.go stays about running a server,
// not building one. Nothing below internal/domain's ports should know
// which adapter backs them.
package di

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SamPariat/pricewatch/internal/ai"
	"github.com/SamPariat/pricewatch/internal/ai/gemini"
	"github.com/SamPariat/pricewatch/internal/ai/ollama"
	"github.com/SamPariat/pricewatch/internal/cache"
	"github.com/SamPariat/pricewatch/internal/config"
	"github.com/SamPariat/pricewatch/internal/domain"
	"github.com/SamPariat/pricewatch/internal/httpapi/handlers"
	"github.com/SamPariat/pricewatch/internal/httpapi/middleware"
	"github.com/SamPariat/pricewatch/internal/notify/telegram"
	"github.com/SamPariat/pricewatch/internal/pipeline"
	"github.com/SamPariat/pricewatch/internal/providers"
	"github.com/SamPariat/pricewatch/internal/providers/aviasales"
	"github.com/SamPariat/pricewatch/internal/providers/hotellook"
	"github.com/SamPariat/pricewatch/internal/providers/rental"
	"github.com/SamPariat/pricewatch/internal/scheduler"
	"github.com/SamPariat/pricewatch/internal/service"
	"github.com/SamPariat/pricewatch/internal/store"
	"github.com/SamPariat/pricewatch/internal/telegrambot"
)

const sessionTTL = 7 * 24 * time.Hour

// Container holds every long-lived, fully-wired dependency the server
// needs. main.go builds one via New, hands Container.API to
// httpapi.Router, starts Container.Listener, and calls Close on
// shutdown.
type Container struct {
	Pool      *pgxpool.Pool
	Repo      domain.Repository
	Scheduler *scheduler.Scheduler
	Listener  *telegrambot.Listener
	API       *handlers.API
}

func New(ctx context.Context, cfg config.Config) (*Container, error) {
	if err := store.Migrate(string(cfg.DatabaseURL)); err != nil {
		return nil, fmt.Errorf("di: migrate: %w", err)
	}

	pool, err := pgxpool.New(ctx, string(cfg.DatabaseURL))
	if err != nil {
		return nil, fmt.Errorf("di: create pool: %w", err)
	}

	repo := store.NewPostgres(pool)

	settings, err := repo.GetSettings(ctx)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("di: get settings: %w", err)
	}
	currency := settings.Currency
	if currency == "" {
		currency = "INR"
	}

	ristrettoCache, err := cache.NewRistretto()
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("di: create cache: %w", err)
	}

	// ttlHolder breaks a genuine circular dependency: the provider cache
	// decorator needs a TTL derived from the scheduler (PLAN.md § Caching
	// — "data only changes when the cron fires"), but the scheduler needs
	// the pipeline, which needs the provider registry. ttlHolder.sched is
	// filled in once the scheduler exists, a few lines down; until then
	// it falls back to a flat default.
	ttlHolder := &schedTTL{}

	registry := providers.NewRegistry()
	registry.Register(decorate(aviasales.NewOneWay(string(cfg.TravelpayoutsToken), currency), ristrettoCache, ttlHolder.TTL))
	registry.Register(decorate(aviasales.NewReturn(string(cfg.TravelpayoutsToken), currency), ristrettoCache, ttlHolder.TTL))
	registry.Register(decorate(hotellook.New(string(cfg.TravelpayoutsToken), currency), ristrettoCache, ttlHolder.TTL))
	registry.Register(rental.New()) // no upstream to call yet — see PLAN.md § Known gaps and risks

	pl := &pipeline.Pipeline{
		Registry: registry, Repo: repo, Clock: realClock{},
		Copywriter: buildCopywriter(cfg, ristrettoCache),
	}

	notifier := telegram.New(string(cfg.TelegramBotToken))

	sched := scheduler.New(repo, pl, notifier)
	ttlHolder.sched = sched

	if err := sched.Reload(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("di: scheduler reload: %w", err)
	}

	watches := &service.WatchService{Repo: repo, Sched: sched, Pipeline: pl, Notifier: notifier, Clock: realClock{}}

	// Long-poll listener for inline keyboard button presses (Snooze 7d /
	// Pause / Refresh now — PLAN.md § Telegram).
	listener := &telegrambot.Listener{Bot: notifier, Repo: repo, Sched: sched, Watches: watches}

	// One Session instance shared by AuthService (issues tokens on login)
	// and router.go's RequireAuth middleware (validates them) — both must
	// agree on the same HMAC secret.
	session := middleware.NewSession(string(cfg.SessionSecret))

	api := &handlers.API{
		Watches:   watches,
		Settings:  &service.SettingsService{Repo: repo},
		Runs:      &service.RunsService{Repo: repo},
		Analytics: &service.AnalyticsService{Repo: repo, Sched: sched, Clock: realClock{}},
		Channel:   &service.ChannelService{Notifier: notifier},
		Auth:      &service.AuthService{AdminHash: cfg.AdminPasswordHash, Session: session, SessionTTL: sessionTTL},
		Session:   session,
		Sched:     sched,
	}

	return &Container{Pool: pool, Repo: repo, Scheduler: sched, Listener: listener, API: api}, nil
}

// Close releases everything with a lifetime — called once, on shutdown.
func (c *Container) Close() {
	c.Scheduler.Stop()
	c.Pool.Close()
}

// realClock is the only domain.Clock implementation backed by actual wall
// time — every test uses a fixed fake instead, which is the whole point
// of the Clock port (PLAN.md § Architecture patterns).
type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

// schedTTL exists only to break the registry/scheduler construction cycle
// described where it's created above. TTL falls back to a flat default
// until sched is assigned; every provider fetch after Reload() uses the
// real schedule-derived value.
type schedTTL struct {
	sched *scheduler.Scheduler
}

func (t *schedTTL) TTL(id domain.WatchID) time.Duration {
	if t.sched == nil {
		return 30 * time.Minute
	}
	return t.sched.TTL(id)
}

// buildCopywriter wires the LLM port per PLAN.md § AI features: Gemini
// primary, Ollama fallback, both entirely optional. With neither
// configured this returns a Copywriter around a nil domain.LLM, which
// pipeline.Pipeline.Copywriter.Enhance treats as a no-op — the digest
// always ships via the deterministic template either way.
func buildCopywriter(cfg config.Config, c domain.Cache) *ai.Copywriter {
	var primary, fallback domain.LLM
	if cfg.OllamaURL != "" {
		fallback = ollama.New(cfg.OllamaURL, cfg.OllamaModel)
	}
	if string(cfg.GeminiAPIKey) != "" {
		primary = gemini.New(string(cfg.GeminiAPIKey), cfg.GeminiModel)
	} else {
		primary = fallback
		fallback = nil
	}

	if primary == nil {
		return &ai.Copywriter{}
	}
	return &ai.Copywriter{LLM: ai.WithCache(ai.Chain(primary, fallback), c)}
}

// decorate applies the resilience/caching stack described in PLAN.md
// § Architecture patterns to a provider adapter. Every layer satisfies
// domain.Provider itself, so this composes without any adapter knowing
// about retry, caching, or rate limits.
func decorate(p domain.Provider, c domain.Cache, ttlFor func(domain.WatchID) time.Duration) domain.Provider {
	p = providers.WithSingleflight(p)
	p = providers.WithCache(p, c, ttlFor)
	p = providers.WithRateLimit(p, 250) // Travelpayouts allows 300/min on /v1/prices/calendar; stay under it
	p = providers.WithBreaker(p)
	p = providers.WithRetry(p, 3, 500*time.Millisecond)
	p = providers.WithLogging(p)
	return p
}
