-- name: CreateWorktree :exec
INSERT INTO worktrees (
    id,
    session_id,
    name,
    path,
    base_snapshot_id,
    is_active,
    created_at
) VALUES (
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?
);

-- name: GetWorktree :one
SELECT id, session_id, name, path, base_snapshot_id, is_active, created_at
FROM worktrees
WHERE id = ? LIMIT 1;

-- name: GetWorktreeByName :one
SELECT id, session_id, name, path, base_snapshot_id, is_active, created_at
FROM worktrees
WHERE session_id = ? AND name = ? LIMIT 1;

-- name: GetActiveWorktree :one
SELECT id, session_id, name, path, base_snapshot_id, is_active, created_at
FROM worktrees
WHERE session_id = ? AND is_active = 1 LIMIT 1;

-- name: ListWorktrees :many
SELECT id, session_id, name, path, base_snapshot_id, is_active, created_at
FROM worktrees
WHERE session_id = ?
ORDER BY created_at ASC;

-- name: ListAllWorktrees :many
SELECT id, session_id, name, path, base_snapshot_id, is_active, created_at
FROM worktrees
ORDER BY created_at DESC;

-- name: SetWorktreeActive :exec
UPDATE worktrees
SET is_active = ?
WHERE id = ?;

-- name: DeactivateSessionWorktrees :exec
UPDATE worktrees
SET is_active = 0
WHERE session_id = ?;

-- name: DeleteWorktree :exec
DELETE FROM worktrees
WHERE id = ?;

-- name: DeleteSessionWorktrees :exec
DELETE FROM worktrees
WHERE session_id = ?;

-- name: UpdateSessionWorktree :exec
UPDATE sessions
SET worktree_id = ?
WHERE id = ?;

-- name: UpdateSessionForkedFrom :exec
UPDATE sessions
SET forked_from_snapshot_id = ?
WHERE id = ?;
