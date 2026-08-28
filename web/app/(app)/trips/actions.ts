"use server";

import { revalidatePath } from "next/cache";
import { redirect } from "next/navigation";
import { getTranslations } from "next-intl/server";
import { api } from "@/lib/container";
import type { TripInput } from "@/lib/types";

function tripInputFromForm(formData: FormData): TripInput {
  const hour = String(formData.get("hour") ?? "7").padStart(2, "0");
  const minute = String(formData.get("minute") ?? "0").padStart(2, "0");
  return {
    name: String(formData.get("name") ?? ""),
    cron_expr: `${Number(minute)} ${Number(hour)} * * *`,
    timezone: String(formData.get("timezone") ?? "UTC"),
    enabled: true,
  };
}

export type FormState = { error: string | null };

// redirect() throws internally to signal navigation to Next.js's own
// machinery — resolve the outcome first, call redirect() only after the
// try/catch block has exited, same rule as watches/actions.ts.
export async function createTripAction(_prev: FormState, formData: FormData): Promise<FormState> {
  let newId: string;
  try {
    const trip = await api.createTrip(tripInputFromForm(formData));
    newId = trip.id;
  } catch (err) {
    return { error: err instanceof Error ? err.message : (await getTranslations("errors"))("createTripFailed") };
  }
  revalidatePath("/trips");
  redirect(`/trips/${newId}/new-leg`);
}

export async function updateTripAction(id: string, _prev: FormState, formData: FormData): Promise<FormState> {
  try {
    await api.updateTrip(id, tripInputFromForm(formData));
  } catch (err) {
    return { error: err instanceof Error ? err.message : (await getTranslations("errors"))("updateTripFailed") };
  }
  revalidatePath("/trips");
  revalidatePath(`/trips/${id}`);
  redirect(`/trips/${id}`);
}

export async function deleteTripAction(id: string): Promise<void> {
  await api.deleteTrip(id);
  revalidatePath("/trips");
  redirect("/trips");
}
