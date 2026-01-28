-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS read_files (
    session_id TEXT NOT NULL CHECK (session_id != ''),
    path TEXT NOT NULL CHECK (path != ''),
    read_at INTEGER NOT NULL,  -- Unix timestamp in seconds when file was last read
    FOREIGN KEY (session_id) REFERENCES sessions (id) ON DELETE CASCADE,
    PRIMARY KEY (path, session_id)
);

CREATE INDEX IF NOT EXISTS idx_read_files_session_id ON read_files (session_id);
CREATE INDEX IF NOT EXISTS idx_read_files_path ON read_files (path);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_read_files_path;
DROP INDEX IF EXISTS idx_read_files_session_id;
DROP TABLE IF EXISTS read_files;
-- +goose StatementEnd
