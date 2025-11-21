-- +goose Up
ALTER TABLE messages ADD COLUMN metadata TEXT DEFAULT '{}' NOT NULL;

-- +goose Down
ALTER TABLE messages DROP COLUMN metadata;
