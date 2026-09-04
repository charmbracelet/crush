# Plan Mode

> [!NOTE]
> This document was designed for both humans and agents. The **Feature**
> section describes the user-facing behavior. The **Status** section
> tracks the implementation. The **Plan** section describes the
> not-yet-shipped pieces.

## Feature

Plan mode lets the user run an agent that **thinks before it acts**: it can
read the codebase, draft an approach, and outline an implementation, but it
cannot write files or execute commands. This is in addition to the existing
**Build** (normal mode) and **YOLO** (auto-approve all permissions) modes.

The three modes form a loop:

```
Plan  <-->  Build  <-->  YOLO
 ^___________________________|
```

### Why

Issue
[#3364](https://github.com/charmbracelet/crush/issues/3364) asks for a way to
"stick the model into plan mode instead of build mode" so the model deliberates
before writing. A prior attempt,
[PR #3669](https://github.com/charmbracelet/crush/pull/3669) ("Add build and
plan agent modes"), was abandoned.

#### Inspiration from other harnesses

- **OpenCode** (`sst/opencode`): prompts the model to produce a numbered plan
  and pauses before tool calls. We borrow the "deliberate, then act" framing.
- **Kilo Code** (`Kilo-Org/kilocode`): has a separate "Plan" agent with a
  distinct system prompt that disallows write tools. We borrow the
  prompt-based constraint.
- **NanoCoder** (`Nano-Collective/nanocoder`): gates tool execution behind a
  phase tag so the model can only act when the phase is open. We borrow the
  *guard at the tool boundary* idea and use the **agent mode** as the gate
  instead of a per-turn phase.

### State

```go
type AgentMode string

const (
    AgentModeBuild AgentMode = "build"
    AgentModePlan  AgentMode = "plan"
    AgentModeYolo  AgentMode = "yolo"
)
```

State is **hybrid**:

- Per-session value takes precedence.
- Falls back to the workspace default (`options.default_agent_mode` in
  `crush.json` / `crushrc`).
- Falls back to `Build` if neither is set.

### Prompt

In plan mode, the agent's system prompt is the standard `coder.md.tpl` with
the following block injected via `internal/agent/templates/plan.md.tpl`:

```
<mode=plan>
You are in PLAN MODE. Read the codebase and produce a structured plan.
DO NOT write files, edit files, or run commands that modify state. Read-only
inspection is fine.

Your final answer MUST follow this shape:

1. **Goal** - one sentence.
2. **Approach** - bullet list of the high-level steps.
3. **Files** - the files you would change and why.
4. **Risks** - what could go wrong, what to verify.

When you're ready to persist this plan, you can use the `save_plan` tool
with a filename (e.g., "plan_2026-09-02.md") to write the plan to disk.
</mode=plan>
```

In build/yolo mode the agent is unchanged.

### Tool guard

Plan mode is enforced **at the tool boundary** by `hookedTool` in
`internal/agent/hooked_tool.go`. The wrapper inspects `ModeFromContext(ctx)`
and short-circuits any tool whose name is in the write set with a synthetic
response:

```text
[plan mode] <tool_name> skipped - no side effects in plan mode.
Switch to Build (Shift+Tab) to execute.
```

Read-only tools (`view`, `grep`, `glob`, `ls`, `lsp_*`, `sourcegraph`,
`web_search`, `web_fetch`, `agentic_fetch`, `crush_info`, `crush_logs`,
`save_plan`) are permitted because they are the only way the model can plan
or persist a plan.

### Mode switching

The TUI's editor prompt mirrors the current mode:

| Mode  | Icon    | Status message           |
|-------|---------|--------------------------|
| Plan  | ` P `   | "Plan mode (read-only)"  |
| Build | ` > `   | "Build mode"             |
| Yolo  | ` Y `   | "Yolo mode" (current)    |

`Shift+Tab` cycles `Plan -> Build -> Yolo -> Plan`. `Ctrl+Y` toggles Yolo on
top of the current mode (preserves the existing shortcut).

### Saving a plan

In plan mode, the `save_plan` tool is always available. The model calls it
with a filename and the plan content, and the content is written to disk in
the working directory. This is the only mutation a plan-mode agent is
allowed to perform.

## Status

Tracked against the original plan in `## Plan` below.

- [x] `AgentMode` type + `ParseAgentMode` + `Valid()` in
      `internal/agent/agent.go`.
- [x] Plan prompt template `internal/agent/templates/plan.md.tpl`.
- [x] `planPrompt` builder in `internal/agent/prompts.go`.
- [x] `WithMode` / `ModeFromContext` in
      `internal/agent/agent_context.go`.
- [x] Plan-mode tool guard in `internal/agent/hooked_tool.go`
      (`isToolBlockedInPlan`, `newPlanBlockedHookedTool`).
- [x] `Mode` field on `SessionAgentCall` in
      `internal/agent/agent.go`.
- [x] `Mode` carried into the agent context inside `sessionAgent.Run`,
      and `planPrompt` appended when `Mode == AgentModePlan`.
- [x] `Coordinator.AgentMode(sessionID)` and
      `Coordinator.SetAgentMode(sessionID, mode)` methods, with
      per-session state in the coordinator (in-memory; DB persistence
      pending sqlc regeneration).
- [x] `Workspace.AgentMode()` / `Workspace.SetAgentMode()` on the
      interface, with `AppWorkspace` delegating to the coordinator and
      `ClientWorkspace` returning `AgentModeBuild` as a stub.
- [x] `Session.AgentMode` field, DB migration, and CRUD plumbing in
      `internal/db/`, `internal/session/`. The migration file exists
      and `models.go` is hand-patched; `sqlc generate` must be run
      to regenerate `internal/db/*.sql.go` from the updated queries.
- [x] `save_plan` tool implementation in
      `internal/agent/tools/save_plan.go`, registered in coordinator.
- [ ] `config.Options.DefaultAgentMode` + `crushrc` `default_agent_mode`
      builtin.
- [ ] Client/server transport for `AgentMode` over the proto API
      (ClientWorkspace currently a stub).
- [ ] Coordinator persistence — read/write `AgentMode` from
      `session.Service` so it survives process restarts.
- [ ] UI: `Shift+Tab` cycler, plan prompt style, status message.
- [ ] Tool guard coverage — guard must apply even when hooks are
      disabled (currently only wraps in `wrapToolsWithHooks`).
- [ ] Tests: `internal/agent/plan_test.go`, extension of
      `internal/agent/hooked_tool_test.go`, session & UI tests.
- [ ] `task fmt` (gofumpt) and `go test ./...` green.

## Plan

The original plan, kept here so the Status section above has something
to point at. Items already shipped are crossed out.

### ~~New files~~ (shipped so far)

- ~~`internal/agent/plan.go` — `AgentMode` type, parse helpers, prompt-
  injection helper.~~ Shipped as inline additions to `agent.go` plus
  `agent_context.go`. `plan.go` is **not** its own file.
- ~~`internal/agent/templates/plan.md.tpl` — the injected prompt
  block.~~ Shipped.

### Still to ship

- `internal/agent/tools/save_plan.go` — the `save_plan` tool.
- `internal/db/migrations/<timestamp>_add_agent_mode_to_sessions.sql` —
  new `agent_mode TEXT NOT NULL DEFAULT 'build'` column.
- `internal/db/sql/sessions.sql` — read/write the new column (then
  regenerate via `sqlc`).
- `internal/session/session.go` — `Session.AgentMode` field and
  `Service.SetAgentMode(ctx, id, mode)` method.
- `internal/proto/proto.go` and `session.go` — wire `AgentMode` over
  the client/server boundary.
- `internal/workspace/workspace.go` — add `AgentMode` / `SetAgentMode`
  to the interface.
- `internal/workspace/app_workspace.go` and `client_workspace.go` —
  delegate to the coordinator and (for client) to the server.
- `internal/agent/coordinator.go` — add `AgentMode` and `SetAgentMode`
  methods; per-session mode map; pass `Mode` into `SessionAgentCall`
  when dispatching; consult the workspace default and session DB
  value at run time.
- `internal/agent/agent.go` — in `sessionAgent.Run`, when `call.Mode ==
  AgentModePlan`, append the `planPrompt` body to the system prompt
  and put `Mode` into the per-tool context via `WithMode`.
- `internal/config/config.go` — `Options.DefaultAgentMode` field
  (type `AgentMode`).
- `internal/shellconfig/` — `default_agent_mode` builtin.
- `internal/cmd/run.go` / `run_other.go` — `--mode` CLI flag.
- `internal/ui/model/ui.go` and `keys.go` — `Shift+Tab` cycles mode
  and dispatches `SetAgentMode`; prompt function picks the right
  style per mode.
- `internal/ui/styles/styles.go` and `quickstyle.go` — `Plan` prompt
  style (focused/blurred icon + dots).

### Out of scope (ponytail)

ponytail: per-tool "approval preview" diffs and per-tool plan-time
"dry-run" output are interesting but require a tool-result rewrite.
Add when a real user asks. The current guard is the smallest thing
that satisfies #3364.

## Testing

- `internal/agent/plan_test.go` — `parseAgentMode`, `isToolBlockedInPlan`,
  context round-trip.
- `internal/agent/hooked_tool_test.go` (extend) — write tool blocked
  in plan mode; read tool passes; build mode is unchanged.
- `internal/session/session_test.go` (extend) — session-level override
  wins; fallback to global; default to `Build`.
- `internal/ui/model/ui_test.go` (extend) — `Shift+Tab` cycles; prompt
  reflects mode; `Yolo` prompt still wins when yolo is on.

Run: `go test ./...`.

## Manual smoke

```bash
go build .
./crush
# press Shift+Tab until prompt shows " P " and status says "Plan mode".
# ask: "refactor the bash tool to support streaming output"
# expect: a structured plan, no file changes.
# (optional) ask: "save this plan to plan_streaming.md"
# expect: plan file written; tool result visible in transcript.
# press Shift+Tab to switch to Build, run the same prompt, expect edits.
```
