# Crush-Headless Architecture

## System Design

### High-Level Flow

```
┌─────────────────────────────────────────────────┐
│ CLI Entry (crush-headless)                      │
│  --show-thinking, --thinking-format, --output   │
└─────────────────┬───────────────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────────────┐
│ HeadlessRunner                                  │
│  - Config loader (minimal)                      │
│  - Provider factory                             │
│  - Tool registry (lazy init)                    │
└─────────────────┬───────────────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────────────┐
│ StreamingAgent                                  │
│  - No message service                           │
│  - No pub/sub                                   │
│  - Direct provider.StreamResponse()             │
│  - In-memory delta tracking                     │
└─────────────────┬───────────────────────────────┘
                  │
          ┌───────┴────────┐
          ▼                ▼
    ┌─────────┐      ┌─────────────┐
    │ Stdout  │      │ Stderr      │
    │ Content │      │ Thinking    │
    │ Tool    │      │ Tool Names  │
    │ Output  │      │ Progress    │
    └─────────┘      └─────────────┘
```

## Core Components

### 1. HeadlessRunner

**Location:** `cmd/headless/runner.go`

**Responsibilities:**
- Parse CLI flags
- Load minimal config (providers, tools, LSP settings)
- Initialize provider with headless-optimized prompt
- Orchestrate streaming execution

**Interface:**
```go
type HeadlessRunner struct {
    config        *config.Config
    provider      provider.Provider
    toolRegistry  *LazyToolRegistry
    showThinking  bool
    thinkingFmt   ThinkingFormat
    outputFormat  OutputFormat
}

func (r *HeadlessRunner) Execute(ctx context.Context, prompt string) error
```

**Key Optimizations:**
- No database initialization
- No session creation
- No pub/sub setup
- Lazy tool loading

### 2. StreamingProcessor

**Location:** `internal/headless/processor.go`

**Responsibilities:**
- Process provider events in real-time
- Route content to stdout, thinking to stderr
- Handle tool execution loop
- Format output based on `--output-format`

**Interface:**
```go
type StreamingProcessor struct {
    stdout          io.Writer
    stderr          io.Writer
    thinkingBuf     strings.Builder
    contentBuf      strings.Builder
    toolExecutor    *DirectToolExecutor
}

func (p *StreamingProcessor) processStream(events <-chan ProviderEvent) error
```

**Event Handling:**
```go
switch event.Type {
case EventThinkingDelta:
    p.handleThinking(event.Thinking)
case EventSignatureDelta:
    p.handleSignature(event.Signature)
case EventContentDelta:
    p.handleContent(event.Content)
case EventToolUseStart:
    p.handleToolStart(event.ToolCall)
case EventToolUseDelta:
    p.handleToolDelta(event.ToolCall)
case EventToolUseStop:
    p.handleToolStop(event.ToolCall)
case EventComplete:
    return p.handleComplete(event.Response)
case EventError:
    return event.Error
}
```

### 3. DirectToolExecutor

**Location:** `internal/headless/tools.go`

**Responsibilities:**
- Execute tools without permission checks
- Lazy-initialize LSP clients on demand
- Capture tool output
- Handle errors gracefully

**Interface:**
```go
type DirectToolExecutor struct {
    tools      map[string]tools.BaseTool
    lspCache   map[string]*lsp.Client
    cwd        string
}

func (e *DirectToolExecutor) Execute(ctx context.Context, call ToolCall) ToolResult
func (e *DirectToolExecutor) getLSPClient(lang string) *lsp.Client
```

**Key Differences from Crush:**
- No permission service layer
- No auto-approval flow
- Direct execution with context
- Parallel execution for read-only tools (future)

### 4. ThinkingFormatter

**Location:** `internal/headless/thinking.go`

**Responsibilities:**
- Format thinking/reasoning output
- Support multiple formats (JSON, text, none)
- Stream to stderr without blocking content

**Interface:**
```go
type ThinkingFormat string

const (
    ThinkingFormatJSON ThinkingFormat = "json"
    ThinkingFormatText ThinkingFormat = "text"
    ThinkingFormatNone ThinkingFormat = "none"
)

func FormatThinking(format ThinkingFormat, signature, thinking string) []byte
```

**JSON Format:**
```json
{
  "type": "extended_thinking",
  "signature": "<signature>",
  "content": "Let me analyze this step by step..."
}
```

**Text Format:**
```
[THINKING: <signature>]
Let me analyze this step by step...
[/THINKING]
```

### 5. OutputFormatter

**Location:** `internal/headless/output.go`

**Responsibilities:**
- Format final output based on `--output-format`
- Collect metadata (tokens, cost, files modified)
- Support text, JSON, diff modes

**Interface:**
```go
type OutputFormat string

const (
    OutputFormatText OutputFormat = "text"
    OutputFormatJSON OutputFormat = "json"
    OutputFormatDiff OutputFormat = "diff"
)

type OutputResult struct {
    Content       string              `json:"content"`
    Thinking      []ThinkingBlock     `json:"thinking,omitempty"`
    ToolCalls     []ToolCallSummary   `json:"tool_calls,omitempty"`
    FilesModified []string            `json:"files_modified,omitempty"`
    TokensUsed    TokenUsage          `json:"tokens_used,omitempty"`
    Cost          float64             `json:"cost,omitempty"`
}

func FormatOutput(format OutputFormat, result OutputResult) ([]byte, error)
```

## Removed Components

Components from Crush that are **not needed** in headless:

1. **Database Layer** (`internal/db/`)
   - No SQLite connection
   - No migrations
   - No persistence

2. **Pub/Sub System** (`internal/pubsub/`)
   - No broker
   - No event channels
   - No subscribers

3. **Message Service** (`internal/message/`)
   - Keep type definitions only
   - Remove CRUD operations
   - Remove DB serialization

4. **Session Service** (`internal/session/`)
   - No session creation
   - No title generation
   - No cost tracking (move to output formatter)

5. **Permission Service** (`internal/permission/`)
   - No request/approval flow
   - No allowed tools list (all tools allowed)
   - No notification system

6. **TUI Components** (`internal/tui/`)
   - No Bubbletea models
   - No spinner
   - No interactive components

7. **History Service** (`internal/history/`)
   - No file history tracking
   - No version management

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

### Detailed Step-by-Step

1. **Initialization (< 200ms)**
   ```go
   // Parse flags
   flags := parseFlags(os.Args)

   // Load minimal config
   cfg := config.LoadMinimal(cwd)

   // Create provider (no DB, no session)
   provider := provider.NewProvider(cfg.Provider,
       provider.WithSystemMessage(headlessPrompt))

   // Setup tool registry (lazy)
   tools := NewLazyToolRegistry(cfg)
   ```

2. **Streaming Execution**
   ```go
   // Build initial message
   messages := []message.Message{
       {Role: User, Parts: []ContentPart{Text: prompt}}
   }

   // Start streaming
   events := provider.StreamResponse(ctx, messages, tools.GetTools())

   // Process events
   processor.processStream(events)
   ```

3. **Event Processing Loop**
   ```go
   for event := range events {
       switch event.Type {
       case EventThinkingDelta:
           if showThinking {
               stderr.Write(formatThinking(event.Thinking))
           }

       case EventContentDelta:
           stdout.Write([]byte(event.Content))

       case EventToolUseStart:
           stderr.Write(formatToolStart(event.ToolCall))

       case EventToolUseStop:
           result := executor.Execute(ctx, event.ToolCall)
           messages = append(messages, toolResultMessage(result))
           // Loop continues with tool results

       case EventComplete:
           return outputFormatter.Format(result)
       }
   }
   ```

4. **Tool Execution (when needed)**
   ```go
   // Collect all tool calls from current turn
   toolCalls := collectToolCalls(event)

   // Execute sequentially (or parallel for read-only)
   results := make([]ToolResult, len(toolCalls))
   for i, call := range toolCalls {
       results[i] = executor.Execute(ctx, call)
   }

   // Create tool result message
   toolMsg := message.Message{
       Role: Tool,
       Parts: resultsToContentParts(results),
   }

   // Continue streaming with tool results
   messages = append(messages, assistantMsg, toolMsg)
   events = provider.StreamResponse(ctx, messages, tools.GetTools())
   ```

## Memory Model

### Current Crush (with DB)
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

### Headless (in-memory only)
```
Heap Allocation:
- Config: ~1MB
- Provider Client: ~3MB
- Tool Registry: ~2MB
- Streaming Buffers: ~5MB
- LSP (lazy): ~4MB when needed
Total: ~11-15MB baseline
```

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
