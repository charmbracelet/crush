-- +goose Up
-- +goose StatementBegin
ALTER TABLE notebook_entries
    ADD COLUMN segment_number INTEGER NOT NULL DEFAULT 0;

CREATE TABLE IF NOT EXISTS processed_segments (
    session_id TEXT NOT NULL,
    turn_number INTEGER NOT NULL,
    segment_number INTEGER NOT NULL,
    start_index INTEGER NOT NULL,
    end_index INTEGER NOT NULL,
    state TEXT NOT NULL DEFAULT 'unprocessed',
    retry_count INTEGER NOT NULL DEFAULT 0,
    last_attempt_at INTEGER,
    created_at INTEGER NOT NULL,
    PRIMARY KEY (session_id, turn_number, segment_number),
    FOREIGN KEY (session_id) REFERENCES sessions (id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_processed_segments_state
    ON processed_segments (session_id, state);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_processed_segments_state;
DROP TABLE IF EXISTS processed_segments;
ALTER TABLE notebook_entries DROP COLUMN segment_number;
-- +goose StatementEnd
