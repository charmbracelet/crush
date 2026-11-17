-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS disabled_skills (
    name TEXT PRIMARY KEY
);
CREATE TABLE IF NOT EXISTS enabled_skills (
    name TEXT PRIMARY KEY
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS enabled_skills;
DROP TABLE IF EXISTS disabled_skills;
-- +goose StatementEnd
