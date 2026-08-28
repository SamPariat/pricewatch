package airbnb

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/chromedp/chromedp"

	"github.com/SamPariat/pricewatch/internal/domain"
	"github.com/SamPariat/pricewatch/internal/logging"
	"github.com/SamPariat/pricewatch/internal/providers"
)

// fetchTimeout bounds one scrape attempt — a full page render plus price
// hydration is slower than a JSON API call, but still needs a hard cap
// so a hung/blocked page can't wedge a scheduler fire indefinitely.
const fetchTimeout = 30 * time.Second

// priceSelectors are candidate CSS selectors for Airbnb's rendered price
// element, tried in order. book-it-default is the sidebar's "book it"
// widget — confirmed against a live listing page (2026-08-28): its text
// starts with the dated total, e.g. "₹61,560 Show price breakdown for
// 5 nights ₹61,560 for 5 nights...", and parsePrice takes the first
// amount it finds. Airbnb's markup/class names are otherwise unofficial
// and unstable (the atomic CSS classes churn constantly; data-testid
// values are comparatively durable but not guaranteed) — this WILL need
// updating as their frontend changes, an ongoing maintenance cost, not a
// one-time build cost.
var priceSelectors = []string{
	`[data-testid="book-it-default"]`,
	`[data-testid="price-item"]`,
	`[data-plugin-id="PRICE_LOCKUP"]`,
}

type Provider struct {
	browser *Browser
}

func New(browser *Browser) *Provider {
	return &Provider{browser: browser}
}

func (p *Provider) Kind() domain.AssetKind { return domain.AssetLodgingAirbnb }

func (p *Provider) Fetch(ctx context.Context, w domain.Watch) ([]domain.Quote, error) {
	params, err := w.DecodeLodgingParams()
	if err != nil {
		return nil, err
	}

	target, err := withDateParams(params.URL, params.CheckIn, params.CheckOut, params.Guests)
	if err != nil {
		return nil, fmt.Errorf("airbnb: build listing url: %w", err)
	}

	tabCtx, cancel := p.browser.NewTab(fetchTimeout)
	defer cancel()

	start := time.Now()
	raw, err := scrapePriceText(tabCtx, target)
	ok := err == nil
	logging.From(ctx).Info().Str("watch_id", string(w.ID)).Str("url", params.URL).
		Dur("elapsed_ms", time.Since(start)).Bool("ok", ok).Msg("airbnb: scrape attempt")
	if err != nil {
		return nil, fmt.Errorf("airbnb: scrape %s: %w", params.URL, err)
	}

	priceMinor, currency, err := parsePrice(raw)
	if err != nil {
		return nil, fmt.Errorf("airbnb: parse price %q: %w", raw, err)
	}

	now := time.Now().UTC()
	return []domain.Quote{{
		WatchID:     w.ID,
		FetchedAt:   now,
		Provider:    "airbnb",
		PriceMinor:  priceMinor,
		Currency:    currency,
		DepartDate:  params.CheckIn,
		ReturnDate:  params.CheckOut,
		DeepLink:    params.URL,
		Fingerprint: providers.Fingerprint("airbnb", params.URL, params.CheckIn, params.CheckOut),
	}}, nil
}

// scrapePriceText navigates to the listing and returns the first
// candidate price element's text — trying each of priceSelectors in
// turn, since Airbnb doesn't consistently expose one stable selector
// across listing types.
//
// The price element itself renders near-empty at Page's "load" event
// (confirmed 2026-08-28: a DOM dump taken right at load contained no
// currency amounts at all) — its text only hydrates a few seconds later
// via a client-side fetch. chromedp.WaitVisible only confirms the
// element exists, not that it has content yet, so a Text() call right
// after it is a race that can return an empty or partial string. Polling
// the element's own innerText until it's non-empty avoids that.
func scrapePriceText(ctx context.Context, target string) (string, error) {
	if err := chromedp.Run(ctx, chromedp.Navigate(target)); err != nil {
		return "", fmt.Errorf("navigate: %w", err)
	}

	var lastErr error
	for _, sel := range priceSelectors {
		var text string
		// The predicate returns '' (falsy, Poll keeps waiting) until the
		// element's text actually contains a currency amount — its
		// innerText is non-empty well before that, briefly holding a
		// "loading" placeholder while the price itself is still an
		// in-flight fetch.
		expr := fmt.Sprintf(`(() => {
			const t = document.querySelector(%q)?.innerText || '';
			return /[₹$€£]\s?\d/.test(t) ? t : '';
		})()`, sel)
		err := chromedp.Run(ctx, chromedp.Poll(expr, &text, chromedp.WithPollingInterval(500*time.Millisecond)))
		if err == nil && strings.TrimSpace(text) != "" {
			return text, nil
		}
		lastErr = err
	}
	return "", fmt.Errorf("no price element matched any known selector: %w", lastErr)
}

// withDateParams appends check-in/check-out/guest query params Airbnb's
// listing page reads to render a dated total price instead of "Add
// dates".
func withDateParams(raw, checkIn, checkOut string, guests int) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("check_in", checkIn)
	q.Set("check_out", checkOut)
	if guests > 0 {
		q.Set("adults", strconv.Itoa(guests))
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// priceAmount matches a currency symbol/code followed by a
// possibly-comma-grouped, possibly-decimal number — e.g. "₹12,345",
// "$1,234.50", "INR 12,345".
var priceAmount = regexp.MustCompile(`([₹$€£]|[A-Z]{3}\s?)\s?([\d,]+(?:\.\d+)?)`)

var currencySymbols = map[string]string{
	"₹": "INR", "$": "USD", "€": "EUR", "£": "GBP",
}

// parsePrice extracts a minor-unit price and ISO currency code from
// Airbnb's rendered price text, which varies in shape ("₹12,345 total",
// "₹4,115 / night", "$220 total before taxes").
func parsePrice(text string) (minor int64, currency string, err error) {
	m := priceAmount.FindStringSubmatch(text)
	if m == nil {
		return 0, "", fmt.Errorf("no currency amount found in %q", text)
	}
	sym := strings.TrimSpace(m[1])
	currency = currencySymbols[sym]
	if currency == "" {
		currency = sym // already a 3-letter code, e.g. "INR"
	}

	numStr := strings.ReplaceAll(m[2], ",", "")
	amount, perr := strconv.ParseFloat(numStr, 64)
	if perr != nil {
		return 0, "", fmt.Errorf("parse amount %q: %w", numStr, perr)
	}
	return int64(amount*100 + 0.5), currency, nil
}
