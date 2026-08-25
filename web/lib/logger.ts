import "server-only";

// Mirrors internal/logging on the Go side: structured JSON to stdout in
// production (Docker's log driver already rotates it — see
// docker-compose.yml), readable text in development. debug is suppressed
// in production the same way LOG_LEVEL=info suppresses it on the backend
// by default — this side has no separate LOG_LEVEL knob since call volume
// here is just the admin panel's own traffic, not worth the extra config.
const isProd = process.env.NODE_ENV === "production";

type Level = "debug" | "info" | "warn" | "error";
type Fields = Record<string, unknown>;

function emit(level: Level, msg: string, fields: Fields) {
  if (level === "debug" && isProd) return;

  if (isProd) {
    console.log(JSON.stringify({ time: new Date().toISOString(), level: level.toUpperCase(), msg, ...fields }));
    return;
  }
  const extra = Object.entries(fields)
    .map(([k, v]) => `${k}=${typeof v === "string" ? v : JSON.stringify(v)}`)
    .join(" ");
  console.log(`${new Date().toISOString()} ${level.toUpperCase()} ${msg}${extra ? " " + extra : ""}`);
}

export const logger = {
  debug: (msg: string, fields: Fields = {}) => emit("debug", msg, fields),
  info: (msg: string, fields: Fields = {}) => emit("info", msg, fields),
  warn: (msg: string, fields: Fields = {}) => emit("warn", msg, fields),
  error: (msg: string, fields: Fields = {}) => emit("error", msg, fields),
};

// bodyPreview caps how much of a response body lands in a log line —
// enough to see a real Fiber error message, not so much that one bad
// call floods the log. Matches internal/logging.HTTPResponse's warn-level
// cap on the Go side.
export function bodyPreview(text: string, n = 2000): string {
  return text.length > n ? text.slice(0, n) + "...(truncated)" : text;
}
