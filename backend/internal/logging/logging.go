// Package logging wraps slog with context-carried loggers and a redaction type for secrets.
package logging

import (
	"context"
	"log/slog"
	"os"
)

// Secret renders as [REDACTED] in any slog output, so a token declared with
// this type can never end up in a log line by accident.
type Secret string

func (s Secret) LogValue() slog.Value { return slog.StringValue("[REDACTED]") }
func (s Secret) String() string       { return "[REDACTED]" }

type ctxKey struct{}

// New builds the root logger. JSON in production, text in development.
func New(env string, level slog.Level) *slog.Logger {
	opts := &slog.HandlerOptions{Level: level}
	var handler slog.Handler
	if env == "production" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}
	return slog.New(handler)
}

// With attaches a logger carrying the given attributes to the context.
func With(ctx context.Context, args ...any) context.Context {
	l := From(ctx).With(args...)
	return context.WithValue(ctx, ctxKey{}, l)
}

// From returns the logger carried on the context, or the default logger if none.
func From(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(ctxKey{}).(*slog.Logger); ok {
		return l
	}
	return slog.Default()
}
