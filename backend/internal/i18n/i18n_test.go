package i18n

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestSharedLocaleFiles_BackendKeysMatch guards against the shared
// /locales/{en,hi}.json files drifting apart under this package's own
// "backend" key — the same risk the old hand-rolled catalog test caught,
// now checked directly against the JSON on disk rather than a Go map,
// since the JSON is the actual source of truth (also read directly by
// web/lib/i18n via next-intl).
func TestSharedLocaleFiles_BackendKeysMatch(t *testing.T) {
	en := backendKeys(t, "en.json")
	hi := backendKeys(t, "hi.json")
	for k := range en {
		if _, ok := hi[k]; !ok {
			t.Errorf("backend key %q exists in en.json but not hi.json", k)
		}
	}
	for k := range hi {
		if _, ok := en[k]; !ok {
			t.Errorf("backend key %q exists in hi.json but not en.json", k)
		}
	}
	if len(en) == 0 {
		t.Fatal("expected a non-empty backend key set — localesDir() may be resolving to the wrong directory")
	}
}

func backendKeys(t *testing.T, file string) map[string]string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(localesDir(), file))
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	backend, ok := doc["backend"].(map[string]any)
	if !ok {
		t.Fatalf("%s has no \"backend\" section", file)
	}
	flat := map[string]string{}
	flatten("", backend, flat)
	return flat
}

func TestParse(t *testing.T) {
	cases := map[string]Locale{
		"":                        EN,
		"hi":                      HI,
		"hi-IN":                   HI,
		"hi-IN,hi;q=0.9,en;q=0.8": HI,
		"en-US,en;q=0.9":          EN,
		"fr-FR,fr;q=0.9":          EN, // unsupported language falls back to EN
		"en;q=0.5,hi;q=0.9":       EN, // Parse takes the first listed tag, not the highest q-value
	}
	for in, want := range cases {
		if got := Parse(in); got != want {
			t.Errorf("Parse(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestValid(t *testing.T) {
	if !Valid("en") || !Valid("hi") {
		t.Error("expected en and hi to be valid locales")
	}
	if Valid("fr") || Valid("") {
		t.Error("expected an unsupported or empty locale to be invalid")
	}
}

func TestT_FallsBackToEnglishThenKey(t *testing.T) {
	enWant := T(EN, "meta.ok")
	if got := T(HI, "meta.ok"); got == "" || got == enWant {
		t.Errorf("T(HI, meta.ok) = %q, want the distinct HI translation", got)
	}
	if got := T(Locale("fr"), "meta.ok"); got != enWant {
		t.Errorf("T(fr, meta.ok) = %q, want the EN fallback %q", got, enWant)
	}
	if got := T(EN, "no.such.key"); got != "no.such.key" {
		t.Errorf("T for a missing key = %q, want the bare key back", got)
	}
}

func TestT_SubstitutesNamedPlaceholders(t *testing.T) {
	got := T(EN, "validation.invalid_kind", "Kind", "bogus_kind")
	want := `invalid kind "bogus_kind"`
	if got != want {
		t.Errorf("T with args = %q, want %q", got, want)
	}
}

func TestT_MultipleAndMissingPlaceholders(t *testing.T) {
	got := T(EN, "digest.all_time_low", "Price", "INR 8,412", "Date", "Aug 25")
	want := "All-time low: INR 8,412 on Aug 25"
	if got != want {
		t.Errorf("T with multiple args = %q, want %q", got, want)
	}

	// A placeholder with no matching kv entry is left as-is, not blanked
	// — see substitute's own doc comment on why that's the safer failure
	// mode.
	got2 := T(EN, "digest.all_time_low", "Price", "INR 8,412")
	if got2 != "All-time low: INR 8,412 on {Date}" {
		t.Errorf("T with a missing arg = %q, want the placeholder left intact", got2)
	}
}

func TestSubstitute_NoArgsReturnsStringUnchanged(t *testing.T) {
	got := substitute("plain string with {NoMatch}", nil)
	if got != "plain string with {NoMatch}" {
		t.Errorf("substitute with nil kv = %q, want the input unchanged", got)
	}
}

func TestWithAndFrom(t *testing.T) {
	ctx := With(context.Background(), HI)
	if got := From(ctx); got != HI {
		t.Errorf("From(With(ctx, HI)) = %q, want HI", got)
	}
	if got := From(context.Background()); got != EN {
		t.Errorf("From(bare context) = %q, want EN default", got)
	}
}
