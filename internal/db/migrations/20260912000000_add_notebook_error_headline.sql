-- +goose Up
ALTER TABLE notebook_entries ADD COLUMN error_headline TEXT DEFAULT '' NOT NULL;

-- +goose Down
ALTER TABLE notebook_entries DROP COLUMN error_headline;
