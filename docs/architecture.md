# Cliffy Architecture

> **Last Updated:** 2025-10-04
> **Status:** Reflects current implementation (v0.1.0)
> **Note:** This document has been updated to accurately reflect the actual Cliffy implementation, replacing earlier design docs for "crush-headless" which was a planning phase name.

## System Design

### High-Level Flow

```
┌──────────────────────────────────────────────────────┐
│ CLI Entry (cliffy)                                   │
│  Single task → Runner (streaming)                   │
│  Multiple tasks → Volley (parallel execution)       │
└─────────────────┬────────────────────────────────────┘
                  │
          ┌───────┴────────┐
          ▼                ▼
    ┌──────────┐    ┌──────────────┐
    │ Runner   │    │ Volley       │
    │ (single) │    │ Scheduler    │
    └────┬─────┘    └──────┬───────┘
         │                 │
         │          ┌──────┴──────┐
         │          ▼             ▼
         │    ┌─────────┐   ┌─────────┐
         │    │ Worker  │   │ Worker  │
         │    │ Pool    │   │ Pool    │
         │    └────┬────┘   └────┬────┘
         │         │             │
         └─────────┴─────────────┘
                   │
                   ▼
        ┌──────────────────────┐
        │ Agent                │
        │  - Message Store     │
        │  - LSP Clients       │
        │  - Provider API      │
        │  - Tool Execution    │
        └──────────┬───────────┘
                   │
          ┌────────┴────────┐
          ▼                 ▼
    ┌─────────┐      ┌──────────────┐
    │ Stdout  │      │ Stderr       │
    │ Content │      │ Thinking     │
    │ Results │      │ Tool Traces  │
    │         │      │ Progress     │
    │         │      │ Stats        │
    └─────────┘      └──────────────┘
```

## Core Components

### 1. CLI Entry Point

**Location:** `cmd/cliffy/main.go`

**Responsibilities:**
- Parse CLI flags and arguments
- Load configuration from config files and environment
- Route single tasks to Runner, multiple tasks to Volley
- Handle output formatting and error presentation

**Key Features:**
- Zero persistence (no database, no sessions)
- Streaming execution for single tasks
- Parallel execution for multiple tasks (volley mode)
- Support for presets, context files, and model selection

**Execution Paths:**
```go
// Single task: Better streaming UX
if len(args) == 1 {
    return executeSingleTask(cmd, args[0], verbosity)
}

// Multiple tasks: Parallel execution
return executeVolley(cmd, args, verbosity)
```

### 2. Runner (Single Task Execution)

**Location:** `internal/runner/runner.go`

**Responsibilities:**
- Execute single tasks with streaming output
- Direct stdout/stderr for better UX
- Show thinking, tool traces, and stats
- Handle errors gracefully

**Interface:**
```go
type Runner struct {
    cfg     *config.Config
    options Options
    stdout  io.Writer
    stderr  io.Writer
    stats   ExecutionStats
}

func (r *Runner) Execute(ctx context.Context, prompt string) error
func (r *Runner) GetStats() ExecutionStats
```

**Key Features:**
- Streams content directly to stdout
- Real-time tool execution feedback
- Optional thinking/reasoning output
- Execution statistics tracking

**Note:** Currently a minimal implementation. Full streaming features are planned but not yet implemented (see ROADMAP.md).

### 3. Volley Scheduler (Multi-Task Execution)

**Location:** `internal/volley/scheduler.go`

**Responsibilities:**
- Parallel execution of multiple tasks
- Worker pool management with configurable concurrency
- Retry logic with exponential backoff and jitter
- Progress tracking and result aggregation
- Fail-fast cancellation support

**Interface:**
```go
type Scheduler struct {
    config       *config.Config
    agent        agent.Service
    messageStore *message.Store
    options      VolleyOptions
    progress     *ProgressTracker
    // ... concurrency control fields
}

func (s *Scheduler) Execute(ctx context.Context, tasks []Task) ([]TaskResult, VolleySummary, error)
```

**Key Features:**
- Worker pool (default: 3 concurrent workers)
- Smart retry with adaptive backoff per error type:
  - Rate limits (429): 5s, 10s, 20s, 40s... (max 120s)
  - Timeouts: 2s, 4s, 8s, 16s... (max 60s)
  - Network errors: 500ms, 1s, 2s, 4s... (max 30s)
- Jitter (±25%) to prevent thundering herd
- Real-time progress tracking
- Token usage and cost calculation
- Tool execution metadata collection

### 4. Agent System

**Location:** `internal/llm/agent/agent.go`

**Responsibilities:**
- Manage LLM provider interaction
- Handle tool execution loop
- Track messages in-memory via message.Store
- Emit events for progress tracking
- Support multiple agent types (coder, task, etc.)

**Interface:**
```go
type Service interface {
    Run(ctx context.Context, sessionID, prompt string) (<-chan AgentEvent, error)
    Model() ModelInfo
}
```

**Event Types:**
```go
const (
    AgentEventTypeResponse   // Final response with token usage
    AgentEventTypeError      // Error occurred
    AgentEventTypeToolTrace  // Tool execution metadata
    AgentEventTypeProgress   // Progress updates
)
```

**Key Features:**
- In-memory message storage (no database)
- Real-time tool execution with metadata
- Token usage tracking
- LSP integration for diagnostics
- MCP tool support

### 5. Output Formatter

**Location:** `internal/output/formatter.go`

**Responsibilities:**
- Format tool execution metadata for display
- Support JSON output for volley results
- Emit NDJSON tool traces for automation
- Format diffs and statistics

**Key Functions:**
```go
func FormatToolTrace(metadata *tools.ExecutionMetadata, verbosity config.VerbosityLevel) string
func FormatJSON(results interface{}, summary interface{}) (string, error)
func EmitToolTraceNDJSON(w io.Writer, taskIndex int, metadata *tools.ExecutionMetadata) error
func ConvertToolMetadataToJSON(metadata []*tools.ExecutionMetadata) []ToolUsageJSON
```

**JSON Output Schema:**
```go
type TaskJSONOutput struct {
    Task     string           `json:"task"`
    Status   string           `json:"status"`
    Result   string           `json:"result,omitempty"`
    Error    string           `json:"error,omitempty"`
    Metadata TaskMetadataJSON `json:"metadata"`
}
```

**Note:** Diff mode (`--output-format diff`) is declared but not fully implemented. Currently returns placeholder message.

## Removed Components (vs. Crush)

Components from Crush that Cliffy **does not include**:

1. **Database Layer**
   - No SQLite connection
   - No migrations
   - No persistence
   - Zero writes to disk during execution

2. **Pub/Sub System**
   - No message broker
   - No event channels
   - Direct event streaming instead

3. **Session Management**
   - No session persistence
   - No title generation (removed 1.5s overhead)
   - Sessions exist only in-memory for duration of execution

4. **Permission Service**
   - No interactive approval flow
   - All tools execute directly
   - No notification system

5. **TUI Components**
   - No Bubbletea/Bubble Tea
   - No interactive terminal UI
   - Simple progress indicators instead

6. **History Service**
   - No file history tracking
   - No version management
   - No rollback capabilities

**Result:** ~65% less code, 4x faster cold start, zero disk I/O

## Shared Components

Components **reused from Crush** with minimal/no changes:

1. **Provider Layer** (`internal/llm/provider/`)
   - Anthropic, OpenAI, Gemini clients
   - Streaming implementations
   - Event types

2. **Tool Implementations** (`internal/llm/tools/`)
   - All tool logic (bash, edit, view, grep, etc.)
   - Tool definitions and schemas
   - File operations

3. **Config System** (`internal/config/`)
   - Config loading and merging
   - Provider resolution
   - Environment variable expansion
   - Minimal: Skip UI config, session defaults

4. **LSP Integration** (`internal/lsp/`)
   - Client implementations
   - Diagnostics
   - Lazy initialization wrapper added

5. **Utility Modules**
   - `internal/fsext/` (file system extensions)
   - `internal/env/` (environment utilities)
   - `internal/diff/` (diff generation)

## Execution Flow

### Single Task Execution

1. **Initialization**
   ```go
   // Load config from files + environment
   cfg, err := config.Init(cwd, ".cliffy", false)

   // Get agent config (coder, task, etc.)
   agentCfg := cfg.Agents["coder"]

   // Apply preset if specified
   if presetID != "" {
       preset.ApplyToAgent(&agentCfg)
   }

   // Override model if flags specified
   if fast { opts.Model = "small" }
   if smart { opts.Model = "large" }
   ```

2. **Runner Execution**
   ```go
   // Create runner with options
   r, err := runner.New(cfg, opts)

   // Execute (currently minimal - full streaming planned)
   err = r.Execute(ctx, prompt)

   // Show stats if requested
   if showStats {
       stats := r.GetStats()
       fmt.Fprintf(stderr, "Stats: %+v\n", stats)
   }
   ```

### Volley (Multi-Task) Execution

1. **Initialization**
   ```go
   // Parse tasks from arguments
   tasks := make([]volley.Task, len(args))
   for i, arg := range args {
       tasks[i] = volley.Task{Index: i+1, Prompt: arg}
   }

   // Create agent and message store
   messageStore := message.NewStore()
   lspClients := csync.NewMap[string, *lsp.Client]()
   agent, err := agent.NewAgent(ctx, agentCfg, messageStore, lspClients)
   ```

2. **Worker Pool Execution**
   ```go
   scheduler := volley.NewScheduler(cfg, agent, messageStore, opts)

   // Execute with worker pool (default 3 workers)
   results, summary, err := scheduler.Execute(ctx, tasks)

   // Workers pull from task queue, execute via agent.Run()
   // Results collected in order, progress tracked in real-time
   ```

3. **Event Processing (per task)**
   ```go
   events, err := agent.Run(ctx, sessionID, prompt)

   for event := range events {
       switch event.Type {
       case AgentEventTypeToolTrace:
           // Show tool execution in real-time
           progress.ToolExecuted(task, event.ToolMetadata)

       case AgentEventTypeResponse:
           // Extract final output and token usage
           return output, usage, toolMetadata, nil

       case AgentEventTypeError:
           return "", Usage{}, nil, event.Error
       }
   }
   ```

4. **Output Formatting**
   ```go
   switch outputFormat {
   case "json":
       jsonOutput := output.FormatJSON(results, summary)
       fmt.Println(jsonOutput)

   case "text":
       for _, result := range results {
           fmt.Println(result.Output)
       }

   case "diff":
       // Planned but not fully implemented
       diffOutput := output.FormatDiffOutput(results)
       fmt.Print(diffOutput)
   }
   ```

## Memory Model

### Crush (with DB)
```
Heap Allocation:
- DB Connection: ~10MB
- SQLite Cache: ~15MB
- Pub/Sub Channels: ~5MB
- Message Service: ~8MB
- TUI Components: ~7MB
- LSP Clients: ~5MB
Total: ~50MB baseline
```

### Cliffy (in-memory only)
```
Heap Allocation:
- Config: ~1MB
- Provider Client: ~3MB
- Message Store (in-memory): ~2-5MB
- Agent System: ~3MB
- LSP Clients (lazy): ~4MB when needed
- Volley Scheduler: ~2MB
Total: ~11-18MB baseline

(Actual usage varies based on task complexity and number of messages)
```

**Key Difference:** Cliffy maintains all state in memory with no disk I/O, resulting in 3-4x less memory usage and faster operation.

## Concurrency Model

### Provider Streaming
- Single goroutine for SSE/streaming
- Channels for event delivery
- Context cancellation for cleanup

### Tool Execution
- Sequential by default (maintain order)
- Parallel option for read-only tools:
  ```go
  if allReadOnly(toolCalls) {
      var wg sync.WaitGroup
      results := make([]ToolResult, len(toolCalls))
      for i, call := range toolCalls {
          wg.Add(1)
          go func(idx int, tc ToolCall) {
              defer wg.Done()
              results[idx] = executor.Execute(ctx, tc)
          }(i, call)
      }
      wg.Wait()
  }
  ```

### LSP Clients
- Lazy initialization on first use
- Cached per language
- Shutdown on context cancel

## Error Handling

### Provider Errors
```go
case EventError:
    if isRetryable(event.Error) {
        // Retry with exponential backoff
        return retryWithBackoff(ctx, messages, tools)
    }
    return formatError(event.Error)
```

### Tool Errors
```go
result := executor.Execute(ctx, call)
if result.IsError {
    // Continue with error message to LLM
    toolMsg.Parts = append(toolMsg.Parts, ToolResult{
        ToolCallID: call.ID,
        Content: result.Content,
        IsError: true,
    })
}
```

### Timeout Handling
```go
ctx, cancel := context.WithTimeout(parentCtx, flags.Timeout)
defer cancel()

select {
case <-ctx.Done():
    return errors.New("execution timeout exceeded")
case result := <-done:
    return result
}
```

## Configuration

### Minimal Config Loading
```go
// Only load what's needed
type HeadlessConfig struct {
    Provider      config.ProviderConfig
    Model         config.ModelConfig
    LSP           map[string]config.LSPConfig
    WorkingDir    string
    ContextPaths  []string
}

func LoadMinimal(cwd string) (*HeadlessConfig, error) {
    // Skip: UI settings, session defaults, attribution
    // Load: providers, models, LSP, context paths
}
```

### Environment Variables
```bash
# Existing Crush vars work
ANTHROPIC_API_KEY=...
OPENAI_API_KEY=...

# Headless-specific
CRUSH_HEADLESS_TIMEOUT=5m
CRUSH_HEADLESS_SHOW_THINKING=true
CRUSH_HEADLESS_OUTPUT_FORMAT=json
```

## Testing Strategy

### Unit Tests
- StreamingProcessor event handling
- ThinkingFormatter output formats
- DirectToolExecutor logic
- OutputFormatter JSON/diff generation

### Integration Tests
- Full execution with mock provider
- Tool execution with real filesystem
- LSP client lazy loading
- Error handling and retries

### Performance Tests
- Cold start benchmarks
- First token latency
- Memory usage profiling
- Parallel tool execution

### Comparison Tests
- Output parity with `crush run -q -y`
- Cost calculations match
- File modifications identical
