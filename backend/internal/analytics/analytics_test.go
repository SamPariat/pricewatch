package analytics

import (
	"testing"
	"time"

	"github.com/SamPariat/pricewatch/internal/domain"
)

var base = time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC)

func day(daysAgo int) time.Time {
	return base.AddDate(0, 0, -daysAgo)
}

func sample(daysAgo int, min, med, max int64) domain.PriceSample {
	return domain.PriceSample{SampleDate: day(daysAgo), MinMinor: min, MedianMinor: med, MaxMinor: max}
}

func TestDeltaVsYesterday(t *testing.T) {
	samples := []domain.PriceSample{
		sample(1, 9800, 10000, 10500),
		sample(0, 9300, 9500, 10100),
	}
	got := DeltaVsYesterday(samples, base)
	if !got.OK {
		t.Fatalf("expected OK, got %+v", got)
	}
	if got.AbsMinor != -500 {
		t.Errorf("AbsMinor = %d, want -500", got.AbsMinor)
	}
	if got.Pct != -5.0 {
		t.Errorf("Pct = %v, want -5.0", got.Pct)
	}
}

func TestDeltaVsYesterday_NoPriorDay(t *testing.T) {
	samples := []domain.PriceSample{sample(0, 9300, 9500, 10100)}
	got := DeltaVsYesterday(samples, base)
	if got.OK {
		t.Fatalf("expected OK=false with no prior-day sample, got %+v", got)
	}
}

func TestDeltaVsYesterday_Empty(t *testing.T) {
	got := DeltaVsYesterday(nil, base)
	if got.OK {
		t.Fatalf("expected OK=false with no samples, got %+v", got)
	}
}

func TestRollingWindow(t *testing.T) {
	samples := []domain.PriceSample{
		sample(4, 8000, 8500, 9000),
		sample(3, 8200, 8600, 9100),
		sample(2, 7900, 8400, 8900),
		sample(1, 8300, 8700, 9200),
		sample(0, 8100, 8550, 9050),
	}
	got := RollingWindow(samples, 5, base)
	if !got.OK {
		t.Fatalf("expected OK, got %+v", got)
	}
	if got.LowMinor != 7900 {
		t.Errorf("LowMinor = %d, want 7900", got.LowMinor)
	}
	if got.MedianMinor != 8550 {
		t.Errorf("MedianMinor = %d, want 8550", got.MedianMinor)
	}
	if got.HighMinor != 9200 {
		t.Errorf("HighMinor = %d, want 9200", got.HighMinor)
	}
	if got.N != 5 {
		t.Errorf("N = %d, want 5", got.N)
	}
}

func TestRollingWindow_OutsideWindowExcluded(t *testing.T) {
	samples := []domain.PriceSample{
		sample(30, 1, 1, 1), // far outside a 5-day window — must not affect result
		sample(0, 9000, 9000, 9000),
	}
	got := RollingWindow(samples, 5, base)
	if got.N != 1 {
		t.Fatalf("N = %d, want 1 (30-days-ago sample must be excluded)", got.N)
	}
	if got.LowMinor != 9000 {
		t.Errorf("LowMinor = %d, want 9000", got.LowMinor)
	}
}

func TestPercentileRank(t *testing.T) {
	samples := []domain.PriceSample{
		sample(4, 8000, 8500, 9000),
		sample(3, 8200, 8600, 9100),
		sample(2, 7900, 8400, 8900),
		sample(1, 8300, 8700, 9200),
		sample(0, 8100, 8550, 9050),
	}
	pct, ok := PercentileRank(samples, 8100, 5, base)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if pct != 60.0 {
		t.Errorf("pct = %v, want 60.0 (3 of 5 days at least as expensive as 8100)", pct)
	}
}

func TestPercentileRank_EmptyWindow(t *testing.T) {
	samples := []domain.PriceSample{sample(0, 8000, 8000, 8000)}
	// Ask about a window ending long before any sample exists.
	_, ok := PercentileRank(samples, 8000, 5, base.AddDate(0, 0, -100))
	if ok {
		t.Fatal("expected ok=false when no samples fall in the window")
	}
}

func TestAllTimeLowOf(t *testing.T) {
	samples := []domain.PriceSample{
		sample(4, 8000, 8500, 9000),
		sample(3, 8200, 8600, 9100),
		sample(10, 7000, 7200, 7500), // lowest, and outside any "rolling" window — must still count
		sample(0, 8100, 8550, 9050),
	}
	got := AllTimeLowOf(samples)
	if !got.OK {
		t.Fatal("expected OK")
	}
	if got.PriceMinor != 7000 {
		t.Errorf("PriceMinor = %d, want 7000", got.PriceMinor)
	}
	if !got.Date.Equal(day(10)) {
		t.Errorf("Date = %v, want %v", got.Date, day(10))
	}
}

func TestAllTimeLowOf_Empty(t *testing.T) {
	got := AllTimeLowOf(nil)
	if got.OK {
		t.Fatal("expected OK=false for empty input")
	}
}
