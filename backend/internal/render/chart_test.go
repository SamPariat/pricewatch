package render

import (
	"bytes"
	"testing"
	"time"

	"github.com/sampariat/prices-reminder/internal/domain"
)

var pngMagic = []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}

func TestChart_ProducesValidPNG(t *testing.T) {
	samples := []domain.PriceSample{
		{SampleDate: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), MinMinor: 800000, MedianMinor: 850000, MaxMinor: 900000},
		{SampleDate: time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC), MinMinor: 790000, MedianMinor: 840000, MaxMinor: 890000},
		{SampleDate: time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC), MinMinor: 810000, MedianMinor: 860000, MaxMinor: 910000},
	}

	png, err := Chart(samples, "INR")
	if err != nil {
		t.Fatalf("Chart: %v", err)
	}
	if len(png) == 0 {
		t.Fatal("expected non-empty PNG bytes")
	}
	if !bytes.HasPrefix(png, pngMagic) {
		t.Error("output does not start with the PNG magic bytes")
	}
}

func TestChart_HandlesUnsortedInput(t *testing.T) {
	// Deliberately out of order — Chart must sort internally rather than
	// assuming the caller did, since Repository.ListPriceSamples already
	// sorts but a future caller might not.
	samples := []domain.PriceSample{
		{SampleDate: time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC), MinMinor: 810000, MedianMinor: 860000, MaxMinor: 910000},
		{SampleDate: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), MinMinor: 800000, MedianMinor: 850000, MaxMinor: 900000},
	}
	png, err := Chart(samples, "INR")
	if err != nil {
		t.Fatalf("Chart: %v", err)
	}
	if !bytes.HasPrefix(png, pngMagic) {
		t.Error("output does not start with the PNG magic bytes")
	}
}

func TestChart_EmptySamples_ReturnsError(t *testing.T) {
	if _, err := Chart(nil, "INR"); err == nil {
		t.Fatal("expected an error when there are no samples to chart")
	}
}

func TestChart_SingleSample_DoesNotPanic(t *testing.T) {
	samples := []domain.PriceSample{
		{SampleDate: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), MinMinor: 800000, MedianMinor: 850000, MaxMinor: 900000},
	}
	if _, err := Chart(samples, "INR"); err != nil {
		t.Fatalf("Chart with a single sample should not error, got: %v", err)
	}
}
