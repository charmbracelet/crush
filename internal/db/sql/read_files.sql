-- name: RecordFileRead :exec
INSERT INTO read_files (
    session_id,
    path,
    read_at,
    last_read_at,
    last_read_mtime_ns,
    last_read_size
) VALUES (
    ?,
    ?,
    strftime('%s', 'now'),
    strftime('%s', 'now'),
    ?,
    ?
) ON CONFLICT(path, session_id) DO UPDATE SET
    read_at = excluded.read_at,
    last_read_at = excluded.last_read_at,
    last_read_mtime_ns = excluded.last_read_mtime_ns,
    last_read_size = excluded.last_read_size;

-- name: RecordFileIncludedInContext :exec
INSERT INTO read_files (
    session_id,
    path,
    read_at,
    last_included_at,
    last_included_mtime_ns,
    last_included_size,
    last_included_epoch
) VALUES (
    ?,
    ?,
    strftime('%s', 'now'),
    strftime('%s', 'now'),
    ?,
    ?,
    ?
) ON CONFLICT(path, session_id) DO UPDATE SET
    read_at = excluded.read_at,
    last_included_at = excluded.last_included_at,
    last_included_mtime_ns = excluded.last_included_mtime_ns,
    last_included_size = excluded.last_included_size,
    last_included_epoch = excluded.last_included_epoch;

-- name: GetFileRead :one
SELECT * FROM read_files
WHERE session_id = ? AND path = ? LIMIT 1;

-- name: ListSessionReadFiles :many
SELECT * FROM read_files
WHERE session_id = ?
  AND (last_read_at > 0 OR last_included_at > 0)
ORDER BY MAX(last_read_at, last_included_at) DESC;
