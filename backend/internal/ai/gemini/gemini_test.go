package gemini

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sampariat/prices-reminder/internal/domain"
)

func newTestLLM(t *testing.T, handler http.HandlerFunc) *LLM {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	l := New("test-key", "")
	l.baseURL = srv.URL
	return l
}

func TestComplete_ParsesCandidateText(t *testing.T) {
	l := newTestLLM(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.RawQuery, "key=test-key") {
			t.Errorf("expected api key in query, got %q", r.URL.RawQuery)
		}
		var req generateRequest
		json.NewDecoder(r.Body).Decode(&req)
		if req.SystemInstruction == nil || req.SystemInstruction.Parts[0].Text != "sys" {
			t.Errorf("unexpected system instruction: %+v", req.SystemInstruction)
		}
		if req.Contents[0].Parts[0].Text != "user text" {
			t.Errorf("unexpected user content: %+v", req.Contents)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"a punchy reaction"}]}}]}`))
	})

	got, err := l.Complete(context.Background(), domain.Prompt{System: "sys", User: "user text"})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if got != "a punchy reaction" {
		t.Errorf("got %q, want %q", got, "a punchy reaction")
	}
}

func TestComplete_NoCandidates_ReturnsError(t *testing.T) {
	l := newTestLLM(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"candidates":[]}`))
	})

	if _, err := l.Complete(context.Background(), domain.Prompt{User: "hi"}); err == nil {
		t.Fatal("expected an error when the response has no candidates")
	}
}

func TestComplete_UpstreamFailure_ReturnsError(t *testing.T) {
	l := newTestLLM(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"error":"rate limited"}`))
	})

	if _, err := l.Complete(context.Background(), domain.Prompt{User: "hi"}); err == nil {
		t.Fatal("expected an error on a non-200 response")
	}
}
