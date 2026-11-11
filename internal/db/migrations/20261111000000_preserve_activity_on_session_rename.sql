-- +goose Up
-- +goose StatementBegin
DROP TRIGGER IF EXISTS update_sessions_updated_at;

-- Renaming a session is not conversation activity.
CREATE TRIGGER update_sessions_updated_at
AFTER UPDATE OF message_count, prompt_tokens, completion_tokens, cost,
    summary_message_id, todos, channel ON sessions
BEGIN
UPDATE sessions SET updated_at = strftime('%s', 'now')
WHERE id = new.id;
END;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS update_sessions_updated_at;

CREATE TRIGGER update_sessions_updated_at
AFTER UPDATE ON sessions
BEGIN
UPDATE sessions SET updated_at = strftime('%s', 'now')
WHERE id = new.id;
END;
-- +goose StatementEnd
