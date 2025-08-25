-- +goose Up
-- +goose StatementBegin
-- TODOs table for task tracking per session
CREATE TABLE IF NOT EXISTS todos (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    content TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('pending', 'in_progress', 'completed')),
    created_at INTEGER NOT NULL,  -- Unix timestamp in milliseconds
    updated_at INTEGER NOT NULL,  -- Unix timestamp in milliseconds
    FOREIGN KEY (session_id) REFERENCES sessions (id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_todos_session_id ON todos (session_id);
CREATE INDEX IF NOT EXISTS idx_todos_status ON todos (status);

CREATE TRIGGER IF NOT EXISTS update_todos_updated_at
AFTER UPDATE ON todos
BEGIN
    UPDATE todos SET updated_at = strftime('%s', 'now') * 1000
    WHERE id = new.id;
END;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS update_todos_updated_at;
DROP INDEX IF EXISTS idx_todos_session_id;
DROP INDEX IF EXISTS idx_todos_status;
DROP TABLE IF EXISTS todos;
-- +goose StatementEnd