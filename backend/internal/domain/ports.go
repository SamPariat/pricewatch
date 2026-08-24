package domain

import (
	"context"
	"time"
)

// Provider fetches Quotes for one AssetKind from one upstream source
// (Aviasales, Hotellook, ...). Adapters live in internal/providers;
// resilience (retry, breaker, cache, rate limit, singleflight) is layered
// on as decorators that also satisfy this interface — see PLAN.md
// § Architecture patterns.
type Provider interface {
	Kind() AssetKind
	Fetch(ctx context.Context, w Watch) ([]Quote, error)
}

type NotifierStatus string

const (
	NotifierLinked       NotifierStatus = "linked"
	NotifierDisconnected NotifierStatus = "disconnected"
)

// Notifier delivers a rendered digest. The telegram adapter is the only
// real implementation; noop backs dry-run mode as a Null Object rather
// than an if-branch scattered through the pipeline.
type Notifier interface {
	Send(ctx context.Context, t Target, m Message) error
	Status(ctx context.Context) (NotifierStatus, error)
}

// Repository is the single persistence port — one interface across every
// table, not one per table, so the domain depends on "how we persist a
// watch's lifecycle" rather than on individual SQL statements.
type Repository interface {
	CreateWatch(ctx context.Context, w Watch) (Watch, error)
	GetWatch(ctx context.Context, id WatchID) (Watch, error)
	ListWatches(ctx context.Context) ([]Watch, error)
	UpdateWatch(ctx context.Context, w Watch) (Watch, error)
	DeleteWatch(ctx context.Context, id WatchID) error

	InsertQuotes(ctx context.Context, qs []Quote) error
	UpsertPriceSample(ctx context.Context, s PriceSample) error
	ListPriceSamples(ctx context.Context, watchID WatchID, since time.Time) ([]PriceSample, error)

	CreateDigestRun(ctx context.Context, r DigestRun) error
	FinishDigestRun(ctx context.Context, r DigestRun) error
	ListRecentDigestRuns(ctx context.Context, limit int) ([]DigestRun, error)
	InsertRunEvent(ctx context.Context, e RunEvent) error
	ListRunEvents(ctx context.Context, runID RunID) ([]RunEvent, error)

	GetWatchState(ctx context.Context, watchID WatchID) (WatchState, error)
	// UpsertWatchState writes run-health fields only (LastSuccessAt,
	// LastAttemptAt, LastError, ConsecutiveFailures) — it never touches
	// SnoozedUntil, so a normal scheduled run can't silently clear an
	// active snooze. Use SetSnooze for that.
	UpsertWatchState(ctx context.Context, s WatchState) error
	SetSnooze(ctx context.Context, watchID WatchID, until *time.Time) error

	GetSettings(ctx context.Context) (Settings, error)
	UpdateSettings(ctx context.Context, s Settings) error
}

// Loader produces the bytes to cache on a miss — typically a Provider.Fetch
// result already marshaled to JSON.
type Loader func() ([]byte, error)

// Cache wraps GetOrLoad rather than separate Get/Set so callers can't
// forget the load step and callers never race on a cold key —
// implementations are expected to use singleflight internally.
type Cache interface {
	GetOrLoad(ctx context.Context, key string, ttl time.Duration, load Loader) ([]byte, error)
}

// Prompt is provider-agnostic input to an LLM adapter (gemini, ollama).
type Prompt struct {
	System string
	User   string
}

// LLM is optional end to end — every caller must have a deterministic
// fallback for when this is nil or Complete errors. See PLAN.md § AI
// features guardrails: an LLM must never be the source of a number.
type LLM interface {
	Complete(ctx context.Context, p Prompt) (string, error)
}

// Clock exists so analytics and scheduling logic can be tested against a
// fixed instant instead of real wall-clock time.
type Clock interface {
	Now() time.Time
}
