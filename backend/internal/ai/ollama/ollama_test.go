package ollama

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/SamPariat/pricewatch/internal/domain"
)

func TestComplete_SendsModelAndParsesResponse(t *testing.T) {
	var captured generateRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/generate" {
			t.Errorf("path = %q, want /api/generate", r.URL.Path)
		}
		json.NewDecoder(r.Body).Decode(&captured)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"response":"a punchy reaction"}`))
	}))
	defer srv.Close()

	l := New(srv.URL, "")
	got, err := l.Complete(context.Background(), domain.Prompt{System: "sys", User: "user text"})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if got != "a punchy reaction" {
		t.Errorf("got %q, want %q", got, "a punchy reaction")
	}
	if captured.Model != "llama3.2" || captured.System != "sys" || captured.Prompt != "user text" || captured.Stream {
		t.Errorf("unexpected request: %+v", captured)
	}
}

func TestComplete_UpstreamFailure_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	l := New(srv.URL, "")
	if _, err := l.Complete(context.Background(), domain.Prompt{User: "hi"}); err == nil {
		t.Fatal("expected an error on a non-200 response")
	}
}
