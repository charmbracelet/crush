-- +goose Up
-- +goose StatementBegin

-- Add archived_at column to sessions table.
-- When set, the session is archived and its snapshots can be garbage collected.
ALTER TABLE sessions ADD COLUMN archived_at INTEGER;

CREATE INDEX IF NOT EXISTS idx_sessions_archived_at ON sessions(archived_at);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_sessions_archived_at;

-- SQLite doesn't support DROP COLUMN directly, but goose handles this
-- by recreating the table. For simplicity, we leave the column.

-- +goose StatementEnd
