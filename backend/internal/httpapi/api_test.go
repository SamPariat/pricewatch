package httpapi_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	v1 "github.com/sampariat/prices-reminder/internal/httpapi/presenter/v1"
)

func TestCreateWatch_OneWay_NoReturnDateRequired(t *testing.T) {
	env := newTestEnv(t)
	cookie := env.login(t)

	body := map[string]any{
		"name": "one way", "kind": "flight_one_way", "cron_expr": "0 7 * * *", "timezone": "UTC",
		"params": map[string]any{"origin": "BLR", "destination": "GOI", "depart_date": "2026-12-10"},
	}
	resp := env.do(t, http.MethodPost, "/api/watches", body, cookie)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (one-way watches don't need a return_date)", resp.StatusCode)
	}
}

func TestCreateWatch_Return_MissingReturnDate_Returns400(t *testing.T) {
	env := newTestEnv(t)
	cookie := env.login(t)

	body := map[string]any{
		"name": "missing return", "kind": "flight_return", "cron_expr": "0 7 * * *", "timezone": "UTC",
		"params": map[string]any{"origin": "BLR", "destination": "GOI", "depart_date": "2026-12-10"},
	}
	resp := env.do(t, http.MethodPost, "/api/watches", body, cookie)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (a return-kind watch needs a return_date)", resp.StatusCode)
	}
}

func TestCreateWatch_Hotel(t *testing.T) {
	env := newTestEnv(t)
	cookie := env.login(t)

	body := map[string]any{
		"name": "hotel test", "kind": "hotel", "cron_expr": "0 7 * * *", "timezone": "UTC",
		"params": map[string]any{"location": "Goa", "check_in": "2026-12-10", "check_out": "2026-12-15"},
	}
	resp := env.do(t, http.MethodPost, "/api/watches", body, cookie)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201", resp.StatusCode)
	}
}

func TestSettings_GetAndUpdate(t *testing.T) {
	env := newTestEnv(t)
	cookie := env.login(t)

	getResp := env.do(t, http.MethodGet, "/api/settings", nil, cookie)
	var s v1.Settings
	decodeJSON(t, getResp, &s)
	if s.Currency != "INR" {
		t.Errorf("default currency = %q, want INR", s.Currency)
	}

	s.DryRun = true
	s.TelegramChatID = "-100123456"
	updateResp := env.do(t, http.MethodPatch, "/api/settings", s, cookie)
	if updateResp.StatusCode != http.StatusOK {
		t.Fatalf("update status = %d", updateResp.StatusCode)
	}

	getResp2 := env.do(t, http.MethodGet, "/api/settings", nil, cookie)
	var s2 v1.Settings
	decodeJSON(t, getResp2, &s2)
	if !s2.DryRun || s2.TelegramChatID != "-100123456" {
		t.Errorf("settings did not persist: %+v", s2)
	}
}

func TestRuns_EmptyInitially(t *testing.T) {
	env := newTestEnv(t)
	cookie := env.login(t)

	resp := env.do(t, http.MethodGet, "/api/runs", nil, cookie)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var runs []v1.DigestRun
	decodeJSON(t, resp, &runs)
	if len(runs) != 0 {
		t.Errorf("expected no runs yet, got %d", len(runs))
	}
}

func TestRuns_AfterRunNow_ShowsFailedRun(t *testing.T) {
	env := newTestEnv(t) // empty provider registry — the run will fail at fetch
	cookie := env.login(t)

	createResp := env.do(t, http.MethodPost, "/api/watches", flightWatchBody("run test", "0 7 * * *"), cookie)
	var created v1.Watch
	decodeJSON(t, createResp, &created)

	env.do(t, http.MethodPost, "/api/watches/"+created.ID+"/run", nil, cookie)

	resp := env.do(t, http.MethodGet, "/api/runs", nil, cookie)
	var runs []v1.DigestRun
	decodeJSON(t, resp, &runs)
	if len(runs) == 0 {
		t.Fatal("expected at least one recorded run after POST .../run")
	}
	if runs[0].Status != "failed" {
		t.Errorf("run status = %q, want failed (no provider registered)", runs[0].Status)
	}

	eventsResp := env.do(t, http.MethodGet, "/api/runs/"+runs[0].RunID+"/events", nil, cookie)
	var events []v1.RunEvent
	decodeJSON(t, eventsResp, &events)
	if len(events) == 0 {
		t.Error("expected at least one run event for the failed run")
	}
}

func TestAPIVersion_AbsentHeader_DefaultsToOldest(t *testing.T) {
	env := newTestEnv(t)
	resp := env.do(t, http.MethodGet, "/api/meta", nil, "")
	if got := resp.Header.Get("X-API-Version"); got != "1" {
		t.Errorf("X-API-Version = %q, want 1 (the oldest supported, per PLAN.md)", got)
	}
}

func TestAPIVersion_Unsupported_Returns400(t *testing.T) {
	env := newTestEnv(t)
	req := httptest.NewRequest(http.MethodGet, "/api/meta", nil)
	req.Header.Set("X-API-Version", "99")
	resp, err := env.app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for an out-of-range version", resp.StatusCode)
	}
}

func TestChannelStatus_ReturnsNotifierStatus(t *testing.T) {
	env := newTestEnv(t) // wired with noop.New(), whose Status() returns "disconnected"
	cookie := env.login(t)

	resp := env.do(t, http.MethodGet, "/api/channel/status", nil, cookie)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var body struct {
		Status string `json:"status"`
	}
	decodeJSON(t, resp, &body)
	if body.Status != "disconnected" {
		t.Errorf("status = %q, want disconnected (noop notifier)", body.Status)
	}
}

func TestAnalyticsSummary_CountsWatches(t *testing.T) {
	env := newTestEnv(t)
	cookie := env.login(t)

	env.do(t, http.MethodPost, "/api/watches", flightWatchBody("a", "0 7 * * *"), cookie)
	env.do(t, http.MethodPost, "/api/watches", flightWatchBody("b", "0 8 * * *"), cookie)

	resp := env.do(t, http.MethodGet, "/api/analytics/summary", nil, cookie)
	var summary v1.AnalyticsSummary
	decodeJSON(t, resp, &summary)
	if summary.TotalWatches != 2 || summary.EnabledWatches != 2 {
		t.Errorf("summary = %+v, want 2 total and 2 enabled", summary)
	}
	// Never run, so unstaged watches with a schedule are considered
	// stale — see the same check exercised in TestCreateWatch_ThenListAndGet.
	if summary.StaleWatches != 2 {
		t.Errorf("StaleWatches = %d, want 2 (neither watch has ever run)", summary.StaleWatches)
	}
}
