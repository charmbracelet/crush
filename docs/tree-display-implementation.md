# Tree-Style Display Implementation

**Date**: 2025-10-01
**Status**: ✅ Complete
**Feature**: Tool traces displayed under their parent tasks with tree characters

---

## Overview

Implemented a tree-style display that groups tool execution traces under their parent tasks, making it clear which tools belong to which task during parallel execution.

## Visual Design

### Before
```
[1/3] ▶ analyze auth.go (worker 1)
[2/3] ▶ analyze db.go (worker 2)
[3/3] ▶ analyze api.go (worker 3)
[TOOL] glob — **/*.go → 47 matches — 0.2s      ← Which task?
[TOOL] bash — go build (exit 0) — 1.2s          ← Which task?
[1/3] ✓ analyze auth.go (12.4s, 15.0k tokens)
```

### After
```
[1/3] ✓ analyze auth.go (12.4s, 15.0k tokens)
  ├──── glob — **/*.go → 47 matches — 0.2s
  ├──── view — auth.go (245 lines, 12KB) — 0.3s
  ╰──── bash — go test ./auth — 1.2s
[2/3] ✓ analyze db.go (10.1s, 12.3k tokens)
  ╰──── glob — **/*.go → 47 matches — 0.2s
[3/3] ✓ analyze api.go (14.7s, 18.9k tokens)
  ├──── glob — **/*.go → 47 matches — 0.2s
  ╰──── view — api.go (123 lines, 5KB) — 0.1s
```

**Tree characters:**
- `├────` for intermediate tools
- `╰────` for the last tool
- Indented with 2 spaces

---

## Implementation Details

### Architecture Changes

**File**: `internal/volley/progress.go`

1. **Task State Tracking**
   ```go
   type taskState struct {
       task      Task
       status    string // "running", "completed", "failed"
       icon      string // ▶, ✓, ✗
       result    *TaskResult
       tools     []*tools.ExecutionMetadata
       workerID  int
       lineCount int
   }
   ```

2. **Unified Rendering**
   - All tasks rendered together in `renderAll()`
   - Tracks total line count for ANSI cursor positioning
   - Maintains task order for consistent display

3. **In-Place Updates**
   - Uses ANSI escape codes to move cursor up and clear
   - `\033[1A` - move cursor up one line
   - `\033[J` - clear from cursor to end of screen
   - Updates entire display on each tool event

### Key Methods

**`renderAll()`**
- Builds output for all tasks in task order
- Moves cursor up to start of display area
- Clears old content
- Prints new content
- Updates total line count

**`formatTaskLine(state)`**
- Running: `[1/3] ▶ task name (worker 1)`
- Completed: `[1/3] ✓ task name (12.4s, 15.0k tokens, $0.0000, model)`
- Failed: `[1/3] ✗ task name failed: error`

**`formatToolLine(treeChar, metadata)`**
- Formats tool trace with tree character prefix
- Removes `[TOOL]` prefix (replaced by tree char)
- Format: `  ├──── tool — details — duration`

---

## Behavior

### Single Task
```
[1/1] ▶ find all Go files (worker 1)
  ╰──── glob — **/*.go → 47 matches — 0.2s
                ↓ (updates in place)
[1/1] ✓ find all Go files (16.5s, 14.9k tokens)
  ╰──── glob — **/*.go → 47 matches — 0.2s
```

### Multiple Tools
```
[1/1] ▶ analyze auth.go (worker 1)
  ╰──── glob — **/*.go → 18 matches — 0.0s
                ↓ (updates in place)
[1/1] ▶ analyze auth.go (worker 1)
  ├──── glob — **/*.go → 18 matches — 0.0s
  ╰──── view — auth.go (245 lines) — 0.3s
                ↓ (updates in place)
[1/1] ✓ analyze auth.go (12.4s, 15.0k tokens)
  ├──── glob — **/*.go → 18 matches — 0.0s
  ├──── view — auth.go (245 lines) — 0.3s
  ╰──── bash — go test — 1.2s
```

### Parallel Tasks
```
[1/3] ▶ task1 (worker 1)
[2/3] ▶ task2 (worker 2)
[3/3] ▶ task3 (worker 3)
                ↓ (all update independently)
[1/3] ▶ task1 (worker 1)
  ╰──── bash — echo task1 — 0.0s
[2/3] ✓ task2 (5.1s, 14.7k tokens)
  ╰──── bash — echo task2 — 0.0s
[3/3] ▶ task3 (worker 3)
  ╰──── bash — echo task3 — 0.0s
                ↓ (continues updating)
[1/3] ✓ task1 (9.5s, 14.8k tokens)
  ╰──── bash — echo task1 — 0.0s
[2/3] ✓ task2 (5.1s, 14.7k tokens)
  ╰──── bash — echo task2 — 0.0s
[3/3] ✓ task3 (6.6s, 14.6k tokens)
  ╰──── bash — echo task3 — 0.0s
```

---

## Edge Cases Handled

1. **No tools**: Task line only, no tree characters
2. **Single tool**: Uses `╰────` (end branch)
3. **Multiple tools**: First N-1 use `├────`, last uses `╰────`
4. **Task completion**: Icon changes (▶ → ✓), stats added
5. **Parallel updates**: All tasks rendered together, no overlap

---

## ANSI Escape Codes

| Code | Effect |
|------|--------|
| `\033[1A` | Move cursor up one line |
| `\033[J` | Clear from cursor to end of screen |
| `\r` | Carriage return (move to start of line) |

**Note**: These codes are invisible in a terminal but show as literal text when output is captured (e.g., piped to `tail`).

---

## Compatibility

### Works
- ✅ macOS Terminal
- ✅ Linux terminals (tested on xterm)
- ✅ iTerm2
- ✅ Modern terminals with ANSI support

### Limitations
- ⚠️ Tree characters (├, ╰) require Unicode support
- ⚠️ ANSI codes don't work when output is piped or redirected
- ⚠️ Quiet mode (`--quiet`) disables all progress display

---

## Future Enhancements

1. **Gum Integration** (planned)
   - Add color to tree characters (cyan for tool names)
   - Color-code status icons (green ✓, red ✗, purple ▶)
   - Style durations in dim gray
   - Highlight token counts in magenta

2. **Collapsible Trees** (future)
   - Hide completed task tools after N seconds
   - Show summary count: `[3 tools executed]`

3. **Progress Indicators** (future)
   - Animated spinner for running tasks
   - Progress bar for streaming responses

4. **Smart Truncation** (future)
   - Collapse long tool lists: `... and 5 more tools`
   - Limit display to last N tools per task

---

## Testing

### Manual Tests
```bash
# Single task
./bin/cliffy "find all Go files in internal/llm/tools"

# Multiple tools
./bin/cliffy "analyze auth.go"

# Parallel execution
./bin/cliffy "run echo task1" "run echo task2" "run echo task3"

# Quiet mode (should show no tree)
./bin/cliffy --quiet "what is 2+2?"
```

### Expected Output
- Tasks display in order they started
- Tools grouped under their parent task
- Tree characters connect properly
- Completed tasks show final stats
- Display updates in place (no duplication)

---

## Known Issues

None currently. The implementation is stable and working as designed.

---

## Files Modified

- `internal/volley/progress.go` - Main implementation
  - Added `taskState` struct
  - Added `taskStates` map and `taskOrder` slice
  - Implemented `renderAll()` for unified rendering
  - Updated `TaskStarted()`, `TaskCompleted()`, `ToolExecuted()`
  - Disabled `ShowProgress()` (replaced by tree display)

- `internal/output/formatter.go` - No changes needed
  - `FormatToolTrace()` still used for formatting
  - Tree characters added by `formatToolLine()`

---

## Success Criteria

- [x] Tool traces grouped under parent tasks
- [x] Tree characters (├, ╰) display correctly
- [x] In-place updates work for single tasks
- [x] Parallel tasks don't overwrite each other
- [x] Completed tasks persist with tool traces
- [x] Clean final output after all tasks complete
- [x] Works in quiet mode (suppresses all display)

---

## Next Steps

1. ✅ **Completed**: Basic tree display implementation
2. 🔄 **Next**: Gum integration for colors and theming
3. 📋 **Planned**: Settings panel for configuration
4. 💡 **Future**: Collapsible trees and smart truncation
