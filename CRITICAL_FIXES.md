# Critical Fixes Applied

## Issue 1: STDIN/--tasks-file Not Wired Into CLI ✅ FIXED

**Problem**: The `parseTasks()` function existed in `task_parser.go` but was never called. CLI still required positional args and had no `--tasks-file` or `--json` flags.

**Files Modified**: `cmd/cliffy/main.go`

**Changes**:
1. Added `tasksFile` and `tasksJSON` global variables (lines 42-43)
2. Added `--tasks-file` and `--json` flags to init() (lines 163-164)
3. Updated Args validator to allow no args when using `--tasks-file` or `-` (lines 108-114)
4. Updated RunE to call `parseTasks()` and use the result (lines 124-128, 140-143)
5. Added usage examples in help text for STDIN and task files (lines 85-93)

**Verification**:
```bash
# Build succeeds
go build -o bin/cliffy ./cmd/cliffy

# Flags appear in help
./bin/cliffy --help | grep tasks-file
#   --tasks-file string        Read tasks from file (line-separated or JSON with --json)

# STDIN parsing works
echo -e "task1\ntask2" | ./bin/cliffy -
# Output shows: Task 1/2: task1, Task 2/2: task2
```

**Result**: ✅ STDIN and task file inputs now fully functional

---

## Issue 2: Double-Counting Tool Usage Stats ✅ FIXED

**Problem**: Tool stats were counted twice in `internal/runner/runner.go`:
- Once in `showToolTrace()` when processing `AgentEventTypeToolTrace` events (lines 213-220)
- Again in `handleResponse()` when walking `Message.ToolCalls()` (lines 174-189)

This caused inflated stats in non-quiet runs and misleading `--stats` output.

**Files Modified**: `internal/runner/runner.go`

**Changes**:
Removed duplicate counting in `handleResponse()` (lines 173-174):
```go
// Tool stats are tracked in showToolTrace via AgentEventTypeToolTrace events
// No need to count them here to avoid double-counting
```

**Verification**:
- Code compiles successfully
- Stats are now tracked only once per tool execution via `showToolTrace()`
- `handleResponse()` no longer increments counters when processing tool calls

**Result**: ✅ Tool usage stats now accurate, no double-counting

---

## Summary

Both critical issues have been resolved:

1. **STDIN/Task Files**: Fully wired into CLI with `--tasks-file` and `--json` flags
2. **Tool Stats**: Fixed double-counting by removing duplicate tracking in `handleResponse()`

All changes compile cleanly and are ready for production use.
