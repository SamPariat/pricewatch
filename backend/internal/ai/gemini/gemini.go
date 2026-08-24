// Package gemini adapts Google AI Studio's Gemini API to domain.LLM. Plain
// REST over net/http rather than the official SDK — the request shape is
// small and stable, and every other external adapter in this codebase
// (aviasales, hotellook, telegram) is written the same way rather than
// pulling in a client library for one endpoint.
package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/sampariat/prices-reminder/internal/domain"
)

const defaultBaseURL = "https://generativelanguage.googleapis.com/v1beta/models"

type LLM struct {
	apiKey     string
	model      string
	baseURL    string
	httpClient *http.Client
}

// New builds a Gemini-backed domain.LLM. model defaults to gemini-2.5-flash
// (PLAN.md § AI features: "1,500 req/day, no card — default, ~50x
// headroom") when empty.
func New(apiKey, model string) *LLM {
	if model == "" {
		model = "gemini-2.5-flash"
	}
	return &LLM{apiKey: apiKey, model: model, baseURL: defaultBaseURL, httpClient: &http.Client{Timeout: 15 * time.Second}}
}

type generateRequest struct {
	SystemInstruction *content  `json:"system_instruction,omitempty"`
	Contents          []content `json:"contents"`
}

type content struct {
	Parts []part `json:"parts"`
}

type part struct {
	Text string `json:"text"`
}

type generateResponse struct {
	Candidates []struct {
		Content content `json:"content"`
	} `json:"candidates"`
}

func (l *LLM) Complete(ctx context.Context, p domain.Prompt) (string, error) {
	reqBody := generateRequest{
		Contents: []content{{Parts: []part{{Text: p.User}}}},
	}
	if p.System != "" {
		reqBody.SystemInstruction = &content{Parts: []part{{Text: p.System}}}
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("gemini: marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/%s:generateContent?key=%s", l.baseURL, l.model, l.apiKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("gemini: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := l.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("gemini: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("gemini: read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("gemini: upstream returned %d: %s", resp.StatusCode, respBody)
	}

	var out generateResponse
	if err := json.Unmarshal(respBody, &out); err != nil {
		return "", fmt.Errorf("gemini: decode response: %w", err)
	}
	if len(out.Candidates) == 0 || len(out.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("gemini: response had no candidates")
	}
	return out.Candidates[0].Content.Parts[0].Text, nil
}
