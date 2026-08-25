import "server-only";
import { redirect } from "next/navigation";
import { bodyPreview, type Logger } from "./logger";
import type {
  AnalyticsSummary, ChannelStatus, DigestRun, Envelope, History, Meta,
  RunEvent, Settings, Watch, WatchInput,
} from "./types";

// Matches backend/internal/httpapi/handlers.APIVersion — no shared source
// between the two languages in this monorepo, so this is kept in sync by
// hand, same as lib/types.ts's DTOs.
const API_VERSION = "v1";

export const SESSION_COOKIE = "pw_session";

// Every error this client can throw extends this, so a caller can catch
// broadly ("anything from the API layer failed") or narrow with
// instanceof when it cares which kind.
export abstract class ApiClientError extends Error {}

// ApiError is a real HTTP response with a non-2xx status — status is the
// HTTP status code; message/errorMessage come from the response
// envelope's own message/error fields (see internal/httpapi/envelope on
// the Go side), not a guess.
export class ApiError extends ApiClientError {
  constructor(
    public status: number,
    message: string,
    public errorMessage: string | null,
  ) {
    super(message);
  }
}

// ApiNetworkError means fetch itself threw — no response exists at all
// (connection refused, DNS, timeout). Previously this case rethrew the
// raw fetch error unwrapped, so nothing could distinguish it from an
// ApiError via instanceof; this closes that gap.
export class ApiNetworkError extends ApiClientError {
  constructor(
    message: string,
    public cause: unknown,
  ) {
    super(message);
  }
}

export interface ApiClientDeps {
  baseUrl: string;
  getCookie: () => Promise<string | undefined>;
  logger: Logger;
  fetchImpl: typeof fetch;
}

// ApiClient is the one thing in this codebase that calls Fiber — every
// dependency is constructor-injected (base URL, cookie source, logger,
// even the fetch implementation) so a future test could construct one
// against a mock without touching the network. See lib/container.ts for
// where the real instance gets wired — that's a module-level composition
// root, not a request-scoped DI container (Next.js Server
// Components/Actions have no such thing), constructed once per server
// process, same lifecycle the old plain-object client had.
export class ApiClient {
  constructor(private deps: ApiClientDeps) {}

  private async request<T>(path: string, init?: RequestInit): Promise<T> {
    const method = init?.method ?? "GET";
    const cookie = await this.deps.getCookie();
    const start = Date.now();

    let res: Response;
    try {
      res = await this.deps.fetchImpl(`${this.deps.baseUrl}${path}`, {
        ...init,
        headers: {
          "Content-Type": "application/json",
          ...(cookie ? { Cookie: cookie } : {}),
          ...(init?.headers ?? {}),
        },
        // Every route this client calls is either mutation or freshness-
        // sensitive; watch history is the one exception, and its own
        // caching is Fiber's job (schedule-derived) — this layer stays
        // uncached, not a second, independent cache to keep in sync.
        cache: "no-store",
      });
    } catch (err) {
      this.deps.logger.error("api call: request failed", {
        method,
        path,
        duration_ms: Date.now() - start,
        error: err instanceof Error ? err.message : String(err),
      });
      throw new ApiNetworkError(`request to ${path} failed`, err);
    }

    const duration_ms = Date.now() - start;

    if (res.status === 401) {
      // Fiber is the sole authority on session validity — an absent,
      // expired, or tampered cookie surfaces here as a 401 from the API,
      // not from anything Next.js checked itself. proxy.ts catches the
      // common "never logged in" case earlier and more cheaply; this is
      // the authoritative fallback for every other case.
      this.deps.logger.info("api call: session invalid, redirecting to login", { method, path, duration_ms });
      redirect("/login");
    }
    if (!res.ok) {
      const text = await res.text().catch(() => "");
      this.deps.logger.warn("api call failed", { method, path, status: res.status, duration_ms, body: bodyPreview(text) });
      throw new ApiError(res.status, ...parseErrorBody(text, res.statusText));
    }

    this.deps.logger.debug("api call", { method, path, status: res.status, duration_ms });

    if (res.status === 204) {
      return undefined as T;
    }
    const envelope = (await res.json()) as Envelope<T>;
    return envelope.data;
  }

  getWatches() {
    return this.request<Watch[]>(`/api/${API_VERSION}/watches`);
  }
  getWatch(id: string) {
    return this.request<Watch>(`/api/${API_VERSION}/watches/${id}`);
  }
  createWatch(data: WatchInput) {
    return this.request<Watch>(`/api/${API_VERSION}/watches`, { method: "POST", body: JSON.stringify(data) });
  }
  updateWatch(id: string, data: WatchInput) {
    return this.request<Watch>(`/api/${API_VERSION}/watches/${id}`, { method: "PATCH", body: JSON.stringify(data) });
  }
  deleteWatch(id: string) {
    return this.request<void>(`/api/${API_VERSION}/watches/${id}`, { method: "DELETE" });
  }
  runWatchNow(id: string) {
    return this.request<{ message: string }>(`/api/${API_VERSION}/watches/${id}/run`, { method: "POST" });
  }
  getHistory(id: string, range = "90d") {
    return this.request<History>(`/api/${API_VERSION}/watches/${id}/history?range=${range}`);
  }
  getSettings() {
    return this.request<Settings>(`/api/${API_VERSION}/settings`);
  }
  updateSettings(data: Settings) {
    return this.request<Settings>(`/api/${API_VERSION}/settings`, { method: "PATCH", body: JSON.stringify(data) });
  }
  getRuns() {
    return this.request<DigestRun[]>(`/api/${API_VERSION}/runs`);
  }
  getRunEvents(runId: string) {
    return this.request<RunEvent[]>(`/api/${API_VERSION}/runs/${runId}/events`);
  }
  getAnalyticsSummary() {
    return this.request<AnalyticsSummary>(`/api/${API_VERSION}/analytics/summary`);
  }
  getMeta() {
    return this.request<Meta>(`/api/${API_VERSION}/meta`);
  }
  getChannelStatus() {
    return this.request<{ status: ChannelStatus }>(`/api/${API_VERSION}/channel/status`);
  }

  // login/logout don't go through request<T>() above: login's success
  // response is a 204 with a Set-Cookie header, not an enveloped JSON
  // body, so there's nothing for request<T>() to unwrap — and neither
  // call should trigger the 401-redirect branch above (401 here is a
  // login rejection to show as form feedback, not an expired session to
  // bounce the user out of). Deliberately doesn't call cookies().set() or
  // redirect() itself — those are Server-Action-bound Next.js APIs that
  // stay owned by web/app/login/actions.ts, keeping this class a pure
  // network/protocol layer that isn't tied to one calling context.
  async login(password: string): Promise<{ cookieValue: string } | { error: string }> {
    const start = Date.now();
    let res: Response;
    try {
      res = await this.deps.fetchImpl(`${this.deps.baseUrl}/api/${API_VERSION}/auth/login`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ password }),
        cache: "no-store",
      });
    } catch (err) {
      this.deps.logger.error("login: request failed", {
        duration_ms: Date.now() - start,
        error: err instanceof Error ? err.message : String(err),
      });
      return { error: "Could not reach the server — try again." };
    }
    const duration_ms = Date.now() - start;

    if (res.status === 401) {
      // Never log the password itself — only that an attempt was made
      // and rejected. Repeated hits here are the signal worth having.
      this.deps.logger.warn("login: rejected", { status: 401, duration_ms });
      return { error: "Incorrect password." };
    }
    if (!res.ok) {
      const text = await res.text().catch(() => "");
      this.deps.logger.warn("login: unexpected response", { status: res.status, duration_ms, body: bodyPreview(text) });
      return { error: "Something went wrong — try again." };
    }
    this.deps.logger.info("login: succeeded", { duration_ms });

    const setCookies = res.headers.getSetCookie();
    const raw = setCookies.find((c) => c.startsWith(`${SESSION_COOKIE}=`));
    if (!raw) {
      return { error: "Login succeeded but no session was issued — try again." };
    }
    return { cookieValue: raw.split(";")[0].slice(SESSION_COOKIE.length + 1) };
  }

  async logout(): Promise<{ status: number }> {
    const start = Date.now();
    const res = await this.deps.fetchImpl(`${this.deps.baseUrl}/api/${API_VERSION}/auth/logout`, {
      method: "POST",
      cache: "no-store",
    });
    this.deps.logger.info("logout", { status: res.status, duration_ms: Date.now() - start });
    return { status: res.status };
  }
}

// parseErrorBody reads the enveloped {message, error, ...} shape the
// backend now always sends on failure; falls back to the raw text (or
// statusText) if the body isn't JSON, so this degrades gracefully rather
// than throwing while trying to report an error.
function parseErrorBody(text: string, statusText: string): [message: string, errorMessage: string | null] {
  try {
    const env = JSON.parse(text) as Envelope<unknown>;
    if (typeof env.message === "string" && env.message) {
      return [env.message, env.error ?? env.message];
    }
  } catch {
    // not JSON — fall through to the raw text/statusText fallback below
  }
  const fallback = text || statusText;
  return [fallback, fallback];
}
