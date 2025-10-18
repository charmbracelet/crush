-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS command_history (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    command TEXT NOT NULL,
    created_at INTEGER NOT NULL,  -- Unix timestamp in milliseconds
    updated_at INTEGER NOT NULL,  -- Unix timestamp in milliseconds
    FOREIGN KEY (session_id) REFERENCES sessions (id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_command_history_session_id ON command_history(session_id);
CREATE INDEX IF NOT EXISTS idx_command_history_created_at ON command_history(created_at);

CREATE TRIGGER IF NOT EXISTS update_command_history_updated_at
AFTER UPDATE ON command_history
BEGIN
    UPDATE command_history SET updated_at = strftime('%s', 'now')
    WHERE id = new.id;
END;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS update_command_history_updated_at;
DROP INDEX IF EXISTS idx_command_history_created_at;
DROP INDEX IF EXISTS idx_command_history_session_id;
DROP TABLE IF EXISTS command_history;
-- +goose StatementEnd