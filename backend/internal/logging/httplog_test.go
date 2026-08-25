package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

// captured decodes the single JSON log line a test run produced, so
// assertions can check level/fields without depending on zerolog's exact
// field ordering or console formatting.
func captured(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()
	line := strings.TrimSpace(buf.String())
	if line == "" {
		t.Fatal("expected a log line, got none")
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(line), &m); err != nil {
		t.Fatalf("decode log line %q: %v", line, err)
	}
	return m
}

func loggerCtx(buf *bytes.Buffer) context.Context {
	l := zerolog.New(buf).Level(zerolog.DebugLevel)
	return context.WithValue(context.Background(), ctxKey{}, &l)
}

func TestHTTPResponse_Success_LogsAtDebug(t *testing.T) {
	buf := &bytes.Buffer{}
	ctx := loggerCtx(buf)
	HTTPResponse(ctx, "aviasales: calendar", 200, 50*time.Millisecond, []byte(`{"success":true}`))

	m := captured(t, buf)
	if m["level"] != "debug" {
		t.Errorf("level = %v, want debug", m["level"])
	}
	if msg, _ := m["message"].(string); !strings.Contains(msg, "aviasales: calendar") {
		t.Errorf("message %q missing service identifier", msg)
	}
	if status, _ := m["status"].(float64); status != 200 {
		t.Errorf("status field = %v, want 200", m["status"])
	}
}

func TestHTTPResponse_ErrorStatus_LogsAtWarn(t *testing.T) {
	buf := &bytes.Buffer{}
	ctx := loggerCtx(buf)
	HTTPResponse(ctx, "gemini: generateContent", 404, 10*time.Millisecond, []byte(`{"error":"not found"}`))

	m := captured(t, buf)
	if m["level"] != "warn" {
		t.Errorf("level = %v, want warn", m["level"])
	}
	if msg, _ := m["message"].(string); !strings.Contains(msg, "upstream error response") {
		t.Errorf("message %q missing error framing", msg)
	}
	if body, _ := m["body"].(string); body != `{"error":"not found"}` {
		t.Errorf("body field = %q, want the full error body", body)
	}
}

func TestHTTPResponse_ShortBody_NotTruncated(t *testing.T) {
	buf := &bytes.Buffer{}
	ctx := loggerCtx(buf)
	HTTPResponse(ctx, "ollama: generate", 200, time.Millisecond, []byte("short"))

	m := captured(t, buf)
	if body, _ := m["body"].(string); body != "short" {
		t.Errorf("body field = %q, want %q unmodified", body, "short")
	}
}

func TestHTTPResponse_LongBody_TruncatedWithMarker(t *testing.T) {
	buf := &bytes.Buffer{}
	ctx := loggerCtx(buf)
	long := strings.Repeat("x", bodyPreviewWarn+100)
	HTTPResponse(ctx, "telegram: sendMessage", 500, time.Millisecond, []byte(long))

	m := captured(t, buf)
	body, ok := m["body"].(string)
	if !ok {
		t.Fatalf("body field is %T, want string", m["body"])
	}
	if !strings.HasSuffix(body, "...(truncated)") {
		t.Errorf("body field = %q, want it to end with the truncation marker", body)
	}
	if len(body) != bodyPreviewWarn+len("...(truncated)") {
		t.Errorf("truncated body length = %d, want exactly %d chars of body plus the marker", len(body), bodyPreviewWarn)
	}
}
