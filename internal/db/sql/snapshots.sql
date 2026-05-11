-- name: CreateSnapshot :exec
INSERT INTO snapshots (
    id,
    session_id,
    message_id,
    parent_snapshot_id,
    git_commit_hash,
    description,
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

-- name: GetSnapshot :one
SELECT *
FROM snapshots
WHERE id = ? LIMIT 1;

-- name: GetSnapshotByMessage :one
SELECT *
FROM snapshots
WHERE message_id = ? LIMIT 1;

-- name: ListSnapshots :many
SELECT *
FROM snapshots
WHERE session_id = ?
ORDER BY created_at ASC;

-- name: DeleteSnapshot :exec
DELETE FROM snapshots
WHERE id = ?;

-- name: DeleteSessionSnapshots :exec
DELETE FROM snapshots
WHERE session_id = ?;
