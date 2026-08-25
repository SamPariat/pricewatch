// Package handlers holds the Fiber v3 route handlers — the controller
// layer. Each handler binds the HTTP request, calls exactly one method
// on a service (internal/service), and renders the result via
// internal/httpapi/envelope. Business logic (validation, orchestration,
// aggregating multiple ports into one view) lives in the service layer,
// not here — see PLAN.md § Architecture patterns for why this split
// exists: an HTTP controller orchestrating use cases is the driving side
// of hexagonal architecture, so depending on concrete service types here
// is normal; only internal/domain itself must stay pure.
package handlers

import (
	"github.com/SamPariat/pricewatch/internal/httpapi/middleware"
	"github.com/SamPariat/pricewatch/internal/scheduler"
	"github.com/SamPariat/pricewatch/internal/service"
)

// APIVersion is the single source of truth for the URL-prefix version
// this server speaks — router.go reads it to build the /api/{APIVersion}
// group, and GetMeta reports it. Defined here rather than in package
// httpapi itself so both packages can read it without an import cycle
// (router.go already imports handlers).
const APIVersion = "v1"

type API struct {
	Watches   *service.WatchService
	Settings  *service.SettingsService
	Runs      *service.RunsService
	Analytics *service.AnalyticsService
	Channel   *service.ChannelService
	Auth      *service.AuthService
	Session   *middleware.Session
	// Sched is used directly only by router.go's historyCache middleware
	// (a pure HTTP-caching concern — deriving a Cache-Control TTL, not a
	// business operation), never by a handler method. Everything a
	// handler needs from the scheduler goes through WatchService instead.
	Sched *scheduler.Scheduler
}
