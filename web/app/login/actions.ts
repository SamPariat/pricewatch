"use server";

import { cookies } from "next/headers";
import { redirect } from "next/navigation";
import { SESSION_COOKIE } from "@/lib/api";
import { bodyPreview, logger } from "@/lib/logger";

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

  const start = Date.now();
  const res = await fetch(`${API_URL}/api/auth/login`, {
    method: "POST",
    headers: { "Content-Type": "application/json", "X-API-Version": "1" },
    body: JSON.stringify({ password }),
    cache: "no-store",
  });
  const duration_ms = Date.now() - start;

  if (res.status === 401) {
    // Never log the password itself — only that an attempt was made and
    // rejected. Repeated hits here are the signal worth having (someone
    // guessing), which a plain "Incorrect password" toast on its own
    // leaves no server-side trace of.
    logger.warn("login: rejected", { status: 401, duration_ms });
    return "Incorrect password.";
  }
  if (!res.ok) {
    const text = await res.text().catch(() => "");
    logger.warn("login: unexpected response", { status: res.status, duration_ms, body: bodyPreview(text) });
    return "Something went wrong — try again.";
  }
  logger.info("login: succeeded", { duration_ms });

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
  const start = Date.now();
  const res = await fetch(`${API_URL}/api/auth/logout`, { method: "POST", cache: "no-store" });
  logger.info("logout", { status: res.status, duration_ms: Date.now() - start });
  const store = await cookies();
  store.delete(SESSION_COOKIE);
  redirect("/login");
}
