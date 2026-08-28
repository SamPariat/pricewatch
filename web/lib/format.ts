import type { Watch } from "./types";

// Only the kindOneWay/kindReturn keys from the "watchForm" namespace are
// used here — callers pass a next-intl translator already scoped to it
// (getTranslations("watchForm") on the server, useTranslations("watchForm")
// on the client), so this stays a plain function rather than needing its
// own message loading.
type WatchFormTranslator = (key: "kindOneWay" | "kindReturn") => string;

// Matches backend/internal/render's groupThousands exactly (Western
// 3-digit grouping) — the panel and the digest must agree on how a price
// reads, so this doesn't independently choose en-IN grouping just
// because the default currency happens to be INR. Deliberately not
// locale-dependent, unlike the date formatting below — digits are
// numerals, not prose, and PLAN.md's "never let anything but Go produce
// a number" spirit extends to not letting the browser's locale silently
// change how one reads either.
export function formatPrice(minor: number, currency: string): string {
  const whole = Math.round(minor / 100);
  return `${currency} ${whole.toLocaleString("en-US")}`;
}

export function watchTitle(watch: Pick<Watch, "name" | "kind" | "params">, t: WatchFormTranslator): string {
  if (watch.name) return watch.name;
  const p = watch.params;
  const suffix = watch.kind === "flight_return" ? t("kindReturn") : t("kindOneWay");
  return `${p.origin} → ${p.destination} · ${suffix}`;
}

export function watchDateRange(watch: Pick<Watch, "params">, locale: string): string {
  const p = watch.params;
  return p.return_date
    ? `${formatShortDate(p.depart_date, locale)} – ${formatShortDate(p.return_date, locale)}`
    : formatShortDate(p.depart_date, locale);
}

export function watchSubtitle(watch: Pick<Watch, "kind" | "params">, t: WatchFormTranslator, locale: string): string {
  const kindLabel = watch.kind === "flight_return" ? t("kindReturn") : t("kindOneWay");
  return `${kindLabel} · ${watchDateRange(watch, locale)}`;
}

function localeTag(locale: string): string {
  return locale === "hi" ? "hi-IN" : "en-US";
}

function formatShortDate(iso: string, locale: string): string {
  const d = new Date(iso + "T00:00:00Z");
  if (Number.isNaN(d.getTime())) return iso;
  return d.toLocaleDateString(localeTag(locale), { month: "short", day: "numeric", timeZone: "UTC" });
}
