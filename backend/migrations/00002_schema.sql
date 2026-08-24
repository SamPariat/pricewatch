-- +goose Up

CREATE TABLE watches (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name          text NOT NULL DEFAULT '',
    kind          text NOT NULL,
    enabled       boolean NOT NULL DEFAULT true,
    cron_expr     text NOT NULL,
    timezone      text NOT NULL DEFAULT 'UTC',
    params        jsonb NOT NULL,
    threshold_pct double precision NOT NULL DEFAULT 15,
    created_at    timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE quotes (
    id            bigserial PRIMARY KEY,
    watch_id      uuid NOT NULL REFERENCES watches(id) ON DELETE CASCADE,
    fetched_at    timestamptz NOT NULL,
    -- The UTC calendar date of fetched_at, computed in Go and stored
    -- explicitly rather than as an expression index: timestamptz::date
    -- depends on the session's TimeZone setting, so Postgres refuses it
    -- as IMMUTABLE and won't allow it in an index expression at all.
    fetched_date  date NOT NULL,
    provider      text NOT NULL,
    price_minor   bigint NOT NULL,
    currency      text NOT NULL,
    depart_date   text NOT NULL,
    return_date   text NOT NULL DEFAULT '',
    stops         int NOT NULL DEFAULT 0,
    carrier       text NOT NULL DEFAULT '',
    deep_link     text NOT NULL DEFAULT '',
    fingerprint   text NOT NULL,
    raw           jsonb
);

-- Retries are idempotent: refetching the same (watch, provider, dates,
-- fingerprint) on the same calendar day upserts nothing new.
CREATE UNIQUE INDEX quotes_dedup_idx ON quotes (
    watch_id, provider, depart_date, return_date, fingerprint, fetched_date
);
CREATE INDEX quotes_watch_fetched_idx ON quotes (watch_id, fetched_at DESC);

CREATE TABLE price_samples (
    watch_id     uuid NOT NULL REFERENCES watches(id) ON DELETE CASCADE,
    sample_date  date NOT NULL,
    min_minor    bigint NOT NULL,
    median_minor bigint NOT NULL,
    max_minor    bigint NOT NULL,
    n_quotes     int NOT NULL,
    updated_at   timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (watch_id, sample_date)
);

CREATE TABLE digest_runs (
    run_id       text PRIMARY KEY,
    watch_id     uuid NOT NULL REFERENCES watches(id) ON DELETE CASCADE,
    started_at   timestamptz NOT NULL,
    finished_at  timestamptz,
    status       text NOT NULL DEFAULT 'pending',
    message_body text NOT NULL DEFAULT '',
    error        text NOT NULL DEFAULT ''
);
CREATE INDEX digest_runs_watch_idx ON digest_runs (watch_id, started_at DESC);

CREATE TABLE run_events (
    id       bigserial PRIMARY KEY,
    run_id   text NOT NULL REFERENCES digest_runs(run_id) ON DELETE CASCADE,
    at       timestamptz NOT NULL DEFAULT now(),
    stage    text NOT NULL,
    level    text NOT NULL,
    msg      text NOT NULL,
    fields   jsonb
);
CREATE INDEX run_events_run_idx ON run_events (run_id, at);

CREATE TABLE watch_state (
    watch_id              uuid PRIMARY KEY REFERENCES watches(id) ON DELETE CASCADE,
    last_success_at       timestamptz,
    last_attempt_at       timestamptz,
    last_error            text NOT NULL DEFAULT '',
    consecutive_failures  int NOT NULL DEFAULT 0
);

-- Singleton: exactly one settings row, enforced by the fixed id.
CREATE TABLE settings (
    id                 boolean PRIMARY KEY DEFAULT true CHECK (id),
    telegram_chat_id   text NOT NULL DEFAULT '',
    quiet_hours_start  text NOT NULL DEFAULT '',
    quiet_hours_end    text NOT NULL DEFAULT '',
    currency           text NOT NULL DEFAULT 'INR',
    dry_run            boolean NOT NULL DEFAULT false
);
INSERT INTO settings (id) VALUES (true);

-- +goose Down
DROP TABLE settings;
DROP TABLE watch_state;
DROP TABLE run_events;
DROP TABLE digest_runs;
DROP TABLE price_samples;
DROP TABLE quotes;
DROP TABLE watches;
