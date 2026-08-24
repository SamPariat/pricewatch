package domain

import (
	"encoding/json"
	"time"
)

// RunID is a ULID minted once per cron run and threaded through the whole
// pipeline (fetch → normalize → persist → analyze → render → send) via
// context, so every log line and every run_events row for one run shares
// one ID — see internal/logging.
type RunID string

type RunStatus string

const (
	RunPending RunStatus = "pending"
	RunSuccess RunStatus = "success"
	RunFailed  RunStatus = "failed"
)

// DigestRun is one row per cron firing for one watch — success or failure,
// always written, so "why didn't the digest arrive" is answerable from data.
type DigestRun struct {
	RunID       RunID
	WatchID     WatchID
	StartedAt   time.Time
	FinishedAt  *time.Time
	Status      RunStatus
	MessageBody string
	Error       string
}

type LogLevel string

const (
	LevelDebug LogLevel = "debug"
	LevelInfo  LogLevel = "info"
	LevelWarn  LogLevel = "warn"
	LevelError LogLevel = "error"
)

// RunEvent is one step in a run's timeline — what the panel's run-history
// view renders, and what GET /api/runs/:run_id/events returns.
type RunEvent struct {
	RunID  RunID
	At     time.Time
	Stage  string
	Level  LogLevel
	Msg    string
	Fields json.RawMessage
}

// WatchState tracks a watch's health independent of any single run, so the
// UI can tell "silently broken" apart from "price hasn't moved" — see
// PLAN.md § Freshness. If LastSuccessAt is older than 2× the watch's cron
// interval, the staleness badge fires.
type WatchState struct {
	WatchID             WatchID
	LastSuccessAt       *time.Time
	LastAttemptAt       *time.Time
	LastError           string
	ConsecutiveFailures int
}
