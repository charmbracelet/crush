# Plan: Select Compaction Method UI and Logic

## The Core Solution
Provide a config/state toggle and a UI menu that allows the user to choose between the two conflicting methods of handling large contexts:
1. **Auto-compaction**: Backend-managed context summarization.
2. **LLM/User-driven compaction**: Enables the `<context_status>` message and allows the LLM to use the `new_session` tool.

## Actionable Plan

### 1. Configuration/State
- Add a new state/config value for "Compaction Method". It should have two enum-like states: `CompactionAuto` and `CompactionLLM`. Let's decide where this lives (likely in the UI model or global config). Default to `Auto-compaction` for legacy behavior stability.

### 2. UI Updates (Main Commands Menu)
- In the UI's main Commands menu, add a new menu item called "Select compaction method".
- Selecting it should open a window resembling the "Select Reasoning Effort" window, giving the two options: "Auto-compaction" and "LLM/User-driven compaction".
- Upadate UI state based on this selection.

### 3. Backend Agent Logic Updates
- **For "Auto-compaction" selection**:
  - The injection of the `<context_status>` message (implemented previously in `contextStatusMessage`) must be disabled. This keeps the LLM blind to token limits and stops it from proactively firing `new_session`.
  - The existing `StopWhen` trigger remains identical to its current logic, firing `shouldSummarize = true`.

- **For "LLM/User-driven compaction" selection**:
  - Disable the internal `StopWhen` trigger so `shouldSummarize` never gets set for token reasons.
  - Ensure the `<context_status>` system message *is* injected so the LLM knows to invoke the `new_session` tool at 75%.

## Bug Fix: Compaction Flags Not Applied on Mode Switch

### Root Cause
`UpdateModels` (`coordinator.go:892`) is the function called when the user switches compaction mode via the UI (`ActionSelectCompactionMethod` in `ui.go:1360`). However, `UpdateModels` only hot-swaps models (`SetModels`) and tools (`SetTools`) on the existing `sessionAgent` — it **never re-evaluates** the `disableAutoSummarize`/`disableContextStatus` flags.

These flags are plain `bool` fields on the `sessionAgent` struct (`agent.go:109-110`), set once at construction time inside `buildAgent` (`coordinator.go:371-380`). There is no setter method on the `SessionAgent` interface to update them after construction.

So when the user switches from Auto → LLM mode, `compactionFlags()` is never called again, and the agent continues running with its original flag values (i.e., `disableContextStatus=true`), which means `<context_status>` is never injected.

### Fix
1. Add a `SetCompactionFlags(disableAutoSummarize, disableContextStatus bool)` method to the `SessionAgent` interface and implement it on `sessionAgent` using thread-safe `csync.Value[bool]` fields (since these are read during `PrepareStep`/`StopWhen` which run concurrently).
2. In `UpdateModels`, after rebuilding models/tools, also re-derive and apply the compaction flags via the new setter.
3. Convert the `disableAutoSummarize` and `disableContextStatus` fields from plain `bool` to `*csync.Value[bool]` for thread safety (matching the pattern used by `largeModel`, `smallModel`, etc.).
4. Update all read sites (`PrepareStep`, `StopWhen`, `contextStatusMessage`) to use `.Get()` on the csync values.
5. Add/update tests to verify flags are re-applied when `UpdateModels` is called after a config change.
