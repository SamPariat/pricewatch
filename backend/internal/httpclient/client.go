// Package httpclient is the one place every outbound call to a
// third-party API goes through — Aviasales, Gemini, and Ollama each used
// to build their own *http.Client and duplicate the same
// request→read-body→log sequence by hand (Discord is the one exception:
// discordgo manages its own HTTP client internally). Do centralizes
// that; each adapter still owns its own request construction (headers, query
// params, body) since that's genuinely provider-specific.
package httpclient

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/SamPariat/pricewatch/internal/logging"
)

type Client struct {
	http *http.Client
}

// New builds a Client with a fixed per-request timeout. Adapters keep
// their own existing timeout values (15s/20s/30s) — this doesn't change
// any of them, just where the *http.Client lives.
func New(timeout time.Duration) *Client {
	return &Client{http: &http.Client{Timeout: timeout}}
}

// Do executes req, reads the full response body, and logs it via
// logging.HTTPResponse(ctx, label, status, duration, body) — DEBUG with a
// preview on success, WARN with a longer preview on any status >= 400 —
// before returning the status code and body for the caller to unmarshal.
// label identifies the integration and call (e.g. "aviasales: calendar",
// "gemini: generateContent") for that log line; it must be a caller-written
// literal, never derived from the request, since the request may carry a
// token in its URL or headers that must never be logged.
func (c *Client) Do(ctx context.Context, req *http.Request, label string) (status int, body []byte, err error) {
	start := time.Now()
	resp, err := c.http.Do(req)
	if err != nil {
		// Deliberately not %w-wrapped: Go's http.Client wraps network
		// failures in *url.Error, whose Error() string includes the full
		// request URL — every caller of this client puts a token in that
		// URL (query string or path), so propagating the raw error risks
		// leaking it into a returned or logged error message.
		return 0, nil, fmt.Errorf("%s: request failed", label)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, fmt.Errorf("%s: read response: %w", label, err)
	}
	logging.HTTPResponse(ctx, label, resp.StatusCode, time.Since(start), respBody)

	return resp.StatusCode, respBody, nil
}
