import "server-only";
import { cookies } from "next/headers";
import { redirect } from "next/navigation";
import type {
  AnalyticsSummary,
  ChannelStatus,
  DigestRun,
  History,
  Meta,
  RunEvent,
  Settings,
  Watch,
  WatchInput,
} from "./types";

// Internal docker-network URL — the browser never talks to Fiber
// directly, only Next.js server-side code does. The session cookie is
// signed and verified entirely by Fiber (see app/login/actions.ts);
// Next.js just relays the opaque token, so it never needs the signing
// secret at all.
const API_URL = process.env.API_INTERNAL_URL ?? "http://app:8080";
export const SESSION_COOKIE = "pw_session";

export class ApiError extends Error {
  status: number;
  constructor(status: number, message: string) {
    super(message);
    this.status = status;
  }
}

async function cookieHeader(): Promise<string | undefined> {
  const store = await cookies();
  const token = store.get(SESSION_COOKIE)?.value;
  return token ? `${SESSION_COOKIE}=${token}` : undefined;
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const cookie = await cookieHeader();
  const res = await fetch(`${API_URL}${path}`, {
    ...init,
    headers: {
      "Content-Type": "application/json",
      "X-API-Version": "1",
      ...(cookie ? { Cookie: cookie } : {}),
      ...(init?.headers ?? {}),
    },
    // Every route this client calls is either mutation or freshness-
    // sensitive; watch history is the one exception, and its own caching
    // is Fiber's job (schedule-derived, PLAN.md § Caching) — the Next.js
    // layer stays uncached, not a second, independent cache to keep in
    // sync with the first.
    cache: "no-store",
  });

  if (res.status === 401) {
    // Fiber is the sole authority on session validity — an absent,
    // expired, or tampered cookie surfaces here as a 401 from the API,
    // not from anything Next.js checked itself. proxy.ts catches the
    // common "never logged in" case earlier and more cheaply; this is
    // the authoritative fallback for every other case (expiry, a
    // tampered cookie value, Fiber restarting with the same secret but
    // stale sessions, etc).
    redirect("/login");
  }
  if (!res.ok) {
    const text = await res.text().catch(() => "");
    throw new ApiError(res.status, text || res.statusText);
  }
  if (res.status === 204) {
    return undefined as T;
  }
  return res.json() as Promise<T>;
}

export const api = {
  getWatches: () => request<Watch[]>("/api/watches"),
  getWatch: (id: string) => request<Watch>(`/api/watches/${id}`),
  createWatch: (data: WatchInput) =>
    request<Watch>("/api/watches", { method: "POST", body: JSON.stringify(data) }),
  updateWatch: (id: string, data: WatchInput) =>
    request<Watch>(`/api/watches/${id}`, { method: "PATCH", body: JSON.stringify(data) }),
  deleteWatch: (id: string) => request<void>(`/api/watches/${id}`, { method: "DELETE" }),
  runWatchNow: (id: string) =>
    request<{ message: string }>(`/api/watches/${id}/run`, { method: "POST" }),
  getHistory: (id: string, range = "90d") =>
    request<History>(`/api/watches/${id}/history?range=${range}`),
  getSettings: () => request<Settings>("/api/settings"),
  updateSettings: (data: Settings) =>
    request<Settings>("/api/settings", { method: "PATCH", body: JSON.stringify(data) }),
  getRuns: () => request<DigestRun[]>("/api/runs"),
  getRunEvents: (runId: string) => request<RunEvent[]>(`/api/runs/${runId}/events`),
  getAnalyticsSummary: () => request<AnalyticsSummary>("/api/analytics/summary"),
  getMeta: () => request<Meta>("/api/meta"),
  getChannelStatus: () => request<{ status: ChannelStatus }>("/api/channel/status"),
};
