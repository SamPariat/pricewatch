"use server";

import { revalidatePath } from "next/cache";
import { api } from "@/lib/container";
import type { FormState } from "@/app/(app)/watches/actions";

export async function updateSettingsAction(_prev: FormState, formData: FormData): Promise<FormState> {
  try {
    await api.updateSettings({
      telegram_chat_id: String(formData.get("telegram_chat_id") ?? ""),
      quiet_hours_start: String(formData.get("quiet_hours_start") ?? ""),
      quiet_hours_end: String(formData.get("quiet_hours_end") ?? ""),
      currency: String(formData.get("currency") ?? "INR"),
      dry_run: formData.get("dry_run") === "on",
    });
  } catch (err) {
    return { error: err instanceof Error ? err.message : "Failed to save settings" };
  }
  revalidatePath("/settings");
  return { error: null };
}
