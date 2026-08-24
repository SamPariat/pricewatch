import type { HotelParams, Watch } from "./types";
import { isFlightParams } from "./types";

// Matches backend/internal/render's groupThousands exactly (Western
// 3-digit grouping) — the panel and the Telegram digest must agree on
// how a price reads, so this doesn't independently choose en-IN grouping
// just because the default currency happens to be INR.
export function formatPrice(minor: number, currency: string): string {
  const whole = Math.round(minor / 100);
  return `${currency} ${whole.toLocaleString("en-US")}`;
}

export function watchTitle(watch: Pick<Watch, "name" | "kind" | "params">): string {
  if (watch.name) return watch.name;
  const p = watch.params;
  if (isFlightParams(p)) {
    const suffix = watch.kind === "flight_return" ? "return" : "one-way";
    return `${p.origin} → ${p.destination} · ${suffix}`;
  }
  return p.location;
}

export function watchDateRange(watch: Pick<Watch, "kind" | "params">): string {
  const p = watch.params;
  if (isFlightParams(p)) {
    return p.return_date ? `${formatShortDate(p.depart_date)} – ${formatShortDate(p.return_date)}` : formatShortDate(p.depart_date);
  }
  const hp = p as HotelParams;
  return `${formatShortDate(hp.check_in)} – ${formatShortDate(hp.check_out)}`;
}

export function watchSubtitle(watch: Pick<Watch, "kind" | "params">): string {
  const kindLabel: Record<string, string> = {
    flight_return: "Return",
    flight_one_way: "One-way",
    hotel: "Hotel",
    rental: "Rental",
  };
  return `${kindLabel[watch.kind] ?? watch.kind} · ${watchDateRange(watch)}`;
}

function formatShortDate(iso: string): string {
  const d = new Date(iso + "T00:00:00Z");
  if (Number.isNaN(d.getTime())) return iso;
  return d.toLocaleDateString("en-US", { month: "short", day: "numeric", timeZone: "UTC" });
}
