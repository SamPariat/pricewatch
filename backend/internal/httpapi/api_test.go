package httpapi_test

import (
	"net/http"
	"testing"

	v1 "github.com/SamPariat/pricewatch/internal/httpapi/presenter/v1"
)

func TestCreateWatch_OneWay_NoReturnDateRequired(t *testing.T) {
	env := newTestEnv(t)
	cookie := env.login(t)
	tripID := seedTrip(t, env, "0 7 * * *")

	body := map[string]any{
		"name": "one way", "kind": "flight_one_way", "trip_id": tripID,
		"params": map[string]any{"origin": "BLR", "destination": "GOI", "depart_date": "2026-12-10"},
	}
	resp := env.do(t, http.MethodPost, apiPrefix+"/watches", body, cookie)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (one-way watches don't need a return_date)", resp.StatusCode)
	}
}

func TestCreateWatch_Return_MissingReturnDate_Returns400(t *testing.T) {
	env := newTestEnv(t)
	cookie := env.login(t)
	tripID := seedTrip(t, env, "0 7 * * *")

	body := map[string]any{
		"name": "missing return", "kind": "flight_return", "trip_id": tripID,
		"params": map[string]any{"origin": "BLR", "destination": "GOI", "depart_date": "2026-12-10"},
	}
	resp := env.do(t, http.MethodPost, apiPrefix+"/watches", body, cookie)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (a return-kind watch needs a return_date)", resp.StatusCode)
	}
}

func TestSettings_GetAndUpdate(t *testing.T) {
	env := newTestEnv(t)
	cookie := env.login(t)

	getResp := env.do(t, http.MethodGet, apiPrefix+"/settings", nil, cookie)
	s := decodeData[v1.Settings](t, getResp)
	if s.Currency != "INR" {
		t.Errorf("default currency = %q, want INR", s.Currency)
	}

	s.DryRun = true
	s.DiscordChannelID = "-100123456"
	updateResp := env.do(t, http.MethodPatch, apiPrefix+"/settings", s, cookie)
	if updateResp.StatusCode != http.StatusOK {
		t.Fatalf("update status = %d", updateResp.StatusCode)
	}

	getResp2 := env.do(t, http.MethodGet, apiPrefix+"/settings", nil, cookie)
	s2 := decodeData[v1.Settings](t, getResp2)
	if !s2.DryRun || s2.DiscordChannelID != "-100123456" {
		t.Errorf("settings did not persist: %+v", s2)
	}
}

func TestRuns_EmptyInitially(t *testing.T) {
	env := newTestEnv(t)
	cookie := env.login(t)

	resp := env.do(t, http.MethodGet, apiPrefix+"/runs", nil, cookie)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	runs := decodeData[[]v1.DigestRun](t, resp)
	if len(runs) != 0 {
		t.Errorf("expected no runs yet, got %d", len(runs))
	}
}

func TestRuns_AfterRunNow_ShowsFailedRun(t *testing.T) {
	env := newTestEnv(t) // empty provider registry — the run will fail at fetch
	cookie := env.login(t)
	tripID := seedTrip(t, env, "0 7 * * *")

	createResp := env.do(t, http.MethodPost, apiPrefix+"/watches", flightWatchBody("run test", tripID), cookie)
	created := decodeData[v1.Watch](t, createResp)

	env.do(t, http.MethodPost, apiPrefix+"/watches/"+created.ID+"/run", nil, cookie)

	resp := env.do(t, http.MethodGet, apiPrefix+"/runs", nil, cookie)
	runs := decodeData[[]v1.DigestRun](t, resp)
	if len(runs) == 0 {
		t.Fatal("expected at least one recorded run after POST .../run")
	}
	if runs[0].Status != "failed" {
		t.Errorf("run status = %q, want failed (no provider registered)", runs[0].Status)
	}

	eventsResp := env.do(t, http.MethodGet, apiPrefix+"/runs/"+runs[0].RunID+"/events", nil, cookie)
	events := decodeData[[]v1.RunEvent](t, eventsResp)
	if len(events) == 0 {
		t.Error("expected at least one run event for the failed run")
	}
}

// TestOldUnversionedPrefix_404s locks down the URL-prefix versioning
// migration: a request to the pre-migration /api/... path (no /v1) must
// not accidentally still work, and Fiber's own unmatched-route error
// must come back enveloped the same as every other error (see
// envelope.ErrorHandler).
func TestOldUnversionedPrefix_404s(t *testing.T) {
	env := newTestEnv(t)
	resp := env.do(t, http.MethodGet, "/api/watches", nil, "")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 for the old unversioned prefix", resp.StatusCode)
	}
	env2 := decodeEnvelope(t, resp)
	if env2.StatusCode != http.StatusNotFound || env2.Error == nil {
		t.Errorf("envelope = %+v, want a 404 envelope with a non-nil error", env2)
	}
}

func TestChannelStatus_ReturnsNotifierStatus(t *testing.T) {
	env := newTestEnv(t) // wired with noop.New(), whose Status() returns "disconnected"
	cookie := env.login(t)

	resp := env.do(t, http.MethodGet, apiPrefix+"/channel/status", nil, cookie)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	type statusBody struct {
		Status string `json:"status"`
	}
	body := decodeData[statusBody](t, resp)
	if body.Status != "disconnected" {
		t.Errorf("status = %q, want disconnected (noop notifier)", body.Status)
	}
}

func TestAnalyticsSummary_CountsWatches(t *testing.T) {
	env := newTestEnv(t)
	cookie := env.login(t)
	tripA := seedTrip(t, env, "0 7 * * *")
	tripB := seedTrip(t, env, "0 8 * * *")

	env.do(t, http.MethodPost, apiPrefix+"/watches", flightWatchBody("a", tripA), cookie)
	env.do(t, http.MethodPost, apiPrefix+"/watches", flightWatchBody("b", tripB), cookie)

	resp := env.do(t, http.MethodGet, apiPrefix+"/analytics/summary", nil, cookie)
	summary := decodeData[v1.AnalyticsSummary](t, resp)
	if summary.TotalWatches != 2 || summary.EnabledWatches != 2 {
		t.Errorf("summary = %+v, want 2 total and 2 enabled", summary)
	}
	// Never run, so unstaged watches with a schedule are considered
	// stale — see the same check exercised in TestCreateWatch_ThenListAndGet.
	if summary.StaleWatches != 2 {
		t.Errorf("StaleWatches = %d, want 2 (neither watch has ever run)", summary.StaleWatches)
	}
}
