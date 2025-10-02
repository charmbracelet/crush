# Cliffy Observability Implementation Plan

**Goal**: Transform cliffy from "fast but opaque" to "fast and trustworthy" through enhanced observability without sacrificing performance.

**Core Principle**: Show detailed information by default, hide behind `--quiet` for clean output.

---

## Phase 1: Enhanced Tool Logging (Priority 1)

### Current State
```
Result
[TOOL] view
[TOOL] glob
```

### Target State (Default)
```
Result

[TOOL] read: src/main.rs (800 lines, 45KB) — 1.2s
[TOOL] glob: **/*.rs → 79 matches — 0.3s
[TOOL] write: docs/output.md (created, 234 lines) — 0.1s
```

### Target State (with `--quiet`)
```
Result
```

### Implementation Steps

**1.1 Extend ToolResponse Metadata** (`internal/llm/tools/tools.go`)
```go
type ToolResponse struct {
    Content  string
    Metadata ToolMetadata  // NEW
}

type ToolMetadata struct {
    FilePath     string
    Operation    string  // "read", "write", "created", "modified"
    LineCount    int
    ByteSize     int64
    MatchCount   int     // for grep/glob
    ExitCode     *int    // for bash
    Duration     time.Duration
    ErrorMessage string
}
```

**1.2 Update Each Tool** (`internal/llm/tools/*.go`)
- `view.go`: Return file path, line count, byte size, duration
- `glob.go`: Return pattern, match count, duration
- `write.go` / `edit.go`: Return file path, operation type, line count, duration
- `bash.go`: Return command, exit code, duration
- `grep.go` / `rg.go`: Return pattern, match count, duration

**1.3 Format Tool Output** (`cmd/cliffy/main.go` or new `internal/output/tools.go`)
```go
func FormatToolUsage(metadata ToolMetadata, quiet bool) string {
    if quiet {
        return ""
    }

    // Format based on tool type
    switch {
    case metadata.Operation == "read":
        return fmt.Sprintf("[TOOL] read: %s (%d lines, %s) — %.1fs",
            metadata.FilePath, metadata.LineCount,
            formatBytes(metadata.ByteSize), metadata.Duration.Seconds())
    case metadata.Operation == "glob":
        return fmt.Sprintf("[TOOL] glob: %s → %d matches — %.1fs",
            metadata.Pattern, metadata.MatchCount, metadata.Duration.Seconds())
    // ... etc
    }
}
```

**Testing**
- Run `cliffy "list all Go files"` → should show detailed tool trace
- Run `cliffy --quiet "list all Go files"` → should show only result
- Verify timing is accurate (use time.Now() around tool execution)

---

## Phase 2: Three-Tier Verbosity System

### Verbosity Levels

| Level | Flag | Behavior | Use Case |
|-------|------|----------|----------|
| **Normal** (new default) | none | Show tool traces with timing | Development, debugging |
| **Quiet** | `--quiet` / `-q` | Results only, no tool info | Scripting, clean output |
| **Verbose** | `--verbose` / `-v` | Normal + thinking + events | Deep debugging |

### Implementation Steps

**2.1 Add Verbosity Config** (`internal/config/config.go`)
```go
type Config struct {
    // ... existing fields
    Verbosity VerbosityLevel
}

type VerbosityLevel int

const (
    VerbosityNormal  VerbosityLevel = iota  // Default: show tools
    VerbosityQuiet                          // --quiet: results only
    VerbosityVerbose                        // --verbose: everything
)
```

**2.2 Update CLI Flags** (`cmd/cliffy/root.go`)
```go
// Make --quiet and --verbose mutually exclusive
rootCmd.Flags().BoolP("quiet", "q", false, "Suppress tool traces and progress (results only)")
rootCmd.Flags().BoolP("verbose", "v", false, "Show tool traces, thinking, and detailed events")

// In PreRun, validate:
if quiet && verbose {
    return errors.New("--quiet and --verbose are mutually exclusive")
}
```

**2.3 Thread Verbosity Through Stack**
- `executeVolley()`: Pass verbosity to scheduler
- `volley.Scheduler`: Pass to progress tracker and output formatter
- `agent.Run()`: Conditional event emission based on verbosity

**2.4 Conditional Output**
```go
// In output formatting
if verbosity == VerbosityQuiet {
    fmt.Println(result.Content)  // Result only
    return
}

// Normal: show tools
fmt.Println(result.Content)
for _, tool := range result.ToolsUsed {
    fmt.Println(FormatToolUsage(tool.Metadata, false))
}

// Verbose: show everything
if verbosity == VerbosityVerbose {
    fmt.Println("\n--- Thinking ---")
    fmt.Println(result.ThinkingContent)
    fmt.Println("\n--- Events ---")
    for _, event := range result.Events {
        fmt.Printf("[%s] %s\n", event.Type, event.Data)
    }
}
```

---

## Phase 3: Streaming Progress for Long Tasks (Priority 2)

### Current State
```
[waiting silently for 15 seconds...]
Result appears
```

### Target State
```
[1/3] ▶ Analyze large file...
[1/3] ⋯ Reading 50k lines...
[1/3] ⋯ Parsing structure...
[1/3] ✓ Analyze large file (15.2s)
```

### Implementation Steps

**3.1 Add Progress Event Type** (`internal/llm/agent/agent.go`)
```go
type AgentEventType string

const (
    AgentEventTypeResponse  AgentEventType = "response"
    AgentEventTypeError     AgentEventType = "error"
    AgentEventTypeProgress  AgentEventType = "progress"  // NEW
    AgentEventTypeThinking  AgentEventType = "thinking"
    AgentEventTypeToolCall  AgentEventType = "tool_call"
)

type ProgressData struct {
    Stage    string  // "Reading files", "Parsing", "Generating"
    Current  int     // Optional: current item
    Total    int     // Optional: total items
    Message  string  // Human-readable update
}
```

**3.2 Emit Progress from Tools** (`internal/llm/tools/*.go`)
- Tools that take >2s should emit progress events
- Example in `view.go`:
```go
func (t *ViewTool) Run(ctx context.Context, params ToolCall) (ToolResponse, error) {
    start := time.Now()

    // Emit progress event if file is large
    if fileSize > 1MB {
        emitProgress(ctx, "Reading large file...")
    }

    data, err := os.ReadFile(path)

    if len(data) > 10000 {
        emitProgress(ctx, "Processing %d lines...", lineCount)
    }

    // ... process ...
}

func emitProgress(ctx context.Context, format string, args ...interface{}) {
    // Extract event channel from context and send progress event
    if ch, ok := ctx.Value(EventChannelKey).(chan AgentEvent); ok {
        ch <- AgentEvent{
            Type: AgentEventTypeProgress,
            Data: ProgressData{Message: fmt.Sprintf(format, args...)},
        }
    }
}
```

**3.3 Update Volley Progress Tracker** (`internal/volley/progress.go`)
- Listen for `AgentEventTypeProgress` events
- Update display with progress indicators
- Use state machine: `▶` (start) → `⋯` (progress) → `✓` (done) / `✗` (error) / `↻` (retry)

**3.4 Display Logic** (`cmd/cliffy/main.go`)
```go
// In volley result processing
for event := range agent.Events {
    switch event.Type {
    case agent.AgentEventTypeProgress:
        if verbosity != VerbosityQuiet {
            fmt.Printf("\r[%d/%d] ⋯ %s", taskNum, total, event.Data.Message)
        }
    case agent.AgentEventTypeResponse:
        if verbosity != VerbosityQuiet {
            fmt.Printf("\r[%d/%d] ✓ %s (%.1fs)\n", taskNum, total, taskName, duration)
        }
    }
}
```

**3.5 Threshold-Based Progress**
- Only show progress for tasks >5 seconds
- Update progress max every 2-3 seconds (avoid spam)
- Clear progress line when task completes

---

## Phase 4: Better Error Messages (Priority 1)

### Current State
```
Completed: 19/20 tasks
Retries: 1 total
```

### Target State
```
Volley Summary
═══════════════════════════════════════════════════════════════

Completed:  19/20 tasks
Failed:     1/20 tasks
Duration:   42.1s
Tokens:     125.4k (avg 6.3k/task)
Cost:       $0.00 (free tier)

Failed tasks:
  [5/20] Read nonexistent-file.rs
    ✗ Error: File not found: nonexistent-file.rs
    ↻ Retried: 3 times
    Last error: FileNotFoundError after 3 attempts
```

### Implementation Steps

**4.1 Track Detailed Error Info** (`internal/volley/task.go`)
```go
type TaskResult struct {
    // ... existing fields
    Error        error
    Retries      int
    RetryHistory []RetryAttempt  // NEW
}

type RetryAttempt struct {
    AttemptNum int
    Error      error
    Timestamp  time.Time
    WasRetried bool
}
```

**4.2 Capture Retry Details** (`internal/volley/scheduler.go`)
```go
// In retry loop
for attempt := 1; attempt <= maxRetries; attempt++ {
    result, err := executeTask(ctx, task)

    if err != nil {
        retryAttempt := RetryAttempt{
            AttemptNum: attempt,
            Error:      err,
            Timestamp:  time.Now(),
            WasRetried: attempt < maxRetries && isRetryable(err),
        }
        task.Result.RetryHistory = append(task.Result.RetryHistory, retryAttempt)

        if !isRetryable(err) || attempt == maxRetries {
            task.Result.Error = err
            return result, err
        }

        time.Sleep(backoff(attempt))
        continue
    }

    return result, nil
}
```

**4.3 Enhanced Summary Output** (`cmd/cliffy/main.go`)
```go
func PrintVolleySummary(results []TaskResult, verbosity VerbosityLevel) {
    completed := countCompleted(results)
    failed := countFailed(results)
    totalRetries := sumRetries(results)

    fmt.Println("\nVolley Summary")
    fmt.Println("═══════════════════════════════════════════════════════════════")
    fmt.Printf("\nCompleted:  %d/%d tasks\n", completed, len(results))

    if failed > 0 {
        fmt.Printf("Failed:     %d/%d tasks\n", failed, len(results))
    }

    fmt.Printf("Duration:   %.1fs\n", totalDuration.Seconds())
    fmt.Printf("Tokens:     %s (avg %s/task)\n", formatTokens(totalTokens), formatTokens(avgTokens))
    fmt.Printf("Cost:       %s\n", formatCost(totalCost))

    // Show failed tasks (even in quiet mode)
    if failed > 0 {
        fmt.Println("\nFailed tasks:")
        for i, result := range results {
            if result.Error != nil {
                fmt.Printf("  [%d/%d] %s\n", i+1, len(results), result.Task.Prompt)
                fmt.Printf("    ✗ Error: %s\n", result.Error)

                if result.Retries > 0 {
                    fmt.Printf("    ↻ Retried: %d times\n", result.Retries)

                    if verbosity != VerbosityQuiet {
                        // Show retry history
                        for _, retry := range result.RetryHistory {
                            status := "✗"
                            if retry.WasRetried {
                                status = "↻"
                            }
                            fmt.Printf("      %s Attempt %d: %s\n", status, retry.AttemptNum, retry.Error)
                        }
                    }
                }
            }
        }
    }
}
```

---

## Phase 5: Diff Preview for File Modifications

### Current State
```
Task complete
[TOOL] write
```

### Target State
```
Task complete: Modified src/components.rs

[DIFF]
+ /// Mana component for magic casting
+ #[derive(Component, Reflect)]
+ pub struct Mana {
+     pub current: i32,
+     pub max: i32,
+ }

Modified: 1 file, +6 lines, 0 deletions
Run `git diff` to review changes
```

### Implementation Steps

**5.1 Capture File State Before Write** (`internal/llm/tools/write.go`, `edit.go`)
```go
func (t *WriteTool) Run(ctx context.Context, params ToolCall) (ToolResponse, error) {
    // Read existing file (if exists)
    var beforeContent []byte
    var existed bool
    if _, err := os.Stat(path); err == nil {
        beforeContent, _ = os.ReadFile(path)
        existed = true
    }

    // Write new content
    err := os.WriteFile(path, newContent, 0644)

    // Generate diff
    var diff string
    if existed {
        diff = generateUnifiedDiff(path, beforeContent, newContent)
    } else {
        diff = fmt.Sprintf("+ Created %s (%d lines)", path, len(lines))
    }

    return ToolResponse{
        Content: fmt.Sprintf("Modified %s", path),
        Metadata: ToolMetadata{
            FilePath:  path,
            Operation: operation,  // "created" or "modified"
            Diff:      diff,       // NEW field
        },
    }, nil
}
```

**5.2 Unified Diff Generation** (`internal/llm/tools/diff.go`)
```go
func generateUnifiedDiff(path string, before, after []byte) string {
    // Use go-diff library or simple line-by-line comparison
    // Format: unified diff format (+ for additions, - for deletions)

    beforeLines := strings.Split(string(before), "\n")
    afterLines := strings.Split(string(after), "\n")

    var diff strings.Builder
    diff.WriteString(fmt.Sprintf("--- %s (before)\n", path))
    diff.WriteString(fmt.Sprintf("+++ %s (after)\n", path))

    // Simple diff logic (or use github.com/sergi/go-diff)
    additions := len(afterLines) - len(beforeLines)

    // Show first 20 lines of changes
    for i := 0; i < min(20, len(afterLines)); i++ {
        if i >= len(beforeLines) || beforeLines[i] != afterLines[i] {
            diff.WriteString(fmt.Sprintf("+ %s\n", afterLines[i]))
        }
    }

    return diff.String()
}
```

**5.3 Display Diff in Output** (`cmd/cliffy/main.go`)
```go
// After task completes
if result.ToolsUsed.HasWrites() && verbosity != VerbosityQuiet {
    fmt.Printf("\n[DIFF]\n%s\n", result.ToolsUsed.GetDiff())

    stats := result.ToolsUsed.GetChangeStats()
    fmt.Printf("Modified: %d file(s), +%d lines, -%d deletions\n",
        stats.Files, stats.Additions, stats.Deletions)

    if isGitRepo() {
        fmt.Println("Run `git diff` to review changes")
    }
}
```

**5.4 Aggregate Diffs for Volley**
- Track all file modifications across tasks
- Show summary at end: "Modified 5 files across 3 tasks"
- Optionally show consolidated diff

---

## Phase 6: Smart Output Formatting

### Goal
Structure complex outputs for easier scanning and programmatic parsing.

### Implementation Steps

**6.1 Detect Output Type** (`internal/output/formatter.go`)
```go
type OutputType int

const (
    OutputTypeRaw OutputType = iota
    OutputTypeList
    OutputTypeTable
    OutputTypeJSON
    OutputTypeCode
)

func DetectOutputType(content string) OutputType {
    // Heuristics:
    // - Starts with [, { → JSON
    // - Multiple "- " lines → List
    // - Multiple "|" with --- separator → Table
    // - Starts with ``` → Code
    // - Otherwise → Raw
}
```

**6.2 Format Based on Type**
```go
func FormatOutput(content string, outputType OutputType) string {
    switch outputType {
    case OutputTypeList:
        return formatList(content)  // Add emoji bullets, indentation
    case OutputTypeTable:
        return formatTable(content) // Align columns, add borders
    case OutputTypeJSON:
        return formatJSON(content)  // Pretty-print with indentation
    default:
        return content
    }
}

func formatList(content string) string {
    lines := strings.Split(content, "\n")
    var formatted strings.Builder

    // Group by categories (detect headers like "Components:", "Resources:")
    for _, line := range lines {
        if strings.HasSuffix(line, ":") {
            // Header
            formatted.WriteString(fmt.Sprintf("\n%s\n", aurora.Bold(line)))
        } else if strings.HasPrefix(line, "- ") {
            // List item
            formatted.WriteString(fmt.Sprintf("  • %s\n", strings.TrimPrefix(line, "- ")))
        } else {
            formatted.WriteString(line + "\n")
        }
    }

    return formatted.String()
}
```

**6.3 Apply in Output Pipeline**
```go
// In result processing
formattedContent := content
if verbosity == VerbosityNormal || verbosity == VerbosityVerbose {
    outputType := DetectOutputType(content)
    formattedContent = FormatOutput(content, outputType)
}
fmt.Println(formattedContent)
```

---

## Phase 7: JSON Output Improvements

### Current Issues
- Inconsistent structure: sometimes raw value, sometimes `{"result": "..."}`
- No metadata in JSON output
- Hard to parse programmatically

### Target State
```json
{
  "task": "What is 5 + 3?",
  "result": "8",
  "metadata": {
    "duration_ms": 1234,
    "tokens": {
      "prompt": 150,
      "completion": 10,
      "total": 160
    },
    "cost": 0.0,
    "model": "grok-4-fast:free",
    "tools_used": [
      {
        "name": "view",
        "file": "src/main.rs",
        "duration_ms": 450
      }
    ],
    "retries": 0
  }
}
```

### Implementation Steps

**7.1 Define JSON Output Schema** (`internal/output/json.go`)
```go
type JSONOutput struct {
    Task     string        `json:"task"`
    Result   string        `json:"result"`
    Metadata TaskMetadata  `json:"metadata"`
}

type TaskMetadata struct {
    DurationMS int64            `json:"duration_ms"`
    Tokens     TokenMetadata    `json:"tokens"`
    Cost       float64          `json:"cost"`
    Model      string           `json:"model"`
    ToolsUsed  []ToolUsageJSON  `json:"tools_used"`
    Retries    int              `json:"retries"`
    Error      string           `json:"error,omitempty"`
}

type TokenMetadata struct {
    Prompt     int `json:"prompt"`
    Completion int `json:"completion"`
    Total      int `json:"total"`
}

type ToolUsageJSON struct {
    Name       string `json:"name"`
    File       string `json:"file,omitempty"`
    DurationMS int64  `json:"duration_ms"`
    ExitCode   *int   `json:"exit_code,omitempty"`
}
```

**7.2 Convert TaskResult to JSON** (`cmd/cliffy/main.go`)
```go
func outputJSON(result TaskResult) error {
    output := JSONOutput{
        Task:   result.Task.Prompt,
        Result: result.Content,
        Metadata: TaskMetadata{
            DurationMS: result.Duration.Milliseconds(),
            Tokens: TokenMetadata{
                Prompt:     result.TokenUsage.PromptTokens,
                Completion: result.TokenUsage.CompletionTokens,
                Total:      result.TokenUsage.TotalTokens,
            },
            Cost:      calculateCost(result.TokenUsage, result.Model),
            Model:     result.Model,
            ToolsUsed: convertToolsToJSON(result.ToolsUsed),
            Retries:   result.Retries,
        },
    }

    if result.Error != nil {
        output.Metadata.Error = result.Error.Error()
    }

    jsonBytes, err := json.MarshalIndent(output, "", "  ")
    if err != nil {
        return err
    }

    fmt.Println(string(jsonBytes))
    return nil
}
```

**7.3 Volley JSON Output**
```go
// For volley mode, output array of results
type VolleyJSONOutput struct {
    Tasks    []JSONOutput    `json:"tasks"`
    Summary  VolleySummary   `json:"summary"`
}

type VolleySummary struct {
    TotalTasks     int     `json:"total_tasks"`
    CompletedTasks int     `json:"completed_tasks"`
    FailedTasks    int     `json:"failed_tasks"`
    TotalDuration  int64   `json:"total_duration_ms"`
    TotalTokens    int     `json:"total_tokens"`
    TotalCost      float64 `json:"total_cost"`
}
```

---

## Phase 8: Result Aggregation (Optional `--summarize`)

### Goal
Automatically synthesize results from multiple tasks for easier analysis.

### Implementation Steps

**8.1 Add `--summarize` Flag** (`cmd/cliffy/root.go`)
```go
volleyCmd.Flags().Bool("summarize", false, "Generate aggregated summary of all task results")
```

**8.2 Collect Results for Summarization** (`cmd/cliffy/volley.go`)
```go
if summarize {
    // After all tasks complete
    summaryPrompt := buildSummaryPrompt(results)
    summaryTask := volley.Task{
        Prompt: summaryPrompt,
        Context: "You are summarizing multiple task results. Be concise.",
    }

    // Run summary task
    summaryResult := runSingleTask(summaryTask)

    // Display
    fmt.Println("\n═══════════════════════════════════════════════════════════════")
    fmt.Println("Aggregated Summary")
    fmt.Println("═══════════════════════════════════════════════════════════════\n")
    fmt.Println(summaryResult.Content)
}
```

**8.3 Build Summary Prompt**
```go
func buildSummaryPrompt(results []TaskResult) string {
    var prompt strings.Builder
    prompt.WriteString("Summarize the following task results:\n\n")

    for i, result := range results {
        prompt.WriteString(fmt.Sprintf("Task %d: %s\n", i+1, result.Task.Prompt))
        prompt.WriteString(fmt.Sprintf("Result: %s\n\n", truncate(result.Content, 200)))
    }

    prompt.WriteString("Provide:\n")
    prompt.WriteString("1. Overall findings across all tasks\n")
    prompt.WriteString("2. Key patterns or trends\n")
    prompt.WriteString("3. Notable outliers or issues\n")
    prompt.WriteString("4. Aggregated statistics (if applicable)\n")

    return prompt.String()
}
```

---

## Phase 9: Context Validation

### Goal
Show and validate shared context before execution.

### Implementation Steps

**9.1 Add `--show-context` Flag** (`cmd/cliffy/root.go`)
```go
rootCmd.Flags().Bool("show-context", false, "Display context being used before execution")
```

**9.2 Display Context Preview**
```go
if showContext || interactive {
    contextPreview := buildContextPreview(config.Context)
    fmt.Println("Context preview:")
    fmt.Println(contextPreview)
    fmt.Printf("\nContext size: %d tokens\n", estimateTokens(config.Context))

    if interactive {
        fmt.Print("\nProceed? [Y/n]: ")
        var response string
        fmt.Scanln(&response)
        if strings.ToLower(response) == "n" {
            return fmt.Errorf("user cancelled")
        }
    }
}
```

**9.3 Context Preview Format**
```go
func buildContextPreview(context string) string {
    lines := strings.Split(context, "\n")

    if len(lines) <= 10 {
        return context
    }

    // Show first 5 and last 5 lines
    preview := strings.Join(lines[:5], "\n")
    preview += fmt.Sprintf("\n... (%d more lines) ...\n", len(lines)-10)
    preview += strings.Join(lines[len(lines)-5:], "\n")

    return preview
}
```

---

## Implementation Order

### Sprint 1: Core Observability (Highest Impact)
1. **Enhanced Tool Logging** (Phase 1) - 4 hours
2. **Three-Tier Verbosity** (Phase 2) - 2 hours
3. **Better Error Messages** (Phase 4) - 3 hours

**Deliverable**: `cliffy` with detailed tool traces by default, `--quiet` for clean output, much better error reporting.

### Sprint 2: Progress & Feedback
4. **Streaming Progress** (Phase 3) - 6 hours
5. **Diff Preview** (Phase 5) - 4 hours

**Deliverable**: Live progress updates on long tasks, visual diffs for file modifications.

### Sprint 3: Output Quality
6. **Smart Output Formatting** (Phase 6) - 4 hours
7. **JSON Output Improvements** (Phase 7) - 3 hours

**Deliverable**: Better formatted output, consistent JSON structure for programmatic use.

### Sprint 4: Advanced Features (Optional)
8. **Result Aggregation** (Phase 8) - 4 hours
9. **Context Validation** (Phase 9) - 2 hours

**Deliverable**: `--summarize` flag for synthesized results, context preview.

---

## Testing Strategy

### Unit Tests
- Tool metadata extraction
- Diff generation
- JSON output schema
- Progress event emission

### Integration Tests
- Volley execution with verbosity levels
- Error handling and retry reporting
- File modification tracking
- Context validation

### Manual Testing Scenarios
1. **Tool Logging**: Run complex task, verify tool traces are informative
2. **Progress**: Run task that reads large file, verify progress updates
3. **Errors**: Trigger retryable error, verify retry history in summary
4. **Diffs**: Modify file, verify diff output is accurate
5. **JSON**: Run with `--output-format json`, verify schema compliance
6. **Quiet Mode**: Run with `--quiet`, verify clean output

### Performance Regression
- Benchmark tool metadata collection overhead
- Ensure progress events don't slow down execution
- Verify diff generation doesn't block writes

---

## Success Metrics

**Observability Goals**
- [ ] Users can understand what cliffy is doing at any moment
- [ ] Errors are immediately actionable (no log diving)
- [ ] Long tasks provide feedback (no "is it stuck?" anxiety)
- [ ] File modifications are transparent (diffs shown)

**UX Goals**
- [ ] Default output is informative without being verbose
- [ ] `--quiet` provides clean output for scripting
- [ ] `--verbose` shows everything for debugging
- [ ] JSON output is consistent and programmatically parsable

**Performance Goals**
- [ ] Tool metadata adds <50ms overhead per task
- [ ] Progress events add <100ms overhead for long tasks
- [ ] Diff generation doesn't block write operations

---

## Migration Notes

### Breaking Changes
- **None expected** - new features are additive

### Deprecations
- Current minimal tool output (`[TOOL] view`) will become verbose by default
- Users wanting old behavior should use `--quiet`

### Documentation Updates
- README: Update examples to show new output
- CLI help: Document verbosity flags
- Examples: Add `--quiet` and `--verbose` usage

---

## Future Enhancements (Out of Scope)

These are good ideas from the feedback but not in current plan:

- **Task Dependencies**: Requires DAG scheduler, significant rework
- **Task Grouping**: Needs new CLI syntax and volley orchestration
- **Task Templates**: Config file management system
- **Workflow Integration**: Git integration, output directories
- **Dry-Run Mode**: Requires tool execution simulation
- **Undo Command**: Needs state tracking and rollback logic
- **Interactive Mode**: Clarification prompts for vague tasks
- **Conditional Execution**: Advanced error handling strategies

These could be future phases once core observability is solid.

---

## Questions / Decisions Needed

1. **Diff Library**: Use existing library (github.com/sergi/go-diff) or write simple differ?
2. **Progress Threshold**: 5 seconds before showing progress, or configurable?
3. **Token Estimation**: Use tiktoken-go or simple heuristic for context preview?
4. **Summary Model**: Use same model or always use "smart" model for `--summarize`?
5. **Color Output**: Use color in normal mode, or only with explicit flag?

---

**Timeline Estimate**: 32 hours total (4 days of focused work)

**Priority**: Start with Sprint 1 (9 hours) for maximum impact.
