# Cliffy Visual System - Implementation Plan

## Overview

This document outlines the implementation plan for the enhanced Cliffy visual task processing system. The plan is structured in phases to allow incremental rollout and testing.

## Current State Inventory

### Existing Files
```
internal/llm/tools/ascii.go          - Symbol constants (69 lines)
internal/volley/progress.go          - Progress tracking (512 lines)
internal/volley/task.go               - Task definitions (122 lines)
internal/llm/agent/agent.go           - Agent events
cmd/cliffy/main.go                    - CLI entry point
```

### Existing Symbols (16 total)
```go
// Task states (8)
AsciiTaskComplete  = "●"    // Solid dot
AsciiTaskQueued    = "○"    // Hollow dot
AsciiTaskSpinner0-3 = "◴◵◶◷" // Spinner frames

// Tool states (2)
AsciiToolSuccess   = "▣"    // Success
AsciiToolFailed    = "☒"    // Failed

// Tree structure (4)
AsciiTreeBranch    = "╮"    // Branch down
AsciiTreeMid       = "├"    // Middle connector
AsciiTreeLast      = "╰"    // Last connector
AsciiTreeLine      = "───"  // Horizontal line

// Branding (2)
AsciiTennisRacketHead = "◍" // Header icon
AsciiCliffy + variants      // Character art
```

## Implementation Phases

### Phase 1: Enhanced Symbol Library (Quick Win)
**Goal**: Add processing state symbols without breaking existing functionality
**Time**: 2-4 hours
**Risk**: Low

#### 1.1 Extend `/internal/llm/tools/ascii.go`

Add new symbol constants:

```go
// Task Processing States (fine-grained progress)
AsciiTaskInitializing = "◔"  // 25% filled - starting up
AsciiTaskProcessing   = "◑"  // 50% filled - actively working
AsciiTaskFinalizing   = "◕"  // 75% filled - wrapping up
AsciiTaskFailed       = "◌"  // Dotted - failed
AsciiTaskCanceled     = "⦿"  // Ringed - canceled
AsciiTaskCached       = "◉"  // Double - cached result

// Tool Processing States (more detail)
AsciiToolInitializing = "▤"  // Starting
AsciiToolRunning      = "▥"  // Executing
AsciiToolComplex      = "▦"  // Nested operations
AsciiToolBlocked      = "▩"  // Error/blocked

// Worker States
AsciiWorkerIdle       = "⬡"  // Hexagon - available
AsciiWorkerActive     = "⬢"  // Filled hex - working
AsciiWorkerOverload   = "⬣"  // Black hex - at capacity

// Data Flow Indicators
AsciiFlowSimple       = "→"  // Basic flow
AsciiFlowStrong       = "⇒"  // Emphasized flow
AsciiFlowFast         = "⇨"  // Rapid processing
AsciiFlowRetry        = "⟲"  // Retry loop
AsciiFlowReturn       = "⤴"  // Error return
AsciiFlowFeedback     = "↻"  // Tool feedback

// Status Indicators
AsciiWarning          = "⚠"  // Warning
AsciiError            = "✗"  // Error
AsciiBlocked          = "⊗"  // Cannot proceed
AsciiPaused           = "⏸"  // Waiting
AsciiRateLimit        = "⚡"  // Rate limited

// Container/Structure
AsciiContainer        = "□"  // Task container
AsciiSolidBlock       = "█"  // Operational unit
AsciiOutput           = "▮"  // Output marker
```

**Files to modify**:
- `/internal/llm/tools/ascii.go` (+40 lines)

**Testing**:
```bash
go test ./internal/llm/tools
```

---

### Phase 2: Basic Processing States (Core Enhancement)
**Goal**: Show ○ → ◔ → ◑ → ◕ → ● progression
**Time**: 4-6 hours
**Risk**: Low-Medium

#### 2.1 Update Task States in `/internal/volley/task.go`

Add new task status constants:

```go
const (
    TaskStatusPending      TaskStatus = "pending"      // ○
    TaskStatusInitializing TaskStatus = "initializing" // ◔
    TaskStatusProcessing   TaskStatus = "processing"   // ◑
    TaskStatusFinalizing   TaskStatus = "finalizing"   // ◕
    TaskStatusSuccess      TaskStatus = "success"      // ●
    TaskStatusFailed       TaskStatus = "failed"       // ◌
    TaskStatusCanceled     TaskStatus = "canceled"     // ⦿
)
```

#### 2.2 Update Progress Tracker `/internal/volley/progress.go`

Modify `formatTaskLine()` to use new symbols:

```go
func (p *ProgressTracker) formatTaskLine(displayNum int, state *taskState) string {
    var icon string
    switch state.status {
    case "pending":
        icon = tools.AsciiTaskQueued        // ○
    case "initializing":
        icon = tools.AsciiTaskInitializing  // ◔
    case "processing":
        icon = tools.AsciiTaskProcessing    // ◑
    case "finalizing":
        icon = tools.AsciiTaskFinalizing    // ◕
    case "completed":
        icon = tools.AsciiTaskComplete      // ●
    case "failed":
        icon = tools.AsciiTaskFailed        // ◌
    case "canceled":
        icon = tools.AsciiTaskCanceled      // ⦿
    case "running":
        // Use progressive states instead of spinner
        // Or keep spinner for fast animation
        icon = tools.AsciiTaskProcessing    // ◑
    }
    // ... rest of formatting
}
```

#### 2.3 Update Agent to Emit Progress Events

Modify `/internal/llm/agent/agent.go` to emit state changes:

```go
// At different stages of agent.Run()
func (a *agent) Run(ctx context.Context, sessionID string, content string, attachments ...message.Attachment) (<-chan AgentEvent, error) {
    eventChan := make(chan AgentEvent)

    go func() {
        defer close(eventChan)

        // Initializing phase
        eventChan <- AgentEvent{
            Type:     AgentEventTypeProgress,
            Progress: "initializing",
        }

        // Load context, prepare tools...

        // Processing phase
        eventChan <- AgentEvent{
            Type:     AgentEventTypeProgress,
            Progress: "processing",
        }

        // Make LLM calls, execute tools...

        // Finalizing phase
        eventChan <- AgentEvent{
            Type:     AgentEventTypeProgress,
            Progress: "finalizing",
        }

        // Format response...
    }()

    return eventChan, nil
}
```

#### 2.4 Wire Up State Changes in Scheduler

Modify `/internal/volley/scheduler.go` worker function:

```go
func (s *Scheduler) worker(ctx context.Context, workerID int, taskQueue <-chan Task) {
    for task := range taskQueue {
        // ... existing code ...

        // Listen for progress events
        for event := range eventChan {
            switch event.Type {
            case agent.AgentEventTypeProgress:
                s.updateTaskStatus(task, event.Progress)
                s.progress.TaskProgress(task, event.Progress)
            // ... other event types ...
            }
        }
    }
}
```

**Files to modify**:
- `/internal/volley/task.go` (+4 status constants)
- `/internal/volley/progress.go` (~30 lines modified)
- `/internal/llm/agent/agent.go` (~20 lines modified)
- `/internal/volley/scheduler.go` (~15 lines modified)

**Testing**:
```bash
# Run a simple task and verify state transitions
./bin/cliffy --verbose "analyze auth.go"

# Expected output:
# 1   ◔ analyze auth.go (worker 1)    <- initializing
# 1   ◑ analyze auth.go (worker 1)    <- processing
# 1   ◕ analyze auth.go (worker 1)    <- finalizing
# 1   ● analyze auth.go               <- complete
```

---

### Phase 3: Enhanced Tool Visualization (Medium Impact)
**Goal**: Show tool states and nested operations
**Time**: 3-5 hours
**Risk**: Low

#### 3.1 Add Tool State Tracking

Modify `/internal/llm/tools/execution.go` (or create if missing):

```go
type ExecutionState string

const (
    ExecutionStateInitializing ExecutionState = "initializing" // ▤
    ExecutionStateRunning      ExecutionState = "running"      // ▥
    ExecutionStateSuccess      ExecutionState = "success"      // ▣
    ExecutionStateFailed       ExecutionState = "failed"       // ▩
)

type ExecutionMetadata struct {
    // ... existing fields ...
    State ExecutionState  // Add this field
}
```

#### 3.2 Update Progress Display for Tools

Modify `/internal/volley/progress.go`:

```go
func (p *ProgressTracker) formatToolLine(treeChar string, metadata *tools.ExecutionMetadata) string {
    var toolIcon string

    switch metadata.State {
    case tools.ExecutionStateInitializing:
        toolIcon = tools.AsciiToolInitializing  // ▤
    case tools.ExecutionStateRunning:
        toolIcon = tools.AsciiToolRunning       // ▥
    case tools.ExecutionStateSuccess:
        toolIcon = tools.AsciiToolSuccess       // ▣
    case tools.ExecutionStateFailed:
        toolIcon = tools.AsciiToolBlocked       // ▩
    default:
        // Fallback to existing logic
        if metadata.ExitCode != nil && *metadata.ExitCode != 0 {
            toolIcon = tools.AsciiToolFailed
        } else {
            toolIcon = tools.AsciiToolSuccess
        }
    }

    // ... rest of formatting
}
```

**Files to modify**:
- `/internal/llm/tools/execution.go` (create or modify)
- `/internal/volley/progress.go` (~20 lines)

**Testing**:
```bash
# Run task with tools
./bin/cliffy --verbose "refactor auth.go to use bcrypt"

# Expected output:
# 1 ╮ ◑ refactor auth.go (worker 1)
#   ├───▤ read     auth.go  starting...
#   ├───▣ read     auth.go  0.2s
#   ├───▥ edit     auth.go  editing...
#   ╰───▣ edit     auth.go  0.4s
```

---

### Phase 4: Worker Visualization (Optional Feature)
**Goal**: Show worker pool state with `--workers-view` flag
**Time**: 6-8 hours
**Risk**: Medium

#### 4.1 Add CLI Flag

Modify `/cmd/cliffy/main.go`:

```go
var (
    verbose     bool
    workersView bool  // Add this
    // ... other flags ...
)

func init() {
    // ... existing flags ...
    rootCmd.PersistentFlags().BoolVar(&workersView, "workers-view", false, "Show worker pool visualization")
}
```

#### 4.2 Create Worker Visualizer

Create `/internal/volley/worker_visualizer.go`:

```go
package volley

import (
    "fmt"
    "io"
    "sync"

    "github.com/bwl/cliffy/internal/llm/tools"
)

type WorkerVisualizer struct {
    enabled    bool
    out        io.Writer
    mu         sync.Mutex
    workers    map[int]*WorkerState  // workerID -> state
    queueSize  int
}

type WorkerState struct {
    ID       int
    Status   string  // "idle", "active", "overload"
    TaskID   int
    TaskDesc string
}

func (wv *WorkerVisualizer) RenderWorkerView() {
    wv.mu.Lock()
    defer wv.mu.Unlock()

    // Print header
    fmt.Fprintf(wv.out, "║ Worker Pool ║\n")

    // Print each worker
    for i := 1; i <= len(wv.workers); i++ {
        state := wv.workers[i]
        var icon string

        switch state.Status {
        case "idle":
            icon = tools.AsciiWorkerIdle      // ⬡
        case "active":
            icon = tools.AsciiWorkerActive    // ⬢
        case "overload":
            icon = tools.AsciiWorkerOverload  // ⬣
        }

        if state.TaskID > 0 {
            fmt.Fprintf(wv.out, "║ Worker %d %s ║ %s %d  %s\n",
                i, icon, tools.AsciiFlowSimple, state.TaskID, state.TaskDesc)
        } else {
            fmt.Fprintf(wv.out, "║ Worker %d %s ║ (idle)\n", i, icon)
        }
    }

    // Print queue
    fmt.Fprintf(wv.out, "║═══════════║\n")
    fmt.Fprintf(wv.out, "║   Queue   ║ %d tasks waiting\n", wv.queueSize)
}
```

#### 4.3 Integrate with Progress Tracker

Modify `/internal/volley/progress.go`:

```go
type ProgressTracker struct {
    // ... existing fields ...
    workerView *WorkerVisualizer  // Add this
}

func (p *ProgressTracker) renderAll() {
    // ... existing render logic ...

    // If worker view enabled, render it
    if p.workerView != nil && p.workerView.enabled {
        p.workerView.RenderWorkerView()
    }
}
```

**Files to create/modify**:
- `/internal/volley/worker_visualizer.go` (create, ~150 lines)
- `/internal/volley/progress.go` (~30 lines)
- `/cmd/cliffy/main.go` (~5 lines)

**Testing**:
```bash
./bin/cliffy --workers-view --verbose "task1" "task2" "task3" "task4" "task5"

# Expected output:
# ◍═══╕  5 tasks volleyed ⬢⬢⬢ (3 workers)
#
# ║ Worker 1 ⬢ ║ → 1  task1
# ║ Worker 2 ⬢ ║ → 2  task2
# ║ Worker 3 ⬢ ║ → 3  task3
# ║═══════════║
# ║   Queue   ║ 2 tasks waiting
```

---

### Phase 5: Retry & Error Visualization (Quality of Life)
**Goal**: Show retry loops and error chains clearly
**Time**: 4-6 hours
**Risk**: Low-Medium

#### 5.1 Enhanced Retry Display

Modify `/internal/volley/progress.go`:

```go
func (p *ProgressTracker) TaskRetrying(task Task, attempt int, delay time.Duration) {
    if !p.enabled {
        return
    }

    p.mu.Lock()
    defer p.mu.Unlock()

    state := p.taskStates[task.Index]
    if state != nil {
        state.status = "retrying"
        state.retryAttempt = attempt
        state.retryDelay = delay
    }

    // Render retry status
    fmt.Fprintf(p.out, "%d   %s %s (attempt %d) %s\n",
        task.Index,
        tools.AsciiTaskFailed,      // ◌
        truncate(task.Prompt, 50),
        attempt,
        tools.AsciiFlowRetry)       // ⟲

    fmt.Fprintf(p.out, "    %s Retrying in %.1fs...\n",
        tools.AsciiPaused,          // ⏸
        delay.Seconds())
}
```

#### 5.2 Error Chain Visualization

Add new method to show error context:

```go
func (p *ProgressTracker) TaskError(task Task, err error, metadata *ErrorMetadata) {
    if !p.enabled {
        return
    }

    p.mu.Lock()
    defer p.mu.Unlock()

    // Show error with return arrow
    fmt.Fprintf(p.out, "    %s Error: %v\n",
        tools.AsciiFlowReturn,      // ⤴
        err)

    // If rate limited, show special icon
    if metadata != nil && metadata.IsRateLimit {
        fmt.Fprintf(p.out, "    %s Rate limited (HTTP %d)\n",
            tools.AsciiRateLimit,   // ⚡
            metadata.StatusCode)
    }
}
```

**Files to modify**:
- `/internal/volley/progress.go` (~40 lines)
- `/internal/volley/task.go` (~15 lines for ErrorMetadata)

**Testing**:
```bash
# Trigger a retry scenario
./bin/cliffy --verbose "make failing API call"

# Expected output:
# 1   ◌ make failing API call (attempt 1) ⟲
#     ⤴ Error: rate limited (429)
#     ⏸ Retrying in 2.0s...
# 1   ◔ make failing API call (worker 1, attempt 2)
```

---

### Phase 6: Advanced Features (Future Enhancements)
**Goal**: Power user features and debugging
**Time**: 8-12 hours
**Risk**: Medium-High

These are optional enhancements for future iterations:

#### 6.1 Metrics View
- Add `--metrics` flag
- Show detailed token usage, timing, cost breakdown
- Create `/internal/volley/metrics.go`

#### 6.2 Timeline View
- Add `--timeline` flag
- Gantt-chart style task execution visualization
- Create `/internal/volley/timeline.go`

#### 6.3 Data Flow Diagrams
- Add `--flow-diagram` flag
- Show task pipeline with arrows and containers
- Create `/internal/volley/flow_diagram.go`

#### 6.4 Cache Indicators
- Show ◉ for cached results
- Track cache hit rate
- Display cache savings

---

## File Structure Summary

### New Files to Create
```
internal/volley/worker_visualizer.go   (~150 lines)  [Phase 4]
internal/volley/metrics.go             (~200 lines)  [Phase 6]
internal/volley/timeline.go            (~180 lines)  [Phase 6]
internal/volley/flow_diagram.go        (~220 lines)  [Phase 6]
docs/cliffy-visual-system.md           (existing)
docs/cliffy-visual-mockups.md          (existing)
docs/cliffy-visual-implementation.md   (this file)
```

### Existing Files to Modify
```
internal/llm/tools/ascii.go            (+40 lines)   [Phase 1]
internal/volley/task.go                (+20 lines)   [Phase 2]
internal/volley/progress.go            (~100 lines)  [Phases 2-5]
internal/llm/agent/agent.go            (~20 lines)   [Phase 2]
internal/volley/scheduler.go           (~30 lines)   [Phase 2]
internal/llm/tools/execution.go        (+15 lines)   [Phase 3]
cmd/cliffy/main.go                     (+10 lines)   [Phase 4]
```

### Total Line Count Estimate
- New code: ~750 lines
- Modified code: ~235 lines
- Documentation: ~1200 lines
- **Total: ~2185 lines**

---

## Testing Strategy

### Unit Tests
```bash
# Test symbol constants
go test ./internal/llm/tools -v -run TestAsciiSymbols

# Test progress rendering
go test ./internal/volley -v -run TestProgressTracker

# Test state transitions
go test ./internal/volley -v -run TestTaskStates
```

### Integration Tests
```bash
# Test single task with progress states
./bin/cliffy --verbose "analyze auth.go"

# Test parallel execution
./bin/cliffy --verbose "task1" "task2" "task3"

# Test retry scenarios
./bin/cliffy --verbose "simulate rate limit error"

# Test worker view
./bin/cliffy --workers-view "task1" "task2" "task3" "task4" "task5"
```

### Regression Tests
```bash
# Ensure existing functionality still works
./bin/cliffy "simple task"                    # Should work
./bin/cliffy --fast "quick task"              # Should work
./bin/cliffy --estimate "task1" "task2"       # Should work
```

### Visual Tests
Run in different terminal emulators:
- iTerm2 (macOS)
- Terminal.app (macOS)
- GNOME Terminal (Linux)
- Windows Terminal (Windows)
- tmux
- screen

---

## Rollout Plan

### Week 1: Foundation
- [ ] Phase 1: Symbol library (Day 1-2)
- [ ] Phase 2: Basic processing states (Day 3-5)

### Week 2: Enhancement
- [ ] Phase 3: Tool visualization (Day 1-2)
- [ ] Phase 5: Retry/error viz (Day 3-4)
- [ ] Testing and bug fixes (Day 5)

### Week 3: Polish
- [ ] Phase 4: Worker view (optional)
- [ ] Documentation updates
- [ ] User testing and feedback

### Week 4+: Advanced Features (as needed)
- [ ] Phase 6: Metrics, timeline, diagrams
- [ ] Performance optimization
- [ ] Color support (future TUI)

---

## Success Metrics

### Quantitative
- [ ] All existing tests pass
- [ ] New features covered by tests (>80% coverage)
- [ ] No performance regression (<5% slower)
- [ ] Works in all major terminal emulators

### Qualitative
- [ ] Users can instantly understand task state
- [ ] Error scenarios are clear and actionable
- [ ] Visual output is consistent and polished
- [ ] "Cliffy look" is distinctive and memorable

---

## Risk Mitigation

### Risk: Breaking existing functionality
- **Mitigation**: Keep old symbols as fallbacks, extensive regression testing

### Risk: Terminal compatibility issues
- **Mitigation**: Test across terminals, provide ASCII-only mode

### Risk: Performance impact from frequent re-renders
- **Mitigation**: Throttle render calls, optimize cursor movements

### Risk: User confusion with new symbols
- **Mitigation**: Add `--help-symbols` command, clear documentation

---

## Future Considerations

### Color Support
When adding TUI/color support later:
```go
// Example color mapping
var colorMap = map[string]lipgloss.Style{
    "success":    lipgloss.NewStyle().Foreground(lipgloss.Color("10")),  // Green
    "processing": lipgloss.NewStyle().Foreground(lipgloss.Color("11")),  // Yellow
    "error":      lipgloss.NewStyle().Foreground(lipgloss.Color("9")),   // Red
}
```

### Interactive Mode
Future interactive features:
- Collapse/expand tasks with keyboard
- Filter by status
- Jump to failed tasks
- Real-time log tailing

### Export Formats
- JSON output with symbols
- HTML report with color
- SVG timeline diagrams

---

## Appendix: Symbol Unicode Reference

Quick reference for developers:

```
Task States:
  ○  U+25CB  WHITE CIRCLE
  ◔  U+25D4  CIRCLE WITH UPPER RIGHT QUADRANT BLACK
  ◑  U+25D1  CIRCLE WITH RIGHT HALF BLACK
  ◕  U+25D5  CIRCLE WITH ALL BUT UPPER LEFT QUADRANT BLACK
  ●  U+25CF  BLACK CIRCLE
  ◌  U+25CC  DOTTED CIRCLE
  ⦿  U+29BF  CIRCLED BULLET
  ◉  U+25C9  FISHEYE

Workers:
  ⬡  U+2B21  WHITE HEXAGON
  ⬢  U+2B22  BLACK HEXAGON
  ⬣  U+2B23  HORIZONTAL BLACK HEXAGON

Tools:
  ▣  U+25A3  WHITE SQUARE CONTAINING BLACK SMALL SQUARE
  ▤  U+25A4  SQUARE WITH HORIZONTAL FILL
  ▥  U+25A5  SQUARE WITH VERTICAL FILL
  ▦  U+25A6  SQUARE WITH ORTHOGONAL CROSSHATCH FILL
  ▩  U+25A9  SQUARE WITH DIAGONAL CROSSHATCH FILL
  ☒  U+2612  BALLOT BOX WITH X

Flow:
  →  U+2192  RIGHTWARDS ARROW
  ⇒  U+21D2  RIGHTWARDS DOUBLE ARROW
  ⇨  U+21E8  RIGHTWARDS WHITE ARROW
  ⟲  U+27F2  ANTICLOCKWISE GAPPED CIRCLE ARROW
  ⤴  U+2934  ARROW POINTING RIGHTWARDS THEN CURVING UPWARDS

Status:
  ⚠  U+26A0  WARNING SIGN
  ✗  U+2717  BALLOT X
  ⊗  U+2297  CIRCLED TIMES
  ⏸  U+23F8  DOUBLE VERTICAL BAR
  ⚡  U+26A1  HIGH VOLTAGE SIGN

Structure:
  ╮  U+256E  BOX DRAWINGS LIGHT ARC DOWN AND LEFT
  ├  U+251C  BOX DRAWINGS LIGHT VERTICAL AND RIGHT
  ╰  U+2570  BOX DRAWINGS LIGHT ARC UP AND RIGHT
  ─  U+2500  BOX DRAWINGS LIGHT HORIZONTAL
  ═  U+2550  BOX DRAWINGS DOUBLE HORIZONTAL
  ║  U+2551  BOX DRAWINGS DOUBLE VERTICAL
```

---

## Questions & Decisions Log

### Q: Should we keep the spinner animation or use progressive states?
**A**: Keep spinner as fallback, but prefer progressive states (○◔◑◕●) for clarity. Use spinner only when state can't be determined.

### Q: How often should we update the display?
**A**: Throttle to max 10 updates/second per task to avoid flicker.

### Q: Should worker view be default or opt-in?
**A**: Opt-in via `--workers-view` flag. Default is compact mode.

### Q: What about accessibility?
**A**: All symbols have semantic names. Screen readers will read state names, not symbols.

### Q: How to handle narrow terminals?
**A**: Auto-detect terminal width, truncate descriptions as needed, maintain symbol visibility.

---

## Getting Started

To begin implementation:

1. **Read the design doc**: `docs/cliffy-visual-system.md`
2. **Review mockups**: `docs/cliffy-visual-mockups.md`
3. **Start with Phase 1**: Add symbols to `ascii.go`
4. **Test as you go**: Run `./bin/cliffy` after each change
5. **Get feedback**: Show progress to users early

**First PR**: Phase 1 + Phase 2 (symbols + basic states)
**Target**: Merge within 1 week

Let's build the visual Cliffy experience! 🎾
