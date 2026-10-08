-- +goose Up
-- +goose StatementBegin
-- The ledger holds a session's distilled memory as append-only rows: what
-- happened, what was decided, and which entries were later retired. Compaction
-- renders a checkpoint from these rows without a model when any exist.
CREATE TABLE IF NOT EXISTS compaction_ledger (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    seq INTEGER NOT NULL CHECK (seq >= 0),
    kind TEXT NOT NULL,
    text TEXT NOT NULL,
    relevance INTEGER NOT NULL DEFAULT 0 CHECK (relevance >= 0),
    sources TEXT NOT NULL DEFAULT '[]',
    retires TEXT NOT NULL DEFAULT '[]',
    created_at INTEGER NOT NULL,  -- Unix timestamp in seconds
    FOREIGN KEY (session_id) REFERENCES sessions (id) ON DELETE CASCADE
);

-- One row per (session, seq): the seq is the ledger's only clock, so a
-- duplicate would fork history.
CREATE UNIQUE INDEX IF NOT EXISTS idx_compaction_ledger_session_seq
    ON compaction_ledger (session_id, seq);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_compaction_ledger_session_seq;
DROP TABLE IF EXISTS compaction_ledger;
-- +goose StatementEnd
