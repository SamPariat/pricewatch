package telegram

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/sampariat/prices-reminder/internal/domain"
)

// capturingServer records every request it receives so tests can assert
// on what was actually sent, and returns a scriptable Telegram-shaped
// response.
type capturingServer struct {
	*httptest.Server
	mu       sync.Mutex
	requests []capturedRequest
	fail     bool
}

type capturedRequest struct {
	Path        string
	ContentType string
	Body        []byte
	Form        *http.Request // parsed multipart form, nil for JSON calls
}

func newCapturingServer(t *testing.T) *capturingServer {
	t.Helper()
	cs := &capturingServer{}
	cs.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ct := r.Header.Get("Content-Type")
		var rec capturedRequest
		rec.ContentType = ct

		if strings.HasPrefix(ct, "multipart/form-data") {
			if err := r.ParseMultipartForm(10 << 20); err != nil {
				t.Fatalf("parse multipart: %v", err)
			}
			rec.Form = r
		} else {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("read body: %v", err)
			}
			rec.Body = body
		}
		rec.Path = r.URL.Path

		cs.mu.Lock()
		cs.requests = append(cs.requests, rec)
		fail := cs.fail
		cs.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		if fail {
			w.Write([]byte(`{"ok":false,"description":"forced failure"}`))
			return
		}
		w.Write([]byte(`{"ok":true}`))
	}))
	return cs
}

func (cs *capturingServer) last() capturedRequest {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	return cs.requests[len(cs.requests)-1]
}

func (cs *capturingServer) count() int {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	return len(cs.requests)
}

func newTestNotifier(baseURL string) *Notifier {
	return NewForTest("test-token", baseURL)
}

func TestSend_PlainMessage_HitsSendMessage(t *testing.T) {
	srv := newCapturingServer(t)
	defer srv.Close()
	n := newTestNotifier(srv.URL)

	err := n.Send(context.Background(), domain.Target{ChatID: "123"}, domain.Message{Text: "<b>hi</b>"})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if got := srv.last().Path; !strings.HasSuffix(got, "/sendMessage") {
		t.Errorf("hit path %q, want it to end in /sendMessage", got)
	}

	var req sendMessageReq
	if err := json.Unmarshal(srv.last().Body, &req); err != nil {
		t.Fatal(err)
	}
	if req.ChatID != "123" || req.Text != "<b>hi</b>" || req.ParseMode != "HTML" {
		t.Errorf("unexpected request body: %+v", req)
	}
}

func TestSend_WithImage_HitsSendPhoto(t *testing.T) {
	srv := newCapturingServer(t)
	defer srv.Close()
	n := newTestNotifier(srv.URL)

	err := n.Send(context.Background(), domain.Target{ChatID: "123"}, domain.Message{
		Text: "caption text", ImagePNG: []byte{0x89, 'P', 'N', 'G'},
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	last := srv.last()
	if !strings.HasSuffix(last.Path, "/sendPhoto") {
		t.Errorf("hit path %q, want it to end in /sendPhoto", last.Path)
	}
	if got := last.Form.FormValue("chat_id"); got != "123" {
		t.Errorf("chat_id = %q, want 123", got)
	}
	if got := last.Form.FormValue("caption"); got != "caption text" {
		t.Errorf("caption = %q, want %q", got, "caption text")
	}
	if _, _, err := last.Form.FormFile("photo"); err != nil {
		t.Errorf("expected a photo file part: %v", err)
	}
}

func TestSend_WithButtons_IncludesInlineKeyboard(t *testing.T) {
	srv := newCapturingServer(t)
	defer srv.Close()
	n := newTestNotifier(srv.URL)

	buttons := [][]domain.Button{{{Label: "Snooze 7d", Callback: "snooze:7"}, {Label: "Pause", Callback: "pause"}}}
	err := n.Send(context.Background(), domain.Target{ChatID: "123"}, domain.Message{Text: "hi", Buttons: buttons})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}

	var req sendMessageReq
	json.Unmarshal(srv.last().Body, &req)
	if req.ReplyMarkup == nil || len(req.ReplyMarkup.InlineKeyboard) != 1 || len(req.ReplyMarkup.InlineKeyboard[0]) != 2 {
		t.Fatalf("expected a 1x2 inline keyboard, got %+v", req.ReplyMarkup)
	}
	if req.ReplyMarkup.InlineKeyboard[0][0].CallbackData != "snooze:7" {
		t.Errorf("unexpected callback data: %+v", req.ReplyMarkup.InlineKeyboard[0][0])
	}
}

func TestSend_LongMessage_SplitsAcrossMultipleCalls(t *testing.T) {
	srv := newCapturingServer(t)
	defer srv.Close()
	n := newTestNotifier(srv.URL)

	// Three sections each just under half the limit — two should fit in
	// one message, the third needs a second call.
	section := strings.Repeat("x", messageLimit/2-10)
	text := strings.Join([]string{section, section, section}, "\n\n")

	if err := n.Send(context.Background(), domain.Target{ChatID: "123"}, domain.Message{Text: text}); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if got := srv.count(); got < 2 {
		t.Fatalf("expected the message to split into at least 2 calls, got %d", got)
	}
}

func TestSend_ImageOnlyAttachedToFirstPart(t *testing.T) {
	srv := newCapturingServer(t)
	defer srv.Close()
	n := newTestNotifier(srv.URL)

	section := strings.Repeat("x", messageLimit/2-10)
	text := strings.Join([]string{section, section, section}, "\n\n")

	if err := n.Send(context.Background(), domain.Target{ChatID: "123"}, domain.Message{
		Text: text, ImagePNG: []byte{0x89, 'P', 'N', 'G'},
	}); err != nil {
		t.Fatalf("Send: %v", err)
	}

	cs := srv
	cs.mu.Lock()
	reqs := append([]capturedRequest(nil), cs.requests...)
	cs.mu.Unlock()

	if len(reqs) < 2 {
		t.Fatalf("expected at least 2 calls, got %d", len(reqs))
	}
	if !strings.HasSuffix(reqs[0].Path, "/sendPhoto") {
		t.Errorf("first call = %q, want it to end in /sendPhoto", reqs[0].Path)
	}
	for i, r := range reqs[1:] {
		if !strings.HasSuffix(r.Path, "/sendMessage") {
			t.Errorf("call %d = %q, want it to end in /sendMessage (no image on later parts)", i+1, r.Path)
		}
	}
}

func TestSend_UpstreamFailure_ReturnsError(t *testing.T) {
	srv := newCapturingServer(t)
	srv.fail = true
	defer srv.Close()
	n := newTestNotifier(srv.URL)

	err := n.Send(context.Background(), domain.Target{ChatID: "123"}, domain.Message{Text: "hi"})
	if err == nil {
		t.Fatal("expected an error when the Telegram API reports ok=false")
	}
}

func TestStatus_OK_ReturnsLinked(t *testing.T) {
	srv := newCapturingServer(t)
	defer srv.Close()
	n := newTestNotifier(srv.URL)

	status, err := n.Status(context.Background())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if status != domain.NotifierLinked {
		t.Errorf("status = %q, want linked", status)
	}
}

func TestStatus_Failure_ReturnsDisconnectedNotError(t *testing.T) {
	srv := newCapturingServer(t)
	srv.fail = true
	defer srv.Close()
	n := newTestNotifier(srv.URL)

	status, err := n.Status(context.Background())
	if err != nil {
		t.Fatalf("Status should not return an error on API failure, got: %v", err)
	}
	if status != domain.NotifierDisconnected {
		t.Errorf("status = %q, want disconnected", status)
	}
}

func TestSplitMessage_FitsUnderLimit(t *testing.T) {
	parts := splitMessage("short text", 4096)
	if len(parts) != 1 || parts[0] != "short text" {
		t.Errorf("got %v, want a single unsplit part", parts)
	}
}

func TestSplitMessage_PacksMultipleSectionsPerPart(t *testing.T) {
	text := strings.Join([]string{"a", "b", "c"}, "\n\n")
	parts := splitMessage(text, 100)
	if len(parts) != 1 {
		t.Fatalf("expected 3 small sections to pack into 1 part, got %d: %v", len(parts), parts)
	}
}

func TestSplitMessage_NeverExceedsLimit(t *testing.T) {
	limit := 50
	sections := make([]string, 10)
	for i := range sections {
		sections[i] = strings.Repeat(strconv.Itoa(i), 20)
	}
	parts := splitMessage(strings.Join(sections, "\n\n"), limit)
	for i, p := range parts {
		if len(p) > limit {
			t.Errorf("part %d has length %d, exceeds limit %d", i, len(p), limit)
		}
	}
}
