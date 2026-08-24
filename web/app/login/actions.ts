"use server";

import { cookies } from "next/headers";
import { redirect } from "next/navigation";
import { SESSION_COOKIE } from "@/lib/api";

const API_URL = process.env.API_INTERNAL_URL ?? "http://app:8080";

// Relays the password to Fiber's own /api/auth/login, then copies
// Fiber's Set-Cookie verbatim onto the browser-facing response. Next.js
// never verifies or signs anything itself — the bcrypt check and the
// HMAC signing both happen entirely in Fiber, so this container never
// needs ADMIN_PASSWORD_HASH or SESSION_SECRET at all.
export async function login(_prevState: string | null, formData: FormData): Promise<string | null> {
  const password = formData.get("password");
  if (typeof password !== "string" || password.length === 0) {
    return "Enter a password.";
  }

  const res = await fetch(`${API_URL}/api/auth/login`, {
    method: "POST",
    headers: { "Content-Type": "application/json", "X-API-Version": "1" },
    body: JSON.stringify({ password }),
    cache: "no-store",
  });

  if (res.status === 401) {
    return "Incorrect password.";
  }
  if (!res.ok) {
    return "Something went wrong — try again.";
  }

  const setCookies = res.headers.getSetCookie();
  const raw = setCookies.find((c) => c.startsWith(`${SESSION_COOKIE}=`));
  if (!raw) {
    return "Login succeeded but no session was issued — try again.";
  }
  const value = raw.split(";")[0].slice(SESSION_COOKIE.length + 1);

  const store = await cookies();
  store.set(SESSION_COOKIE, value, {
    httpOnly: true,
    secure: process.env.NODE_ENV === "production",
    sameSite: "lax",
    path: "/",
    maxAge: 60 * 60 * 24 * 7, // 7 days — matches backend handlers.API.SessionTTL
  });

  redirect("/watches");
}

export async function logout(): Promise<void> {
  await fetch(`${API_URL}/api/auth/logout`, { method: "POST", cache: "no-store" });
  const store = await cookies();
  store.delete(SESSION_COOKIE);
  redirect("/login");
}
