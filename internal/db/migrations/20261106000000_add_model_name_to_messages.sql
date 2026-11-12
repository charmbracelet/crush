-- +goose Up
-- +goose StatementBegin
-- Add model name to messages table. Stores the resolved display name of
-- the model at the time the message was created, so the UI can still show
-- a friendly name after the model is renamed or removed from config.
ALTER TABLE messages ADD COLUMN model_name TEXT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Remove model name column from messages table
ALTER TABLE messages DROP COLUMN model_name;
-- +goose StatementEnd
