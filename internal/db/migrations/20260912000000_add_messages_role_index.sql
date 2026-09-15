-- +goose Up
-- +goose StatementBegin
-- Prompt history reads every user message. Without this, SQLite walks the
-- whole messages table. role first so it can seek; created_at second so the
-- rows come back ordered and the ORDER BY needs no sort.
CREATE INDEX IF NOT EXISTS idx_messages_role_created_at ON messages (role, created_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_messages_role_created_at;
-- +goose StatementEnd
