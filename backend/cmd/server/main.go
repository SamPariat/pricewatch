// Command server starts the HTTP server. Adapter wiring itself lives in
// internal/di — this file is about running a server, not building one.
package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/gofiber/fiber/v3/middleware/requestid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
	zlog "github.com/rs/zerolog/log"

	"github.com/SamPariat/pricewatch/internal/config"
	"github.com/SamPariat/pricewatch/internal/di"
	"github.com/SamPariat/pricewatch/internal/httpapi"
	"github.com/SamPariat/pricewatch/internal/httpapi/envelope"
	"github.com/SamPariat/pricewatch/internal/logging"
)

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
	zlog.Logger = logger

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	c, err := di.New(ctx, cfg)
	if err != nil {
		logger.Error().Err(err).Msg("startup: build container")
		os.Exit(1)
	}
	defer c.Close()

	go c.Listener.Run(ctx)

	app := fiber.New(fiber.Config{
		AppName:      "prices-reminder",
		ReadTimeout:  cfg.HTTPTimeout,
		WriteTimeout: cfg.HTTPTimeout,
		ErrorHandler: envelope.ErrorHandler,
	})

	app.Use(requestid.New())
	app.Use(recover.New())
	app.Use(requestLogger(logger))

	ready := &readiness{pool: c.Pool, startedAt: time.Now()}

	app.Get("/healthz", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})
	app.Get("/readyz", ready.handler)

	httpapi.Router(app, c.API)

	errCh := make(chan error, 1)
	go func() {
		errCh <- app.Listen(":" + cfg.Port)
	}()

	select {
	case err := <-errCh:
		if err != nil {
			logger.Error().Err(err).Msg("server: listen failed")
			os.Exit(1)
		}
	case <-ctx.Done():
		logger.Info().Msg("server: shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := app.ShutdownWithContext(shutdownCtx); err != nil {
			logger.Error().Err(err).Msg("server: shutdown error")
		}
	}
}

// requestLogger logs each request with the request ID Fiber v3's requestid
// middleware already attached, so a client-visible X-Request-Id maps
// directly to a server log line.
func requestLogger(base zerolog.Logger) fiber.Handler {
	return func(c fiber.Ctx) error {
		start := time.Now()
		err := c.Next()

		// Fiber's ErrorHandler runs OUTSIDE the whole middleware chain —
		// router.go calls it only after app.next(ctx), which is what this
		// middleware's own c.Next() call is one link inside of — so at
		// this point, for any request that failed via a returned error,
		// c.Response().StatusCode() is still whatever it was before the
		// error (default 200), not the real status the client will
		// receive. Read the status directly off the error when there is
		// one, mirroring exactly what envelope.ErrorHandler does, so this
		// log line never reports 200 for a request that actually 404'd.
		status := c.Response().StatusCode()
		if err != nil {
			status = fiber.StatusInternalServerError
			if fe, ok := err.(*fiber.Error); ok {
				status = fe.Code
			}
		}
		base.Info().
			Str("method", c.Method()).
			Str("path", c.Path()).
			Int("status", status).
			Int64("duration_ms", time.Since(start).Milliseconds()).
			Str("request_id", requestid.FromContext(c)).
			Msg("request")
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
