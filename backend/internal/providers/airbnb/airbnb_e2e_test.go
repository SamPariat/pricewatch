//go:build chromedp_e2e

// This file needs a real Chromium/Chrome binary to run — gated behind
// the chromedp_e2e build tag so `go test ./...` doesn't fail on a
// machine/CI image without one. Run explicitly with:
//
//	go test -tags chromedp_e2e ./internal/providers/airbnb/...
package airbnb

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestScrapePriceText_StaticFixture serves a trimmed local HTML fixture
// standing in for a real Airbnb listing page's price markup, and
// confirms the scraper can find and read it. Real Airbnb markup drifts
// over time — this only proves the scrape mechanics work against a known
// shape, not that priceSelectors currently matches the live site.
func TestScrapePriceText_StaticFixture(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html><body><div data-testid="price-item">₹12,345 total</div></body></html>`))
	}))
	defer srv.Close()

	b, err := NewBrowser()
	if err != nil {
		t.Fatalf("NewBrowser: %v", err)
	}
	defer b.Close()

	ctx, cancel := b.NewTab(10 * time.Second)
	defer cancel()

	got, err := scrapePriceText(ctx, srv.URL)
	if err != nil {
		t.Fatalf("scrapePriceText: %v", err)
	}
	if got != "₹12,345 total" {
		t.Errorf("scrapePriceText = %q, want %q", got, "₹12,345 total")
	}
}
