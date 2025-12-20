-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS rate_limit_usage (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    window_start INTEGER NOT NULL UNIQUE,  -- Unix timestamp for the start of the rate limit window
    message_count INTEGER NOT NULL DEFAULT 0,  -- Number of messages in this window
    created_at INTEGER NOT NULL DEFAULT (strftime('%s', 'now'))  -- When this record was created
);

-- Index on window_start for efficient lookups
CREATE INDEX idx_rate_limit_usage_window_start ON rate_limit_usage(window_start);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_rate_limit_usage_window_start;
DROP TABLE IF EXISTS rate_limit_usage;
-- +goose StatementEnd
