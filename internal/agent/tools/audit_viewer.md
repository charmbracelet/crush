# audit_viewer

Query and inspect the SecOps tamper-evident audit log. Use this tool to review security-relevant events, investigate incidents, and verify audit chain integrity.

## Actions

- **query** — Search audit events with optional filters (actor, action, risk level, time range, session).
- **verify** — Verify the HMAC integrity chain of all in-memory audit events. Returns OK or the index of the first tampered record.
- **summary** — Produce a statistical summary of audit events (event counts by action, risk level, actor).

## Parameters

- `action` (required): One of `query`, `verify`, `summary`
- `actor` (optional): Filter by actor name (for `query` and `summary`)
- `event_action` (optional): Filter by event action type (e.g. `security_scan`, `compliance_check`, `command_execute`)
- `risk_level` (optional): Filter by risk level: LOW, MEDIUM, HIGH, CRITICAL
- `session_id` (optional): Filter by session ID
- `since` (optional): ISO 8601 start timestamp or relative duration (e.g. `1h`, `24h`, `7d`)
- `limit` (optional): Maximum number of results to return (default 50, max 500)

## Examples

Query the last 20 HIGH-risk events:
```json
{"action": "query", "risk_level": "HIGH", "limit": 20}
```

Verify audit chain integrity:
```json
{"action": "verify"}
```

Summarise activity from the last 24 hours:
```json
{"action": "summary", "since": "24h"}
```
