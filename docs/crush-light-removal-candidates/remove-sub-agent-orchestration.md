# Remove sub-agent orchestration

- **Suggested branch:** `draft/remove-sub-agent-orchestration`
- **Risk level:** `high`

## Short summary
Remove the `agent` tool, nested task-session orchestration, sub-agent prompt/templates, and other recursion-specific agent behavior.

## Why this is a removal candidate
Sub-agents are an explicitly approved removal target and create nested session, prompt, and tool-surface complexity that is not required for the lightweight fork.

## Why the chosen risk level applies
Sub-agents touch coordinator tool construction, session ID conventions, prompt templates, tests/fixtures, and remote research helpers. They are conceptually bounded, but the behavior is spread across agent runtime and session handling.

## User-visible behavior affected
Users lose the ability to delegate work to nested agents or task sessions from inside a live agent conversation.

## Code entrypoints
- `internal/agent/agent_tool.go`
- `internal/agent/coordinator.go`
- `internal/session/session.go`
- `internal/agent/templates/agent_tool.md`

## Known touch points
- Files/packages: internal/agent/{agent_tool.go,coordinator.go,prompts.go}, internal/session/session.go, internal/message/** if nested session rendering differs
- Config: agent allowed_tools defaults and any task-agent config assumptions
- Docs/tests: agent templates, agent testdata/fixtures, README or help copy mentioning sub-agents
- API/server: any agent/session payload fields or session-title conventions used only for sub-agent runs
- UI: nested session displays, agent task status text, prompt queue affordances
- Persistence/data model: task-session IDs and parent_session_id behavior must be preserved only where still needed by kept features

## Dependencies on kept features
Must preserve normal single-agent sessions, model switching, summaries, and session CRUD.

## Things that must be preserved while removing it
Keep core session management, file history, and server agent endpoints for the primary agent.

## Suggested removal order
Before removing parallel execution if parallel wrappers exist only to support nested agents, and before fantasy replacement so the surface area shrinks first.

## Acceptance criteria for the future implementation PR
- No `agent` tool remains.
- No nested-agent prompt/template/session scaffolding remains unless reused by a kept feature.
- Tests and docs no longer mention sub-agent delegation.
- Primary agent sessions still work normally.

## Open questions / uncertainties
- Whether title/task session helpers still serve any kept summarization/title-generation path after agent-tool removal.
- Whether prompt queue UI assumes nested sessions exist.
