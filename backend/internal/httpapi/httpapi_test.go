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

	"github.com/SamPariat/pricewatch/internal/httpapi"
	"github.com/SamPariat/pricewatch/internal/httpapi/envelope"
	"github.com/SamPariat/pricewatch/internal/httpapi/handlers"
	apimw "github.com/SamPariat/pricewatch/internal/httpapi/middleware"
	"github.com/SamPariat/pricewatch/internal/notify/noop"
	"github.com/SamPariat/pricewatch/internal/pipeline"
	"github.com/SamPariat/pricewatch/internal/providers"
	"github.com/SamPariat/pricewatch/internal/scheduler"
	"github.com/SamPariat/pricewatch/internal/service"
	"github.com/SamPariat/pricewatch/internal/store/storetest"
)

const testPassword = "correct-horse-battery-staple"

// apiPrefix is the versioned base path every test request is built
// against — see handlers.APIVersion.
const apiPrefix = "/api/" + handlers.APIVersion

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
	notifier := noop.New()
	sched := scheduler.New(repo, pl, notifier)
	if err := sched.Reload(context.Background()); err != nil {
		t.Fatalf("scheduler reload: %v", err)
	}
	t.Cleanup(sched.Stop)

	clock := fixedClock{time.Now().UTC()}
	watches := &service.WatchService{Repo: repo, Sched: sched, Pipeline: pl, Notifier: notifier, Clock: clock}
	session := apimw.NewSession("test-secret")

	api := &handlers.API{
		Watches:   watches,
		Settings:  &service.SettingsService{Repo: repo},
		Runs:      &service.RunsService{Repo: repo},
		Analytics: &service.AnalyticsService{Repo: repo, Sched: sched, Clock: clock},
		Channel:   &service.ChannelService{Notifier: notifier},
		Auth:      &service.AuthService{AdminHash: string(hash), Session: session, SessionTTL: time.Hour},
		Session:   session,
		Sched:     sched,
	}

	// Mirrors cmd/server/main.go's fiber.Config exactly — in particular
	// the envelope ErrorHandler, since these tests assert on the
	// enveloped error shape it produces.
	app := fiber.New(fiber.Config{ErrorHandler: envelope.ErrorHandler})
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
	resp := e.do(t, http.MethodPost, apiPrefix+"/auth/login", map[string]string{"password": testPassword}, "")
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

// decodeData unwraps the {data, message, error, status_code} envelope
// every response now carries and returns just the Data field, decoded
// into T — see internal/httpapi/envelope.
func decodeData[T any](t *testing.T, resp *http.Response) T {
	t.Helper()
	defer resp.Body.Close()
	var env struct {
		Data T `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return env.Data
}

// decodeEnvelope decodes the full envelope, for tests that need to
// assert on message/error/status_code directly rather than just the
// unwrapped data.
func decodeEnvelope(t *testing.T, resp *http.Response) envelope.Envelope {
	t.Helper()
	defer resp.Body.Close()
	var env envelope.Envelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	return env
}
