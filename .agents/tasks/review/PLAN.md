# Plan

## Issues to Fix

We address 7 of the 10 review issues. Issues #3 (stale token counts — by design, one-step lag), #9 (compaction dialog without session — minor UX, not a bug), and #10 (int vs int64 — works correctly) are dismissed with code comments where appropriate.

### Fix 1: Context status message role (Issue #1 🔴)
Change `fantasy.NewUserMessage` to `fantasy.NewSystemMessage` in `contextStatusMessage()`. Update the comment to match. Update tests in `context_status_test.go` to expect system role.

### Fix 2: Validate empty summary in `new_session` tool (Issue #2 🔴)
Add `strings.TrimSpace(params.Summary) == ""` check in the tool handler. Return `fantasy.ToolResponse{IsError: true}` with a descriptive message instead of propagating `NewSessionError`. Update `TestNewSessionToolEmptySummary` to expect a tool error response instead of a Go error.

### Fix 3: Clamp `usedPct` to 100 (Issue #4 🟡)
Add `usedPct = min(usedPct, 100)` after computing it. Update the overflow test to expect `used_pct: 100` instead of `125`. Add a code comment about `remaining` already being clamped.

### Fix 4: Only register `new_session` tool when compaction is LLM (Issue #5 🟡)
In `buildTools()`, conditionally add `NewNewSessionTool()` based on compaction method. Pass `disableContextStatus` (or the compaction method) into `buildTools`. When `disableContextStatus` is true, skip adding the tool.

### Fix 5: Add code comment about stale token counts (Issue #3 🟡)
Add a comment in `contextStatusMessage()` documenting the one-step lag.

### Fix 6: Add trailing newline to `new_session.md` (Issue #7 🟡)
Simple file fix.

### Fix 7: Normalize CompactionMethod zero value (Issue #8 ⚪️)
In `setDefaults()` or equivalent, normalize `""` to `CompactionAuto` so the zero value is never ambiguous.

## Dismissed Issues
- **Issue #3** (stale tokens): Inherent to the architecture; the previous step's usage is the best available data. Adding a comment is sufficient.
- **Issue #6** (no user notification on session transition): This is a UX design decision, not a bug. The new session starts with the summary as context, which is the intended behavior. Could be improved later but out of scope for a bug-fix pass.
- **Issue #9** (compaction dialog without session): Minor UX inconsistency, not a bug.
- **Issue #10** (int vs int64 mismatch): Works correctly with explicit casts. Not a bug.
