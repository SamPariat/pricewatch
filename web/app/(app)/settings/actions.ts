"use server";

import { cookies } from "next/headers";
import { revalidatePath } from "next/cache";
import { getTranslations } from "next-intl/server";
import { api } from "@/lib/container";
import { LOCALE_COOKIE, isLocale } from "@/i18n/request";
import type { FormState } from "@/app/(app)/watches/actions";

// language drives both the panel's own UI (via the pw_locale cookie —
// web/i18n/request.ts) and the Discord digest/alert text (via
// Settings.language on the backend — see backend/internal/i18n). One
// form field, two places it takes effect, kept in sync here rather than
// exposing two separate controls for what the user experiences as one
// setting.
export async function updateSettingsAction(_prev: FormState, formData: FormData): Promise<FormState> {
  const rawLocale = formData.get("language");
  const t = await getTranslations("settings");
  try {
    await api.updateSettings({
      discord_channel_id: String(formData.get("discord_channel_id") ?? ""),
      quiet_hours_start: String(formData.get("quiet_hours_start") ?? ""),
      quiet_hours_end: String(formData.get("quiet_hours_end") ?? ""),
      currency: String(formData.get("currency") ?? "INR"),
      dry_run: formData.get("dry_run") === "on",
      language: String(rawLocale ?? "en"),
    });
  } catch (err) {
    return { error: err instanceof Error ? err.message : t("failedToSave") };
  }
  if (isLocale(rawLocale)) {
    const store = await cookies();
    store.set(LOCALE_COOKIE, rawLocale, { path: "/", maxAge: 60 * 60 * 24 * 365 });
  }
  revalidatePath("/settings");
  return { error: null };
}
