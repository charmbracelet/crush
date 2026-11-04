-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS skills_disabled_servers (
    name TEXT PRIMARY KEY
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS skills_disabled_servers;
-- +goose StatementEnd
