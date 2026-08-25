import "server-only";
import { cookies } from "next/headers";
import { ApiClient, SESSION_COOKIE } from "./api";
import { ConsoleLogger } from "./logger";

async function getCookie(): Promise<string | undefined> {
  const store = await cookies();
  const token = store.get(SESSION_COOKIE)?.value;
  return token ? `${SESSION_COOKIE}=${token}` : undefined;
}

// The composition root for the frontend — the one place ApiClient's
// dependencies get wired together, kept separate from both the class
// definition (lib/api.ts) and the logger implementation (lib/logger.ts).
// A module-level singleton, constructed once per server process — the
// same lifecycle the old plain-object client had, just with the wiring
// isolated here instead of hardcoded inside api.ts itself.
export const api = new ApiClient({
  baseUrl: process.env.API_INTERNAL_URL ?? "http://app:8080",
  getCookie,
  logger: new ConsoleLogger(),
  fetchImpl: fetch,
});
