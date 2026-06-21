-- +goose Up
-- +goose StatementBegin
ALTER TABLE sessions ADD COLUMN summary_message_id TEXT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE sessions DROP COLUMN summary_message_id;
-- +goose StatementEnd
