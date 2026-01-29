-- name: RecordFileRead :exec
INSERT INTO read_files (
    session_id,
    path,
    read_at
) VALUES (
    ?,
    ?,
    strftime('%s', 'now')
) ON CONFLICT(path, session_id) DO UPDATE SET
    read_at = excluded.read_at;

-- name: GetFileRead :one
SELECT * FROM read_files
WHERE session_id = ? AND path = ? LIMIT 1;
