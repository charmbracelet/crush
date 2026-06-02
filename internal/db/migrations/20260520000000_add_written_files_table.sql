-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS written_files (
    session_id TEXT NOT NULL CHECK (session_id != ''),
    path TEXT NOT NULL CHECK (path != ''),
    written_at INTEGER NOT NULL,
    FOREIGN KEY (session_id) REFERENCES sessions (id) ON DELETE CASCADE,
    PRIMARY KEY (path, session_id)
);

CREATE INDEX IF NOT EXISTS idx_written_files_session_id ON written_files (session_id);
CREATE INDEX IF NOT EXISTS idx_written_files_path ON written_files (path);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_written_files_path;
DROP INDEX IF EXISTS idx_written_files_session_id;
DROP TABLE IF EXISTS written_files;
-- +goose StatementEnd
