# Streaming Event Architecture Research

**Date**: 2025-10-01
**Status**: Research Complete, Ready for Implementation
**Estimated Effort**: 4-6 hours for full streaming implementation

---

## Executive Summary

Cliffy already has a streaming event architecture in place but it's currently underutilized. The agent emits events through a channel (`<-chan AgentEvent`), but the volley scheduler only waits for the final event. By processing all events in real-time, we can achieve:

1. **Real-time tool traces** - Show tool execution as it happens
2. **Progress feedback** - Display updates during long operations
3. **Better UX** - Solve "is it stuck?" anxiety

**Recommendation**: Implement streaming tool traces (Option B) instead of post-execution display (Option A).

---

## Current Architecture

### Event Flow

```
Provider (OpenRouter/etc)
    ↓ StreamResponse() returns <-chan ProviderEvent
Agent (agent.go)
    ↓ Run() returns <-chan AgentEvent
    ↓ Processes provider events in real-time
    ↓ BUT: Only emits ONE final event
Volley Scheduler (scheduler.go:225-252)
    ↓ Waits for final AgentEventTypeResponse
    ↓ Ignores intermediate events
Output (main.go)
    ↓ Displays result only
```

### Key Files and Functions

**Agent Event System** (`internal/llm/agent/agent.go`)
- **Line 53**: `Run() (<-chan AgentEvent, error)` - Returns event stream
- **Line 296**: Creates buffered channel: `events := make(chan AgentEvent, 1)`
- **Line 333**: Sends final event: `events <- result`
- **Line 474**: `eventChan := a.provider.StreamResponse()` - Receives provider stream
- **Line 508-593**: Tool execution loop - WHERE WE NEED TO EMIT EVENTS

**Event Types** (`internal/llm/agent/agent.go:24-32`)
```go
const (
    AgentEventTypeError     AgentEventType = "error"
    AgentEventTypeResponse  AgentEventType = "response"
    AgentEventTypeSummarize AgentEventType = "summarize"
    AgentEventTypeToolTrace AgentEventType = "tool_trace"  // NEW - not yet emitted
    AgentEventTypeProgress  AgentEventType = "progress"    // NEW - not yet emitted
)
```

**Volley Event Processing** (`internal/volley/scheduler.go:225-252`)
```go
for event := range events {
    switch event.Type {
    case agent.AgentEventTypeError:
        return "", Usage{}, event.Error
    case agent.AgentEventTypeResponse:
        // Extract final output
        return output, usage, nil
    }
}
```

**Problem**: Only processes Error and Response events. ToolTrace and Progress events would be ignored even if emitted.

---

## Infrastructure Already in Place

### ✅ What We Have

1. **Event streaming architecture** - Agent → Volley already uses channels
2. **Event type definitions** - `AgentEventTypeToolTrace` and `AgentEventTypeProgress` already defined
3. **Metadata collection** - All tools capture `ExecutionMetadata` (completed in Sprint 1)
4. **Formatting logic** - `internal/output/formatter.go` can format tool traces
5. **Progress tracker** - `internal/volley/progress.go` handles display
6. **Verbosity system** - Three-tier verbosity already implemented

### ❌ What's Missing

1. **Event emission from agent** - Tools execute but don't emit trace events
2. **Event processing in volley** - Scheduler ignores non-final events
3. **Context plumbing** - Event channel not available in tool execution context
4. **Progress display** - Tracker doesn't have method to show tool traces

---

## Two Implementation Options

### Option A: Post-Execution Display (Simple)

**Approach**: Extract tool metadata from message history after task completes.

**Pros**:
- ✅ Low risk (no architecture changes)
- ✅ Can be done in 2-3 hours
- ✅ Achieves core goal: see what tools did

**Cons**:
- ❌ Not real-time (displays after completion)
- ❌ Doesn't solve "is it stuck?" problem
- ❌ Doesn't enable progress events
- ❌ Requires parsing message history

**Implementation**:
```go
// In scheduler.go after task completes
messages, _ := s.messageStore.List(ctx, sessionID)
for _, msg := range messages {
    if msg.Role == message.Tool {
        for _, toolResult := range msg.ToolResults() {
            // Extract metadata and display
        }
    }
}
```

**Verdict**: ⚠️ Works but misses the real opportunity for streaming.

---

### Option B: Real-Time Streaming (Recommended) ⭐

**Approach**: Emit tool trace events during execution via existing event channel.

**Pros**:
- ✅ Real-time feedback during execution
- ✅ Solves "is it stuck?" anxiety
- ✅ Enables progress events (Phase 3 of plan)
- ✅ Uses existing streaming infrastructure
- ✅ Parallel-safe (each task has own channel)
- ✅ Aligns with user feedback priorities

**Cons**:
- ⚠️ More complex (need context plumbing)
- ⚠️ Requires coordination between agent/volley

**Estimated Effort**: 4-6 hours

**Why This is Better**:
1. **Foundation for future features** - Progress events come for free
2. **Better UX** - See tools execute in real-time
3. **Architectural alignment** - Uses the streaming pattern that's already there
4. **Minimal overhead** - Events only sent when needed

---

## Detailed Implementation Plan (Option B)

### Phase 1: Emit Tool Events from Agent (1.5 hours)

**Location**: `internal/llm/agent/agent.go:508-593`

**Problem**: Tool execution happens inside `streamAndHandleEvents()`, but the `events` channel isn't accessible in the tool execution loop.

**Solution**: Pass event channel through context.

#### Step 1.1: Define context key for event channel

```go
// Add near top of agent.go (after imports)
type eventChanKeyType struct{}
var eventChanKey = eventChanKeyType{}
```

#### Step 1.2: Add event channel to context

In `streamAndHandleEvents()` function (line ~455):

```go
func (a *agent) streamAndHandleEvents(ctx context.Context, sessionID string, msgHistory []message.Message) (...) {
    ctx = context.WithValue(ctx, tools.SessionIDContextKey, sessionID)

    // NEW: Create event channel for tool emissions
    // Use buffered channel to avoid blocking tools
    toolEventChan := make(chan AgentEvent, 10)
    ctx = context.WithValue(ctx, eventChanKey, toolEventChan)

    // Start goroutine to forward tool events to main events channel
    go func() {
        for toolEvent := range toolEventChan {
            // Forward to main events channel (need to get reference)
            // This is where we need to thread the main eventsChan
        }
    }()

    // ... rest of function
}
```

**Wait, there's a problem**: We need access to the main `events` channel that was created in `Run()`.

**Better approach**: Pass the events channel directly through context:

```go
// In Run() at line 296
events := make(chan AgentEvent, 10)  // Increase buffer for tool events

// In the goroutine (line 311)
go func() {
    // ... existing setup ...

    // NEW: Pass events channel through context for tool emissions
    genCtx = context.WithValue(genCtx, eventChanKey, events)

    result := a.processGeneration(genCtx, sessionID, content, attachmentParts)
    // ... rest of function
}()
```

#### Step 1.3: Emit tool events after execution

In tool execution loop (lines 549-593):

```go
go func() {
    response, err := tool.Run(ctx, tools.ToolCall{
        ID:    toolCall.ID,
        Name:  toolCall.Name,
        Input: toolCall.Input,
    })
    resultChan <- toolExecResult{response: response, err: err}
}()

// ... wait for result ...

case result := <-resultChan:
    toolResponse = result.response
    toolErr = result.err

    // NEW: Emit tool trace event
    if toolResponse.ExecutionMetadata != nil {
        if eventChan, ok := ctx.Value(eventChanKey).(chan AgentEvent); ok {
            select {
            case eventChan <- AgentEvent{
                Type:         AgentEventTypeToolTrace,
                ToolMetadata: toolResponse.ExecutionMetadata,
            }:
            default:
                // Don't block if channel is full
                slog.Warn("Tool event channel full, dropping event")
            }
        }
    }
```

**Files to modify**:
- `internal/llm/agent/agent.go`

**Testing checkpoint**: At this point, events are emitted but not yet processed.

---

### Phase 2: Process Tool Events in Volley (1.5 hours)

**Location**: `internal/volley/scheduler.go:206-256`

#### Step 2.1: Update event processing loop

Current code (simplified):
```go
func (s *Scheduler) executeViaAgent(ctx context.Context, prompt string) (string, Usage, error) {
    events, err := s.agent.Run(ctx, sessionID, prompt)

    for event := range events {
        switch event.Type {
        case agent.AgentEventTypeError:
            return "", Usage{}, event.Error
        case agent.AgentEventTypeResponse:
            // ... extract output ...
            return output, usage, nil
        }
    }
}
```

New code:
```go
func (s *Scheduler) executeViaAgent(ctx context.Context, prompt string, task Task) (string, Usage, []*tools.ExecutionMetadata, error) {
    events, err := s.agent.Run(ctx, sessionID, prompt)
    if err != nil {
        return "", Usage{}, nil, fmt.Errorf("failed to run agent: %w", err)
    }

    var output string
    var usage Usage
    var toolMetadata []*tools.ExecutionMetadata

    for event := range events {
        switch event.Type {
        case agent.AgentEventTypeToolTrace:
            // NEW: Real-time tool trace display
            if s.options.Verbosity != config.VerbosityQuiet {
                s.progress.ToolExecuted(task, event.ToolMetadata)
            }
            // Collect metadata for result
            toolMetadata = append(toolMetadata, event.ToolMetadata)

        case agent.AgentEventTypeProgress:
            // NEW: Progress updates (Phase 4)
            if s.options.Verbosity != config.VerbosityQuiet {
                s.progress.ShowProgress(task, event.Progress)
            }

        case agent.AgentEventTypeError:
            return "", Usage{}, toolMetadata, event.Error

        case agent.AgentEventTypeResponse:
            // Extract output (existing logic)
            messages, err := s.messageStore.List(ctx, sessionID)
            if err != nil {
                return "", Usage{}, toolMetadata, fmt.Errorf("failed to list messages: %w", err)
            }

            for _, msg := range messages {
                if msg.Role == message.Assistant {
                    output += msg.Content().Text
                }
            }

            usage = Usage{
                InputTokens:  event.TokenUsage.InputTokens,
                OutputTokens: event.TokenUsage.OutputTokens,
                TotalTokens:  event.TokenUsage.InputTokens + event.TokenUsage.OutputTokens,
            }

            return output, usage, toolMetadata, nil
        }
    }

    return output, usage, toolMetadata, nil
}
```

#### Step 2.2: Update executeTask to store metadata

```go
func (s *Scheduler) executeTask(ctx context.Context, workerID int, task Task) TaskResult {
    // ... existing setup ...

    // Execute via agent
    output, usage, toolMetadata, err := s.executeViaAgent(ctx, prompt, task)

    if err != nil {
        // ... error handling ...
    }

    // Success
    result.Status = TaskStatusSuccess
    result.Output = output
    result.TokensInput = usage.InputTokens
    result.TokensOutput = usage.OutputTokens
    result.TokensTotal = usage.TotalTokens
    result.Cost = s.calculateCost(usage)
    result.Duration = time.Since(startTime)
    result.Error = nil
    result.Model = s.agent.Model().ID
    result.ToolMetadata = toolMetadata  // NEW

    s.progress.TaskCompleted(task, result)

    return result
}
```

**Files to modify**:
- `internal/volley/scheduler.go`

---

### Phase 3: Display Tool Traces (1 hour)

**Location**: `internal/volley/progress.go`

#### Step 3.1: Add ToolExecuted method

```go
func (p *ProgressTracker) ToolExecuted(task Task, metadata *tools.ExecutionMetadata) {
    if !p.enabled || metadata == nil {
        return
    }

    // Format tool trace using existing formatter
    trace := output.FormatToolTrace(metadata, config.VerbosityNormal)
    if trace != "" {
        // Print to stderr to avoid mixing with stdout
        fmt.Fprintln(os.Stderr, trace)
    }
}
```

#### Step 3.2: Add ShowProgress method (for Phase 4)

```go
func (p *ProgressTracker) ShowProgress(task Task, message string) {
    if !p.enabled {
        return
    }

    // Use carriage return to update same line
    fmt.Fprintf(os.Stderr, "\r[%d/%d] ⋯ %s", task.Index, p.totalTasks, message)
}
```

**Files to modify**:
- `internal/volley/progress.go`

**Files to import**:
- `internal/output` (for FormatToolTrace)

---

### Phase 4: Progress Events (2 hours - Optional/Future)

**Location**: Individual tool files (`internal/llm/tools/*.go`)

#### Step 4.1: Add progress helper function

```go
// In tools.go or new progress.go
func emitProgress(ctx context.Context, message string) {
    if eventChan, ok := ctx.Value(eventChanKey).(chan agent.AgentEvent); ok {
        select {
        case eventChan <- agent.AgentEvent{
            Type:     agent.AgentEventTypeProgress,
            Progress: message,
        }:
        default:
            // Don't block
        }
    }
}
```

**Problem**: tools.go can't import agent (circular dependency)

**Solution**: Define progress function in agent package and access via context:

```go
// In agent.go
type progressFuncKeyType struct{}
var progressFuncKey = progressFuncKeyType{}

type ProgressFunc func(message string)

// In streamAndHandleEvents()
progressFunc := func(message string) {
    if eventChan, ok := ctx.Value(eventChanKey).(chan AgentEvent); ok {
        select {
        case eventChan <- AgentEvent{
            Type:     AgentEventTypeProgress,
            Progress: message,
        }:
        default:
        }
    }
}
ctx = context.WithValue(ctx, progressFuncKey, ProgressFunc(progressFunc))
```

#### Step 4.2: Use in tools for long operations

In `view.go`:
```go
func (v *viewTool) Run(ctx context.Context, call ToolCall) (ToolResponse, error) {
    start := time.Now()

    fileInfo, err := os.Stat(filePath)
    if err != nil {
        return NewTextErrorResponse(fmt.Sprintf("failed to stat file: %v", err)), nil
    }

    // NEW: Show progress for large files
    if fileInfo.Size() > 1*1024*1024 { // >1MB
        if progressFunc, ok := ctx.Value(progressFuncKey).(ProgressFunc); ok {
            progressFunc("Reading large file...")
        }
    }

    // ... rest of function
}
```

Similar updates in:
- `bash.go` - Show command being executed for long-running commands
- `glob.go` - Show progress when scanning many directories
- `grep.go` - Show progress when searching large codebases

---

## Testing Strategy

### Unit Tests

```go
// Test tool event emission
func TestAgentEmitsToolEvents(t *testing.T) {
    // Create agent with mock tool
    // Run task that uses tool
    // Verify ToolTrace event is emitted
}

// Test event channel doesn't block
func TestToolEventNonBlocking(t *testing.T) {
    // Fill event channel buffer
    // Execute tool
    // Verify it doesn't hang
}

// Test verbosity filtering
func TestQuietModeNoEvents(t *testing.T) {
    // Run with VerbosityQuiet
    // Verify no tool traces displayed
}
```

### Integration Tests

```bash
# Test 1: Single tool execution
./bin/cliffy "summarize README.md"
# Expected: [TOOL] read: README.md (N lines, X KB) — 0.Xs

# Test 2: Multiple tools
./bin/cliffy "find all .go files in internal/"
# Expected: [TOOL] glob: internal/**/*.go → N matches — 0.Xs

# Test 3: Quiet mode
./bin/cliffy --quiet "summarize README.md"
# Expected: Only result text, no [TOOL] lines

# Test 4: Parallel execution
./bin/cliffy "task1" "task2" "task3"
# Expected: Tool traces interleaved correctly, no corruption

# Test 5: Long operation with progress (Phase 4)
./bin/cliffy "analyze large-file.txt"
# Expected: [TOOL] read: large-file.txt (reading...)
#           [TOOL] read: large-file.txt (50k lines, 2.5MB) — 1.2s
```

### Manual Testing Checklist

- [ ] Single task shows tool traces
- [ ] Multiple tasks show traces in correct order
- [ ] `--quiet` suppresses traces
- [ ] `--verbose` shows traces (same as normal for now)
- [ ] No duplicate events
- [ ] No dropped events
- [ ] No deadlocks/hangs
- [ ] Stderr output doesn't corrupt stdout
- [ ] Tool metadata is accurate (files, durations, counts)
- [ ] Progress updates work for long operations (Phase 4)

---

## Benefits of Streaming Approach

### 1. Real-Time Feedback
**Before**: Silent for 10+ seconds, then results appear
**After**: See each tool execute as it happens

```
[1/3] ▶ Analyzing codebase structure
[TOOL] glob: **/*.rs → 45 matches — 0.2s
[TOOL] read: src/lib.rs (890 lines, 35KB) — 0.8s
[TOOL] read: src/main.rs (123 lines, 4KB) — 0.1s
[1/3] ✓ Analyzing codebase structure (2.4s)
```

### 2. Debugging & Trust
Users can verify:
- Which files were actually read
- What patterns were searched
- What commands were run
- How long each operation took

### 3. Foundation for Progress Events
Phase 4 comes almost for free:
```
[TOOL] bash: cargo build (running...)
[TOOL] bash: cargo build (compiling...)
[TOOL] bash: cargo build (linking...)
[TOOL] bash: cargo build (exit 0) — 15.3s
```

### 4. Parallel Safety
Each task gets its own event channel, so:
- No race conditions
- No output corruption
- Events stay with correct task

### 5. Minimal Overhead
- Events only emitted when tools execute
- Non-blocking send (won't slow down tools)
- Display only when verbosity allows
- Buffer prevents backpressure

---

## Implementation Timeline

| Phase | Description | Effort | Dependencies |
|-------|-------------|--------|--------------|
| 1 | Emit tool events from agent | 1.5h | None |
| 2 | Process events in volley | 1.5h | Phase 1 |
| 3 | Display tool traces | 1h | Phase 1, 2 |
| 4 | Progress events (optional) | 2h | Phase 1, 2, 3 |
| **Total** | **Full streaming implementation** | **4-6h** | |

**Recommended first session**: Phases 1-3 (4 hours) to get core streaming working.
**Second session**: Phase 4 (2 hours) for progress events if desired.

---

## Risks & Mitigations

### Risk 1: Deadlock from full channel
**Mitigation**: Use buffered channel (10 events) + non-blocking send with select/default

### Risk 2: Event order corruption in parallel execution
**Mitigation**: Each task has own event channel, events stay isolated

### Risk 3: Performance overhead
**Mitigation**: Events only sent when verbosity allows, minimal serialization

### Risk 4: Stderr/stdout mixing
**Mitigation**: Tool traces to stderr, results to stdout (standard practice)

---

## Comparison with Crush (Parent Project)

Crush has similar architecture but:
- Uses TUI (bubbletea) for display
- Has session persistence
- More complex state management

Cliffy advantages for streaming:
- Simpler output model (stdout/stderr)
- No TUI to coordinate with
- Can use terminal control codes directly

---

## Next Steps

1. **Approve plan** - Decide on Option A (simple) vs Option B (streaming)
2. **Implement Phase 1** - Emit tool events from agent
3. **Implement Phase 2** - Process events in volley
4. **Implement Phase 3** - Display tool traces
5. **Test incrementally** - Each phase is independently testable
6. **Optional Phase 4** - Add progress events for long operations

**Recommendation**: Proceed with Option B (streaming) for maximum value and future-proofing.
