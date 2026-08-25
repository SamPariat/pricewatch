package httpapi_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/SamPariat/pricewatch/internal/domain"
	v1 "github.com/SamPariat/pricewatch/internal/httpapi/presenter/v1"
)

func flightWatchBody(name, cronExpr string) map[string]any {
	return map[string]any{
		"name":          name,
		"kind":          "flight_return",
		"cron_expr":     cronExpr,
		"timezone":      "UTC",
		"threshold_pct": 15,
		"params": map[string]any{
			"origin": "BLR", "destination": "GOI",
			"depart_date": "2026-12-10", "return_date": "2026-12-15",
		},
	}
}

func TestMeta_ReturnsCurrentVersion(t *testing.T) {
	env := newTestEnv(t)
	resp := env.do(t, http.MethodGet, apiPrefix+"/meta", nil, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	meta := decodeData[v1.Meta](t, resp)
	if meta.CurrentVersion != "v1" {
		t.Errorf("CurrentVersion = %q, want v1", meta.CurrentVersion)
	}
	if len(meta.SupportedVersions) != 1 || meta.SupportedVersions[0] != "v1" {
		t.Errorf("SupportedVersions = %v, want [v1]", meta.SupportedVersions)
	}
}

func TestLogin_WrongPassword_Returns401(t *testing.T) {
	env := newTestEnv(t)
	resp := env.do(t, http.MethodPost, apiPrefix+"/auth/login", map[string]string{"password": "wrong"}, "")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

func TestLogin_CorrectPassword_SetsCookie(t *testing.T) {
	env := newTestEnv(t)
	cookie := env.login(t)
	if cookie == "" {
		t.Fatal("expected a non-empty session cookie")
	}
}

func TestWatches_RequireAuth(t *testing.T) {
	env := newTestEnv(t)
	resp := env.do(t, http.MethodGet, apiPrefix+"/watches", nil, "")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 without a session cookie", resp.StatusCode)
	}
}

func TestWatches_InvalidCookie_Rejected(t *testing.T) {
	env := newTestEnv(t)
	resp := env.do(t, http.MethodGet, apiPrefix+"/watches", nil, "not-a-real-token")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 with a bogus cookie", resp.StatusCode)
	}
}

func TestCreateWatch_ThenListAndGet(t *testing.T) {
	env := newTestEnv(t)
	cookie := env.login(t)

	createResp := env.do(t, http.MethodPost, apiPrefix+"/watches", flightWatchBody("Goa trip", "0 7 * * *"), cookie)
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("create status = %d", createResp.StatusCode)
	}
	created := decodeData[v1.Watch](t, createResp)
	if created.ID == "" {
		t.Fatal("expected a generated ID")
	}
	if created.Name != "Goa trip" || created.Kind != "flight_return" {
		t.Errorf("unexpected created watch: %+v", created)
	}

	listResp := env.do(t, http.MethodGet, apiPrefix+"/watches", nil, cookie)
	list := decodeData[[]v1.Watch](t, listResp)
	if len(list) != 1 {
		t.Fatalf("got %d watches, want 1", len(list))
	}

	getResp := env.do(t, http.MethodGet, apiPrefix+"/watches/"+created.ID, nil, cookie)
	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("get status = %d", getResp.StatusCode)
	}
	got := decodeData[v1.Watch](t, getResp)
	if got.ID != created.ID {
		t.Errorf("got ID %q, want %q", got.ID, created.ID)
	}
	// Never run yet — the staleness badge must fire rather than silently
	// looking identical to "price hasn't moved" (PLAN.md § Freshness).
	if !got.Stale {
		t.Error("expected a never-run watch to be flagged stale")
	}
}

func TestCreateWatch_InvalidKind_Returns400(t *testing.T) {
	env := newTestEnv(t)
	cookie := env.login(t)

	body := flightWatchBody("bad", "0 7 * * *")
	body["kind"] = "not_a_real_kind"
	resp := env.do(t, http.MethodPost, apiPrefix+"/watches", body, cookie)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	env2 := decodeEnvelope(t, resp)
	if env2.Error == nil {
		t.Error("expected the error field to be populated on a 400")
	}
}

func TestCreateWatch_InvalidCron_Returns400(t *testing.T) {
	env := newTestEnv(t)
	cookie := env.login(t)

	body := flightWatchBody("bad cron", "not a cron expression")
	resp := env.do(t, http.MethodPost, apiPrefix+"/watches", body, cookie)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestCreateWatch_ParamsMismatchedToKind_Returns400(t *testing.T) {
	env := newTestEnv(t)
	cookie := env.login(t)

	body := map[string]any{
		"name": "bad params", "kind": "flight_return", "cron_expr": "0 7 * * *", "timezone": "UTC",
		"params": map[string]any{"location": "Goa"}, // hotel-shaped params on a flight watch
	}
	resp := env.do(t, http.MethodPost, apiPrefix+"/watches", body, cookie)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestUpdateWatch_ChangesFields(t *testing.T) {
	env := newTestEnv(t)
	cookie := env.login(t)

	createResp := env.do(t, http.MethodPost, apiPrefix+"/watches", flightWatchBody("Original", "0 7 * * *"), cookie)
	created := decodeData[v1.Watch](t, createResp)

	body := flightWatchBody("Renamed", "0 8 * * *")
	updateResp := env.do(t, http.MethodPatch, apiPrefix+"/watches/"+created.ID, body, cookie)
	if updateResp.StatusCode != http.StatusOK {
		t.Fatalf("update status = %d", updateResp.StatusCode)
	}
	updated := decodeData[v1.Watch](t, updateResp)
	if updated.Name != "Renamed" || updated.CronExpr != "0 8 * * *" {
		t.Errorf("update did not apply: %+v", updated)
	}
}

func TestUpdateWatch_NotFound_Returns404(t *testing.T) {
	env := newTestEnv(t)
	cookie := env.login(t)

	resp := env.do(t, http.MethodPatch, apiPrefix+"/watches/does-not-exist", flightWatchBody("x", "0 7 * * *"), cookie)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

func TestDeleteWatch_ThenGet404(t *testing.T) {
	env := newTestEnv(t)
	cookie := env.login(t)

	createResp := env.do(t, http.MethodPost, apiPrefix+"/watches", flightWatchBody("to delete", "0 7 * * *"), cookie)
	created := decodeData[v1.Watch](t, createResp)

	delResp := env.do(t, http.MethodDelete, apiPrefix+"/watches/"+created.ID, nil, cookie)
	if delResp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete status = %d", delResp.StatusCode)
	}

	getResp := env.do(t, http.MethodGet, apiPrefix+"/watches/"+created.ID, nil, cookie)
	if getResp.StatusCode != http.StatusNotFound {
		t.Fatalf("get after delete status = %d, want 404", getResp.StatusCode)
	}
}

func TestRunWatchNow_NoProviderRegistered_Returns502(t *testing.T) {
	env := newTestEnv(t) // deliberately built with an empty provider registry
	cookie := env.login(t)

	createResp := env.do(t, http.MethodPost, apiPrefix+"/watches", flightWatchBody("no provider", "0 7 * * *"), cookie)
	created := decodeData[v1.Watch](t, createResp)

	runResp := env.do(t, http.MethodPost, apiPrefix+"/watches/"+created.ID+"/run", nil, cookie)
	if runResp.StatusCode != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502 when no provider is registered for the watch's kind", runResp.StatusCode)
	}
}

func TestGetWatch_IncludesPriceSummary_WhenSamplesExist(t *testing.T) {
	env := newTestEnv(t)
	cookie := env.login(t)

	createResp := env.do(t, http.MethodPost, apiPrefix+"/watches", flightWatchBody("with price", "0 7 * * *"), cookie)
	created := decodeData[v1.Watch](t, createResp)

	now := time.Now().UTC()
	env.repo.UpsertPriceSample(t.Context(), domain.PriceSample{
		WatchID: domain.WatchID(created.ID), SampleDate: dayOnly(now.AddDate(0, 0, -1)),
		MinMinor: 850000, MedianMinor: 850000, MaxMinor: 850000, NQuotes: 1, UpdatedAt: now.AddDate(0, 0, -1),
	})
	env.repo.UpsertPriceSample(t.Context(), domain.PriceSample{
		WatchID: domain.WatchID(created.ID), SampleDate: dayOnly(now),
		MinMinor: 800000, MedianMinor: 800000, MaxMinor: 800000, NQuotes: 1, UpdatedAt: now,
	})

	getResp := env.do(t, http.MethodGet, apiPrefix+"/watches/"+created.ID, nil, cookie)
	got := decodeData[v1.Watch](t, getResp)

	if got.Price == nil {
		t.Fatal("expected a price summary once samples exist")
	}
	if got.Price.PriceMinor != 800000 {
		t.Errorf("PriceMinor = %d, want 800000 (today's sample)", got.Price.PriceMinor)
	}
	if got.Price.DeltaPct == nil {
		t.Fatal("expected a delta once two consecutive days of samples exist")
	}
	wantDelta := (800000.0 - 850000.0) / 850000.0 * 100
	if diff := *got.Price.DeltaPct - wantDelta; diff > 0.0001 || diff < -0.0001 {
		t.Errorf("DeltaPct = %v, want %v", *got.Price.DeltaPct, wantDelta)
	}
	if got.LastUpdatedAt == nil {
		t.Error("expected LastUpdatedAt to be set from the newest sample")
	}
}

func dayOnly(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func TestGetHistory_EmptyForNewWatch(t *testing.T) {
	env := newTestEnv(t)
	cookie := env.login(t)

	createResp := env.do(t, http.MethodPost, apiPrefix+"/watches", flightWatchBody("history test", "0 7 * * *"), cookie)
	created := decodeData[v1.Watch](t, createResp)

	histResp := env.do(t, http.MethodGet, apiPrefix+"/watches/"+created.ID+"/history?range=30d", nil, cookie)
	if histResp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", histResp.StatusCode)
	}
	hist := decodeData[v1.History](t, histResp)
	if len(hist.Samples) != 0 {
		t.Errorf("expected no samples for a brand-new watch, got %d", len(hist.Samples))
	}
}
