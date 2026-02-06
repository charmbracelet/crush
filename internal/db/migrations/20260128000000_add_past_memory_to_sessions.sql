-- +goose Up
-- +goose StatementBegin
ALTER TABLE sessions ADD COLUMN past_memory TEXT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE sessions DROP COLUMN past_memory;
-- +goose StatementEnd
