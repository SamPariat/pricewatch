-- +goose Up
-- Splits the single "Note" summary requests carried before into a short
-- Title (what/where — the heading the panel renders) and a Note that
-- stays a supporting detail line (typically the raw URL, which can be
-- long and shouldn't be what a reviewer reads first).
ALTER TABLE requests ADD COLUMN title text NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE requests DROP COLUMN title;
