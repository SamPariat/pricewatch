// Package hotellook adapts Travelpayouts' Hotellook /api/v2/cache.json
// endpoint to domain.Provider, serving domain.AssetHotel watches.
//
// Travelpayouts' public docs for this specific endpoint are thin — the
// field names below (hotelId, hotelName, priceFrom, priceAvg) follow the
// convention documented for Hotellook's related endpoints and used by
// several public client libraries, but were not confirmed against a live
// response as of writing. Verify against a real API call before trusting
// this in production; if field names differ, only calendarResponse-style
// parsing below needs to change, not the domain.Provider contract.
package hotellook

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"time"

	"github.com/SamPariat/pricewatch/internal/domain"
	"github.com/SamPariat/pricewatch/internal/httpclient"
	"github.com/SamPariat/pricewatch/internal/providers"
)

const defaultBaseURL = "https://engine.hotellook.com/api/v2/cache.json"

type Provider struct {
	token      string
	currency   string
	limit      int
	baseURL    string
	httpClient *httpclient.Client
}

func New(token, currency string) *Provider {
	return &Provider{
		token:      token,
		currency:   currency,
		limit:      20,
		baseURL:    defaultBaseURL,
		httpClient: httpclient.New(15 * time.Second),
	}
}

func (p *Provider) Kind() domain.AssetKind { return domain.AssetHotel }

type hotelResult struct {
	HotelID   int     `json:"hotelId"`
	HotelName string  `json:"hotelName"`
	PriceFrom float64 `json:"priceFrom"`
	PriceAvg  float64 `json:"priceAvg"`
	Stars     int     `json:"stars"`
}

func (p *Provider) Fetch(ctx context.Context, w domain.Watch) ([]domain.Quote, error) {
	params, err := w.DecodeHotelParams()
	if err != nil {
		return nil, err
	}

	q := url.Values{}
	q.Set("location", params.Location)
	q.Set("checkIn", params.CheckIn)
	q.Set("checkOut", params.CheckOut)
	q.Set("currency", p.currency)
	q.Set("limit", fmt.Sprintf("%d", p.limit))
	q.Set("token", p.token)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"?"+q.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("hotellook: build request: %w", err)
	}

	status, respBody, err := p.httpClient.Do(ctx, req, "hotellook: cache.json")
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("hotellook: unexpected status %d", status)
	}

	var results []hotelResult
	if err := json.Unmarshal(respBody, &results); err != nil {
		return nil, fmt.Errorf("hotellook: decode response: %w", err)
	}

	quotes := make([]domain.Quote, 0, len(results))
	now := time.Now().UTC()
	for _, h := range results {
		price := h.PriceAvg
		if price == 0 {
			price = h.PriceFrom
		}
		quotes = append(quotes, domain.Quote{
			WatchID:    w.ID,
			FetchedAt:  now,
			Provider:   "hotellook",
			PriceMinor: toMinorUnits(price),
			Currency:   p.currency,
			DepartDate: params.CheckIn,
			ReturnDate: params.CheckOut,
			Carrier:    h.HotelName,
			DeepLink:   deepLink(h.HotelID, params.CheckIn, params.CheckOut),
			Fingerprint: providers.Fingerprint(
				"hotellook", params.Location, params.CheckIn, params.CheckOut, fmt.Sprintf("%d", h.HotelID),
			),
		})
	}
	return quotes, nil
}

func toMinorUnits(price float64) int64 {
	return int64(math.Round(price * 100))
}

func deepLink(hotelID int, checkIn, checkOut string) string {
	// Best-effort link, not yet carrying an affiliate marker — see
	// PLAN.md § Further suggestions, "Affiliate deep links".
	u := url.URL{Scheme: "https", Host: "search.hotellook.com", Path: "/hotels"}
	q := u.Query()
	q.Set("hotelId", fmt.Sprintf("%d", hotelID))
	q.Set("checkIn", checkIn)
	q.Set("checkOut", checkOut)
	u.RawQuery = q.Encode()
	return u.String()
}
