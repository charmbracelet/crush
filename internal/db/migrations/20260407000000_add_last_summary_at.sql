-- Add last_summary_at column to sessions to track when summarization last occurred.
-- This prevents repeated summarization in rapid succession.

-- +goose Up
-- +goose StatementBegin
ALTER TABLE sessions ADD COLUMN last_summary_at INTEGER;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE sessions DROP COLUMN last_summary_at;
-- +goose StatementEnd
