package airbnb

import "testing"

func TestParsePrice(t *testing.T) {
	cases := []struct {
		in         string
		wantMinor  int64
		wantCcy    string
		wantErrMsg string
	}{
		{"₹12,345 total", 1234500, "INR", ""},
		{"$1,234.50 total before taxes", 123450, "USD", ""},
		{"₹4,115 / night", 411500, "INR", ""},
		{"INR 12,345 total", 1234500, "INR", ""},
		{"no price here", 0, "", "no currency amount"},
	}
	for _, c := range cases {
		minor, ccy, err := parsePrice(c.in)
		if c.wantErrMsg != "" {
			if err == nil {
				t.Errorf("parsePrice(%q): expected error, got none", c.in)
			}
			continue
		}
		if err != nil {
			t.Fatalf("parsePrice(%q): unexpected error: %v", c.in, err)
		}
		if minor != c.wantMinor || ccy != c.wantCcy {
			t.Errorf("parsePrice(%q) = (%d, %q), want (%d, %q)", c.in, minor, ccy, c.wantMinor, c.wantCcy)
		}
	}
}

func TestWithDateParams(t *testing.T) {
	got, err := withDateParams("https://www.airbnb.com/rooms/12345", "2026-12-10", "2026-12-15", 2)
	if err != nil {
		t.Fatal(err)
	}
	want := "https://www.airbnb.com/rooms/12345?adults=2&check_in=2026-12-10&check_out=2026-12-15"
	if got != want {
		t.Errorf("withDateParams = %q, want %q", got, want)
	}
}
