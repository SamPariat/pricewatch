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

// field returns the value of the named field, or "" if not present —
// keeps assertions below reading like "what does the Change field say"
// rather than re-implementing a linear search in every test.
func field(e domain.Embed, name string) (string, bool) {
	for _, f := range e.Fields {
		if f.Name == name {
			return f.Value, true
		}
	}
	return "", false
}

func TestDigest_IncludesPriceAndTitle(t *testing.T) {
	w := flightWatch(t)
	e := Digest(w, Analysis{PriceMinor: 841200, Currency: "INR"})

	if e.Title != "BLR → GOI · return" {
		t.Errorf("Title = %q, want the derived route title", e.Title)
	}
	if !strings.Contains(e.Description, "INR 8,412") {
		t.Errorf("missing formatted price in Description: %q", e.Description)
	}
}

// TestDigest_ZeroValueLocale_DefaultsToEnglish guards the compatibility
// promise Analysis.Locale's doc comment makes: every call site written
// before locale existed (and every render_test.go case above that never
// sets it) must keep rendering in English, not fall over or render blank
// keys.
func TestDigest_ZeroValueLocale_DefaultsToEnglish(t *testing.T) {
	w := flightWatch(t)
	e := Digest(w, Analysis{PriceMinor: 841200, Currency: "INR", Delta: analytics.Delta{Pct: -4.6, OK: true}})
	change, ok := field(e, "Change")
	if !ok || !strings.Contains(change, "down") {
		t.Errorf("expected English 'down' with a zero-value Locale, got Change=%q ok=%v", change, ok)
	}
}

// TestDigest_Hindi_TranslatesLabelsButNotTheNumbers is the end-to-end
// check that Locale actually reaches every templated piece — prices,
// dates, and percentages must stay numeric regardless of language (see
// PLAN.md § AI features' "never let anything but Go produce a number"
// rule, which applies just as much to i18n templates as to the LLM).
func TestDigest_Hindi_TranslatesLabelsButNotTheNumbers(t *testing.T) {
	w := flightWatch(t)
	e := Digest(w, Analysis{
		PriceMinor: 841200, Currency: "INR", Locale: i18n.HI,
		Delta:      analytics.Delta{AbsMinor: -50000, Pct: -4.6, OK: true},
		Percentile: 87, PercentileOK: true,
	})
	change, _ := field(e, "बदलाव")
	if !strings.Contains(change, "नीचे") {
		t.Errorf("expected the Hindi 'down' word in the Change field, got: %q", change)
	}
	percentile, _ := field(e, "90-दिन पर्सेंटाइल")
	if percentile != "पिछले 90 दिनों के 87% से सस्ता" {
		t.Errorf("percentile field = %q, want the Hindi percentile line", percentile)
	}
	if !strings.Contains(e.Description, "INR 8,412") {
		t.Errorf("expected the price to stay numeric/untranslated, got: %q", e.Description)
	}
	if !strings.Contains(e.Title, "· राउंड ट्रिप") {
		t.Errorf("expected the Hindi round-trip suffix in the title, got: %q", e.Title)
	}
}

func TestDigest_DownDelta_ShowsDownArrowAndGreenColor(t *testing.T) {
	w := flightWatch(t)
	e := Digest(w, Analysis{PriceMinor: 841200, Currency: "INR", Delta: analytics.Delta{AbsMinor: -50000, Pct: -4.6, OK: true}})

	change, ok := field(e, "Change")
	if !ok {
		t.Fatal("expected a Change field")
	}
	if !strings.Contains(change, "▼ 4.6%") {
		t.Errorf("expected down arrow with positive magnitude, got: %q", change)
	}
	if !strings.Contains(change, "down") {
		t.Errorf("expected the word 'down', got: %q", change)
	}
	if e.Color != colorDown {
		t.Errorf("Color = %#x, want colorDown (%#x) for a price drop", e.Color, colorDown)
	}
}

func TestDigest_UpDelta_ShowsUpArrowAndRedColor(t *testing.T) {
	w := flightWatch(t)
	e := Digest(w, Analysis{PriceMinor: 900000, Currency: "INR", Delta: analytics.Delta{AbsMinor: 50000, Pct: 12.0, OK: true}})

	change, ok := field(e, "Change")
	if !ok {
		t.Fatal("expected a Change field")
	}
	if !strings.Contains(change, "▲ 12.0%") {
		t.Errorf("expected up arrow, got: %q", change)
	}
	if !strings.Contains(change, "up") {
		t.Errorf("expected the word 'up', got: %q", change)
	}
	if e.Color != colorUp {
		t.Errorf("Color = %#x, want colorUp (%#x) for a price rise", e.Color, colorUp)
	}
}

func TestDigest_NoDelta_OmitsChangeFieldAndUsesNeutralColor(t *testing.T) {
	w := flightWatch(t)
	e := Digest(w, Analysis{PriceMinor: 841200, Currency: "INR"}) // Delta.OK is false (zero value)

	if _, ok := field(e, "Change"); ok {
		t.Errorf("expected no Change field when Delta.OK is false, got fields: %+v", e.Fields)
	}
	if e.Color != colorNeutral {
		t.Errorf("Color = %#x, want colorNeutral (%#x) with no delta", e.Color, colorNeutral)
	}
}

func TestDigest_Percentile_Renders(t *testing.T) {
	w := flightWatch(t)
	e := Digest(w, Analysis{PriceMinor: 841200, Currency: "INR", Percentile: 87, PercentileOK: true})

	got, ok := field(e, "90-day percentile")
	if !ok || got != "Cheaper than 87% of the last 90 days" {
		t.Errorf("percentile field = %q, ok=%v, want the full sentence", got, ok)
	}
}

func TestDigest_AllTimeLow_OnlyRendersWhenBelowCurrentPrice(t *testing.T) {
	w := flightWatch(t)

	below := Digest(w, Analysis{
		PriceMinor: 900000, Currency: "INR",
		AllTimeLow: analytics.AllTimeLow{PriceMinor: 794000, Date: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), OK: true},
	})
	if _, ok := field(below, "All-time low"); !ok {
		t.Errorf("expected an All-time low field when current price is above it, got fields: %+v", below.Fields)
	}

	// Current price IS the all-time low — the field would be redundant
	// with the headline price, so it must not render.
	atLow := Digest(w, Analysis{
		PriceMinor: 794000, Currency: "INR",
		AllTimeLow: analytics.AllTimeLow{PriceMinor: 794000, Date: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), OK: true},
	})
	if _, ok := field(atLow, "All-time low"); ok {
		t.Errorf("did not expect an All-time low field when current price already is the low, got fields: %+v", atLow.Fields)
	}
}

func TestDigest_NearestMatch_NotesTheShiftedDatesInFooter(t *testing.T) {
	w := flightWatch(t)
	e := Digest(w, Analysis{
		PriceMinor: 841200, Currency: "INR",
		NearestMatch: DateRange{Depart: "2026-12-24", Return: "2026-12-26"},
	})

	if !strings.Contains(e.Footer, "No exact match") {
		t.Errorf("expected a footer note about the fallback match, got: %q", e.Footer)
	}
	if !strings.Contains(e.Footer, "Dec 24") || !strings.Contains(e.Footer, "Dec 26") {
		t.Errorf("expected the actual matched dates in the footer, got: %q", e.Footer)
	}
}

func TestDigest_ExactMatch_OmitsFooter(t *testing.T) {
	w := flightWatch(t)
	e := Digest(w, Analysis{PriceMinor: 841200, Currency: "INR"})

	if e.Footer != "" {
		t.Errorf("did not expect a footer for the zero-value NearestMatch, got: %q", e.Footer)
	}
}

func TestDigest_NearestMatch_OneWay_OmitsArrow(t *testing.T) {
	w := flightWatch(t)
	e := Digest(w, Analysis{
		PriceMinor: 841200, Currency: "INR",
		NearestMatch: DateRange{Depart: "2026-12-24"}, // no Return — one-way
	})

	if !strings.Contains(e.Footer, "Dec 24") {
		t.Errorf("footer = %q, want the matched depart date", e.Footer)
	}
	if strings.Contains(e.Footer, "→") {
		t.Errorf("footer = %q, did not expect an arrow with no return date", e.Footer)
	}
}

func TestDigest_NamedWatchUsesNameOverDerivedTitle(t *testing.T) {
	w := flightWatch(t)
	w.Name = "Christmas trip"
	e := Digest(w, Analysis{PriceMinor: 841200, Currency: "INR"})

	if e.Title != "Christmas trip" {
		t.Errorf("Title = %q, want watch.Name to take priority", e.Title)
	}
}

func TestCombineDigest_Pluralizes(t *testing.T) {
	one := CombineDigest([]domain.Embed{{Title: "section a"}}, "", i18n.EN)
	if !strings.Contains(one.Content, "1 watch") || strings.Contains(one.Content, "1 watches") {
		t.Errorf("expected singular 'watch', got: %q", one.Content)
	}

	two := CombineDigest([]domain.Embed{{Title: "section a"}, {Title: "section b"}}, "", i18n.EN)
	if !strings.Contains(two.Content, "2 watches") {
		t.Errorf("expected plural 'watches', got: %q", two.Content)
	}
	if len(two.Embeds) != 2 {
		t.Errorf("expected both embeds present, got %d", len(two.Embeds))
	}
}

// TestCombineDigest_Hindi_NoPluralSuffix guards against reintroducing an
// English-only "es" suffix hack — Hindi doesn't inflect the noun for
// count the way English does, so digest.header_plural and
// digest.header_singular are deliberately identical strings in the
// shared locales/hi.json catalog.
func TestCombineDigest_Hindi_NoPluralSuffix(t *testing.T) {
	two := CombineDigest([]domain.Embed{{Title: "a"}, {Title: "b"}}, "", i18n.HI)
	if !strings.Contains(two.Content, "2 वॉच") {
		t.Errorf("expected the Hindi header with count 2, got: %q", two.Content)
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
