package telegram

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestGetUpdates_ParsesCallbackQuery(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true,"result":[{"update_id":100,"callback_query":{"id":"cb1","data":"snooze:watch123"}}]}`))
	}))
	defer srv.Close()

	n := NewForTest("test-token", srv.URL)
	updates, err := n.GetUpdates(context.Background(), 0, 5)
	if err != nil {
		t.Fatalf("GetUpdates: %v", err)
	}
	if len(updates) != 1 {
		t.Fatalf("got %d updates, want 1", len(updates))
	}
	if updates[0].UpdateID != 100 {
		t.Errorf("UpdateID = %d, want 100", updates[0].UpdateID)
	}
	if updates[0].CallbackQuery == nil || updates[0].CallbackQuery.Data != "snooze:watch123" {
		t.Fatalf("unexpected callback query: %+v", updates[0].CallbackQuery)
	}
}

func TestGetUpdates_SendsOffsetAndTimeout(t *testing.T) {
	var gotQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true,"result":[]}`))
	}))
	defer srv.Close()

	n := NewForTest("test-token", srv.URL)
	if _, err := n.GetUpdates(context.Background(), 42, 30); err != nil {
		t.Fatalf("GetUpdates: %v", err)
	}
	if gotQuery.Get("offset") != "42" {
		t.Errorf("offset = %q, want 42", gotQuery.Get("offset"))
	}
	if gotQuery.Get("timeout") != "30" {
		t.Errorf("timeout = %q, want 30", gotQuery.Get("timeout"))
	}
}

func TestGetUpdates_UpstreamFailure_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":false}`))
	}))
	defer srv.Close()

	n := NewForTest("test-token", srv.URL)
	if _, err := n.GetUpdates(context.Background(), 0, 5); err == nil {
		t.Fatal("expected an error when getUpdates reports ok=false")
	}
}

func TestAnswerCallbackQuery_SendsCorrectBody(t *testing.T) {
	var captured answerCallbackQueryReq
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&captured)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	n := NewForTest("test-token", srv.URL)
	if err := n.AnswerCallbackQuery(context.Background(), "cb1", "Snoozed for 7 days."); err != nil {
		t.Fatalf("AnswerCallbackQuery: %v", err)
	}
	if captured.CallbackQueryID != "cb1" || captured.Text != "Snoozed for 7 days." {
		t.Errorf("unexpected request body: %+v", captured)
	}
}
