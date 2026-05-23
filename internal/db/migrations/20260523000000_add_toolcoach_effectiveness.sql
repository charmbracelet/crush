-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS toolcoach_effectiveness (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    pattern_id TEXT NOT NULL,
    session_id TEXT NOT NULL,
    fired_count INTEGER NOT NULL DEFAULT 0,
    acted_count INTEGER NOT NULL DEFAULT 0,
    ignored_count INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL DEFAULT (strftime('%s', 'now')),
    FOREIGN KEY (session_id) REFERENCES sessions (id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_toolcoach_eff_pattern ON toolcoach_effectiveness (pattern_id);
CREATE INDEX IF NOT EXISTS idx_toolcoach_eff_session ON toolcoach_effectiveness (session_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_toolcoach_eff_session;
DROP INDEX IF EXISTS idx_toolcoach_eff_pattern;
DROP TABLE IF EXISTS toolcoach_effectiveness;
-- +goose StatementEnd
