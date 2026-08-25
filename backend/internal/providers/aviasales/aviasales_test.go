package aviasales

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/SamPariat/pricewatch/internal/domain"
)

func fixtureServer(t *testing.T, path string) *httptest.Server {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	}))
}

func returnWatch(t *testing.T) domain.Watch {
	t.Helper()
	returnDate := "2026-12-15"
	params, err := json.Marshal(domain.FlightParams{
		Origin: "BLR", Destination: "GOI", DepartDate: "2026-12-10", ReturnDate: &returnDate,
	})
	if err != nil {
		t.Fatal(err)
	}
	return domain.Watch{ID: "w1", Kind: domain.AssetFlightReturn, Params: params}
}

func TestFetch_Return_ParsesAllDates(t *testing.T) {
	srv := fixtureServer(t, "../testdata/aviasales_calendar_return.json")
	defer srv.Close()

	p := NewReturn("test-token", "inr")
	p.baseURL = srv.URL

	quotes, err := p.Fetch(context.Background(), returnWatch(t))
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(quotes) != 3 {
		t.Fatalf("got %d quotes, want 3", len(quotes))
	}
}

func TestFetch_Return_ConvertsToMinorUnits(t *testing.T) {
	srv := fixtureServer(t, "../testdata/aviasales_calendar_return.json")
	defer srv.Close()

	p := NewReturn("test-token", "inr")
	p.baseURL = srv.URL

	quotes, err := p.Fetch(context.Background(), returnWatch(t))
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	byDate := make(map[string]domain.Quote, len(quotes))
	for _, q := range quotes {
		byDate[q.DepartDate] = q
	}

	if got := byDate["2026-12-10"].PriceMinor; got != 841200 {
		t.Errorf("2026-12-10 PriceMinor = %d, want 841200 (8412.00)", got)
	}
	if got := byDate["2026-12-12"].PriceMinor; got != 794050 {
		t.Errorf("2026-12-12 PriceMinor = %d, want 794050 (7940.50, fractional price)", got)
	}
}

func TestFetch_Return_SetsReturnDate(t *testing.T) {
	srv := fixtureServer(t, "../testdata/aviasales_calendar_return.json")
	defer srv.Close()

	p := NewReturn("test-token", "inr")
	p.baseURL = srv.URL

	quotes, err := p.Fetch(context.Background(), returnWatch(t))
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	for _, q := range quotes {
		if q.ReturnDate == "" {
			t.Errorf("quote for %s: expected a return date on a Return-kind provider", q.DepartDate)
		}
	}
}

func TestFetch_OneWay_LeavesReturnDateEmpty(t *testing.T) {
	srv := fixtureServer(t, "../testdata/aviasales_calendar_return.json")
	defer srv.Close()

	p := NewOneWay("test-token", "inr")
	p.baseURL = srv.URL

	params, _ := json.Marshal(domain.FlightParams{Origin: "BLR", Destination: "GOI", DepartDate: "2026-12-10"})
	w := domain.Watch{ID: "w2", Kind: domain.AssetFlightOneWay, Params: params}

	quotes, err := p.Fetch(context.Background(), w)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	for _, q := range quotes {
		// The fixture's return_at is populated (as real calendar responses
		// often carry stale return-leg data even for one-way requests) —
		// a one-way provider must ignore it regardless.
		if q.ReturnDate != "" {
			t.Errorf("quote for %s: one-way provider must not set ReturnDate, got %q", q.DepartDate, q.ReturnDate)
		}
	}
}

func TestFetch_UpstreamFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"success":false,"data":{}}`))
	}))
	defer srv.Close()

	p := NewReturn("test-token", "inr")
	p.baseURL = srv.URL

	if _, err := p.Fetch(context.Background(), returnWatch(t)); err == nil {
		t.Fatal("expected an error when upstream reports success=false")
	}
}

func TestFetch_NonOKStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	p := NewReturn("test-token", "inr")
	p.baseURL = srv.URL

	if _, err := p.Fetch(context.Background(), returnWatch(t)); err == nil {
		t.Fatal("expected an error on a non-200 status")
	}
}

func TestFetch_FingerprintIsDeterministic(t *testing.T) {
	srv := fixtureServer(t, "../testdata/aviasales_calendar_return.json")
	defer srv.Close()

	p := NewReturn("test-token", "inr")
	p.baseURL = srv.URL

	first, err := p.Fetch(context.Background(), returnWatch(t))
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	second, err := p.Fetch(context.Background(), returnWatch(t))
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	firstByDate := make(map[string]string, len(first))
	for _, q := range first {
		firstByDate[q.DepartDate] = q.Fingerprint
	}
	for _, q := range second {
		if firstByDate[q.DepartDate] != q.Fingerprint {
			t.Errorf("fingerprint for %s changed between identical fetches: %q vs %q",
				q.DepartDate, firstByDate[q.DepartDate], q.Fingerprint)
		}
	}
}
