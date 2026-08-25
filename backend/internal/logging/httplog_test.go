package logging

import (
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"
)

// capturingHandler is a minimal slog.Handler that records every call for
// assertion — good enough to check level and attributes without pulling
// in a testing/slogtest dependency.
type capturingHandler struct {
	records []slog.Record
}

func (h *capturingHandler) Enabled(context.Context, slog.Level) bool { return true }
func (h *capturingHandler) Handle(_ context.Context, r slog.Record) error {
	h.records = append(h.records, r)
	return nil
}
func (h *capturingHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *capturingHandler) WithGroup(string) slog.Handler      { return h }

func (h *capturingHandler) attr(name string) (any, bool) {
	if len(h.records) == 0 {
		return nil, false
	}
	last := h.records[len(h.records)-1]
	var val any
	var found bool
	last.Attrs(func(a slog.Attr) bool {
		if a.Key == name {
			val, found = a.Value.Any(), true
			return false
		}
		return true
	})
	return val, found
}

func withCapture(t *testing.T) *capturingHandler {
	t.Helper()
	h := &capturingHandler{}
	prev := slog.Default()
	slog.SetDefault(slog.New(h))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return h
}

func TestHTTPResponse_Success_LogsAtDebug(t *testing.T) {
	h := withCapture(t)
	HTTPResponse(context.Background(), "aviasales: calendar", 200, 50*time.Millisecond, []byte(`{"success":true}`))

	if len(h.records) != 1 {
		t.Fatalf("got %d log records, want 1", len(h.records))
	}
	if h.records[0].Level != slog.LevelDebug {
		t.Errorf("level = %v, want Debug", h.records[0].Level)
	}
	if !strings.Contains(h.records[0].Message, "aviasales: calendar") {
		t.Errorf("message %q missing service identifier", h.records[0].Message)
	}
	if status, _ := h.attr("status"); status != int64(200) {
		t.Errorf("status attr = %v, want 200", status)
	}
}

func TestHTTPResponse_ErrorStatus_LogsAtWarn(t *testing.T) {
	h := withCapture(t)
	HTTPResponse(context.Background(), "gemini: generateContent", 404, 10*time.Millisecond, []byte(`{"error":"not found"}`))

	if len(h.records) != 1 {
		t.Fatalf("got %d log records, want 1", len(h.records))
	}
	if h.records[0].Level != slog.LevelWarn {
		t.Errorf("level = %v, want Warn", h.records[0].Level)
	}
	if !strings.Contains(h.records[0].Message, "upstream error response") {
		t.Errorf("message %q missing error framing", h.records[0].Message)
	}
	body, _ := h.attr("body")
	if body != `{"error":"not found"}` {
		t.Errorf("body attr = %q, want the full error body", body)
	}
}

func TestHTTPResponse_ShortBody_NotTruncated(t *testing.T) {
	h := withCapture(t)
	HTTPResponse(context.Background(), "ollama: generate", 200, time.Millisecond, []byte("short"))

	body, _ := h.attr("body")
	if body != "short" {
		t.Errorf("body attr = %q, want %q unmodified", body, "short")
	}
}

func TestHTTPResponse_LongBody_TruncatedWithMarker(t *testing.T) {
	h := withCapture(t)
	long := strings.Repeat("x", bodyPreviewWarn+100)
	HTTPResponse(context.Background(), "telegram: sendMessage", 500, time.Millisecond, []byte(long))

	body, _ := h.attr("body")
	s, ok := body.(string)
	if !ok {
		t.Fatalf("body attr is %T, want string", body)
	}
	if !strings.HasSuffix(s, "...(truncated)") {
		t.Errorf("body attr = %q, want it to end with the truncation marker", s)
	}
	if len(s) != bodyPreviewWarn+len("...(truncated)") {
		t.Errorf("truncated body length = %d, want exactly %d chars of body plus the marker", len(s), bodyPreviewWarn)
	}
}
