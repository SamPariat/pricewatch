package hotellook

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/sampariat/prices-reminder/internal/domain"
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

func hotelWatch(t *testing.T) domain.Watch {
	t.Helper()
	params, err := json.Marshal(domain.HotelParams{Location: "Goa", CheckIn: "2026-12-10", CheckOut: "2026-12-15"})
	if err != nil {
		t.Fatal(err)
	}
	return domain.Watch{ID: "w1", Kind: domain.AssetHotel, Params: params}
}

func TestFetch_ParsesAllHotels(t *testing.T) {
	srv := fixtureServer(t, "../testdata/hotellook_cache.json")
	defer srv.Close()

	p := New("test-token", "inr")
	p.baseURL = srv.URL

	quotes, err := p.Fetch(context.Background(), hotelWatch(t))
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(quotes) != 2 {
		t.Fatalf("got %d quotes, want 2", len(quotes))
	}
}

func TestFetch_PrefersAvgOverFrom(t *testing.T) {
	srv := fixtureServer(t, "../testdata/hotellook_cache.json")
	defer srv.Close()

	p := New("test-token", "inr")
	p.baseURL = srv.URL

	quotes, err := p.Fetch(context.Background(), hotelWatch(t))
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	byName := make(map[string]domain.Quote, len(quotes))
	for _, q := range quotes {
		byName[q.Carrier] = q // hotel name is carried in the Carrier field for hotel quotes
	}

	if got := byName["Taj Exotica Resort & Spa, Goa"].PriceMinor; got != 920000 {
		t.Errorf("PriceMinor = %d, want 920000 (priceAvg 9200.00, not priceFrom 8800)", got)
	}
	if got := byName["The Leela Goa"].PriceMinor; got != 790050 {
		t.Errorf("PriceMinor = %d, want 790050 (priceAvg 7900.50, fractional)", got)
	}
}

func TestFetch_SetsCheckInCheckOutDates(t *testing.T) {
	srv := fixtureServer(t, "../testdata/hotellook_cache.json")
	defer srv.Close()

	p := New("test-token", "inr")
	p.baseURL = srv.URL

	quotes, err := p.Fetch(context.Background(), hotelWatch(t))
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	for _, q := range quotes {
		if q.DepartDate != "2026-12-10" || q.ReturnDate != "2026-12-15" {
			t.Errorf("quote for hotel %q: dates = %s..%s, want 2026-12-10..2026-12-15",
				q.Carrier, q.DepartDate, q.ReturnDate)
		}
	}
}

func TestFetch_NonOKStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	p := New("test-token", "inr")
	p.baseURL = srv.URL

	if _, err := p.Fetch(context.Background(), hotelWatch(t)); err == nil {
		t.Fatal("expected an error on a non-200 status")
	}
}
