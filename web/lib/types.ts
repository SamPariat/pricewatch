// Mirrors backend/internal/httpapi/presenter/v1 — kept in sync by hand
// since this is a two-language monorepo, not generated. If a v1 DTO field
// changes on the Go side, update it here too.

// Flights only — hotel and rental kinds existed in an earlier version of
// this app but were removed (see backend/internal/domain.AssetKind's own
// doc comment: no free, working API was left for either).
export type AssetKind = "flight_one_way" | "flight_return";

export interface FlightParams {
  origin: string;
  destination: string;
  depart_date: string; // YYYY-MM-DD
  return_date?: string;
}

export interface PriceSummary {
  price_minor: number;
  currency: string;
  delta_pct?: number;
  percentile?: number;
}

export interface Watch {
  id: string;
  name: string;
  kind: AssetKind;
  enabled: boolean;
  cron_expr: string;
  timezone: string;
  params: FlightParams;
  threshold_pct: number;
  created_at: string;
  last_updated_at?: string;
  last_error?: string;
  consecutive_failures: number;
  stale: boolean;
  next_run?: string;
  price?: PriceSummary;
}

export interface PriceSample {
  date: string; // YYYY-MM-DD
  min_minor: number;
  median_minor: number;
  max_minor: number;
  n_quotes: number;
}

export interface History {
  samples: PriceSample[];
  last_updated_at?: string;
  next_run?: string;
}

export type RunStatus = "pending" | "success" | "failed";

export interface DigestRun {
  run_id: string;
  watch_id: string;
  started_at: string;
  finished_at?: string;
  status: RunStatus;
  message_body?: string;
  error?: string;
}

export type LogLevel = "debug" | "info" | "warn" | "error";

export interface RunEvent {
  at: string;
  stage: string;
  level: LogLevel;
  msg: string;
  fields?: unknown;
}

export interface Settings {
  discord_channel_id: string;
  quiet_hours_start: string;
  quiet_hours_end: string;
  currency: string;
  dry_run: boolean;
  // "en" or "hi" — drives the Discord digest/alert text (see
  // backend/internal/i18n). The panel's own UI language is a separate,
  // purely client-side cookie (lib/i18n), though the language switcher
  // keeps both in sync by PATCHing this field.
  language: string;
}

// Versioned by URL prefix (/api/v1/...) now, not header negotiation —
// see backend/internal/httpapi/router.go.
export interface Meta {
  current_version: string;
  supported_versions: string[];
}

// Envelope is the shape every /api/v1 response body carries now, success
// or failure — see backend/internal/httpapi/envelope.
export interface Envelope<T> {
  data: T;
  message: string;
  error: string | null;
  status_code: number;
}

export type ChannelStatus = "linked" | "disconnected";

export interface AnalyticsSummary {
  total_watches: number;
  enabled_watches: number;
  stale_watches: number;
  avg_delta_7d_pct?: number;
}

// Mirrors backend/internal/httpapi/handlers/watchRequest — the body
// shape CreateWatch/UpdateWatch accept.
export interface WatchInput {
  name: string;
  kind: AssetKind;
  enabled?: boolean;
  cron_expr: string;
  timezone: string;
  params: FlightParams;
  threshold_pct: number;
}
