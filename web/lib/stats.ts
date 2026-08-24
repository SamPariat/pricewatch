import type { PriceSample } from "./types";

// Simple min/median/all-time-low arithmetic for the detail page's stat
// tiles — deliberately reimplemented here rather than round-tripped from
// the backend, unlike delta/percentile (lib/api.ts's PriceSummary),
// which stay server-side because they're what the Telegram digest also
// computes and must agree with. Min/median over an already-fetched
// sample array carries none of that consistency risk.
function windowed(samples: PriceSample[], days: number): PriceSample[] {
  if (samples.length === 0) return [];
  const asOf = new Date(samples[samples.length - 1].date + "T00:00:00Z");
  const start = new Date(asOf);
  start.setUTCDate(start.getUTCDate() - (days - 1));
  return samples.filter((s) => {
    const d = new Date(s.date + "T00:00:00Z");
    return d >= start && d <= asOf;
  });
}

export function rollingLow(samples: PriceSample[], days: number): number | undefined {
  const window = windowed(samples, days);
  if (window.length === 0) return undefined;
  return Math.min(...window.map((s) => s.min_minor));
}

export function rollingMedian(samples: PriceSample[], days: number): number | undefined {
  const window = windowed(samples, days);
  if (window.length === 0) return undefined;
  const sorted = [...window.map((s) => s.median_minor)].sort((a, b) => a - b);
  const mid = Math.floor(sorted.length / 2);
  return sorted.length % 2 === 1 ? sorted[mid] : (sorted[mid - 1] + sorted[mid]) / 2;
}

export function allTimeLow(samples: PriceSample[]): { minor: number; date: string } | undefined {
  if (samples.length === 0) return undefined;
  let best = samples[0];
  for (const s of samples) if (s.min_minor < best.min_minor) best = s;
  return { minor: best.min_minor, date: best.date };
}
