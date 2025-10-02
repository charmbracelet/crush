# Implementation Guide: Streaming Tool Traces

**Approach**: Real-time event streaming (Option B)
**Estimated Effort**: 4-6 hours
**Prerequisites**: Sprint 1 infrastructure (completed)

---

## Quick Start

### Implementation Phases

```
Phase 1: Emit Events    (1.5h)  →  Tools send trace events
Phase 2: Process Events (1.5h)  →  Volley handles events
Phase 3: Display Traces (1h)    →  Show formatted output
Phase 4: Progress       (2h)    →  Long-running feedback
```

### Expected Result

**Before**:
```bash
./bin/cliffy "analyze src/main.go"
[Analysis result appears after 3 seconds]
```

**After**:
```bash
./bin/cliffy "analyze src/main.go"
[TOOL] read: src/main.go (890 lines, 35KB) — 0.8s
[TOOL] bash: go build (exit 0) — 1.2s
[Analysis result]
```

---

## Phase 1: Emit Tool Events from Agent

### Overview
Pass event channel through context so tools can emit trace events.

### Files to Modify
- `internal/llm/agent/agent.go`

### Step 1.1: Define Context Key

**Location**: `agent.go` (near top, after imports)

```go
// Add after package declaration and imports
type eventChanKeyType struct{}
var eventChanKey = eventChanKeyType{}
```

**Why**: Type-safe context key for event channel.

---

### Step 1.2: Thread Event Channel Through Context

**Location**: `agent.go:292` (in `Run()` function)

**Current code**:
```go
func (a *agent) Run(ctx context.Context, sessionID string, content string, attachments ...message.Attachment) (<-chan AgentEvent, error) {
    if !a.Model().SupportsImages && attachments != nil {
        attachments = nil
    }
    events := make(chan AgentEvent, 1)  // ← Too small!
    if a.IsSessionBusy(sessionID) {
        // ... queue handling ...
    }

    genCtx, cancel := context.WithCancel(ctx)
    a.activeRequests.Set(sessionID, cancel)
    startTime := time.Now()

    go func() {
        // ... existing goroutine ...
    }()
    // ...
}
```

**New code**:
```go
func (a *agent) Run(ctx context.Context, sessionID string, content string, attachments ...message.Attachment) (<-chan AgentEvent, error) {
    if !a.Model().SupportsImages && attachments != nil {
        attachments = nil
    }
    events := make(chan AgentEvent, 10)  // ← CHANGED: Increase buffer for tool events
    if a.IsSessionBusy(sessionID) {
        existing, ok := a.promptQueue.Get(sessionID)
        if !ok {
            existing = []string{}
        }
        existing = append(existing, content)
        a.promptQueue.Set(sessionID, existing)
        return nil, nil
    }

    genCtx, cancel := context.WithCancel(ctx)
    a.activeRequests.Set(sessionID, cancel)
    startTime := time.Now()

    go func() {
        slog.Debug("Request started", "sessionID", sessionID)
        defer log.RecoverPanic("agent.Run", func() {
            events <- a.err(fmt.Errorf("panic while running the agent"))
        })

        // NEW: Add event channel to context
        genCtx = context.WithValue(genCtx, eventChanKey, events)

        var attachmentParts []message.ContentPart
        for _, attachment := range attachments {
            attachmentParts = append(attachmentParts, message.BinaryContent{Path: attachment.FilePath, MIMEType: attachment.MimeType, Data: attachment.Content})
        }
        result := a.processGeneration(genCtx, sessionID, content, attachmentParts)
        // ... rest of function unchanged ...
    }()
    a.eventPromptSent(sessionID)
    return events, nil
}
```

**Changes**:
1. Buffer size increased from 1 to 10 (handle multiple tool events)
2. Add event channel to context via `context.WithValue()`

---

### Step 1.3: Emit Tool Events After Execution

**Location**: `agent.go:542-593` (in `streamAndHandleEvents()`, tool execution loop)

**Current code** (around line 573):
```go
select {
case <-ctx.Done():
    // ... cancellation handling ...
case result := <-resultChan:
    toolResponse = result.response
    toolErr = result.err
}

if toolErr != nil {
    slog.Error("Tool execution error", "toolCall", toolCall.ID, "error", toolErr)
    toolResults[i] = message.ToolResult{
        ToolCallID: toolCall.ID,
        Content:    fmt.Sprintf("Tool execution error: %s", toolErr.Error()),
        IsError:    true,
    }
} else {
    toolResults[i] = message.ToolResult{
        ToolCallID: toolCall.ID,
        Content:    toolResponse.Content,
        Metadata:   toolResponse.Metadata,
        IsError:    toolResponse.IsError,
    }
}
```

**New code**:
```go
select {
case <-ctx.Done():
    // ... cancellation handling (unchanged) ...
case result := <-resultChan:
    toolResponse = result.response
    toolErr = result.err

    // NEW: Emit tool trace event if metadata exists
    if toolResponse.ExecutionMetadata != nil {
        if eventChan, ok := ctx.Value(eventChanKey).(chan AgentEvent); ok {
            // Non-blocking send to avoid deadlock
            select {
            case eventChan <- AgentEvent{
                Type:         AgentEventTypeToolTrace,
                ToolMetadata: toolResponse.ExecutionMetadata,
            }:
                // Event sent successfully
            default:
                // Channel full, log and continue
                slog.Warn("Tool event channel full, event dropped",
                    "tool", toolResponse.ExecutionMetadata.ToolName,
                    "duration", toolResponse.ExecutionMetadata.Duration)
            }
        }
    }
}

if toolErr != nil {
    slog.Error("Tool execution error", "toolCall", toolCall.ID, "error", toolErr)
    toolResults[i] = message.ToolResult{
        ToolCallID: toolCall.ID,
        Content:    fmt.Sprintf("Tool execution error: %s", toolErr.Error()),
        IsError:    true,
    }
} else {
    toolResults[i] = message.ToolResult{
        ToolCallID: toolCall.ID,
        Content:    toolResponse.Content,
        Metadata:   toolResponse.Metadata,
        IsError:    toolResponse.IsError,
    }
}
```

**Key points**:
1. Extract event channel from context
2. Use non-blocking send with `select`/`default` to prevent deadlock
3. Log dropped events (shouldn't happen with buffer size 10)
4. Emit event **before** creating ToolResult (to ensure ordering)

---

### Testing Checkpoint 1

At this point, events are emitted but not displayed yet.

**Test with debug logging**:
```go
// Add temporary debug in agent.go after emitting event
slog.Info("Tool event emitted", "tool", toolResponse.ExecutionMetadata.ToolName)
```

**Run**:
```bash
./bin/cliffy "what files are in internal/llm/tools"
```

**Expected logs**:
```
INFO Tool event emitted tool=glob
```

If you see this log, Phase 1 is working! ✅

---

## Phase 2: Process Tool Events in Volley

### Overview
Update volley scheduler to process all events (not just final one).

### Files to Modify
- `internal/volley/scheduler.go`

---

### Step 2.1: Update executeViaAgent Signature

**Location**: `scheduler.go:206`

**Current signature**:
```go
func (s *Scheduler) executeViaAgent(ctx context.Context, prompt string) (string, Usage, error)
```

**New signature**:
```go
func (s *Scheduler) executeViaAgent(ctx context.Context, prompt string, task Task) (string, Usage, []*tools.ExecutionMetadata, error)
```

**Changes**:
1. Add `task Task` parameter (needed for progress display)
2. Return `[]*tools.ExecutionMetadata` to collect tool info

---

### Step 2.2: Update Event Processing Loop

**Location**: `scheduler.go:207-256`

**Current code**:
```go
func (s *Scheduler) executeViaAgent(ctx context.Context, prompt string) (string, Usage, error) {
    sessionID := fmt.Sprintf("volley-%d", time.Now().UnixNano())

    events, err := s.agent.Run(ctx, sessionID, prompt)
    if err != nil {
        return "", Usage{}, fmt.Errorf("failed to run agent: %w", err)
    }

    if events == nil {
        return "", Usage{}, fmt.Errorf("request was queued unexpectedly")
    }

    var output string
    var usage Usage

    for event := range events {
        switch event.Type {
        case agent.AgentEventTypeError:
            return "", Usage{}, event.Error

        case agent.AgentEventTypeResponse:
            messages, err := s.messageStore.List(ctx, sessionID)
            if err != nil {
                return "", Usage{}, fmt.Errorf("failed to list messages: %w", err)
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

            return output, usage, nil
        }
    }

    return output, usage, nil
}
```

**New code**:
```go
func (s *Scheduler) executeViaAgent(ctx context.Context, prompt string, task Task) (string, Usage, []*tools.ExecutionMetadata, error) {
    sessionID := fmt.Sprintf("volley-%d", time.Now().UnixNano())

    events, err := s.agent.Run(ctx, sessionID, prompt)
    if err != nil {
        return "", Usage{}, nil, fmt.Errorf("failed to run agent: %w", err)
    }

    if events == nil {
        return "", Usage{}, nil, fmt.Errorf("request was queued unexpectedly")
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
            // NEW: Progress updates (for Phase 4)
            if s.options.Verbosity != config.VerbosityQuiet {
                s.progress.ShowProgress(task, event.Progress)
            }

        case agent.AgentEventTypeError:
            return "", Usage{}, toolMetadata, event.Error

        case agent.AgentEventTypeResponse:
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

**Changes**:
1. Added `toolMetadata` slice to collect execution metadata
2. Handle `AgentEventTypeToolTrace` - display and collect
3. Handle `AgentEventTypeProgress` (ready for Phase 4)
4. Return metadata in all return statements

---

### Step 2.3: Update executeTask to Use New Signature

**Location**: `scheduler.go:126-203`

**Current code** (around line 155):
```go
// Execute via agent
output, usage, err := s.executeViaAgent(ctx, prompt)
```

**New code**:
```go
// Execute via agent
output, usage, toolMetadata, err := s.executeViaAgent(ctx, prompt, task)
```

**And later** (around line 190):
```go
result.Status = TaskStatusSuccess
result.Output = output
result.TokensInput = usage.InputTokens
result.TokensOutput = usage.OutputTokens
result.TokensTotal = usage.TotalTokens
result.Cost = s.calculateCost(usage)
result.Duration = time.Since(startTime)
result.Error = nil
result.Model = s.agent.Model().ID
result.ToolMetadata = toolMetadata  // NEW: Store tool metadata

s.progress.TaskCompleted(task, result)

return result
```

---

### Testing Checkpoint 2

At this point, events are processed but not yet displayed.

**Test with debug logging**:
```go
// Add in scheduler.go in ToolTrace case
fmt.Fprintf(os.Stderr, "DEBUG: Tool trace received: %s\n", event.ToolMetadata.ToolName)
```

**Run**:
```bash
./bin/cliffy "what files are in internal/llm/tools"
```

**Expected output**:
```
DEBUG: Tool trace received: glob
[... rest of output ...]
```

If you see this debug line, Phase 2 is working! ✅

---

## Phase 3: Display Tool Traces

### Overview
Add display methods to progress tracker and wire up formatting.

### Files to Modify
- `internal/volley/progress.go`

---

### Step 3.1: Add ToolExecuted Method

**Location**: `progress.go` (add new method after existing methods)

```go
// ToolExecuted displays a tool execution trace
func (p *ProgressTracker) ToolExecuted(task Task, metadata *tools.ExecutionMetadata) {
    if !p.enabled || metadata == nil {
        return
    }

    // Import the formatter (add to imports at top of file)
    // "github.com/bwl/cliffy/internal/output"

    // Format and print tool trace
    trace := output.FormatToolTrace(metadata, config.VerbosityNormal)
    if trace != "" {
        // Print to stderr to avoid mixing with stdout
        fmt.Fprintln(os.Stderr, trace)
    }
}
```

**Required import**:
```go
import (
    // ... existing imports ...
    "github.com/bwl/cliffy/internal/output"
)
```

---

### Step 3.2: Add ShowProgress Method (for Phase 4)

**Location**: `progress.go` (add after ToolExecuted)

```go
// ShowProgress displays a progress update for a running task
func (p *ProgressTracker) ShowProgress(task Task, message string) {
    if !p.enabled {
        return
    }

    // Use carriage return to update same line
    fmt.Fprintf(os.Stderr, "\r[%d/%d] ⋯ %s", task.Index, p.totalTasks, message)
}
```

**Note**: This is for Phase 4 (progress events) but good to add now.

---

### Testing Checkpoint 3

Full end-to-end test!

**Remove all debug logging** from previous checkpoints.

**Test 1: Single tool**
```bash
./bin/cliffy "summarize README.md"
```

**Expected output**:
```
[TOOL] read: README.md (245 lines, 12KB) — 0.3s

[Summary of README]
```

**Test 2: Multiple tools**
```bash
./bin/cliffy "find all Go files in internal/ and count them"
```

**Expected output**:
```
[TOOL] glob: internal/**/*.go → 47 matches — 0.2s
[TOOL] bash: echo 47 | wc -l (exit 0) — 0.1s

There are 47 Go files in the internal/ directory.
```

**Test 3: Quiet mode**
```bash
./bin/cliffy --quiet "summarize README.md"
```

**Expected output**:
```
[Summary of README]
```
(No [TOOL] lines!)

**Test 4: Parallel tasks**
```bash
./bin/cliffy "task1" "task2" "task3"
```

**Expected**: Tool traces interleaved correctly, no corruption.

If all tests pass, Phase 3 is complete! 🎉

---

## Phase 4: Progress Events (Optional)

### Overview
Add progress updates for long-running operations.

### When to Emit Progress

**Guidelines**:
- File operations >1MB
- Commands that might take >3 seconds
- Glob patterns scanning >1000 files
- Any operation the LLM initiates that blocks

---

### Step 4.1: Define Progress Function in Tools

**Problem**: tools.go can't import agent (circular dependency)

**Solution**: Use context to pass progress function

**Location**: `agent.go` (add after eventChanKey)

```go
type progressFuncKeyType struct{}
var progressFuncKey = progressFuncKeyType{}

type ProgressFunc func(message string)
```

**Location**: `agent.go:456` (in streamAndHandleEvents)

```go
// After adding event channel to context
ctx = context.WithValue(ctx, tools.MessageIDContextKey, assistantMsg.ID)

// NEW: Add progress function
progressFunc := ProgressFunc(func(message string) {
    if eventChan, ok := ctx.Value(eventChanKey).(chan AgentEvent); ok {
        select {
        case eventChan <- AgentEvent{
            Type:     AgentEventTypeProgress,
            Progress: message,
        }:
        default:
            // Don't block
        }
    }
})
ctx = context.WithValue(ctx, progressFuncKey, progressFunc)
```

---

### Step 4.2: Use Progress in view.go

**Location**: `internal/llm/tools/view.go`

```go
func (v *viewTool) Run(ctx context.Context, call ToolCall) (ToolResponse, error) {
    start := time.Now()

    var params ViewParams
    if err := json.Unmarshal([]byte(call.Input), &params); err != nil {
        return NewTextErrorResponse("invalid parameters"), nil
    }

    // ... existing validation ...

    fileInfo, err := os.Stat(filePath)
    if err != nil {
        return NewTextErrorResponse(fmt.Sprintf("failed to stat file: %v", err)), nil
    }

    // NEW: Emit progress for large files
    if fileInfo.Size() > 1*1024*1024 { // >1MB
        if progressFunc, ok := ctx.Value(progressFuncKey).(ProgressFunc); ok {
            progressFunc(fmt.Sprintf("Reading large file %s...", filepath.Base(filePath)))
        }
    }

    // ... rest of function unchanged ...
}
```

---

### Step 4.3: Use Progress in bash.go

**Location**: `internal/llm/tools/bash.go`

```go
func (b *bashTool) Run(ctx context.Context, call ToolCall) (ToolResponse, error) {
    // ... existing setup ...

    // NEW: Emit progress before long-running command
    if progressFunc, ok := ctx.Value(progressFuncKey).(ProgressFunc); ok {
        progressFunc(fmt.Sprintf("Running: %s", truncateCommand(params.Command, 50)))
    }

    persistentShell := shell.GetPersistentShell(b.workingDir)
    stdout, stderr, err := persistentShell.Exec(ctx, params.Command)

    // ... rest unchanged ...
}

// Helper function
func truncateCommand(cmd string, maxLen int) string {
    if len(cmd) <= maxLen {
        return cmd
    }
    return cmd[:maxLen-3] + "..."
}
```

---

### Step 4.4: Test Progress Events

**Test with large file**:
```bash
# Create large test file
yes "test line" | head -n 100000 > /tmp/large-test.txt

./bin/cliffy "summarize /tmp/large-test.txt"
```

**Expected output**:
```
[1/1] ⋯ Reading large file large-test.txt...
[TOOL] read: /tmp/large-test.txt (100000 lines, 1.5MB) — 0.8s

[Summary]
```

**Test with long command**:
```bash
./bin/cliffy "run sleep 3 command"
```

**Expected output**:
```
[1/1] ⋯ Running: sleep 3
[TOOL] bash: sleep 3 (exit 0) — 3.0s

[Result]
```

---

## Common Issues & Solutions

### Issue 1: Events Not Appearing

**Symptom**: No [TOOL] lines in output

**Debug checklist**:
1. Check buffer size: `events := make(chan AgentEvent, 10)` ✓
2. Check event emission: Add debug log in agent.go ✓
3. Check event processing: Add debug log in scheduler.go ✓
4. Check verbosity: Run without `--quiet` ✓
5. Check progress tracker enabled: `opts.ShowProgress = true` ✓

**Solution**: Follow testing checkpoints to isolate where events stop flowing.

---

### Issue 2: Deadlock / Hanging

**Symptom**: Command hangs indefinitely

**Cause**: Event channel full, blocking send

**Solution**: Ensure non-blocking send with `select`/`default`:
```go
select {
case eventChan <- event:
    // Sent
default:
    // Full, drop event
}
```

---

### Issue 3: Out of Order Events

**Symptom**: Tool traces appear after result

**Cause**: Buffered channel + async processing

**Solution**: Flush events before final output:
```go
// In agent.go before sending final Response event
time.Sleep(10 * time.Millisecond) // Give tool events time to flush
```

Better solution: Increase buffer size to 20+.

---

### Issue 4: Duplicate Tool Traces

**Symptom**: Same tool shown multiple times

**Cause**: Event emitted AND metadata extracted from messages

**Solution**: Choose one approach (streaming is better).

---

### Issue 5: Missing Tool Metadata

**Symptom**: Event emitted but metadata is nil

**Cause**: Tool didn't populate ExecutionMetadata

**Solution**: Check tool's Run() method populates the field:
```go
response.ExecutionMetadata = &ExecutionMetadata{ ... }
```

---

## Performance Considerations

### Memory Usage
- Event channel buffer: 10 events × ~1KB = ~10KB per task
- Tool metadata: ~200 bytes per tool
- **Total overhead**: <100KB per task

### Latency
- Event emission: <1ms
- Event processing: <5ms
- Display formatting: <10ms
- **Total overhead**: <20ms per tool

### Concurrency
- Each task has separate event channel (thread-safe)
- Non-blocking sends prevent backpressure
- Progress tracker uses mutex (safe)

---

## Rollback Plan

If streaming causes issues, revert to post-execution display:

```bash
git revert HEAD  # Revert last commit
```

Then implement Option A (post-execution) instead:
```go
// In scheduler.go after task completes
messages, _ := s.messageStore.List(ctx, sessionID)
for _, msg := range messages {
    if msg.Role == message.Tool {
        // Extract and display metadata
    }
}
```

---

## Success Criteria

### Functionality
- [ ] Tool traces displayed in real-time
- [ ] Traces show file names, durations, counts
- [ ] `--quiet` suppresses traces
- [ ] Multiple tools shown in order
- [ ] Parallel tasks don't corrupt output
- [ ] Progress events work for long operations (Phase 4)

### Performance
- [ ] <50ms overhead per task
- [ ] No deadlocks or hangs
- [ ] No dropped events (or logged if dropped)

### Code Quality
- [ ] No circular dependencies
- [ ] Clean error handling
- [ ] Consistent with existing patterns
- [ ] Well-documented

---

## Next Steps After Completion

Once streaming is working:

1. **Enhanced error reporting** (Phase 4 of OBSERVABILITY-PLAN.md)
   - Track retry history
   - Show detailed error context

2. **Diff preview** (Phase 5)
   - Show file modifications in real-time
   - Display change statistics

3. **Smart output formatting** (Phase 6)
   - Detect output type (list, table, JSON)
   - Format appropriately

4. **Result aggregation** (Phase 8)
   - `--summarize` flag
   - Synthesize multiple task results

---

## Documentation Updates

After implementation, update:

1. **README.md**: Add examples of tool traces
2. **CLAUDE.md**: Document streaming architecture
3. **CLI help**: Explain verbosity flags
4. **Examples**: Show `--quiet` and `--verbose` usage

---

## Questions?

See `streaming-architecture.md` for architectural details and research findings.

See `current-state.md` for status of Sprint 1 infrastructure.

For issues, check "Common Issues & Solutions" section above.
