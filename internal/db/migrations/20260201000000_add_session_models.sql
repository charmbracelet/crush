-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS session_models (
    session_id TEXT NOT NULL,
    model_type TEXT NOT NULL CHECK (model_type IN ('large', 'small')),
    provider TEXT NOT NULL,
    model TEXT NOT NULL,
    selected_model_json TEXT,
    updated_at INTEGER NOT NULL,
    created_at INTEGER NOT NULL,
    PRIMARY KEY (session_id, model_type),
    FOREIGN KEY (session_id) REFERENCES sessions (id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_session_models_session_id ON session_models (session_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_session_models_session_id;
DROP TABLE IF EXISTS session_models;
-- +goose StatementEnd
