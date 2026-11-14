-- +goose Up
-- +goose StatementBegin
DROP TRIGGER IF EXISTS update_sessions_updated_at;

-- Renaming a session is not conversation activity.
CREATE TRIGGER update_sessions_updated_at
AFTER UPDATE ON sessions
WHEN new.message_count IS NOT old.message_count
    OR new.prompt_tokens IS NOT old.prompt_tokens
    OR new.completion_tokens IS NOT old.completion_tokens
    OR new.cost IS NOT old.cost
    OR new.summary_message_id IS NOT old.summary_message_id
    OR new.todos IS NOT old.todos
    OR new.channel IS NOT old.channel
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
