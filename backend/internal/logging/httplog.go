package logging

import (
	"context"
	"time"
)

// bodyPreviewDebug/Warn cap how much of a response body lands in a log
// line — enough to actually diagnose a schema change or an upstream error
// message, not so much that one noisy call floods the log.
const (
	bodyPreviewDebug = 500
	bodyPreviewWarn  = 2000
)

// HTTPResponse logs one line for a completed call to a third-party API —
// see PLAN.md § Logging: "DEBUG: Provider request/response shapes." A
// successful response logs at DEBUG with a short body preview (off by
// default in production); any status >= 400 logs at WARN with a longer
// preview, since an upstream's error body is normally the only place that
// explains *why* (rate limited, deprecated endpoint, bad auth) — exactly
// the detail a bare "unexpected status 404" throws away.
//
// service identifies the integration and call, e.g. "aviasales: calendar"
// or "telegram: sendMessage" — always a caller-supplied literal, never
// derived from the request, so there's no risk of a token embedded in a
// URL path or query string (see PLAN.md § Traps: "the Travelpayouts token
// travels in a query parameter") leaking through this path. Only the
// upstream's response body is logged, never the request.
func HTTPResponse(ctx context.Context, service string, statusCode int, dur time.Duration, body []byte) {
	log := From(ctx)
	if statusCode >= 400 {
		log.Warn(service+": upstream error response",
			"status", statusCode, "duration_ms", dur.Milliseconds(), "body", preview(body, bodyPreviewWarn))
		return
	}
	log.Debug(service+": upstream response",
		"status", statusCode, "duration_ms", dur.Milliseconds(), "bytes", len(body), "body", preview(body, bodyPreviewDebug))
}

func preview(body []byte, n int) string {
	if len(body) <= n {
		return string(body)
	}
	return string(body[:n]) + "...(truncated)"
}
