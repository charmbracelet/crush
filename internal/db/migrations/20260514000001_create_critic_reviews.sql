-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS critic_reviews (
    id TEXT PRIMARY KEY CHECK (id != ''),
    session_id TEXT NOT NULL CHECK (session_id != ''),
    message_id TEXT NOT NULL CHECK (message_id != ''),
    verdict TEXT NOT NULL CHECK (verdict IN ('approve', 'revise', 'halt')),
    confidence REAL NOT NULL DEFAULT 0.0,
    concerns TEXT NOT NULL DEFAULT '[]',
    summary TEXT NOT NULL DEFAULT '',
    diff_snapshot TEXT NOT NULL DEFAULT '',
    lsp_diagnostics TEXT NOT NULL DEFAULT '[]',
    created_at INTEGER NOT NULL DEFAULT (strftime('%s', 'now')),
    FOREIGN KEY (session_id) REFERENCES sessions (id) ON DELETE CASCADE,
    FOREIGN KEY (message_id) REFERENCES messages (id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_critic_reviews_session_id ON critic_reviews (session_id);
CREATE INDEX IF NOT EXISTS idx_critic_reviews_message_id ON critic_reviews (message_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_critic_reviews_message_id;
DROP INDEX IF EXISTS idx_critic_reviews_session_id;
DROP TABLE IF EXISTS critic_reviews;
-- +goose StatementEnd
