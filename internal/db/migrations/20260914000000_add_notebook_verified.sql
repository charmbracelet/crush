-- +goose Up
ALTER TABLE notebook_entries ADD COLUMN verified TEXT DEFAULT '' NOT NULL;

-- +goose Down
ALTER TABLE notebook_entries DROP COLUMN verified;
