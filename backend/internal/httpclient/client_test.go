package httpclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestDo_Success_ReturnsStatusAndBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	c := New(5 * time.Second)
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL, nil)
	status, body, err := c.Do(context.Background(), req, "test: call")
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if status != http.StatusTeapot {
		t.Errorf("status = %d, want %d", status, http.StatusTeapot)
	}
	if string(body) != `{"ok":true}` {
		t.Errorf("body = %q, want %q", body, `{"ok":true}`)
	}
}

// TestDo_NetworkFailure_NeverLeaksRequestURL guards a real security
// property: every caller of this client embeds a secret (a bot token, an
// API key) in the request URL's path or query string. Go's http.Client
// wraps network-level failures in *url.Error, whose Error() string
// includes the full request URL — if Do propagated that raw error, the
// secret would leak into anything that logs or returns it.
func TestDo_NetworkFailure_NeverLeaksRequestURL(t *testing.T) {
	c := New(time.Second)
	secretURL := "http://127.0.0.1:1/bot-token-shaped-secret-xyz/getMe" // port 1 — connection refused
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, secretURL, nil)

	_, _, err := c.Do(context.Background(), req, "test: call")
	if err == nil {
		t.Fatal("expected an error for a connection that can't be established")
	}
	if strings.Contains(err.Error(), "bot-token-shaped-secret-xyz") {
		t.Fatalf("error leaked the request URL (and so the secret in it): %v", err)
	}
}

func TestDo_ErrorStatus_StillReturnsBodyForCallerToInspect(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"error":"rate limited"}`))
	}))
	defer srv.Close()

	c := New(5 * time.Second)
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL, nil)
	status, body, err := c.Do(context.Background(), req, "test: call")
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if status != http.StatusTooManyRequests {
		t.Errorf("status = %d, want 429", status)
	}
	if !strings.Contains(string(body), "rate limited") {
		t.Errorf("body = %q, want it to contain the upstream's error detail", body)
	}
}
