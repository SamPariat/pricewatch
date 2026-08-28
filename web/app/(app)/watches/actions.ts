"use server";

import { revalidatePath } from "next/cache";
import { redirect } from "next/navigation";
import { getTranslations } from "next-intl/server";
import { api } from "@/lib/container";
import type { AssetKind, FlightParams, LodgingParams, WatchInput } from "@/lib/types";

function watchInputFromForm(formData: FormData): WatchInput {
  const kind = formData.get("kind") as AssetKind;

  const params: FlightParams | LodgingParams =
    kind === "lodging_airbnb"
      ? {
          url: String(formData.get("url") ?? ""),
          check_in: String(formData.get("check_in") ?? ""),
          check_out: String(formData.get("check_out") ?? ""),
          ...(formData.get("guests") ? { guests: Number(formData.get("guests")) } : {}),
        }
      : {
          origin: String(formData.get("origin") ?? "").toUpperCase(),
          destination: String(formData.get("destination") ?? "").toUpperCase(),
          depart_date: String(formData.get("depart_date") ?? ""),
          ...(kind === "flight_return" ? { return_date: String(formData.get("return_date") ?? "") } : {}),
        };

  return {
    name: String(formData.get("name") ?? ""),
    kind,
    enabled: true,
    params,
    threshold_pct: Number(formData.get("threshold_pct") ?? 15),
    trip_id: String(formData.get("trip_id") ?? ""),
  };
}

export type FormState = { error: string | null };

// redirect() throws internally to signal navigation to Next.js's own
// machinery — it must never be called inside a try/catch that could
// swallow that throw, so each action resolves its outcome first and
// only calls redirect() once the try/catch block has exited.
export async function createWatchAction(_prev: FormState, formData: FormData): Promise<FormState> {
  let newId: string;
  let tripId: string;
  try {
    const watch = await api.createWatch(watchInputFromForm(formData));
    newId = watch.id;
    tripId = watch.trip_id;
  } catch (err) {
    return { error: err instanceof Error ? err.message : (await getTranslations("errors"))("createWatchFailed") };
  }
  revalidatePath(`/trips/${tripId}`);
  redirect(`/watches/${newId}`);
}

export async function updateWatchAction(id: string, _prev: FormState, formData: FormData): Promise<FormState> {
  let tripId: string;
  try {
    const watch = await api.updateWatch(id, watchInputFromForm(formData));
    tripId = watch.trip_id;
  } catch (err) {
    return { error: err instanceof Error ? err.message : (await getTranslations("errors"))("updateWatchFailed") };
  }
  revalidatePath(`/trips/${tripId}`);
  revalidatePath(`/watches/${id}`);
  redirect(`/watches/${id}`);
}

export async function deleteWatchAction(id: string): Promise<void> {
  const watch = await api.getWatch(id);
  await api.deleteWatch(id);
  revalidatePath(`/trips/${watch.trip_id}`);
  redirect(`/trips/${watch.trip_id}`);
}

export async function runWatchNowAction(id: string): Promise<{ message: string }> {
  const result = await api.runWatchNow(id);
  revalidatePath(`/watches/${id}`);
  return result;
}

export async function toggleEnabledAction(id: string, watch: WatchInput, enabled: boolean): Promise<void> {
  const updated = await api.updateWatch(id, { ...watch, enabled });
  revalidatePath(`/trips/${updated.trip_id}`);
  revalidatePath(`/watches/${id}`);
}
