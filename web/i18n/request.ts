import "server-only";
import { cookies } from "next/headers";
import { getRequestConfig } from "next-intl/server";

// "en" | "hi" — kept as plain strings rather than importing Locale from
// lib/i18n/locale.ts to dodge a require cycle (that file will want
// LOCALE_COOKIE/isLocale from here in a future pass); duplicating two
// literals is cheaper than the cycle.
export const LOCALES = ["en", "hi"] as const;
export type Locale = (typeof LOCALES)[number];
export const DEFAULT_LOCALE: Locale = "en";
export const LOCALE_COOKIE = "pw_locale";

export function isLocale(v: unknown): v is Locale {
  return v === "en" || v === "hi";
}

// No i18n routing (no [locale] URL segment) — this is a single-admin
// panel, not a public multi-language site, so the language is a
// personal preference stored in a cookie (see
// app/(app)/settings/actions.ts), not something worth encoding into
// every URL. See https://next-intl.dev/docs/usage/configuration#i18n-request.
export default getRequestConfig(async () => {
  const store = await cookies();
  const raw = store.get(LOCALE_COOKIE)?.value;
  const locale = isLocale(raw) ? raw : DEFAULT_LOCALE;

  // Reaches three levels up from web/i18n/ to the monorepo root's
  // locales/ — the same files backend/internal/i18n loads directly (see
  // that package's own doc comment on why this is a shared source of
  // truth rather than two copies). Only this app's own namespaces live
  // at the JSON's root; the "backend" key is Go's alone.
  const messages = (await import(`../../locales/${locale}.json`)).default;
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  const { backend: _backend, ...webMessages } = messages;

  return { locale, messages: webMessages };
});
