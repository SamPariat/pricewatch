// Package httpapi wires the Fiber v3 route tree. Route registration is
// the only thing this file does — auth and the actual handlers each live
// in their own package so this stays a table of what maps to what.
package httpapi

import (
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cache"
	"github.com/gofiber/fiber/v3/middleware/etag"
	"github.com/gofiber/fiber/v3/middleware/limiter"

	"github.com/SamPariat/pricewatch/internal/domain"
	"github.com/SamPariat/pricewatch/internal/httpapi/handlers"
	apimw "github.com/SamPariat/pricewatch/internal/httpapi/middleware"
)

// Router registers every /api/{handlers.APIVersion} route on app. Called
// once from main.go (or internal/di) after api's dependencies are all
// constructed.
func Router(app *fiber.App, api *handlers.API) {
	group := app.Group("/api/" + handlers.APIVersion)
	group.Use(apimw.Locale())
	// Covers /auth/login too — rate-limiting login attempts is a feature,
	// not an oversight.
	group.Use(limiter.New(limiter.Config{Max: 120, Expiration: time.Minute}))

	group.Post("/auth/login", api.Login)
	group.Post("/auth/logout", api.Logout)
	group.Get("/meta", api.GetMeta)

	auth := group.Group("", apimw.RequireAuth(api.Session))

	// PLAN.md § Caching layer 2: browser validation via ETag, on every
	// authenticated GET. Layer 4 (server-side response caching) is
	// deliberately NOT applied here — only to /watches/:id/history below.
	// Watch config (list/get) must reflect create/update/delete
	// immediately; only the price *history* is legitimately schedule-
	// bound, which is what the TTL is derived from in the first place.
	// Caching the list endpoint on that same TTL would mean a freshly
	// created watch could stay invisible for up to 6 hours.
	auth.Use(etag.New())

	auth.Get("/watches", api.ListWatches)
	auth.Post("/watches", api.CreateWatch)
	auth.Get("/watches/:id", api.GetWatch)
	auth.Patch("/watches/:id", api.UpdateWatch) // full-replace semantics, see UpdateWatch
	auth.Delete("/watches/:id", api.DeleteWatch)
	auth.Post("/watches/:id/run", api.RunWatchNow)
	auth.Get("/watches/:id/history", historyCache(api), api.GetHistory)

	auth.Get("/analytics/summary", api.AnalyticsSummary)

	auth.Get("/runs", api.ListRuns)
	auth.Get("/runs/:run_id/events", api.GetRunEvents)

	auth.Get("/settings", api.GetSettings)
	auth.Patch("/settings", api.UpdateSettings) // full-replace semantics, see UpdateSettings

	auth.Get("/channel/status", api.GetChannelStatus)

	// API docs — same auth as everything else here; this is an ops page,
	// not the product surface, so it doesn't need its own access model.
	auth.Get("/docs", api.SwaggerUI)
	auth.Get("/docs/openapi.json", api.OpenAPISpec)
}

// historyCache serves cached watch-history responses for exactly as long
// as PLAN.md § Caching prescribes: "data only changes when the cron
// fires." Scheduler.TTL is the single source of truth for that duration —
// never a guessed constant.
func historyCache(api *handlers.API) fiber.Handler {
	return cache.New(cache.Config{
		ExpirationGenerator: func(c fiber.Ctx, _ *cache.Config) time.Duration {
			return api.Sched.TTL(domain.WatchID(c.Params("id")))
		},
	})
}
