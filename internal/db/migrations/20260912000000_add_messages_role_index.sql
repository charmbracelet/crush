-- +goose Up
-- +goose StatementBegin
-- Prompt history reads every user message in the database. Without an index
-- on role, SQLite answers that by walking the whole messages table in
-- created_at order, which on a large history means touching every row to
-- return a small fraction of them. Leading with role lets it seek straight to
-- the user rows, and keeping created_at second means they come back already
-- ordered, so the ORDER BY needs no sort either.
CREATE INDEX IF NOT EXISTS idx_messages_role_created_at ON messages (role, created_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_messages_role_created_at;
-- +goose StatementEnd
