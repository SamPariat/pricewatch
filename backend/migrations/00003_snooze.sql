-- +goose Up
ALTER TABLE watch_state ADD COLUMN snoozed_until timestamptz;

-- +goose Down
ALTER TABLE watch_state DROP COLUMN snoozed_until;
