// Package aviasales adapts Travelpayouts' Aviasales /v1/prices/calendar
// endpoint to domain.Provider. One HTTP contract serves both
// AssetFlightOneWay and AssetFlightReturn — the API returns one-way fares
// when return_date is omitted — so this package exposes two constructors
// sharing one implementation, differing only in Kind() and whether the
// return leg is requested.
//
// Schema confirmed against Travelpayouts' published API reference
// (travelpayouts.github.io/slate) as of 2026-08.
package aviasales

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"time"

	"github.com/sampariat/prices-reminder/internal/domain"
	"github.com/sampariat/prices-reminder/internal/logging"
	"github.com/sampariat/prices-reminder/internal/providers"
)

const defaultBaseURL = "https://api.travelpayouts.com/v1/prices/calendar"

type Provider struct {
	token      string
	currency   string
	kind       domain.AssetKind
	baseURL    string
	httpClient *http.Client
}

// NewOneWay and NewReturn share every field except Kind and whether the
// return leg is requested — see the package doc comment.
func NewOneWay(token, currency string) *Provider {
	return newProvider(token, currency, domain.AssetFlightOneWay)
}

func NewReturn(token, currency string) *Provider {
	return newProvider(token, currency, domain.AssetFlightReturn)
}

func newProvider(token, currency string, kind domain.AssetKind) *Provider {
	return &Provider{
		token:      token,
		currency:   currency,
		kind:       kind,
		baseURL:    defaultBaseURL,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

func (p *Provider) Kind() domain.AssetKind { return p.kind }

// calendarResponse mirrors the documented /v1/prices/calendar shape:
// {"success":true,"data":{"<date>":{...}}}.
type calendarResponse struct {
	Success bool                   `json:"success"`
	Data    map[string]calendarDay `json:"data"`
}

type calendarDay struct {
	Origin       string  `json:"origin"`
	Destination  string  `json:"destination"`
	Price        float64 `json:"price"`
	Transfers    int     `json:"transfers"`
	Airline      string  `json:"airline"`
	FlightNumber int     `json:"flight_number"`
	DepartureAt  string  `json:"departure_at"`
	ReturnAt     string  `json:"return_at"`
	ExpiresAt    string  `json:"expires_at"`
}

// Fetch requests a full month of calendar data for the watch's route,
// truncating params.DepartDate to its month (yyyy-mm). This deliberately
// returns more than the single date being watched: it's what backfills
// price_samples for the whole month on every routine fetch, not just at
// watch creation (see PLAN.md § Data model, "Backfill on creation").
func (p *Provider) Fetch(ctx context.Context, w domain.Watch) ([]domain.Quote, error) {
	params, err := w.DecodeFlightParams()
	if err != nil {
		return nil, err
	}
	if len(params.DepartDate) < 7 {
		return nil, fmt.Errorf("aviasales: watch %s: invalid depart_date %q", w.ID, params.DepartDate)
	}
	departMonth := params.DepartDate[:7] // yyyy-mm

	q := url.Values{}
	q.Set("origin", params.Origin)
	q.Set("destination", params.Destination)
	q.Set("depart_date", departMonth)
	q.Set("calendar_type", "departure_date")
	q.Set("currency", p.currency)
	q.Set("token", p.token)
	if p.kind == domain.AssetFlightReturn && params.ReturnDate != nil && len(*params.ReturnDate) >= 7 {
		q.Set("return_date", (*params.ReturnDate)[:7])
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"?"+q.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("aviasales: build request: %w", err)
	}

	start := time.Now()
	resp, err := p.httpClient.Do(req)
	if err != nil {
		// The token rides in the query string — never let it reach a log
		// line via the request URL.
		return nil, fmt.Errorf("aviasales: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("aviasales: read response: %w", err)
	}
	logging.HTTPResponse(ctx, "aviasales: calendar", resp.StatusCode, time.Since(start), respBody)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("aviasales: unexpected status %d", resp.StatusCode)
	}

	var body calendarResponse
	if err := json.Unmarshal(respBody, &body); err != nil {
		return nil, fmt.Errorf("aviasales: decode response: %w", err)
	}
	if !body.Success {
		return nil, fmt.Errorf("aviasales: upstream reported success=false")
	}

	quotes := make([]domain.Quote, 0, len(body.Data))
	now := time.Now().UTC()
	for date, day := range body.Data {
		returnDate := ""
		if p.kind == domain.AssetFlightReturn && day.ReturnAt != "" {
			if t, err := time.Parse(time.RFC3339, day.ReturnAt); err == nil {
				returnDate = t.Format("2006-01-02")
			}
		}
		quotes = append(quotes, domain.Quote{
			WatchID:    w.ID,
			FetchedAt:  now,
			Provider:   "aviasales",
			PriceMinor: toMinorUnits(day.Price),
			Currency:   p.currency,
			DepartDate: date,
			ReturnDate: returnDate,
			Stops:      day.Transfers,
			Carrier:    day.Airline,
			DeepLink:   deepLink(params.Origin, params.Destination, date, returnDate),
			Fingerprint: providers.Fingerprint(
				"aviasales", string(p.kind), params.Origin, params.Destination, date, returnDate, day.Airline,
			),
		})
	}
	return quotes, nil
}

func toMinorUnits(price float64) int64 {
	return int64(math.Round(price * 100))
}

func deepLink(origin, destination, departDate, returnDate string) string {
	// Best-effort search-results link, not yet carrying an affiliate
	// marker — see PLAN.md § Further suggestions, "Affiliate deep links".
	u := url.URL{
		Scheme: "https",
		Host:   "www.aviasales.com",
		Path:   "/search",
	}
	q := u.Query()
	q.Set("origin", origin)
	q.Set("destination", destination)
	q.Set("depart_date", departDate)
	if returnDate != "" {
		q.Set("return_date", returnDate)
	}
	u.RawQuery = q.Encode()
	return u.String()
}
