-- +goose Up
-- +goose StatementBegin
ALTER TABLE messages ADD COLUMN tool_chain_summary TEXT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE messages DROP COLUMN tool_chain_summary;
-- +goose StatementEnd
