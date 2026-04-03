-- +goose Up
CREATE TABLE IF NOT EXISTS checkpoints (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    message_id TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL DEFAULT (strftime('%s', 'now')),
    FOREIGN KEY (session_id) REFERENCES sessions (id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS checkpoint_files (
    checkpoint_id TEXT NOT NULL,
    file_id TEXT NOT NULL,
    PRIMARY KEY (checkpoint_id, file_id),
    FOREIGN KEY (checkpoint_id) REFERENCES checkpoints (id) ON DELETE CASCADE,
    FOREIGN KEY (file_id) REFERENCES files (id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_checkpoints_session_id ON checkpoints (session_id);
CREATE INDEX IF NOT EXISTS idx_checkpoints_message_id ON checkpoints (message_id);

-- +goose Down
DROP TABLE IF EXISTS checkpoint_files;
DROP TABLE IF EXISTS checkpoints;
