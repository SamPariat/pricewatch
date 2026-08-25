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
	"github.com/sampariat/prices-reminder/internal/logging"
)

const defaultBaseURL = "https://generativelanguage.googleapis.com/v1beta/models"

type LLM struct {
	apiKey     string
	model      string
	baseURL    string
	httpClient *http.Client
}

// New builds a Gemini-backed domain.LLM. model defaults to gemini-3.6-flash
// when empty — gemini-2.5-flash (the free-tier default named in PLAN.md §
// AI features when this was written) was retired for new callers; verified
// against the live API on 2026-08-24, which names gemini-3.6-flash as its
// replacement.
func New(apiKey, model string) *LLM {
	if model == "" {
		model = "gemini-3.6-flash"
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

	start := time.Now()
	resp, err := l.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("gemini: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("gemini: read response: %w", err)
	}
	logging.HTTPResponse(ctx, "gemini: generateContent", resp.StatusCode, time.Since(start), respBody)

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
