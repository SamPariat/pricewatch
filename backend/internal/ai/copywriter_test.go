package ai

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/sampariat/prices-reminder/internal/analytics"
	"github.com/sampariat/prices-reminder/internal/domain"
	"github.com/sampariat/prices-reminder/internal/render"
)

func TestEnhance_NilLLM_ReturnsTemplateUnchanged(t *testing.T) {
	c := &Copywriter{}
	got := c.Enhance(context.Background(), domain.Watch{}, render.Analysis{}, "template text")
	if got != "template text" {
		t.Errorf("got %q, want template unchanged", got)
	}
}

func TestEnhance_NilCopywriter_ReturnsTemplateUnchanged(t *testing.T) {
	var c *Copywriter
	got := c.Enhance(context.Background(), domain.Watch{}, render.Analysis{}, "template text")
	if got != "template text" {
		t.Errorf("got %q, want template unchanged", got)
	}
}

func TestEnhance_CleanLine_AppendsItalicized(t *testing.T) {
	c := &Copywriter{LLM: &fakeLLM{text: "a solid day to book"}}
	got := c.Enhance(context.Background(), domain.Watch{Name: "BLR to GOI"}, render.Analysis{}, "template text")
	want := "template text\n<i>a solid day to book</i>"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestEnhance_LineWithDigit_RejectedFallsBackToTemplate(t *testing.T) {
	c := &Copywriter{LLM: &fakeLLM{text: "down 12 percent, book now"}}
	got := c.Enhance(context.Background(), domain.Watch{}, render.Analysis{}, "template text")
	if got != "template text" {
		t.Errorf("got %q, want template unchanged when the LLM output contains a digit", got)
	}
}

func TestEnhance_LLMError_ReturnsTemplateUnchanged(t *testing.T) {
	c := &Copywriter{LLM: &fakeLLM{err: errors.New("upstream down")}}
	got := c.Enhance(context.Background(), domain.Watch{}, render.Analysis{}, "template text")
	if got != "template text" {
		t.Errorf("got %q, want template unchanged on LLM error", got)
	}
}

func TestEnhance_EmptyLine_RejectedFallsBackToTemplate(t *testing.T) {
	c := &Copywriter{LLM: &fakeLLM{text: "   "}}
	got := c.Enhance(context.Background(), domain.Watch{}, render.Analysis{}, "template text")
	if got != "template text" {
		t.Errorf("got %q, want template unchanged for a blank LLM response", got)
	}
}

// TestBuildPrompt_NeverContainsComputedNumbers guards PLAN.md § AI features:
// "Never let the model emit prices or numbers... compute in Go, pass as
// context, constrain to phrasing only." buildPrompt is allowed to contain
// fixed, unchanging labels like "90 days" (the analytics window size), but
// must never leak this specific run's price, percentile, or delta figures
// into the prompt — those are exactly the numbers a careless prompt could
// tempt the model into echoing back, wrong.
func TestBuildPrompt_NeverContainsComputedNumbers(t *testing.T) {
	a := render.Analysis{
		PriceMinor:   1234500,
		Percentile:   87.3,
		PercentileOK: true,
		Delta:        analytics.Delta{Pct: -4.2, OK: true},
	}
	p := buildPrompt(domain.Watch{Name: "BLR to GOI"}, a)
	for _, computed := range []string{"1234500", "12345", "87.3", "87", "4.2", "-4.2"} {
		if strings.Contains(p.User, computed) {
			t.Errorf("prompt contains computed value %q, guardrail violated: %q", computed, p.User)
		}
	}
}

func TestSanitize(t *testing.T) {
	cases := []struct {
		in     string
		want   string
		wantOK bool
	}{
		{"a great day to book", "a great day to book", true},
		{`"quoted line"`, "quoted line", true},
		{"line one\nline two", "line one", true},
		{"", "", false},
		{"   ", "", false},
		{"down 12%", "", false},
		{strings.Repeat("x", 201), "", false},
	}
	for _, c := range cases {
		got, ok := sanitize(c.in)
		if ok != c.wantOK || got != c.want {
			t.Errorf("sanitize(%q) = (%q, %v), want (%q, %v)", c.in, got, ok, c.want, c.wantOK)
		}
	}
}
