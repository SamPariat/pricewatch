// Command server is the composition root: the only place adapters are chosen
// and wired together. Nothing below internal/domain's ports should know
// which adapter backs them.
package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/gofiber/fiber/v3/middleware/requestid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/sampariat/prices-reminder/internal/ai"
	"github.com/sampariat/prices-reminder/internal/ai/gemini"
	"github.com/sampariat/prices-reminder/internal/ai/ollama"
	"github.com/sampariat/prices-reminder/internal/cache"
	"github.com/sampariat/prices-reminder/internal/config"
	"github.com/sampariat/prices-reminder/internal/domain"
	"github.com/sampariat/prices-reminder/internal/httpapi"
	"github.com/sampariat/prices-reminder/internal/httpapi/handlers"
	apimw "github.com/sampariat/prices-reminder/internal/httpapi/middleware"
	"github.com/sampariat/prices-reminder/internal/logging"
	"github.com/sampariat/prices-reminder/internal/notify/telegram"
	"github.com/sampariat/prices-reminder/internal/pipeline"
	"github.com/sampariat/prices-reminder/internal/providers"
	"github.com/sampariat/prices-reminder/internal/providers/aviasales"
	"github.com/sampariat/prices-reminder/internal/providers/hotellook"
	"github.com/sampariat/prices-reminder/internal/providers/rental"
	"github.com/sampariat/prices-reminder/internal/scheduler"
	"github.com/sampariat/prices-reminder/internal/store"
	"github.com/sampariat/prices-reminder/internal/telegrambot"
)

const sessionTTL = 7 * 24 * time.Hour

func main() {
	healthcheck := flag.Bool("healthcheck", false, "probe the local /healthz endpoint and exit 0/1 (used as the Docker HEALTHCHECK for this distroless image, which has no shell or curl)")
	flag.Parse()
	if *healthcheck {
		os.Exit(runHealthcheckProbe())
	}

	cfg, err := config.Load()
	if err != nil {
		// Logger isn't built yet — this is the one place a bare
		// os.Stderr write is correct instead of slog.
		os.Stderr.WriteString("config: " + err.Error() + "\n")
		os.Exit(1)
	}

	logger := logging.New(cfg.Env, cfg.LogLevel)
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := store.Migrate(string(cfg.DatabaseURL)); err != nil {
		logger.Error("database: migration failed", "error", err)
		os.Exit(1)
	}

	pool, err := pgxpool.New(ctx, string(cfg.DatabaseURL))
	if err != nil {
		logger.Error("database: failed to create pool", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	repo := store.NewPostgres(pool)

	settings, err := repo.GetSettings(ctx)
	if err != nil {
		logger.Error("startup: get settings", "error", err)
		os.Exit(1)
	}
	currency := settings.Currency
	if currency == "" {
		currency = "INR"
	}

	ristrettoCache, err := cache.NewRistretto()
	if err != nil {
		logger.Error("startup: create cache", "error", err)
		os.Exit(1)
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
		logger.Error("startup: scheduler reload", "error", err)
		os.Exit(1)
	}
	defer sched.Stop()

	// Long-poll listener for inline keyboard button presses (Snooze 7d /
	// Pause / Refresh now — PLAN.md § Telegram). Runs for the process's
	// lifetime; ctx cancellation on shutdown stops it cleanly.
	listener := &telegrambot.Listener{Bot: notifier, Repo: repo, Sched: sched, Pipeline: pl}
	go listener.Run(ctx)

	app := fiber.New(fiber.Config{
		AppName:      "prices-reminder",
		ReadTimeout:  cfg.HTTPTimeout,
		WriteTimeout: cfg.HTTPTimeout,
	})

	app.Use(requestid.New())
	app.Use(recover.New())
	app.Use(requestLogger(logger))

	ready := &readiness{pool: pool, startedAt: time.Now()}

	app.Get("/healthz", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})
	app.Get("/readyz", ready.handler)

	api := &handlers.API{
		Repo: repo, Sched: sched, Pipeline: pl, Notifier: notifier, Clock: realClock{},
		MinVersion: cfg.APIVersionMin, MaxVersion: cfg.APIVersionMax,
		AdminHash: cfg.AdminPasswordHash,
		Session:   apimw.NewSession(string(cfg.SessionSecret)), SessionTTL: sessionTTL,
	}
	httpapi.Router(app, api)

	errCh := make(chan error, 1)
	go func() {
		errCh <- app.Listen(":" + cfg.Port)
	}()

	select {
	case err := <-errCh:
		if err != nil {
			logger.Error("server: listen failed", "error", err)
			os.Exit(1)
		}
	case <-ctx.Done():
		logger.Info("server: shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := app.ShutdownWithContext(shutdownCtx); err != nil {
			logger.Error("server: shutdown error", "error", err)
		}
	}
}

// requestLogger logs each request with the request ID Fiber v3's requestid
// middleware already attached, so a client-visible X-Request-Id maps
// directly to a server log line.
func requestLogger(base *slog.Logger) fiber.Handler {
	return func(c fiber.Ctx) error {
		start := time.Now()
		err := c.Next()

		status := c.Response().StatusCode()
		base.Info("request",
			"method", c.Method(),
			"path", c.Path(),
			"status", status,
			"duration_ms", time.Since(start).Milliseconds(),
			"request_id", requestid.FromContext(c),
		)
		return err
	}
}

// runHealthcheckProbe is invoked as `/server -healthcheck` by Docker's
// HEALTHCHECK, which runs it as a fresh process each interval — it cannot
// signal the already-running server, so it independently hits /healthz
// over loopback and reports success via exit code.
func runHealthcheckProbe() int {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	client := http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://localhost:" + port + "/healthz")
	if err != nil {
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 1
	}
	return 0
}

// readiness reports whether the process can actually serve traffic —
// distinct from /healthz, which only proves the process is running.
type readiness struct {
	pool      *pgxpool.Pool
	startedAt time.Time
}

func (r *readiness) handler(c fiber.Ctx) error {
	pingCtx, cancel := context.WithTimeout(c, 2*time.Second)
	defer cancel()

	if err := r.pool.Ping(pingCtx); err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"status": "unavailable",
			"error":  "database unreachable",
		})
	}

	return c.JSON(fiber.Map{
		"status": "ready",
		"uptime": time.Since(r.startedAt).String(),
	})
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
