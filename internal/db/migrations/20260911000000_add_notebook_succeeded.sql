-- +goose Up
ALTER TABLE notebook_entries ADD COLUMN succeeded INTEGER DEFAULT 1 NOT NULL;

-- +goose Down
ALTER TABLE notebook_entries DROP COLUMN succeeded;
