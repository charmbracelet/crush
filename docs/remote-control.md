# Remote Control Protocol

Session-attached remote control: a live Crush process on your machine
registers sessions with a self-hosted relay. The mobile PWA lists those
sessions, opens one, and steers it. Code execution and filesystem access
stay on the laptop.

Protocol version: **1**

## Architecture

```
Phone PWA  ⇄  Relay (auth + WS broker)  ⇄  Crush TUI (outbound WSS)
```

- Crush dials the relay (outbound only; no inbound ports on the laptop).
- One Crush process may register many sessions on a single WebSocket.
- Multiple Crush processes appear as multiple sessions in the phone list.
- The laptop must stay awake and Crush must keep running.

## Authentication

1. `POST {httpBase}/api/login` with JSON `{"username","password"}`.
2. On HTTP 200 and `success: true`, response includes `token` (JWT).
3. CLI opens `GET {wsBase}/ws/cli?token=...` (token query is v1; prefer short TTL).
4. App opens `GET {wsBase}/ws/app?token=...`.

Credentials must come from environment or config. There is no safe default
password. Prefer `wss://` outside localhost.

Environment (Crush):

| Variable | Purpose |
|----------|---------|
| `CRUSH_REMOTE_URL` | Relay base (`wss://host` or `ws://localhost:8080`) |
| `CRUSH_REMOTE_USER` | Login username |
| `CRUSH_REMOTE_PASS` | Login password (never commit) |

Optional `crush.json`:

```json
{
  "remote_control": {
    "relay_url": "wss://your-host",
    "username": "admin"
  }
}
```

Password is never read from the config file; use `CRUSH_REMOTE_PASS`.

## Frame format

Every WebSocket text frame is JSON:

```json
{
  "type": "register_session",
  "session_id": "uuid",
  "payload": {},
  "timestamp": 1710000000,
  "protocol_version": 1
}
```

| Field | Required | Notes |
|-------|----------|--------|
| `type` | yes | Event name |
| `session_id` | when routing to a session | Crush session id |
| `payload` | often | Event-specific JSON object |
| `timestamp` | yes | Unix seconds |
| `protocol_version` | recommended | `1` |

Max frame size: 1 MiB.

## Events: CLI → App (via relay)

### `register_session`

Advertise or refresh a session that has remote control enabled.

```json
{
  "type": "register_session",
  "session_id": "<id>",
  "payload": {
    "title": "Fix login bug",
    "cwd": "/home/you/proj",
    "busy": false,
    "model": "provider/model",
    "updated_at": 1710000000
  }
}
```

### `unregister_session`

Remove a session from the phone list (`session_id` set; payload optional).

### `session_state`

Busy/title updates without full re-register.

```json
{
  "type": "session_state",
  "session_id": "<id>",
  "payload": {
    "busy": true,
    "title": "Fix login bug",
    "queue_depth": 1
  }
}
```

### `session_snapshot`

Full transcript for a session (after select or `request_snapshot`).

```json
{
  "type": "session_snapshot",
  "session_id": "<id>",
  "payload": {
    "messages": [
      {
        "id": "msg-1",
        "role": "user",
        "content": "hello",
        "created_at": 1710000000
      }
    ]
  }
}
```

### `stream_chunk`

Live text (and future tool activity).

```json
{
  "type": "stream_chunk",
  "session_id": "<id>",
  "payload": {
    "message_id": "msg-2",
    "role": "assistant",
    "content": "partial or full text",
    "done": false
  }
}
```

### `tool_request`

Permission prompt (phone and desktop may both show it).

```json
{
  "type": "tool_request",
  "session_id": "<id>",
  "payload": {
    "request_id": "<permission-id>",
    "tool_name": "bash",
    "description": "Run tests",
    "command": "go test ./...",
    "action": "execute",
    "path": "/home/you/proj"
  }
}
```

### `error`

```json
{
  "type": "error",
  "session_id": "<id>",
  "payload": { "message": "agent busy", "code": "busy" }
}
```

## Events: App → CLI (via relay)

### `send_prompt`

```json
{
  "type": "send_prompt",
  "session_id": "<id>",
  "payload": { "prompt": "run the tests" }
}
```

### `cancel_task`

```json
{ "type": "cancel_task", "session_id": "<id>" }
```

### `tool_response`

```json
{
  "type": "tool_response",
  "session_id": "<id>",
  "payload": {
    "request_id": "<permission-id>",
    "approved": true
  }
}
```

First resolver wins (phone or desktop). Duplicate responses are ignored.

### `request_snapshot`

```json
{ "type": "request_snapshot", "session_id": "<id>" }
```

## Events: Relay → App

### `session_list`

Broadcast whenever the set of registered sessions changes.

```json
{
  "type": "session_list",
  "payload": {
    "sessions": [
      {
        "id": "<id>",
        "title": "Fix login bug",
        "cwd": "/home/you/proj",
        "busy": false,
        "model": "provider/model",
        "updated_at": 1710000000
      }
    ]
  }
}
```

Legacy payloads that are a bare string array of ids remain accepted by older
clients; v1 clients prefer the object form above.

## Multi-session on one CLI socket

The CLI WebSocket is not bound to a single session id. The Crush process:

1. Connects once to `/ws/cli`.
2. Sends `register_session` / `unregister_session` for each enabled session.
3. Sets `session_id` on every outbound event.
4. Handles inbound events by `session_id`.

The relay maps `session_id → CLI connection` and may map many sessions to
one connection. On CLI disconnect, all of that connection's sessions are
removed from the list.

## Security notes

- Anyone with relay credentials can drive the agent on your machine.
- Do not enable yolo / skip-permissions solely because remote control is on.
- Tool approvals appear on phone and desktop; either may approve or deny.
- Prefer TLS (`wss`/`https`) for any non-loopback relay.

## Crush UX (v1)

- Commands palette: **Remote Control** toggles the current session.
- Per-session flag: multiple sessions in one TUI can be enabled independently.
- Status should indicate RC connected and how many sessions are shared.
