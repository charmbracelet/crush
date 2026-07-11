-- name: GetMessage :one
SELECT *
FROM messages
WHERE id = ? LIMIT 1;

-- name: ListMessagesBySession :many
SELECT *
FROM messages
WHERE session_id = ?
ORDER BY created_at ASC;

-- name: CreateMessage :one
INSERT INTO messages (
    id,
    session_id,
    role,
    parts,
    model,
    provider,
    is_summary_message,
    created_at,
    updated_at
) VALUES (
    ?, ?, ?, ?, ?, ?, ?, strftime('%s', 'now'), strftime('%s', 'now')
)
RETURNING *;

-- name: UpdateMessage :exec
UPDATE messages
SET
    parts = ?,
    finished_at = ?,
    updated_at = strftime('%s', 'now')
WHERE id = ?;


-- name: DeleteMessage :exec
DELETE FROM messages
WHERE id = ?;

-- name: DeleteSessionMessages :exec
DELETE FROM messages
WHERE session_id = ?;

-- name: ListUserMessagesBySession :many
SELECT *
FROM messages
WHERE session_id = ? AND role = 'user'
ORDER BY created_at DESC;

-- name: ListAllUserMessages :many
SELECT *
FROM messages
WHERE role = 'user'
ORDER BY created_at DESC;

-- name: ListMessagesFromCheckpoint :many
-- Messages at or after the checkpoint, ordered by insertion (rowid) so the cut
-- is exact even when created_at timestamps collide at second precision.
SELECT m.*
FROM messages m
WHERE m.session_id = sqlc.arg(session_id)
  AND m.rowid >= (
      SELECT cp.rowid FROM messages cp
      WHERE cp.id = sqlc.arg(checkpoint_id) AND cp.session_id = sqlc.arg(session_id)
  )
ORDER BY m.rowid ASC;

-- name: DeleteMessagesFromCheckpoint :exec
DELETE FROM messages
WHERE id IN (
    SELECT m.id
    FROM messages m
    WHERE m.session_id = sqlc.arg(session_id)
      AND m.rowid >= (
          SELECT cp.rowid FROM messages cp
          WHERE cp.id = sqlc.arg(checkpoint_id) AND cp.session_id = sqlc.arg(session_id)
      )
);
