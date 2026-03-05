# Current Design

## Overview

Two features were added on the `new_session_tool` branch:

1. **LLM-driven compaction** via a `new_session` tool that lets the LLM start a fresh session with a summary.
2. **Context status injection** — a `<context_status>` JSON block appended as the last message each turn so the LLM knows how much context is consumed.

## Key Files & Functions

### `internal/config/config.go`
- **`CompactionMethod`** (line 55): `type CompactionMethod string` with constants `CompactionAuto = "auto"` and `CompactionLLM = "llm"`.
- **`Options.CompactionMethod`** (line 275): stored in config, JSON key `compaction_method`.
- Zero value is `""` (not `"auto"`), handled by `default` case in `compactionFlags`.

### `internal/agent/coordinator.go`
- **`compactionFlags()`** (lines 411-418): Converts `CompactionMethod` + `DisableAutoSummarize` into `(disableAutoSummarize bool, disableContextStatus bool)`.
- **`buildAgent()`** (lines 363-406): Calls `compactionFlags()` and passes results into `NewSessionAgent`.
- **`buildTools()`** (lines 420-511): Unconditionally registers `tools.NewNewSessionTool()` at line 457. Tools are then filtered by `agent.AllowedTools`.

### `internal/agent/agent.go`
- **`sessionAgent` struct** (lines 100-116): Has `disableContextStatus *csync.Value[bool]` and `disableAutoSummarize *csync.Value[bool]`.
- **`PrepareStep`** (lines 257-316): At line 295, checks `!a.isSubAgent && !a.disableContextStatus.Get()` before injecting context status as the **last message**.
- **`contextStatusMessage()`** (lines 934-952):
  - Comment says "system message" but creates `fantasy.NewUserMessage`.
  - Uses `s.PromptTokens + s.CompletionTokens` (stale by one step).
  - `remaining` is clamped to 0, but `usedPct` is not clamped to 100.
  - `ContextWindow` is `int`, token counts are `int64` — mixed types with explicit casts.
- **`SetCompactionFlags()`** (lines 1051-1054): Updates both flags at runtime.

### `internal/agent/tools/new_session.go`
- **`NewSessionParams`** (line 10): Has `Summary string` field.
- **`NewNewSessionTool()`** (lines 29-37): Creates tool. Handler returns `&NewSessionError{Summary: params.Summary}` with **no validation** of empty summary.
- **`NewSessionError`** (lines 18-24): Sentinel error type with `Summary` field.

### `internal/agent/tools/new_session.md`
- Tool description (22 lines). References `<context_status>` block as prerequisite.
- **Missing trailing newline** at end of file.

### `internal/ui/model/ui.go`
- **`newSessionMsg`** (lines 381-383): Carries `Summary string`.
- **Handler** (lines 444-447): Calls `m.newSession()` (clears all state) then sends `sendMessageMsg{Content: msg.Summary}`. No user notification of transition.
- **Emission** (lines 2768-2772): `NewSessionError` caught via `errors.As`, converted to `newSessionMsg`.

### `internal/ui/dialog/commands.go`
- **Compaction command** (lines 430-432): Registered unconditionally (not gated by `c.hasSession`), unlike similar settings.

### `internal/ui/dialog/compaction.go`
- Handles `""` → `config.CompactionAuto` normalization (290 lines total).

### `internal/agent/context_status_test.go`
- Tests for `contextStatusMessage`: basic, zero/negative window, overflow (125%), zero tokens, 100%, small window.
- Confirms `used_pct` can exceed 100.

### `internal/agent/tools/new_session_test.go`
- Tests: tool info, error propagation, empty summary (confirms it passes through), error message.
