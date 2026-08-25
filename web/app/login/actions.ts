"use server";

import { cookies } from "next/headers";
import { redirect } from "next/navigation";
import { SESSION_COOKIE } from "@/lib/api";
import { api } from "@/lib/container";

// redirect() throws internally to signal navigation to Next.js's own
// machinery — it must never be called inside a try/catch that could
// swallow that throw, so each action resolves its outcome first and
// only calls redirect() once any try/catch block has exited. Neither
// action here needs one: ApiClient.login()/logout() already handle their
// own request/error logging internally (see lib/api.ts) — this file's
// only job is the two things that stay Server-Action-bound: writing the
// session cookie and navigating.
export async function login(_prevState: string | null, formData: FormData): Promise<string | null> {
  const password = formData.get("password");
  if (typeof password !== "string" || password.length === 0) {
    return "Enter a password.";
  }

  const result = await api.login(password);
  if ("error" in result) {
    return result.error;
  }

  const store = await cookies();
  store.set(SESSION_COOKIE, result.cookieValue, {
    httpOnly: true,
    secure: process.env.NODE_ENV === "production",
    sameSite: "lax",
    path: "/",
    maxAge: 60 * 60 * 24 * 7, // 7 days — matches backend handlers.API.SessionTTL
  });

  redirect("/watches");
}

export async function logout(): Promise<void> {
  await api.logout();
  const store = await cookies();
  store.delete(SESSION_COOKIE);
  redirect("/login");
}
