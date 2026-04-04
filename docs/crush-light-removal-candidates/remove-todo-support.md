# Remove todo support while keeping sessions

- **Suggested branch:** `draft/remove-todo-support`
- **Risk level:** `medium`

## Short summary
Remove the `todos` tool and all todo persistence/serialization/UI/docs while preserving session management, file history, and stale-write protections.

## Why this is a removal candidate
Todo items and lists are explicitly required to be removed even though sessions must remain.

## Why the chosen risk level applies
The tool itself is simple, but todos are now embedded in session types, the active schema/data model, server payloads, and tests. The future PR must clean those links without disturbing sessions, and it does not need to preserve the existing migration history verbatim in this fresh-start fork.

## User-visible behavior affected
Users lose todo-list tracking in sessions; normal session creation, switching, summarization, and history remain.

## Code entrypoints
- `internal/agent/tools/todos.go`
- `internal/session/session.go`
- `internal/db/models.go`
- `internal/db/sql/*.sql`
- `internal/server/server.go`

## Known touch points
- Files/packages: todos tool, session service/types, active DB schema/queries/models, workspace/backend/server session payloads, UI rendering that surfaces todos
- Config: disabled-tools docs/help may mention `todos`
- Docs/tests: README/help copy, session/agent tests, fixtures using todo metadata
- API/server: session read/write payloads and docs exposing todos
- UI: any todo chips/panels/session summaries
- Persistence/data model: sessions.todos column and marshaling helpers

## Dependencies on kept features
Must preserve named sessions, per-session file history/version snapshots, and stale-write protections.

## Things that must be preserved while removing it
Keep session CRUD, message history, summaries, model selection, and file-tracking behavior intact.

## Suggested removal order
Can happen early; useful before slimming session payloads and before fantasy replacement.

## Acceptance criteria for the future implementation PR
- No `todos` tool remains.
- Session types, DB queries, API payloads, and UI no longer include todos.
- The fork's effective initial schema/data model no longer includes todo state.
- Sessions continue to work normally without todo state.
- Tests updated to confirm session preservation without todos.

## Open questions / uncertainties
- If prompt text still mentions todo metadata anywhere, it can be updated directly as part of the removal without blocking the schema/tool cleanup.
