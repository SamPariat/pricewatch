// Package logging wraps zerolog with context-carried loggers and a
// redaction type for secrets.
package logging

import (
	"context"
	"encoding/json"
	"os"
	"time"

	"github.com/rs/zerolog"
	zlog "github.com/rs/zerolog/log"
)

// Secret renders as [REDACTED] however it ends up in a log line — both
// as a plain fmt.Stringer (%v/%s) and via JSON marshaling (zerolog's
// Interface()/Any() methods JSON-encode arbitrary values internally), so
// a token declared with this type can't end up in a log line by
// accident regardless of which path logs it.
type Secret string

func (s Secret) String() string               { return "[REDACTED]" }
func (s Secret) MarshalJSON() ([]byte, error) { return json.Marshal("[REDACTED]") }

type ctxKey struct{}

// New builds the root logger. JSON in production, a readable console
// writer in development.
func New(env string, level zerolog.Level) zerolog.Logger {
	if env == "production" {
		return zerolog.New(os.Stdout).Level(level).With().Timestamp().Logger()
	}
	console := zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}
	return zerolog.New(console).Level(level).With().Timestamp().Logger()
}

// With attaches a logger carrying the given key/value pairs to the
// context — kept call-compatible with the pre-zerolog variadic
// convention (e.g. logging.With(ctx, "run_id", id, "watch_id", w.ID)),
// since this is a low-frequency, context-tagging call, not the
// per-message hot path zerolog's native chaining API exists for (see
// httplog.go's HTTPResponse and every logging.From(ctx).Info()... call).
func With(ctx context.Context, kv ...any) context.Context {
	l := From(ctx).With().Fields(kvToFields(kv)).Logger()
	return context.WithValue(ctx, ctxKey{}, &l)
}

// From returns the logger carried on the context, or the global default
// logger (github.com/rs/zerolog/log.Logger, set once in main.go via New)
// if none. Returns a pointer — zerolog.Logger's Debug/Info/Warn/Error
// methods have pointer receivers, so From(ctx).Info() wouldn't compile
// against a value result.
func From(ctx context.Context) *zerolog.Logger {
	if l, ok := ctx.Value(ctxKey{}).(*zerolog.Logger); ok {
		return l
	}
	return &zlog.Logger
}

func kvToFields(kv []any) map[string]any {
	fields := make(map[string]any, len(kv)/2)
	for i := 0; i+1 < len(kv); i += 2 {
		key, ok := kv[i].(string)
		if !ok {
			continue
		}
		fields[key] = kv[i+1]
	}
	return fields
}
