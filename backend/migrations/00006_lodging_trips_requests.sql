-- +goose Up
CREATE TABLE trips (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name       text NOT NULL,
    cron_expr  text NOT NULL,
    timezone   text NOT NULL DEFAULT 'UTC',
    enabled    boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE watches ADD COLUMN trip_id uuid REFERENCES trips(id) ON DELETE CASCADE;

-- Backfill: every existing watch becomes the sole leg of a new trip that
-- carries its old schedule over, so cron_expr/timezone can move to trips
-- without losing any watch's schedule.
-- +goose StatementBegin
DO $$
DECLARE
    w RECORD;
    new_trip_id uuid;
BEGIN
    FOR w IN SELECT id, name, cron_expr, timezone, enabled, created_at FROM watches LOOP
        INSERT INTO trips (name, cron_expr, timezone, enabled, created_at)
        VALUES (COALESCE(NULLIF(w.name, ''), 'Trip'), w.cron_expr, w.timezone, w.enabled, w.created_at)
        RETURNING id INTO new_trip_id;

        UPDATE watches SET trip_id = new_trip_id WHERE id = w.id;
    END LOOP;
END $$;
-- +goose StatementEnd

ALTER TABLE watches ALTER COLUMN trip_id SET NOT NULL;
ALTER TABLE watches DROP COLUMN cron_expr;
ALTER TABLE watches DROP COLUMN timezone;
-- watches.enabled is kept — it's now the per-leg pause, distinct from
-- trips.enabled (the whole-trip pause).

CREATE TABLE requests (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    kind        text NOT NULL,
    payload     jsonb NOT NULL,
    status      text NOT NULL DEFAULT 'pending',
    note        text NOT NULL DEFAULT '',
    created_at  timestamptz NOT NULL DEFAULT now(),
    resolved_at timestamptz
);
CREATE INDEX requests_status_idx ON requests (status, created_at);

-- +goose Down
DROP TABLE requests;

ALTER TABLE watches ADD COLUMN cron_expr text;
ALTER TABLE watches ADD COLUMN timezone text NOT NULL DEFAULT 'UTC';
UPDATE watches SET cron_expr = t.cron_expr, timezone = t.timezone
    FROM trips t WHERE t.id = watches.trip_id;
ALTER TABLE watches ALTER COLUMN cron_expr SET NOT NULL;
ALTER TABLE watches DROP COLUMN trip_id;
DROP TABLE trips;
