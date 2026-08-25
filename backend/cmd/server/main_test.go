package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/rs/zerolog"
)

// TestRequestLogger_LogsRealStatusOnError locks down a real bug: Fiber's
// ErrorHandler runs OUTSIDE the whole middleware chain (confirmed by
// reading router.go — it's called only after app.next(ctx) fully
// returns), so a naive c.Response().StatusCode() read after c.Next()
// reports the pre-error default (200) for any request that actually
// failed. Caught via a live curl against the real running app, which is
// how this test's assertions were chosen — an unhandled-error request
// really did receive 404 from the client's point of view while the
// request log claimed 200.
func TestRequestLogger_LogsRealStatusOnError(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf)

	app := fiber.New(fiber.Config{ErrorHandler: func(c fiber.Ctx, err error) error {
		code := fiber.StatusInternalServerError
		if fe, ok := err.(*fiber.Error); ok {
			code = fe.Code
		}
		return c.SendStatus(code)
	}})
	app.Use(requestLogger(logger))
	app.Get("/missing", func(c fiber.Ctx) error {
		return fiber.NewError(fiber.StatusNotFound, "not found")
	})

	req, err := http.NewRequest(http.MethodGet, "/missing", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusNotFound {
		t.Fatalf("client received status %d, want 404", resp.StatusCode)
	}

	var logged map[string]any
	line := strings.TrimSpace(buf.String())
	if line == "" {
		t.Fatal("expected a log line, got none")
	}
	if err := json.Unmarshal([]byte(line), &logged); err != nil {
		t.Fatalf("decode log line %q: %v", line, err)
	}
	if status, _ := logged["status"].(float64); status != 404 {
		t.Errorf("logged status = %v, want 404 (the real status the client received) — this is exactly the bug that let a failing request log as 200", logged["status"])
	}
}

func TestRequestLogger_LogsRealStatusOnSuccess(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf)

	app := fiber.New()
	app.Use(requestLogger(logger))
	app.Get("/ok", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	req, err := http.NewRequest(http.MethodGet, "/ok", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.Test(req); err != nil {
		t.Fatal(err)
	}

	var logged map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &logged); err != nil {
		t.Fatalf("decode log line: %v", err)
	}
	if status, _ := logged["status"].(float64); status != 200 {
		t.Errorf("logged status = %v, want 200", logged["status"])
	}
}
