-- +goose Up
ALTER TABLE messages ADD COLUMN is_summary_message INTEGER DEFAULT 0 NOT NULL;

-- +goose Down
ALTER TABLE messages DROP COLUMN is_summary_message;
