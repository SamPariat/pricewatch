package render

import (
	"bytes"
	"fmt"
	"sort"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"

	"github.com/SamPariat/pricewatch/internal/domain"
)

// Chart renders a watch's price history to a PNG for a Discord embed's
// image attachment — pure-Go rendering (no headless browser, no JS),
// which matters for a distroless container with nothing else installed.
// Samples need not be sorted; this sorts them itself so callers can pass
// repository results directly.
func Chart(samples []domain.PriceSample, currency string) ([]byte, error) {
	if len(samples) == 0 {
		return nil, fmt.Errorf("render: chart: no samples to plot")
	}

	sorted := append([]domain.PriceSample(nil), samples...)
	sortByDate(sorted)

	p := plot.New()
	p.Y.Label.Text = currency
	p.X.Tick.Marker = plot.TimeTicks{Format: "Jan 2"}

	medianPts := make(plotter.XYs, len(sorted))
	lowPts := make(plotter.XYs, len(sorted))
	for i, s := range sorted {
		x := float64(s.SampleDate.Unix())
		medianPts[i] = plotter.XY{X: x, Y: float64(s.MedianMinor) / 100}
		lowPts[i] = plotter.XY{X: x, Y: float64(s.MinMinor) / 100}
	}

	medianLine, err := plotter.NewLine(medianPts)
	if err != nil {
		return nil, fmt.Errorf("render: chart: median line: %w", err)
	}
	medianLine.Width = vg.Points(2)

	lowLine, err := plotter.NewLine(lowPts)
	if err != nil {
		return nil, fmt.Errorf("render: chart: low line: %w", err)
	}
	lowLine.Width = vg.Points(1)
	lowLine.Dashes = []vg.Length{vg.Points(3), vg.Points(3)}

	p.Add(medianLine, lowLine)
	p.Legend.Add("median", medianLine)
	p.Legend.Add("low", lowLine)

	writer, err := p.WriterTo(5*vg.Inch, 2.5*vg.Inch, "png")
	if err != nil {
		return nil, fmt.Errorf("render: chart: layout: %w", err)
	}
	var buf bytes.Buffer
	if _, err := writer.WriteTo(&buf); err != nil {
		return nil, fmt.Errorf("render: chart: encode: %w", err)
	}
	return buf.Bytes(), nil
}

func sortByDate(samples []domain.PriceSample) {
	sort.Slice(samples, func(i, j int) bool { return samples[i].SampleDate.Before(samples[j].SampleDate) })
}
