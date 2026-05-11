-- +goose Up
-- Worktrees: managed git worktree state
CREATE TABLE worktrees (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    name TEXT NOT NULL,
    path TEXT NOT NULL,
    base_snapshot_id TEXT,
    is_active INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL,
    
    FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE,
    FOREIGN KEY (base_snapshot_id) REFERENCES snapshots(id) ON DELETE SET NULL,
    UNIQUE(session_id, name)
);

CREATE INDEX idx_worktrees_session_id ON worktrees(session_id);

-- Add worktree reference to sessions for tracking which worktree a session is using
ALTER TABLE sessions ADD COLUMN worktree_id TEXT REFERENCES worktrees(id) ON DELETE SET NULL;

-- Add fork reference for conversation forking
ALTER TABLE sessions ADD COLUMN forked_from_snapshot_id TEXT REFERENCES snapshots(id) ON DELETE SET NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_worktrees_session_id;
DROP TABLE IF EXISTS worktrees;

-- Note: SQLite doesn't support DROP COLUMN directly
-- The new columns will remain but be unused after downgrade
