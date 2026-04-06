-- +goose Up
-- +goose StatementBegin
ALTER TABLE sessions ADD COLUMN revert_message_id TEXT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE sessions DROP COLUMN revert_message_id;
-- +goose StatementEnd
