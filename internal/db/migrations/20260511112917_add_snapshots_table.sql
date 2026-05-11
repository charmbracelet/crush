-- +goose Up
-- +goose StatementBegin

-- Snapshots: filesystem state at each user message
CREATE TABLE IF NOT EXISTS snapshots (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    message_id TEXT NOT NULL,
    parent_snapshot_id TEXT,
    git_commit_hash TEXT NOT NULL,
    description TEXT,
    created_at INTEGER NOT NULL,  -- Unix timestamp in milliseconds
    
    FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE,
    FOREIGN KEY (message_id) REFERENCES messages(id) ON DELETE CASCADE,
    FOREIGN KEY (parent_snapshot_id) REFERENCES snapshots(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_snapshots_session_id ON snapshots(session_id);
CREATE INDEX IF NOT EXISTS idx_snapshots_message_id ON snapshots(message_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_snapshots_message_unique ON snapshots(message_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_snapshots_session_id;
DROP INDEX IF EXISTS idx_snapshots_message_id;
DROP INDEX IF EXISTS idx_snapshots_message_unique;
DROP TABLE IF EXISTS snapshots;

-- +goose StatementEnd
