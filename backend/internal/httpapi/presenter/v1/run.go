package v1

import (
	"encoding/json"
	"time"

	"github.com/sampariat/prices-reminder/internal/domain"
)

type DigestRun struct {
	RunID       string     `json:"run_id"`
	WatchID     string     `json:"watch_id"`
	StartedAt   time.Time  `json:"started_at"`
	FinishedAt  *time.Time `json:"finished_at,omitempty"`
	Status      string     `json:"status"`
	MessageBody string     `json:"message_body,omitempty"`
	Error       string     `json:"error,omitempty"`
}

func DigestRunOf(r domain.DigestRun) DigestRun {
	return DigestRun{
		RunID: string(r.RunID), WatchID: string(r.WatchID), StartedAt: r.StartedAt,
		FinishedAt: r.FinishedAt, Status: string(r.Status), MessageBody: r.MessageBody, Error: r.Error,
	}
}

func DigestRunsOf(runs []domain.DigestRun) []DigestRun {
	out := make([]DigestRun, len(runs))
	for i, r := range runs {
		out[i] = DigestRunOf(r)
	}
	return out
}

type RunEvent struct {
	At     time.Time       `json:"at"`
	Stage  string          `json:"stage"`
	Level  string          `json:"level"`
	Msg    string          `json:"msg"`
	Fields json.RawMessage `json:"fields,omitempty"`
}

func RunEventOf(e domain.RunEvent) RunEvent {
	return RunEvent{At: e.At, Stage: e.Stage, Level: string(e.Level), Msg: e.Msg, Fields: e.Fields}
}

func RunEventsOf(events []domain.RunEvent) []RunEvent {
	out := make([]RunEvent, len(events))
	for i, e := range events {
		out[i] = RunEventOf(e)
	}
	return out
}
