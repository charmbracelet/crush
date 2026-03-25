-- +goose Up
-- +goose StatementBegin
ALTER TABLE sessions ADD COLUMN kind TEXT NOT NULL DEFAULT 'normal' CHECK (kind IN ('normal', 'handoff'));
ALTER TABLE sessions ADD COLUMN handoff_source_session_id TEXT;
ALTER TABLE sessions ADD COLUMN handoff_goal TEXT NOT NULL DEFAULT '';
ALTER TABLE sessions ADD COLUMN handoff_draft_prompt TEXT NOT NULL DEFAULT '';
ALTER TABLE sessions ADD COLUMN handoff_relevant_files TEXT NOT NULL DEFAULT '[]';
CREATE INDEX IF NOT EXISTS idx_sessions_handoff_source_session_id ON sessions (handoff_source_session_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_sessions_handoff_source_session_id;
ALTER TABLE sessions DROP COLUMN handoff_relevant_files;
ALTER TABLE sessions DROP COLUMN handoff_draft_prompt;
ALTER TABLE sessions DROP COLUMN handoff_goal;
ALTER TABLE sessions DROP COLUMN handoff_source_session_id;
ALTER TABLE sessions DROP COLUMN kind;
-- +goose StatementEnd
