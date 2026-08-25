// Package ollama adapts a local Ollama server (POST /api/generate) to
// domain.LLM — the fallback leg of PLAN.md § AI features: "Gemini Flash
// primary, Ollama fallback... local inference being slow doesn't matter
// [for a once-a-day batch job], and it's free, unlimited, and never sends
// your travel plans to a third party."
package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/sampariat/prices-reminder/internal/domain"
	"github.com/sampariat/prices-reminder/internal/logging"
)

type LLM struct {
	baseURL    string
	model      string
	httpClient *http.Client
}

// New builds an Ollama-backed domain.LLM against baseURL (e.g.
// http://localhost:11434). model defaults to llama3.2 when empty — a small
// model is fine here since this only ever writes one short phrase, never
// a number (see the caller in internal/ai/copywriter.go).
func New(baseURL, model string) *LLM {
	if model == "" {
		model = "llama3.2"
	}
	return &LLM{baseURL: baseURL, model: model, httpClient: &http.Client{Timeout: 30 * time.Second}}
}

type generateRequest struct {
	Model  string `json:"model"`
	System string `json:"system,omitempty"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

type generateResponse struct {
	Response string `json:"response"`
}

func (l *LLM) Complete(ctx context.Context, p domain.Prompt) (string, error) {
	reqBody := generateRequest{Model: l.model, System: p.System, Prompt: p.User, Stream: false}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("ollama: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, l.baseURL+"/api/generate", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("ollama: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	start := time.Now()
	resp, err := l.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("ollama: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("ollama: read response: %w", err)
	}
	logging.HTTPResponse(ctx, "ollama: generate", resp.StatusCode, time.Since(start), respBody)

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ollama: upstream returned %d: %s", resp.StatusCode, respBody)
	}

	var out generateResponse
	if err := json.Unmarshal(respBody, &out); err != nil {
		return "", fmt.Errorf("ollama: decode response: %w", err)
	}
	return out.Response, nil
}
