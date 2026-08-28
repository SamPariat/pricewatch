// Package gemini adapts Google AI Studio's Gemini API to domain.LLM. Plain
// REST over net/http rather than the official SDK — the request shape is
// small and stable, and every other external adapter that talks to a
// plain REST API (aviasales, ollama) is written the same way rather than
// pulling in a client library for one endpoint. Discord is the one
// deliberate exception: its Gateway WebSocket protocol is enough of its
// own thing that discordgo earns its keep (see internal/notify/discord).
package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/SamPariat/pricewatch/internal/domain"
	"github.com/SamPariat/pricewatch/internal/httpclient"
)

const defaultBaseURL = "https://generativelanguage.googleapis.com/v1beta/models"

type LLM struct {
	apiKey     string
	model      string
	baseURL    string
	httpClient *httpclient.Client
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
	return &LLM{apiKey: apiKey, model: model, baseURL: defaultBaseURL, httpClient: httpclient.New(15 * time.Second)}
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

	status, respBody, err := l.httpClient.Do(ctx, req, "gemini: generateContent")
	if err != nil {
		return "", err
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("gemini: upstream returned %d: %s", status, respBody)
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
