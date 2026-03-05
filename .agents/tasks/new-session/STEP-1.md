# Create tool definition for `new_session` in `internal/agent/tools/`

Status: COMPLETED

## Sub tasks

1. [x] Analyze existing tool definitions in `internal/agent/tools/` to understand the `fantasy.AgentTool` interface and structure.
2. [x] Create `internal/agent/tools/new_session.go`.
3. [x] Define the tool struct, schema, and its `Execute` method.
4. [x] Define the parameters such as `summary` (string) that the LLM will provide.

## NOTES

Created two files:

- `internal/agent/tools/new_session.go` — Defines `NewSessionParams` (with `summary` field), `NewSessionError` sentinel error type, and `NewNewSessionTool()` constructor using `fantasy.NewAgentTool`. The tool's execute function returns a `&NewSessionError{Summary}` which propagates up through the coordinator to the UI.
- `internal/agent/tools/new_session.md` — Embedded description explaining usage and behavior.

The tool follows the same pattern as other simple tools (e.g., `lsp_restart.go`): a params struct, a name constant (`NewSessionToolName = "new_session"`), and a constructor returning `fantasy.AgentTool`.
