-- name: CreateToolcoachEffectiveness :one
INSERT INTO toolcoach_effectiveness (
    pattern_id,
    session_id,
    fired_count,
    acted_count,
    ignored_count,
    created_at
) VALUES (
    ?, ?, ?, ?, ?, strftime('%s', 'now')
)
RETURNING id, pattern_id, session_id, fired_count, acted_count, ignored_count, created_at
;

-- name: GetToolcoachEffectivenessByPattern :many
SELECT pattern_id, SUM(fired_count) AS total_fired, SUM(acted_count) AS total_acted, SUM(ignored_count) AS total_ignored
FROM toolcoach_effectiveness
WHERE pattern_id = ? AND created_at > ?
GROUP BY pattern_id
;

-- name: PruneToolcoachEffectiveness :execrows
DELETE FROM toolcoach_effectiveness
WHERE created_at < ?
;
