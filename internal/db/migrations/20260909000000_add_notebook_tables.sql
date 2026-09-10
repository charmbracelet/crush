-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS notebook_entries (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    turn_number INTEGER NOT NULL,
    event_number INTEGER NOT NULL DEFAULT 0,
    event_type TEXT NOT NULL DEFAULT 'general',
    title TEXT NOT NULL DEFAULT '',
    entry_text TEXT NOT NULL,
    entry_text_full TEXT,
    token_count INTEGER NOT NULL DEFAULT 0,
    compression_level INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL,
    FOREIGN KEY (session_id) REFERENCES sessions (id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_notebook_session ON notebook_entries (session_id);
CREATE INDEX IF NOT EXISTS idx_notebook_turn ON notebook_entries (session_id, turn_number);
CREATE INDEX IF NOT EXISTS idx_notebook_event_type ON notebook_entries (session_id, event_type);

CREATE TABLE IF NOT EXISTS notebook_tags (
    entry_id TEXT NOT NULL,
    tag TEXT NOT NULL,
    FOREIGN KEY (entry_id) REFERENCES notebook_entries (id) ON DELETE CASCADE,
    PRIMARY KEY (entry_id, tag)
);

CREATE INDEX IF NOT EXISTS idx_notebook_tag ON notebook_tags (tag);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_notebook_tag;
DROP TABLE IF EXISTS notebook_tags;
DROP INDEX IF EXISTS idx_notebook_event_type;
DROP INDEX IF EXISTS idx_notebook_turn;
DROP INDEX IF EXISTS idx_notebook_session;
DROP TABLE IF EXISTS notebook_entries;
-- +goose StatementEnd
