-- +goose Up
-- +goose StatementBegin

CREATE TABLE IF NOT EXISTS chunks (
    chunk_id   INTEGER PRIMARY KEY,
    session_id TEXT REFERENCES sessions(id) ON DELETE CASCADE,
    path       TEXT,
    symbol     TEXT,
    start_line INTEGER,
    end_line   INTEGER,
    content    TEXT NOT NULL,
    file_hash  TEXT,
    model      TEXT NOT NULL,
    dim        INTEGER NOT NULL
) STRICT;

CREATE INDEX IF NOT EXISTS idx_chunks_session_id ON chunks (session_id);
CREATE INDEX IF NOT EXISTS idx_chunks_path ON chunks (path);
CREATE INDEX IF NOT EXISTS idx_chunks_file_hash ON chunks (file_hash);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS chunks;

-- +goose StatementEnd
