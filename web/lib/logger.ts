import "server-only";

// Mirrors internal/logging on the Go side: structured JSON to stdout in
// production (Docker's log driver already rotates it — see
// docker-compose.yml), readable text in development. debug is suppressed
// in production the same way LOG_LEVEL=info suppresses it on the backend
// by default — this side has no separate LOG_LEVEL knob since call volume
// here is just the admin panel's own traffic, not worth the extra config.
type Level = "debug" | "info" | "warn" | "error";
type Fields = Record<string, unknown>;

export interface Logger {
  debug(msg: string, fields?: Fields): void;
  info(msg: string, fields?: Fields): void;
  warn(msg: string, fields?: Fields): void;
  error(msg: string, fields?: Fields): void;
}

// ConsoleLogger is the one real implementation — a class, not a plain
// object, so it can be constructor-injected into ApiClient (see
// lib/api.ts) and swapped for a test double without touching call sites.
export class ConsoleLogger implements Logger {
  private readonly isProd = process.env.NODE_ENV === "production";

  debug(msg: string, fields: Fields = {}) {
    this.emit("debug", msg, fields);
  }
  info(msg: string, fields: Fields = {}) {
    this.emit("info", msg, fields);
  }
  warn(msg: string, fields: Fields = {}) {
    this.emit("warn", msg, fields);
  }
  error(msg: string, fields: Fields = {}) {
    this.emit("error", msg, fields);
  }

  private emit(level: Level, msg: string, fields: Fields) {
    if (level === "debug" && this.isProd) return;

    if (this.isProd) {
      console.log(JSON.stringify({ time: new Date().toISOString(), level: level.toUpperCase(), msg, ...fields }));
      return;
    }
    const extra = Object.entries(fields)
      .map(([k, v]) => `${k}=${typeof v === "string" ? v : JSON.stringify(v)}`)
      .join(" ");
    console.log(`${new Date().toISOString()} ${level.toUpperCase()} ${msg}${extra ? " " + extra : ""}`);
  }
}

// bodyPreview caps how much of a response body lands in a log line —
// enough to see a real Fiber error message, not so much that one bad
// call floods the log. Matches internal/logging.HTTPResponse's warn-level
// cap on the Go side.
export function bodyPreview(text: string, n = 2000): string {
  return text.length > n ? text.slice(0, n) + "...(truncated)" : text;
}
