"use server";

import { revalidatePath } from "next/cache";
import { redirect } from "next/navigation";
import { api } from "@/lib/api";
import type { AssetKind, FlightParams, HotelParams, WatchInput } from "@/lib/types";

function watchInputFromForm(formData: FormData): WatchInput {
  const kind = formData.get("kind") as AssetKind;
  const isFlight = kind === "flight_one_way" || kind === "flight_return";

  const params: FlightParams | HotelParams = isFlight
    ? {
        origin: String(formData.get("origin") ?? "").toUpperCase(),
        destination: String(formData.get("destination") ?? "").toUpperCase(),
        depart_date: String(formData.get("depart_date") ?? ""),
        ...(kind === "flight_return" ? { return_date: String(formData.get("return_date") ?? "") } : {}),
      }
    : {
        location: String(formData.get("location") ?? ""),
        check_in: String(formData.get("check_in") ?? ""),
        check_out: String(formData.get("check_out") ?? ""),
      };

  const hour = String(formData.get("hour") ?? "7").padStart(2, "0");
  const minute = String(formData.get("minute") ?? "0").padStart(2, "0");

  return {
    name: String(formData.get("name") ?? ""),
    kind,
    enabled: true,
    cron_expr: `${Number(minute)} ${Number(hour)} * * *`,
    timezone: String(formData.get("timezone") ?? "UTC"),
    params,
    threshold_pct: Number(formData.get("threshold_pct") ?? 15),
  };
}

export type FormState = { error: string | null };

// redirect() throws internally to signal navigation to Next.js's own
// machinery — it must never be called inside a try/catch that could
// swallow that throw, so each action resolves its outcome first and
// only calls redirect() once the try/catch block has exited.
export async function createWatchAction(_prev: FormState, formData: FormData): Promise<FormState> {
  let newId: string;
  try {
    const watch = await api.createWatch(watchInputFromForm(formData));
    newId = watch.id;
  } catch (err) {
    return { error: err instanceof Error ? err.message : "Failed to create watch" };
  }
  revalidatePath("/watches");
  redirect(`/watches/${newId}`);
}

export async function updateWatchAction(id: string, _prev: FormState, formData: FormData): Promise<FormState> {
  try {
    await api.updateWatch(id, watchInputFromForm(formData));
  } catch (err) {
    return { error: err instanceof Error ? err.message : "Failed to update watch" };
  }
  revalidatePath("/watches");
  revalidatePath(`/watches/${id}`);
  redirect(`/watches/${id}`);
}

export async function deleteWatchAction(id: string): Promise<void> {
  await api.deleteWatch(id);
  revalidatePath("/watches");
  redirect("/watches");
}

export async function runWatchNowAction(id: string): Promise<{ message: string }> {
  const result = await api.runWatchNow(id);
  revalidatePath(`/watches/${id}`);
  return result;
}

export async function toggleEnabledAction(id: string, watch: WatchInput, enabled: boolean): Promise<void> {
  await api.updateWatch(id, { ...watch, enabled });
  revalidatePath("/watches");
  revalidatePath(`/watches/${id}`);
}
