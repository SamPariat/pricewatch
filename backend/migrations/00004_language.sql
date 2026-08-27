-- +goose Up
ALTER TABLE settings ADD COLUMN language text NOT NULL DEFAULT 'en';

-- +goose Down
ALTER TABLE settings DROP COLUMN language;
