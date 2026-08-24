package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"golang.org/x/crypto/bcrypt"

	"github.com/sampariat/prices-reminder/internal/httpapi"
	"github.com/sampariat/prices-reminder/internal/httpapi/handlers"
	apimw "github.com/sampariat/prices-reminder/internal/httpapi/middleware"
	"github.com/sampariat/prices-reminder/internal/notify/noop"
	"github.com/sampariat/prices-reminder/internal/pipeline"
	"github.com/sampariat/prices-reminder/internal/providers"
	"github.com/sampariat/prices-reminder/internal/scheduler"
	"github.com/sampariat/prices-reminder/internal/store/storetest"
)

const testPassword = "correct-horse-battery-staple"

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

type testEnv struct {
	app  *fiber.App
	repo *storetest.FakeRepository
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()

	hash, err := bcrypt.GenerateFromPassword([]byte(testPassword), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}

	repo := storetest.New()
	registry := providers.NewRegistry() // deliberately empty — RunWatchNow's failure path needs this
	pl := &pipeline.Pipeline{Registry: registry, Repo: repo, Clock: fixedClock{time.Now().UTC()}}
	sched := scheduler.New(repo, pl, noop.New())
	if err := sched.Reload(context.Background()); err != nil {
		t.Fatalf("scheduler reload: %v", err)
	}
	t.Cleanup(sched.Stop)

	api := &handlers.API{
		Repo: repo, Sched: sched, Pipeline: pl, Notifier: noop.New(), Clock: fixedClock{time.Now().UTC()},
		MinVersion: 1, MaxVersion: 1, AdminHash: string(hash),
		Session: apimw.NewSession("test-secret"), SessionTTL: time.Hour,
	}

	app := fiber.New()
	httpapi.Router(app, api)

	return &testEnv{app: app, repo: repo}
}

func (e *testEnv) do(t *testing.T, method, path string, body any, cookie string) *http.Response {
	t.Helper()
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	if cookie != "" {
		req.Header.Set("Cookie", apimw.SessionCookie+"="+cookie)
	}
	resp, err := e.app.Test(req)
	if err != nil {
		t.Fatalf("request %s %s: %v", method, path, err)
	}
	return resp
}

// login returns the session cookie value for use in subsequent requests.
func (e *testEnv) login(t *testing.T) string {
	t.Helper()
	resp := e.do(t, http.MethodPost, "/api/auth/login", map[string]string{"password": testPassword}, "")
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("login: status = %d", resp.StatusCode)
	}
	for _, c := range resp.Cookies() {
		if c.Name == apimw.SessionCookie {
			return c.Value
		}
	}
	t.Fatal("login: no session cookie set")
	return ""
}

func decodeJSON(t *testing.T, resp *http.Response, v any) {
	t.Helper()
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}
