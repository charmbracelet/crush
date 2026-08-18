-- name: GetUsageByDay :many
SELECT
    date(created_at, 'unixepoch') as day,
    SUM(prompt_tokens) as prompt_tokens,
    SUM(completion_tokens) as completion_tokens,
    SUM(cost) as cost,
    COUNT(*) as session_count
FROM sessions
WHERE parent_session_id IS NULL
GROUP BY date(created_at, 'unixepoch')
ORDER BY day DESC;

-- name: GetUsageByModel :many
SELECT
    COALESCE(model, 'unknown') as model,
    COALESCE(provider, 'unknown') as provider,
    COUNT(*) as message_count
FROM messages
WHERE role = 'assistant'
GROUP BY model, provider
ORDER BY message_count DESC;

-- name: GetUsageByHour :many
SELECT
    CAST(strftime('%H', created_at, 'unixepoch') AS INTEGER) as hour,
    COUNT(*) as session_count
FROM sessions
WHERE parent_session_id IS NULL
GROUP BY hour
ORDER BY hour;

-- name: GetUsageByDayOfWeek :many
SELECT
    CAST(strftime('%w', created_at, 'unixepoch') AS INTEGER) as day_of_week,
    COUNT(*) as session_count,
    SUM(prompt_tokens) as prompt_tokens,
    SUM(completion_tokens) as completion_tokens
FROM sessions
WHERE parent_session_id IS NULL
GROUP BY day_of_week
ORDER BY day_of_week;

-- name: GetTotalStats :one
SELECT
    COUNT(*) as total_sessions,
    COALESCE(SUM(prompt_tokens), 0) as total_prompt_tokens,
    COALESCE(SUM(completion_tokens), 0) as total_completion_tokens,
    COALESCE(SUM(cost), 0) as total_cost,
    COALESCE(SUM(message_count), 0) as total_messages,
    COALESCE(AVG(prompt_tokens + completion_tokens), 0) as avg_tokens_per_session,
    COALESCE(AVG(message_count), 0) as avg_messages_per_session
FROM sessions
WHERE parent_session_id IS NULL;

-- name: GetRecentActivity :many
SELECT
    date(created_at, 'unixepoch') as day,
    COUNT(*) as session_count,
    SUM(prompt_tokens + completion_tokens) as total_tokens,
    SUM(cost) as cost
FROM sessions
WHERE parent_session_id IS NULL
  AND created_at >= strftime('%s', 'now', '-30 days')
GROUP BY date(created_at, 'unixepoch')
ORDER BY day ASC;

-- name: GetAverageResponseTime :one
SELECT
    CAST(COALESCE(AVG(finished_at - created_at), 0) AS INTEGER) as avg_response_seconds
FROM messages
WHERE role = 'assistant'
  AND finished_at IS NOT NULL
  AND finished_at > created_at;

-- name: GetToolUsage :many
-- Aggregates legacy plain-JSON rows. Compressed rows (zstd, see
-- internal/message/parts_codec.go) cannot be parsed by json_each and
-- are handled by GetCompressedParts in Go. The substr magic-number
-- filter discriminates the two: stored JSON starts with '[' (TEXT),
-- zstd frames start with 0x28B52FFD (BLOB); a TEXT value never
-- compares equal to the BLOB constant in SQLite.
SELECT
    json_extract(value, '$.data.name') as tool_name,
    COUNT(*) as call_count
FROM messages, json_each(parts)
WHERE json_extract(value, '$.type') = 'tool_call'
  AND json_extract(value, '$.data.name') IS NOT NULL
  AND substr(parts, 1, 4) <> X'28B52FFD'
GROUP BY tool_name
ORDER BY call_count DESC;

-- name: GetCompressedParts :many
SELECT
    parts
FROM messages
WHERE substr(parts, 1, 4) = X'28B52FFD';

-- name: GetHourDayHeatmap :many
SELECT
    CAST(strftime('%w', created_at, 'unixepoch') AS INTEGER) as day_of_week,
    CAST(strftime('%H', created_at, 'unixepoch') AS INTEGER) as hour,
    COUNT(*) as session_count
FROM sessions
WHERE parent_session_id IS NULL
GROUP BY day_of_week, hour
ORDER BY day_of_week, hour;
