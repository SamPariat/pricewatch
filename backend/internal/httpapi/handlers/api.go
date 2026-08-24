// Package handlers holds the Fiber v3 route handlers. Each depends on
// domain ports (Repository, Notifier) plus the concrete *scheduler.
// Scheduler and *pipeline.Pipeline — this is the driving side of hexagonal
// architecture (an HTTP controller orchestrating use cases), not the
// domain core, so depending on concrete types here is normal; only
// internal/domain itself must stay pure. See PLAN.md § Architecture
// patterns.
package handlers

import (
	"time"

	"github.com/sampariat/prices-reminder/internal/domain"
	"github.com/sampariat/prices-reminder/internal/httpapi/middleware"
	"github.com/sampariat/prices-reminder/internal/pipeline"
	"github.com/sampariat/prices-reminder/internal/scheduler"
)

type API struct {
	Repo       domain.Repository
	Sched      *scheduler.Scheduler
	Pipeline   *pipeline.Pipeline
	Notifier   domain.Notifier
	Clock      domain.Clock
	MinVersion int
	MaxVersion int
	AdminHash  string
	Session    *middleware.Session
	SessionTTL time.Duration
}
