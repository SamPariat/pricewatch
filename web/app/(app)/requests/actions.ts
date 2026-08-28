"use server";

import { revalidatePath } from "next/cache";
import { api } from "@/lib/container";

export type ActionResult = { error: string | null };

export async function approveRequestAction(id: string): Promise<ActionResult> {
  try {
    await api.approveRequest(id);
  } catch (err) {
    return { error: err instanceof Error ? err.message : "Failed to approve" };
  }
  revalidatePath("/requests");
  revalidatePath("/trips");
  return { error: null };
}

export async function rejectRequestAction(id: string): Promise<ActionResult> {
  await api.rejectRequest(id);
  revalidatePath("/requests");
  return { error: null };
}
