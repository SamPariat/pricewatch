# Travel Price Reminder — Implementation Plan

## Context

A self-hosted service that watches travel prices (flights one-way/return, hotels, rentals) for a configurable set of routes and date windows, and pushes a daily digest to a **Telegram group** shared with friends — plus a **mobile-responsive Next.js panel** for you to configure watches and see how prices have moved.

Greenfield: `/Users/sampariat/Documents/prices-reminder` is empty. Go 1.26.1 and Docker 27.3.1 are installed locally.

Three research findings shaped this plan:

1. **Amadeus Self-Service is dead** — decommissioned **July 17, 2026**, existing keys stopped working. The free flight API every tutorial assumes no longer exists. Replacement: **Travelpayouts**, affiliate-funded (free to call), covering **both** flights (Aviasales) and hotels (Hotellook) under one token.
2. **Fiber v3 is stable and the handler signature changed.** `fiber.Ctx` is a *value* type now, not `*fiber.Ctx`, and it satisfies `context.Context` directly — `c.UserContext()` is gone. Every v2 example will fail to compile.
3. **Telegram removes the single biggest risk in the original design.** whatsmeow is reverse-engineered and a May 2025 ban wave hit low-volume legitimate users. Switching deletes ban risk, the spare SIM, the session store, QR pairing, and the persistent websocket — an entire phase of work and the plan's largest unknown, gone.

### Decisions locked

| Area | Decision |
|---|---|
| Messaging | **Telegram Bot API** behind a `Notifier` port |
| Price data | **Travelpayouts** — Aviasales (flights) + Hotellook (hotels); Airbnb stubbed |
| Panel | **Next.js server-first** (RSC + Server Actions), mobile-responsive, installable as a PWA |
| API | Fiber v3 JSON, **header-based versioning** |
| Access | **Single admin** (you) configures; friends only receive |
| Deployment | **Single VM, Docker Compose** — Fiber + Postgres + Next.js + Caddy |
| AI | Optional `LLM` port; **Gemini Flash** free tier, local Ollama fallback |
| Design | **Claude Design canvas** — web + mobile artboards before any UI code |
| Observability | **`log/slog`** structured JSON, run-ID correlation, redaction by type |
| Freshness | **`last_updated_at` surfaced everywhere** — UI, API headers, digest footer |

Native apps were considered and dropped: Firebase App Distribution on iOS requires an Apple Developer membership ($99/yr) plus per-device UDID registration. A responsive PWA installs to the home screen on both platforms for free and needs no distribution pipeline at all.

**These decisions are settled. Do not relitigate them** — each replaced an alternative for a specific researched reason recorded above and in Known Gaps.

---

## Prerequisites

Obtain before Phase 1; each is free and takes minutes:

| Item | How | Env var |
|---|---|---|
| Telegram bot token | Message `@BotFather` → `/newbot` | `TELEGRAM_BOT_TOKEN` |
| Telegram chat ID | Add bot to the group, then `GET https://api.telegram.org/bot<token>/getUpdates` and read `message.chat.id` (negative for groups) | stored in `settings` |
| Travelpayouts token | Sign up at travelpayouts.com → Profile → API token | `TRAVELPAYOUTS_TOKEN` |
| Gemini key *(optional)* | aistudio.google.com → Get API key | `GEMINI_API_KEY` |
| VM + domain | Any always-on Linux host; domain for Caddy auto-TLS | `APP_DOMAIN` |

Also: `ADMIN_PASSWORD_HASH` (bcrypt), `SESSION_SECRET`, `DATABASE_URL`. All of `TELEGRAM_BOT_TOKEN`, `TRAVELPAYOUTS_TOKEN`, `GEMINI_API_KEY`, `SESSION_SECRET` must be declared as the `Secret` type — see Logging.

---

## Architecture

One always-on Go process owns cron, provider fetchers, the Telegram bot, and the JSON API. Postgres holds config, quotes, and rollups. Next.js renders server-side against the API over the internal Docker network. Caddy terminates TLS.

```
   Telegram  ◀──── outbound only (sendMessage / sendPhoto)
      ▲              long-poll getUpdates — no inbound port needed
      │
   ┌──┴──────────── single VM ────────────────┐
   │  Next.js ──internal──▶ Fiber v3 API      │
   │                          │      │        │
   │                          ▼      ▼        │
   │                     Postgres   cron ─────┼──▶ Travelpayouts
   └───────────▲──────────────────────────────┘
               │ 443 (Caddy, TLS)
            browser / home-screen PWA
```

Telegram's long polling means the bot needs **no public IP and no inbound firewall rule**. That also removes the technical blocker that ruled out serverless earlier — the VM still wins, because Vercel Hobby caps cron at once-per-day with a 300s function limit.

---

## Architecture patterns

**Ports and adapters (hexagonal)**, justified specifically because the two most volatile parts — where prices come from, how messages get delivered — are both external and both likely to be swapped. That's the reason, not fashion.

**Ports:**

```go
type Provider interface {
	Kind() domain.AssetKind
	Fetch(ctx context.Context, w domain.Watch) ([]domain.Quote, error)
}

type Notifier interface {
	Send(ctx context.Context, t Target, m Message) error
	Status(ctx context.Context) (Status, error)
}

type Repository interface { /* watches, quotes, samples, runs */ }
type Cache interface { GetOrLoad(key string, ttl time.Duration, fn Loader) ([]byte, error) }
type LLM interface { Complete(ctx context.Context, p Prompt) (string, error) }
type Clock interface { Now() time.Time }   // makes analytics testable
```

**Adapters:** `aviasales`, `hotellook`, `rental` (stub) · `telegram`, `noop` · `postgres` · `ristretto` · `gemini`, `ollama`.

The domain core — analytics, rendering, pipeline orchestration — imports only ports. No knowledge of HTTP, Postgres, or Telegram, so it tests with plain fakes and no Docker.

### Patterns in play

| Pattern | Where | Why it earns its place |
|---|---|---|
| **Ports & Adapters** | Overall shape | Providers and notifier are expected to be swapped |
| **Strategy** | `Provider`, `Notifier`, `LLM` | Implementation chosen by config at wire-up |
| **Registry** | `providers.Registry` by `AssetKind` | Adding a source = register, not edit a switch |
| **Decorator** | Resilience/cache stack (below) | Highest-value pattern here |
| **Repository** | `store` | Keeps SQL out of the domain |
| **Pipeline** | fetch→normalize→persist→analyze→render→send | Each stage independently testable |
| **Circuit Breaker + Retry** | Provider decorators | One dead upstream must not stall a digest |
| **Null Object** | `noop` notifier, `rental` stub | Dry-run is *swapping an adapter*, not an `if` |
| **Presenter** | Versioned DTOs | One domain, N wire formats — see Versioning |
| **Builder** | Message rendering | Composing digest sections under a length cap |
| **Observer** | Scheduler reload on watch change | Cron reflects DB without restart |

The decorator stack is the standout. Every layer satisfies `Provider`, so resilience composes without touching any adapter:

```go
p := aviasales.New(token)
p = providers.WithSingleflight(p)   // collapse concurrent identical fetches
p = providers.WithCache(p, cache)   // TTL = time until next cron fire
p = providers.WithRateLimit(p, 300) // Travelpayouts: 300 RPM
p = providers.WithBreaker(p)
p = providers.WithRetry(p, 3)
p = providers.WithMetrics(p)
```

**Where not to abstract.** No port for anything with a single implementation and no test-double need — no repository-per-table, no interface over the analytics functions (they're pure; call them directly). Six ports is the target, not fifteen. Hexagonal earns its keep at genuine external boundaries and becomes ceremony everywhere else.

---

## Layout

```
prices-reminder/
├── docker-compose.yml · Caddyfile · .env.example · Makefile
├── backend/
│   ├── cmd/server/main.go            # composition root: ONLY place adapters are chosen
│   ├── migrations/                   # goose
│   └── internal/
│       ├── domain/                   # entities + ports. No external imports.
│       ├── analytics/                # pure functions over price_samples
│       ├── pipeline/                 # orchestration
│       ├── render/                   # Telegram message + chart-image rendering
│       ├── scheduler/                # robfig/cron v3; owns NextRun()/TTL()
│       ├── providers/                # provider.go + decorators + adapters
│       ├── notify/                   # notifier.go + telegram/ noop/
│       ├── ai/                       # llm.go + gemini/ ollama/
│       ├── logging/                  # slog setup, ctx helpers, redaction types
│       ├── cache/ store/ config/
│       └── httpapi/
│           ├── router.go  middleware/  handlers/
│           └── presenter/            # v1/ v2/ — versioned DTOs
├── web/                              # Next.js App Router, Tailwind, shadcn/ui, Recharts
└── design/                           # *.dc.html artboards (Claude Design canvas)
```

---

## API versioning via headers

**Request:** `X-API-Version: 1` — chosen over `Accept: application/vnd.…+json` because it's trivially set as a default header in the fetch wrapper and far easier to read in logs. Media-type versioning is more RESTful and buys nothing here.

```go
func APIVersion(min, max int) fiber.Handler {
	return func(c fiber.Ctx) error {
		v := parseOr(c.Get("X-API-Version"), min)   // absent → OLDEST supported
		if v < min || v > max {
			return fiber.NewError(fiber.StatusBadRequest, "unsupported API version")
		}
		c.Locals("apiVersion", v)
		c.Set("X-API-Version", strconv.Itoa(v))
		if v < max {
			c.Set("Deprecation", "true")
			c.Set("Sunset", sunsetFor(v))           // RFC 8594
		}
		return c.Next()
	}
}
```

Two rules keep this from rotting:

1. **Absent header defaults to the *oldest* supported version, never the latest.** Defaulting to latest silently breaks every client that predates the header — the exact failure versioning exists to prevent.
2. **Version at the presenter boundary, not by forking handlers.** One handler, one domain object, N presenters: `presenter.V1(watch)` / `presenter.V2(watch)`. Duplicating handlers per version is how versioned APIs become unmaintainable — business logic drifts between the copies. This also falls out of hexagonal naturally: presenters are just another adapter.

`GET /api/meta` returns `{min_version, max_version}` — useful for a health dashboard, and the hook you'd need if a second client ever appears.

Since the only client is server-rendered Next.js deployed alongside the API, version skew is short-lived here. Build it anyway — it costs one middleware and one package now, versus a painful retrofit later.

---

## Data model

| Table | Purpose |
|---|---|
| `watches` | `id, name, kind, enabled, cron_expr, timezone, params JSONB, threshold_pct` |
| `quotes` | `watch_id, fetched_at, provider, price_minor, currency, depart_date, return_date, stops, carrier, deep_link, fingerprint, raw JSONB` |
| `price_samples` | Daily rollup: `watch_id, sample_date, min_minor, median_minor, max_minor, n_quotes, updated_at` |
| `digest_runs` | `run_id, watch_id, started_at, finished_at, status, message_body, error` |
| `run_events` | Per-run timeline: `run_id, at, stage, level, msg, fields JSONB` |
| `watch_state` | `watch_id, last_success_at, last_attempt_at, last_error, consecutive_failures` |
| `settings` | Singleton: `telegram_chat_id`, quiet hours, currency, dry-run flag |

No `users` table — single admin, and recipients are just a Telegram chat ID. That's the benefit of the access decision: it removes an entire subsystem.

`Watch.Params` is `JSONB` in the DB and a discriminated struct in Go keyed on `Kind`. This is what makes configurability work without a migration per knob.

- **Unique index** on `quotes(watch_id, provider, depart_date, return_date, fingerprint, fetched_date)` so retries are idempotent.
- **`price_samples` is both analytics backbone and cache tier** — charts and deltas read only this table, so raw `quotes` can be pruned after ~90 days.
- **Backfill on creation:** `/v1/prices/calendar` returns a month of cheapest-per-day in one call. Seed `price_samples` at watch creation, or every new watch shows an empty chart for a week.

---

## Caching

Your governing insight: **data only changes when the cron fires**, so TTLs derive from the schedule rather than being guessed. `robfig/cron` exposes `Entry.Next`, so one function is the single source of truth for every TTL in the system:

```go
func (s *Scheduler) NextRun(watchID string) time.Time
func (s *Scheduler) TTL(watchID string) time.Duration {
	return clamp(time.Until(s.NextRun(watchID)), 30*time.Second, 6*time.Hour)
}
```

Six layers, outermost first. A user hammering refresh is stopped at layer 2 and never reaches Travelpayouts.

| # | Layer | Mechanism |
|---|---|---|
| 1 | **Rate limit** | Fiber v3 `limiter`, sliding window, per-session key |
| 2 | **Browser** | `etag` middleware + `Cache-Control: private, max-age=<TTL>` → 304, or no request at all |
| 3 | **Next.js Data Cache** | `fetch(url, { next: { tags: ['watch-123'], revalidate: TTL } })` |
| 4 | **HTTP response** | Fiber v3 `cache` middleware with `ExpirationGenerator` |
| 5 | **In-process** | `ristretto` behind the `Cache` port, wrapping `Provider` |
| 6 | **`singleflight`** | Collapses concurrent identical upstream calls into one |

Three details worth calling out.

**Fiber v3's cache middleware takes `ExpirationGenerator func(fiber.Ctx, *Config) time.Duration`** — a per-request TTL hook, exactly the shape needed:

```go
app.Use(cache.New(cache.Config{
	ExpirationGenerator: func(c fiber.Ctx, _ *cache.Config) time.Duration {
		return sched.TTL(c.Params("id"))   // expires precisely when new data lands
	},
}))
```

**Push invalidation beats waiting for TTL expiry.** After a cron run completes, Go POSTs to a Next.js route handler that calls `revalidateTag("watch-123")`. So you set generous TTLs *and* see fresh data the instant it exists — no polling, no stale window, and cache correctness stops depending on clock alignment between two services. Also send `X-Next-Run: <RFC3339>` on watch-scoped responses so the `revalidate` value is derived from the same `Scheduler.TTL`, never hardcoded.

**`singleflight`** (`golang.org/x/sync/singleflight`) is the direct answer to refresh-hammering: on a cold cache, 50 simultaneous requests for one watch produce **one** upstream call and 49 waiters. Ten lines, and it's what protects the 300 RPM budget under a thundering herd.

No Redis: one VM, one process, in-process is strictly faster and simpler. Redis only matters if you scale to multiple app containers.

---

## Freshness: `last_updated_at`

This matters more here than in a typical app, because there are **two independent sources of staleness** stacked on top of each other:

1. Your six cache layers, which deliberately hold data until the next cron fire.
2. Travelpayouts itself, which serves **cached** prices (~7-day retention), not live shopping results.

Without a visible timestamp, nobody can tell whether a price is two minutes or two days old — and that is precisely the confusion that ends with someone booking on a stale number. A displayed time converts an invisible risk into an informed judgement.

**One timestamp, propagated end to end.** `last_updated_at` is the `fetched_at` of the newest quote backing the displayed figure — *not* the time the row was cached, and not `now()`. Surface it in all four places:

| Surface | Form |
|---|---|
| API | `X-Data-Fetched-At: <RFC3339>` alongside `X-Next-Run`, plus `last_updated_at` in every presenter DTO |
| Panel | `Updated 6h ago · next check 7:00 AM` on each card; relative text, absolute on hover, `<time datetime>` for correctness |
| Telegram digest | Footer: `Prices as of 07:02 IST · indicative — tap to confirm` |
| Charts | Last point labelled; any gap in `price_samples` rendered as a gap, never interpolated |

**Staleness badge — the part that earns its keep.** `watch_state.last_success_at` lets the UI compare against the schedule: if the newest data is older than **2× the watch's cron interval**, render a warning and show `last_error`. A silently failing watch otherwise looks identical to a watch whose price simply hasn't moved — the most dangerous failure mode in a system like this, because it fails *quietly* and looks like good news ("prices are stable"). `consecutive_failures` also drives an admin-only Telegram alert after N strikes.

Render relative times **on the client** from an absolute ISO string. Computing "6h ago" server-side inside an RSC bakes it into a cached payload, and it will be wrong the moment the cache serves it again — a subtle bug worth avoiding by construction.

---

## Logging and observability

**`log/slog`** (stdlib, structured, zero dependencies): JSON handler in production, text in development, level from env.

**Run-ID correlation is the single most valuable piece.** Every cron run mints a ULID `run_id` that flows through the whole pipeline — fetch → normalize → persist → analyze → render → send — attached to a `*slog.Logger` carried in `context.Context`:

```go
ctx = logging.With(ctx, slog.String("run_id", runID), slog.String("watch_id", w.ID))
logging.From(ctx).Info("fetch complete", "provider", p.Name(), "quotes", len(qs), "ms", took)
```

That one decision is what makes "why didn't the digest arrive?" a solved problem: grep a single ID and the entire run's story appears in order. Without it you're correlating timestamps across six components by hand.

**Redaction by type, not by discipline.** Secrets get a `slog.LogValuer` that renders `[REDACTED]`:

```go
type Secret string
func (s Secret) LogValue() slog.Value { return slog.StringValue("[REDACTED]") }
```

Declare the Telegram bot token, Travelpayouts token, LLM key, and session secret as `Secret` and they become structurally unloggable — no reviewer has to remember. This matters because provider adapters naturally want to log request URLs, and the Travelpayouts token travels in a query param on some endpoints; **strip the query string before logging any provider URL.**

**Logging is a decorator**, consistent with the existing stack: `providers.WithLogging(p)` slots in beside `WithRetry` and `WithBreaker`, so no adapter contains logging code.

| Level | Use |
|---|---|
| `DEBUG` | Provider request/response shapes, cache hit/miss, TTL decisions |
| `INFO` | Run lifecycle, quote counts, message sent, config changes |
| `WARN` | Degraded but working — breaker open, retry, provider fallback, LLM timeout → template |
| `ERROR` | Run failed, send failed, migration failed |

**Access logs:** Fiber v3 `requestid` + `logger` middleware, request ID echoed as `X-Request-Id` so a panel error maps to a server log line.

**Persist a run timeline, not just text.** `run_events` rows give the panel a per-run timeline view — stage, duration, level, outcome. This is what turns the run-history screen from a status column into an actual debugging tool, and it's why logging and the "run history in the panel" suggestion are one feature.

**Practical:** log to **stdout** (12-factor) and let Docker rotate — set `max-size: 10m` / `max-file: 3` in Compose. Unbounded container logs filling the disk is a genuine way to kill a small VM, and the default is unbounded. Skip Loki/Grafana for now: `docker compose logs` plus the run timeline covers a single-VM, single-user system, and the observability stack would outweigh the app.

Expose `/healthz` (liveness) and `/readyz` (DB reachable, scheduler running, last run status) so Compose healthchecks are meaningful.

---

## Telegram

Free, no ban risk, no session store, no pairing flow. Beyond removing risk, it enables things WhatsApp couldn't:

- **Inline keyboard buttons** under each digest — `[Snooze 7d] [Pause] [Refresh]`. Far better than typed chat commands, and the single biggest UX upgrade in this change.
- **`sendPhoto`** — render the price-history chart to PNG server-side (`go-echarts` or `gonum/plot`) and attach it. A chart in the message is worth more than any table of numbers.
- **HTML parse mode**, not MarkdownV2 — the latter requires escaping ~18 reserved characters and is a reliable source of runtime formatting bugs.
- 4096-char limit per message; batch all watches into one and split on section boundaries.

Limits are irrelevant at this scale: ~30 msg/s globally, 20/min per group, 1/s per chat. A daily digest uses one. Incoming updates via `getUpdates` don't count toward the outgoing budget — set `timeout=30` for a single long-lived connection.

Bot token in env; `chat_id` captured once via `getUpdates` after adding the bot to the group, then stored in `settings`.

---

## Panel: server-first Next.js

Both containers share a VM, so **Server Components fetch from Fiber over the internal Docker network**. Two real wins: no CORS to configure, and the API credential stays server-side — it is never shipped to the browser. (This is the advantage a native app would have given up: a token compiled into an APK is extractable.)

"Entirely server-side" isn't quite achievable — four things must be client components:

| Client island | Why |
|---|---|
| Recharts price history | Needs DOM + interaction |
| Watch builder form | Dynamic fields keyed on `kind` |
| Run-now button | Optimistic UI + polling |
| Toasts / live run status | Client state |

So: **RSC for all read paths** (watch list, history, analytics, run log) with tag-based caching, **client islands for interaction**, **Server Actions for mutations** — each calling Fiber then `revalidateTag()` so the UI updates without a manual refetch.

**Auth:** single admin password → httpOnly, `Secure`, `SameSite=Lax` session cookie set by a Server Action. No JWT in browser storage, no token in client JS. About an hour of work for the whole thing, since there's exactly one user.

**Make it a PWA.** A web manifest plus a minimal service worker makes the panel installable to the home screen on both iOS and Android, with an app icon and no browser chrome — most of what a native app gives you here, for free, with no distribution pipeline, no store review, and no $99/yr. Mobile-first Tailwind throughout; this gets used on a phone.

---

## AI features

An `LLM` port wired in `main.go` like any other adapter, **entirely optional** — if unconfigured, every AI path falls back to deterministic templates. The digest must still send when the LLM is down.

What makes this cheap: **it runs once a day inside a cron job.** Latency is irrelevant, and volume is ~1–30 calls/day — orders of magnitude under any free tier.

**Ranked by value per unit of effort:**

1. **Digest copywriting.** Go computes every number; the LLM turns a stats struct into a punchy group message. Highest impact, lowest risk.
2. **Natural-language watch creation.** "cheap flights to Goa for the December long weekend" → structured `Watch` JSON via JSON-schema-constrained output. Works in the panel *and* as a Telegram command.
3. **Anomaly explanation with search grounding.** "Goa spiked 40%" → find the festival, holiday, or event. Genuinely LLM-shaped work statistics can't do.
4. **Weekly recap** across all watches — one narrative instead of seven number blocks.
5. **Destination discovery.** "4-day window, ₹15k" → three options from flexible-date data you already fetched.
6. **Conversational queries** in the group — the bot already receives messages, so transport is free.

**Explicitly not AI:** buy-vs-wait must be **statistics**. Percentile rank over trailing 90 days is correct, explainable, and free; an LLM asked to judge a price produces confident nonsense.

**Free models:**

| Option | Limits | Fit |
|---|---|---|
| **Gemini 2.5 Flash** (AI Studio) | **1,500 req/day**, 1M context, no card | **Default** — ~50× headroom |
| **Groq** (Llama 3.3 70B) | 30 req/min, 300+ tok/s | Fastest; good for chat commands |
| **OpenRouter** | 30+ free models, one key | Best for fallback routing |
| **Cerebras** | 1M tokens/day | Expiring credits, not a true free tier |
| **Ollama, local** | Unlimited, private | Viable with a quantized 3–7B model |

Recommend **Gemini Flash primary, Ollama fallback**. Because the work is batch, local inference being slow doesn't matter — and it's free, unlimited, and never sends your travel plans to a third party.

**Guardrails:**
- **Never let the model emit prices or numbers.** Compute in Go, pass as context, constrain to phrasing. This rule must not bend.
- **JSON schema** for anything parsed back into a `Watch`.
- **Treat provider strings as untrusted** — hotel names and descriptions come from a third party and land in prompts, a real injection path. Delimit them; never let model output trigger side effects unreviewed.
- **Cache LLM responses** on the same schedule-derived TTL, and always fall back to the template on error or timeout.

---

## Implementation phases

0. **Design canvas** — a Claude Design canvas with artboards for **both viewports**: mobile (375px) — watch list, watch detail with chart, watch builder, run timeline; and desktop (1280px) — dashboard, watch detail, settings. Plus the Telegram digest message mocked as it actually renders. Doing this first settles information hierarchy, the freshness/staleness treatment, and the empty and error states *before* any component exists — states that otherwise get invented ad hoc while wiring data. Published as an Artifact you can edit visually and export.
1. **Skeleton** — Go module, Fiber v3, pgxpool, goose, Compose, `slog` setup, `requestid`+`logger` middleware, `/healthz` + `/readyz`. Verify the v3 value-receiver signature compiles before building on it.
2. **Ports + domain** — entities, six interfaces, pure analytics with a fake `Clock`. No I/O, fully unit-tested.
3. **Providers** — registry, `aviasales`, `hotellook`, decorator stack. Table-driven tests against recorded fixtures; no live calls in tests.
4. **Scheduler + pipeline** — `robfig/cron/v3`, DB-driven with hot reload, `SkipIfStillRunning` to prevent overlap, `NextRun`/`TTL`. Mint the `run_id` here and thread it through context; every run writes `digest_runs` + `run_events` and updates `watch_state` either way.
5. **Analytics** — deltas, rolling min/median, all-time-low flag, **percentile rank over trailing 90 days**.
6. **Telegram notifier** — HTML parse mode, batched message, chart PNG via `sendPhoto`, inline keyboards, long-poll loop for button callbacks.
7. **API + versioning + caching + freshness** — routes below, `X-API-Version` middleware, presenters, all six cache layers, `X-Next-Run` + `X-Data-Fetched-At`, revalidation webhook into Next.
8. **Panel** — build to the Phase 0 artboards: RSC pages, client islands, Server Actions, cookie auth, PWA manifest, mobile-first Tailwind, freshness + staleness badges, run timeline.
9. **AI (optional)** — `LLM` port, Gemini adapter, digest copywriting then NL watch creation.
10. **Deploy** — multi-stage distroless Dockerfile, Compose with named volumes, Caddy auto-TLS, nightly `pg_dump`.

```
GET/POST/PATCH/DELETE /api/watches
POST   /api/watches/:id/run          # honours dry-run
GET    /api/watches/:id/history      # price_samples → chart
GET    /api/analytics/summary
GET    /api/runs
GET/PATCH /api/settings
GET    /api/meta                     # {min_version, max_version}
POST   /api/auth/login
```

Middleware: `recover`, `requestid`, `logger`, `limiter`, `etag`, `cache`, `APIVersion`, session auth.
Also `GET /api/runs/:run_id/events` for the run timeline.

---

## Further suggestions

1. **Threshold alerts** on top of the daily digest — a drop past `threshold_pct` pings immediately. Turns a newsletter into something that catches deals.
2. **Flexible-date heatmap.** The calendar endpoint already returns a month per call — the data is free. "Cheapest day to fly" often beats any single quoted price.
3. **Affiliate deep links.** Travelpayouts is affiliate-funded; include your marker and it can pay for its own VM.
4. **Dry-run mode** (the `noop` notifier) — essential while tuning copy, and it keeps the group free of dev spam.
5. **Quiet hours + one batched message.** Nothing kills a group chat faster than six separate 7am bot messages.
6. **Multi-currency + FX normalization** so mixed-currency watches compare honestly.
7. **Run history in the panel** — when a digest doesn't arrive, you need to know whether it was fetch, render, or Telegram.

Deferred: price prediction, group polls, multi-user accounts.

---

## Known gaps and risks

**Travelpayouts prices are cached, not live-shopped** (~7-day retention). Directionally accurate, not bookable. The digest must label them indicative and treat the deep link as truth — otherwise someone books on a stale number. With whatsmeow gone, this is the plan's largest remaining caveat.

**Airbnb has no official API.** The `rental` adapter ships as a registered stub returning "not configured", so the port and UI are ready. Filling it in means an unofficial RapidAPI wrapper or Apify actor (~$50/mo), both brittle. Flagged rather than silently dropped, since it was in your original requirements.

**Host:** if you use Oracle Always Free, note they cut limits (4 OCPU/24GB → 2 OCPU/12GB) on June 15 2026 with no announcement and reclaim idle instances. 2/12 is ample here. Telegram's outbound-only model means a changing IP no longer matters — another benefit of the switch.

---

## Traps a fresh session will fall into

Every one of these is something a model working from training data or a web search will get wrong by default. Check them explicitly.

1. **Fiber v3 handlers take `fiber.Ctx` by value, not `*fiber.Ctx`.** Nearly every Fiber example online is v2 and will not compile. `c.UserContext()` no longer exists — `fiber.Ctx` *is* a `context.Context`, so pass `c` directly into provider calls.
2. **Amadeus Self-Service is dead** (portal decommissioned July 17 2026; keys stopped working). It is the default suggestion in most flight-API tutorials. Do not reach for it. Use Travelpayouts.
3. **The `X-API-Version` default must be the oldest supported version, not the latest.** Defaulting to latest breaks exactly the clients versioning exists to protect.
4. **Use Telegram HTML parse mode, not MarkdownV2.** MarkdownV2 requires escaping ~18 reserved characters; unescaped hotel names and prices will throw 400s at runtime, intermittently, in production only.
5. **Never render relative time ("6h ago") server-side in an RSC.** It gets baked into the cached payload and is wrong on the next cache hit. Send absolute ISO, format on the client.
6. **The Travelpayouts token travels in a query parameter** on some endpoints. Strip query strings before logging any provider URL, or the token lands in your logs.
7. **Docker's default logging is unbounded.** Set `max-size`/`max-file` in Compose or logs will eventually fill a small VM's disk.
8. **Do not let the LLM produce any number.** Compute prices, deltas, and percentiles in Go and pass them in as context; constrain the model to phrasing only.
9. **Travelpayouts prices are cached, not live.** Every surface that shows a price must also show `last_updated_at` and label it indicative.
10. **Version at the presenter boundary; never fork handlers per version.** Forked handlers drift and are how versioned APIs rot.

---

## Verification

1. `docker compose up` → `curl localhost:8080/healthz` returns 200; migrations applied.
2. `go test ./...` → domain/analytics pass with fakes; providers pass against fixtures.
3. Create a watch (BLR→GOA, return, Dec 10–15) → row written, backfill populates `price_samples` immediately.
4. `POST /api/watches/:id/run` **with dry-run on** → digest renders in the run log, `quotes` written, nothing sent to Telegram.
5. **Versioning:** no header → served as v1. `X-API-Version: 1` → 200 with the version echoed. `X-API-Version: 99` → 400. Once v2 exists, a v1 request returns `Deprecation` + `Sunset` and an unchanged v1 body shape.
6. **Caching:** hit `/api/watches/:id/history` twice → second is a 304, with `Cache-Control` and `X-Next-Run` matching the next cron fire. Hammer it 50× concurrently on a cold cache → provider metrics show **one** upstream call. Trigger a run → the revalidation webhook fires and the RSC page shows new data without a hard refresh.
7. Dry-run off, run again → message lands in the Telegram group, HTML formatting correct, chart image attached, under 4096 chars.
8. Tap an inline button (`Snooze 7d`) → callback handled, watch updated, confirmation edited into the message.
9. Set a cron to `*/2 * * * *` temporarily → fires on schedule; two overlapping slow runs don't double-send.
10. Insert backdated `price_samples` → chart renders, percentile and delta lines read correctly.
11. **AI:** unset the LLM key → digest still sends via template fallback. Set it → prose improves and **every number matches the Go-computed stats exactly**.
12. **PWA:** open the panel on a phone, "Add to Home Screen" → launches standalone with an icon; layouts hold at 375px and match the Phase 0 artboards.
13. **Freshness:** every card and the digest footer show a timestamp derived from the newest backing quote. Stop the scheduler and wait past 2× the interval → the staleness badge appears with `last_error`, and the price does *not* silently present as current.
14. **Logging:** run a digest, take the `run_id` from `digest_runs`, and grep it → the full fetch→send story appears in order. `GET /api/runs/:run_id/events` renders the same timeline in the panel. Confirm no log line contains the Telegram or Travelpayouts token (`docker compose logs | grep -i <token-prefix>` returns nothing), including inside provider request URLs.
15. Kill the network briefly → the run records as failed, `consecutive_failures` increments, `last_error` is populated, and it does not hang.

---

## Sources

- [Amadeus Self-Service pricing/decommission](https://developers.amadeus.com/pricing) · [What's New in Fiber v3](https://docs.gofiber.io/blog/whats-new-in-fiber-v3/) · [Fiber v3.0.0](https://github.com/gofiber/fiber/releases/tag/v3.0.0)
- [Fiber v3 cache middleware](https://pkg.go.dev/github.com/gofiber/fiber/v3/middleware/cache) · [Fiber v3 limiter](https://pkg.go.dev/github.com/gofiber/fiber/v3/middleware/limiter)
- [Aviasales flight data API](https://support.travelpayouts.com/hc/en-us/sections/201008338-Aviasales-flight-data-API) · [Hotellook hotels data API](https://support.travelpayouts.com/hc/en-us/articles/115000343268-Hotels-data-API)
- [Telegram Bots FAQ (rate limits)](https://core.telegram.org/bots/faq) · [Telegram bot rate limits 2026](https://pipsync.io/en/blog/telegram-rate-limits)
- [EAS internal distribution](https://docs.expo.dev/build/internal-distribution/) (why native was dropped)
- [Free LLM API tiers compared 2026](https://openrouter.ai/blog/tutorials/free-llm-apis-compared/) · [Free LLM tiers: Groq, Cerebras, Mistral](https://ianlpaterson.com/blog/free-llm-api-2026/)
