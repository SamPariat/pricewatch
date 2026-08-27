package render

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/SamPariat/pricewatch/internal/analytics"
	"github.com/SamPariat/pricewatch/internal/domain"
	"github.com/SamPariat/pricewatch/internal/i18n"
)

func flightWatch(t *testing.T) domain.Watch {
	t.Helper()
	ret := "2026-12-15"
	params, err := json.Marshal(domain.FlightParams{Origin: "BLR", Destination: "GOI", DepartDate: "2026-12-10", ReturnDate: &ret})
	if err != nil {
		t.Fatal(err)
	}
	return domain.Watch{ID: "w1", Kind: domain.AssetFlightReturn, Params: params}
}

func TestDigest_IncludesPriceAndTitle(t *testing.T) {
	w := flightWatch(t)
	text := Digest(w, Analysis{PriceMinor: 841200, Currency: "INR"})

	if !strings.Contains(text, "BLR → GOI") {
		t.Errorf("missing route in title: %q", text)
	}
	if !strings.Contains(text, "INR 8,412") {
		t.Errorf("missing formatted price: %q", text)
	}
}

// TestDigest_ZeroValueLocale_DefaultsToEnglish guards the compatibility
// promise Analysis.Locale's doc comment makes: every call site written
// before locale existed (and every render_test.go case above that never
// sets it) must keep rendering in English, not fall over or render blank
// keys.
func TestDigest_ZeroValueLocale_DefaultsToEnglish(t *testing.T) {
	w := flightWatch(t)
	text := Digest(w, Analysis{PriceMinor: 841200, Currency: "INR", Delta: analytics.Delta{Pct: -4.6, OK: true}})
	if !strings.Contains(text, "down") {
		t.Errorf("expected English 'down' with a zero-value Locale, got: %q", text)
	}
}

// TestDigest_Hindi_TranslatesLabelsButNotTheNumbers is the end-to-end
// check that Locale actually reaches every templated piece — prices,
// dates, and percentages must stay numeric regardless of language (see
// PLAN.md § AI features' "never let anything but Go produce a number"
// rule, which applies just as much to i18n templates as to the LLM).
func TestDigest_Hindi_TranslatesLabelsButNotTheNumbers(t *testing.T) {
	w := flightWatch(t)
	text := Digest(w, Analysis{
		PriceMinor: 841200, Currency: "INR", Locale: i18n.HI,
		Delta:      analytics.Delta{AbsMinor: -50000, Pct: -4.6, OK: true},
		Percentile: 87, PercentileOK: true,
	})
	if !strings.Contains(text, "नीचे") {
		t.Errorf("expected the Hindi 'down' word, got: %q", text)
	}
	if !strings.Contains(text, "पिछले 90 दिनों के 87% से सस्ता") {
		t.Errorf("expected the Hindi percentile line, got: %q", text)
	}
	if !strings.Contains(text, "INR 8,412") {
		t.Errorf("expected the price to stay numeric/untranslated, got: %q", text)
	}
	if !strings.Contains(text, "· राउंड ट्रिप") {
		t.Errorf("expected the Hindi round-trip suffix in the title, got: %q", text)
	}
}

func TestDigest_DownDelta_ShowsDownArrow(t *testing.T) {
	w := flightWatch(t)
	text := Digest(w, Analysis{PriceMinor: 841200, Currency: "INR", Delta: analytics.Delta{AbsMinor: -50000, Pct: -4.6, OK: true}})

	if !strings.Contains(text, "▼ 4.6%") {
		t.Errorf("expected down arrow with positive magnitude, got: %q", text)
	}
	if !strings.Contains(text, "down") {
		t.Errorf("expected the word 'down', got: %q", text)
	}
}

func TestDigest_UpDelta_ShowsUpArrow(t *testing.T) {
	w := flightWatch(t)
	text := Digest(w, Analysis{PriceMinor: 900000, Currency: "INR", Delta: analytics.Delta{AbsMinor: 50000, Pct: 12.0, OK: true}})

	if !strings.Contains(text, "▲ 12.0%") {
		t.Errorf("expected up arrow, got: %q", text)
	}
	if !strings.Contains(text, "up") {
		t.Errorf("expected the word 'up', got: %q", text)
	}
}

func TestDigest_NoDelta_OmitsDeltaLine(t *testing.T) {
	w := flightWatch(t)
	text := Digest(w, Analysis{PriceMinor: 841200, Currency: "INR"}) // Delta.OK is false (zero value)

	if strings.Contains(text, "vs yesterday") {
		t.Errorf("expected no delta text when Delta.OK is false, got: %q", text)
	}
}

func TestDigest_Percentile_Renders(t *testing.T) {
	w := flightWatch(t)
	text := Digest(w, Analysis{PriceMinor: 841200, Currency: "INR", Percentile: 87, PercentileOK: true})

	if !strings.Contains(text, "Cheaper than 87% of the last 90 days") {
		t.Errorf("missing percentile line: %q", text)
	}
}

func TestDigest_AllTimeLow_OnlyRendersWhenBelowCurrentPrice(t *testing.T) {
	w := flightWatch(t)

	below := Digest(w, Analysis{
		PriceMinor: 900000, Currency: "INR",
		AllTimeLow: analytics.AllTimeLow{PriceMinor: 794000, Date: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), OK: true},
	})
	if !strings.Contains(below, "All-time low") {
		t.Errorf("expected all-time-low line when current price is above it: %q", below)
	}

	// Current price IS the all-time low — the line would be redundant
	// with the headline price, so it must not render.
	atLow := Digest(w, Analysis{
		PriceMinor: 794000, Currency: "INR",
		AllTimeLow: analytics.AllTimeLow{PriceMinor: 794000, Date: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), OK: true},
	})
	if strings.Contains(atLow, "All-time low") {
		t.Errorf("did not expect an all-time-low line when current price already is the low: %q", atLow)
	}
}

func TestDigest_NearestMatch_NotesTheShiftedDates(t *testing.T) {
	w := flightWatch(t)
	text := Digest(w, Analysis{
		PriceMinor: 841200, Currency: "INR",
		NearestMatch: DateRange{Depart: "2026-12-24", Return: "2026-12-26"},
	})

	if !strings.Contains(text, "No exact match") {
		t.Errorf("expected a note about the fallback match, got: %q", text)
	}
	if !strings.Contains(text, "Dec 24") || !strings.Contains(text, "Dec 26") {
		t.Errorf("expected the actual matched dates in the note, got: %q", text)
	}
}

func TestDigest_ExactMatch_OmitsNearestMatchNote(t *testing.T) {
	w := flightWatch(t)
	text := Digest(w, Analysis{PriceMinor: 841200, Currency: "INR"})

	if strings.Contains(text, "No exact match") {
		t.Errorf("did not expect a nearest-match note for the zero value, got: %q", text)
	}
}

func TestDigest_NearestMatch_OneWay_OmitsArrow(t *testing.T) {
	w := flightWatch(t)
	text := Digest(w, Analysis{
		PriceMinor: 841200, Currency: "INR",
		NearestMatch: DateRange{Depart: "2026-12-24"}, // no Return — one-way
	})

	var noteLine string
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(line, "No exact match") {
			noteLine = line
		}
	}
	if noteLine == "" {
		t.Fatalf("expected a nearest-match note line, got: %q", text)
	}
	if !strings.Contains(noteLine, "Dec 24") {
		t.Errorf("note line = %q, want the matched depart date", noteLine)
	}
	if strings.Contains(noteLine, "→") {
		t.Errorf("note line = %q, did not expect an arrow with no return date", noteLine)
	}
}

func TestDigest_HotelUsesLocationTitle(t *testing.T) {
	params, err := json.Marshal(domain.HotelParams{Location: "Goa", CheckIn: "2026-12-10", CheckOut: "2026-12-15"})
	if err != nil {
		t.Fatal(err)
	}
	w := domain.Watch{ID: "w2", Kind: domain.AssetHotel, Params: params}
	text := Digest(w, Analysis{PriceMinor: 920000, Currency: "INR"})

	if !strings.Contains(text, "Goa") {
		t.Errorf("expected hotel watch title to use location: %q", text)
	}
}

func TestDigest_NamedWatchUsesNameOverDerivedTitle(t *testing.T) {
	w := flightWatch(t)
	w.Name = "Christmas trip"
	text := Digest(w, Analysis{PriceMinor: 841200, Currency: "INR"})

	if !strings.Contains(text, "Christmas trip") {
		t.Errorf("expected watch.Name to take priority, got: %q", text)
	}
	if strings.Contains(text, "BLR → GOI") {
		t.Errorf("did not expect derived route title when Name is set: %q", text)
	}
}

func TestCombineDigest_Pluralizes(t *testing.T) {
	one := CombineDigest([]string{"section a"}, i18n.EN)
	if !strings.Contains(one, "1 watch") || strings.Contains(one, "1 watches") {
		t.Errorf("expected singular 'watch', got: %q", one)
	}

	two := CombineDigest([]string{"section a", "section b"}, i18n.EN)
	if !strings.Contains(two, "2 watches") {
		t.Errorf("expected plural 'watches', got: %q", two)
	}
	if !strings.Contains(two, "section a") || !strings.Contains(two, "section b") {
		t.Errorf("expected both sections present, got: %q", two)
	}
}

// TestCombineDigest_Hindi_NoPluralSuffix guards against reintroducing an
// English-only "es" suffix hack — Hindi doesn't inflect the noun for
// count the way English does, so digest.header_plural and
// digest.header_singular are deliberately identical strings in the HI
// catalog (see internal/i18n/messages.go).
func TestCombineDigest_Hindi_NoPluralSuffix(t *testing.T) {
	two := CombineDigest([]string{"section a", "section b"}, i18n.HI)
	if !strings.Contains(two, "2 वॉच") {
		t.Errorf("expected the Hindi header with count 2, got: %q", two)
	}
}

func TestGroupThousands(t *testing.T) {
	cases := map[int64]string{
		0:       "0",
		8:       "8",
		841:     "841",
		8412:    "8,412",
		1234567: "1,234,567",
		-8412:   "-8,412",
	}
	for in, want := range cases {
		if got := groupThousands(in); got != want {
			t.Errorf("groupThousands(%d) = %q, want %q", in, got, want)
		}
	}
}
