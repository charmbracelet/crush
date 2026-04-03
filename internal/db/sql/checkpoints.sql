-- name: CreateCheckpoint :one
INSERT INTO checkpoints (id, session_id, message_id, created_at)
VALUES (?, ?, ?, strftime('%s', 'now'))
RETURNING *;

-- name: GetCheckpoint :one
SELECT * FROM checkpoints WHERE id = ? LIMIT 1;

-- name: ListSessionCheckpoints :many
SELECT * FROM checkpoints
WHERE session_id = ?
ORDER BY created_at DESC;

-- name: GetLatestSessionCheckpoint :one
SELECT * FROM checkpoints
WHERE session_id = ?
ORDER BY created_at DESC
LIMIT 1;

-- name: GetCheckpointByMessageID :one
SELECT * FROM checkpoints
WHERE message_id = ?
LIMIT 1;

-- name: DeleteCheckpoint :exec
DELETE FROM checkpoints WHERE id = ?;

-- name: DeleteSessionCheckpoints :exec
DELETE FROM checkpoints WHERE session_id = ?;

-- name: AddCheckpointFile :exec
INSERT INTO checkpoint_files (checkpoint_id, file_id)
VALUES (?, ?);

-- name: ListCheckpointFiles :many
SELECT f.*
FROM files f
INNER JOIN checkpoint_files cf ON f.id = cf.file_id
WHERE cf.checkpoint_id = ?
ORDER BY f.path;

-- name: CountCheckpointFiles :one
SELECT COUNT(*) FROM checkpoint_files WHERE checkpoint_id = ?;
