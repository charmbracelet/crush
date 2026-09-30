-- +goose Up
-- +goose StatementBegin
ALTER TABLE sessions ADD COLUMN summary_cut_message_id TEXT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE sessions DROP COLUMN summary_cut_message_id;
-- +goose StatementEnd
