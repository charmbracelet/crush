# Knowledge

## Project Basics

- **Build**: `go build .`
- **Test**: `task test` or `go test ./...`
- **Format**: `task fmt` (runs `gofumpt -w .`)
- **Lint**: `task lint:fix`
- **Update golden files**: `go test ./... -update`
- **AGENTS.md**: `/home/parallels/Programming/Go/crush/AGENTS.md`

## Critical Rules

- **Always restrict subagent searches to `/home/parallels/Programming/Go/crush/`** — never let them search the whole drive.
- Use `require` from testify, `t.Parallel()` in tests.
- Semantic commits: `fix:`, `feat:`, `chore:`, etc.
- Comments on own lines: start capitalized, end with period.
- Format all Go code after writing it.

## Codebase Discoveries

- `fantasy.ToolResponse{IsError: true}` returns an error to the LLM that it can see and retry, vs a Go `error` which aborts the agent loop.
- `csync.Value[T]` is a thread-safe value wrapper with `.Get()` / `.Set()` methods.
- `compactionFlags()` in `coordinator.go:411-418` maps `CompactionMethod` to two booleans: `(disableAutoSummarize, disableContextStatus)`.
  - `CompactionLLM` → `(true, false)` — context status ON, auto-summarize OFF.
  - Default (including `""` and `"auto"`) → `(disableAutoSummarize, true)` — context status OFF.
- `disableContextStatus` is checked in `PrepareStep` at `agent.go:295` — when `false`, context status is injected.
- `NewSessionError` is a sentinel error that propagates from tool → agent loop → coordinator → UI layer (`ui.go:2770`).
- The `buildTools` function at `coordinator.go:420` unconditionally registers all tools, then filters by `agent.AllowedTools`.
- Test file for `new_session` tool: `internal/agent/tools/new_session_test.go`.
- Test file for context status: `internal/agent/context_status_test.go`.
