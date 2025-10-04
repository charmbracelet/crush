# Implementation Guide - Historical Design Document

> **⚠️ HISTORICAL DOCUMENT**
> **Status:** This was the original design/planning document for "crush-headless"
> **Current Reality:** Cliffy has been implemented with a different approach
> **For Current Architecture:** See [architecture.md](./architecture.md)
> **For Missing Features:** See [ROADMAP.md](./ROADMAP.md)
>
> This document is preserved for historical context and shows the original implementation plan. The actual Cliffy implementation evolved differently, using the volley scheduler pattern instead of the headless runner approach outlined here.

## Original Design: Phase 1 - Fork & Simplify

### Goal
Working prototype with direct streaming, no persistence, 50% code reduction.

### Tasks

#### 1.1 Project Setup
```bash
# Create new directory structure
mkdir -p crush-headless/{cmd/headless,internal/{runner,stream,executor,prompt}}

# Initialize Go module
cd crush-headless
go mod init github.com/charmbracelet/crush-headless
```

#### 1.2 Copy Core Dependencies
```bash
# Copy from crush with minimal changes
cp -r crush/internal/llm/provider internal/llm/provider
cp -r crush/internal/llm/tools internal/llm/tools
cp -r crush/internal/config internal/config
cp -r crush/internal/lsp internal/lsp
cp -r crush/internal/fsext internal/fsext
cp -r crush/internal/env internal/env
cp -r crush/internal/diff internal/diff

# Copy message type definitions only (no service)
cp crush/internal/message/types.go internal/message/types.go
```

#### 1.3 Create HeadlessRunner
**File:** `cmd/headless/runner.go`

```go
package main

import (
    "context"
    "fmt"
    "os"

    "github.com/charmbracelet/crush-headless/internal/config"
    "github.com/charmbracelet/crush-headless/internal/runner"
    "github.com/spf13/cobra"
)

var (
    showThinking   bool
    thinkingFormat string
    outputFormat   string
    timeout        string
    quiet          bool
)

var rootCmd = &cobra.Command{
    Use:   "crush-headless [prompt]",
    Short: "Non-interactive AI coding assistant",
    Args:  cobra.MinimumNArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        ctx := cmd.Context()

        // Load minimal config
        cfg, err := config.LoadMinimal()
        if err != nil {
            return fmt.Errorf("config load failed: %w", err)
        }

        // Create runner
        r, err := runner.New(cfg, runner.Options{
            ShowThinking:   showThinking,
            ThinkingFormat: thinkingFormat,
            OutputFormat:   outputFormat,
            Quiet:          quiet,
        })
        if err != nil {
            return err
        }

        // Execute
        prompt := strings.Join(args, " ")
        return r.Execute(ctx, prompt)
    },
}

func init() {
    rootCmd.Flags().BoolVar(&showThinking, "show-thinking", false, "Show LLM thinking/reasoning")
    rootCmd.Flags().StringVar(&thinkingFormat, "thinking-format", "text", "Format: json|text")
    rootCmd.Flags().StringVar(&outputFormat, "output-format", "text", "Format: text|json|diff")
    rootCmd.Flags().StringVar(&timeout, "timeout", "10m", "Max execution time")
    rootCmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "Minimal output")
}

func main() {
    if err := rootCmd.Execute(); err != nil {
        os.Exit(1)
    }
}
```

#### 1.4 Create StreamingProcessor
**File:** `internal/stream/processor.go`

```go
package stream

import (
    "context"
    "fmt"
    "io"
    "strings"

    "github.com/charmbracelet/crush-headless/internal/executor"
    "github.com/charmbracelet/crush-headless/internal/llm/provider"
    "github.com/charmbracelet/crush-headless/internal/message"
)

type Processor struct {
    stdout   io.Writer
    stderr   io.Writer
    executor *executor.DirectExecutor
    options  Options

    // In-memory tracking
    thinkingBuf  strings.Builder
    contentBuf   strings.Builder
    currentTools []message.ToolCall
}

type Options struct {
    ShowThinking   bool
    ThinkingFormat ThinkingFormat
    Quiet          bool
}

func New(stdout, stderr io.Writer, exec *executor.DirectExecutor, opts Options) *Processor {
    return &Processor{
        stdout:   stdout,
        stderr:   stderr,
        executor: exec,
        options:  opts,
    }
}

func (p *Processor) ProcessStream(ctx context.Context, events <-chan provider.ProviderEvent) error {
    for {
        select {
        case <-ctx.Done():
            return ctx.Err()

        case event, ok := <-events:
            if !ok {
                return nil
            }

            if err := p.handleEvent(ctx, event); err != nil {
                return err
            }
        }
    }
}

func (p *Processor) handleEvent(ctx context.Context, event provider.ProviderEvent) error {
    switch event.Type {
    case provider.EventThinkingDelta:
        return p.handleThinking(event.Thinking)

    case provider.EventSignatureDelta:
        return p.handleSignature(event.Signature)

    case provider.EventContentDelta:
        return p.handleContent(event.Content)

    case provider.EventToolUseStart:
        return p.handleToolStart(event.ToolCall)

    case provider.EventToolUseDelta:
        return p.handleToolDelta(event.ToolCall)

    case provider.EventToolUseStop:
        return p.handleToolStop(event.ToolCall)

    case provider.EventComplete:
        return p.handleComplete(event.Response)

    case provider.EventError:
        return event.Error
    }

    return nil
}

func (p *Processor) handleThinking(thinking string) error {
    if !p.options.ShowThinking {
        return nil
    }

    p.thinkingBuf.WriteString(thinking)

    formatted := FormatThinking(p.options.ThinkingFormat, "", thinking)
    _, err := p.stderr.Write(formatted)
    return err
}

func (p *Processor) handleContent(content string) error {
    p.contentBuf.WriteString(content)
    _, err := p.stdout.Write([]byte(content))
    return err
}

func (p *Processor) handleToolStart(toolCall *message.ToolCall) error {
    if !p.options.Quiet {
        fmt.Fprintf(p.stderr, "[TOOL] %s\n", toolCall.Name)
    }

    p.currentTools = append(p.currentTools, *toolCall)
    return nil
}

func (p *Processor) handleToolDelta(toolCall *message.ToolCall) error {
    // Update tool call input in currentTools
    for i := range p.currentTools {
        if p.currentTools[i].ID == toolCall.ID {
            p.currentTools[i].Input += toolCall.Input
            break
        }
    }
    return nil
}

func (p *Processor) handleToolStop(toolCall *message.ToolCall) error {
    // Mark tool as finished
    for i := range p.currentTools {
        if p.currentTools[i].ID == toolCall.ID {
            p.currentTools[i].Finished = true
            break
        }
    }
    return nil
}

func (p *Processor) handleComplete(response *provider.ProviderResponse) error {
    // If there are tool calls, execute them
    if len(p.currentTools) > 0 {
        return p.executePendingTools(ctx)
    }

    // Otherwise we're done
    return nil
}

func (p *Processor) executePendingTools(ctx context.Context) error {
    results := make([]message.ToolResult, len(p.currentTools))

    for i, toolCall := range p.currentTools {
        if !toolCall.Finished {
            results[i] = message.ToolResult{
                ToolCallID: toolCall.ID,
                Content:    "Tool execution incomplete",
                IsError:    true,
            }
            continue
        }

        result, err := p.executor.Execute(ctx, toolCall)
        if err != nil {
            results[i] = message.ToolResult{
                ToolCallID: toolCall.ID,
                Content:    err.Error(),
                IsError:    true,
            }
        } else {
            results[i] = result
        }
    }

    // Reset current tools
    p.currentTools = nil

    return nil
}
```

#### 1.5 Create DirectToolExecutor
**File:** `internal/executor/executor.go`

```go
package executor

import (
    "context"
    "fmt"

    "github.com/charmbracelet/crush-headless/internal/config"
    "github.com/charmbracelet/crush-headless/internal/llm/tools"
    "github.com/charmbracelet/crush-headless/internal/lsp"
    "github.com/charmbracelet/crush-headless/internal/message"
)

type DirectExecutor struct {
    tools    map[string]tools.BaseTool
    lspCache map[string]*lsp.Client
    cwd      string
}

func New(cfg *config.Config) (*DirectExecutor, error) {
    // Initialize all tools
    toolList := []tools.BaseTool{
        tools.NewBashTool(nil, cfg.WorkingDir, cfg.Options.Attribution),
        tools.NewEditTool(nil, nil, nil, cfg.WorkingDir),
        tools.NewViewTool(nil, nil, cfg.WorkingDir),
        tools.NewGrepTool(cfg.WorkingDir),
        tools.NewGlobTool(cfg.WorkingDir),
        tools.NewWriteTool(nil, nil, nil, cfg.WorkingDir),
        tools.NewLsTool(nil, cfg.WorkingDir),
        // Add more as needed
    }

    toolMap := make(map[string]tools.BaseTool)
    for _, tool := range toolList {
        toolMap[tool.Info().Name] = tool
    }

    return &DirectExecutor{
        tools:    toolMap,
        lspCache: make(map[string]*lsp.Client),
        cwd:      cfg.WorkingDir,
    }, nil
}

func (e *DirectExecutor) Execute(ctx context.Context, call message.ToolCall) (message.ToolResult, error) {
    tool, ok := e.tools[call.Name]
    if !ok {
        return message.ToolResult{
            ToolCallID: call.ID,
            Content:    fmt.Sprintf("tool not found: %s", call.Name),
            IsError:    true,
        }, nil
    }

    // Convert to tools.ToolCall format
    toolCall := tools.ToolCall{
        ID:    call.ID,
        Name:  call.Name,
        Input: call.Input,
    }

    // Execute directly (no permission check)
    response, err := tool.Run(ctx, toolCall)
    if err != nil {
        return message.ToolResult{
            ToolCallID: call.ID,
            Content:    err.Error(),
            IsError:    true,
        }, err
    }

    return message.ToolResult{
        ToolCallID: call.ID,
        Content:    response.Content,
        Metadata:   response.Metadata,
        IsError:    response.IsError,
    }, nil
}

func (e *DirectExecutor) GetTools() []tools.BaseTool {
    result := make([]tools.BaseTool, 0, len(e.tools))
    for _, tool := range e.tools {
        result = append(result, tool)
    }
    return result
}
```

#### 1.6 Testing
```bash
# Build
go build -o crush-headless ./cmd/headless

# Test basic execution
./crush-headless "what files are in the current directory?"

# Test with thinking
./crush-headless --show-thinking "explain the main.go file"

# Test JSON output
./crush-headless --output-format=json "list all Go files" | jq
```

## Phase 2: Thinking Exposure (Week 2)

### Goal
Full reasoning transparency with multiple output formats.

### Tasks

#### 2.1 Implement ThinkingFormatter
**File:** `internal/stream/thinking.go`

```go
package stream

import (
    "encoding/json"
    "fmt"
)

type ThinkingFormat string

const (
    ThinkingFormatJSON ThinkingFormat = "json"
    ThinkingFormatText ThinkingFormat = "text"
    ThinkingFormatNone ThinkingFormat = "none"
)

type ThinkingEvent struct {
    Type      string `json:"type"`
    Signature string `json:"signature,omitempty"`
    Content   string `json:"content"`
}

func FormatThinking(format ThinkingFormat, signature, thinking string) []byte {
    switch format {
    case ThinkingFormatJSON:
        event := ThinkingEvent{
            Type:      "extended_thinking",
            Signature: signature,
            Content:   thinking,
        }
        data, _ := json.Marshal(event)
        return append(data, '\n')

    case ThinkingFormatText:
        if signature != "" {
            return []byte(fmt.Sprintf("\n[THINKING: %s]\n%s\n", signature, thinking))
        }
        return []byte(fmt.Sprintf("\n[THINKING]\n%s\n", thinking))

    case ThinkingFormatNone:
        return nil
    }

    return nil
}
```

#### 2.2 Implement OutputFormatter
**File:** `internal/stream/output.go`

```go
package stream

import (
    "encoding/json"
    "fmt"

    "github.com/charmbracelet/crush-headless/internal/llm/provider"
)

type OutputFormat string

const (
    OutputFormatText OutputFormat = "text"
    OutputFormatJSON OutputFormat = "json"
    OutputFormatDiff OutputFormat = "diff"
)

type ThinkingBlock struct {
    Signature string `json:"signature,omitempty"`
    Content   string `json:"content"`
}

type ToolCallSummary struct {
    Name  string `json:"name"`
    Input string `json:"input,omitempty"`
}

type OutputResult struct {
    Content       string            `json:"content"`
    Thinking      []ThinkingBlock   `json:"thinking,omitempty"`
    ToolCalls     []ToolCallSummary `json:"tool_calls,omitempty"`
    FilesModified []string          `json:"files_modified,omitempty"`
    TokensUsed    *provider.TokenUsage `json:"tokens_used,omitempty"`
    Cost          float64           `json:"cost,omitempty"`
}

func FormatOutput(format OutputFormat, result OutputResult) ([]byte, error) {
    switch format {
    case OutputFormatJSON:
        return json.MarshalIndent(result, "", "  ")

    case OutputFormatText:
        // Content already streamed, just return metadata if any
        if result.TokensUsed != nil {
            meta := fmt.Sprintf("\n\n---\nTokens: %d input, %d output | Cost: $%.4f\n",
                result.TokensUsed.InputTokens,
                result.TokensUsed.OutputTokens,
                result.Cost)
            return []byte(meta), nil
        }
        return nil, nil

    case OutputFormatDiff:
        // TODO: Extract diffs from tool results
        return []byte(result.Content), nil
    }

    return nil, fmt.Errorf("unknown output format: %s", format)
}
```

#### 2.3 Update Runner to Collect Metadata
```go
// In runner.go, track thinking and tool calls
type Runner struct {
    // ... existing fields ...
    thinking      []ThinkingBlock
    toolCalls     []ToolCallSummary
    filesModified []string
}

func (r *Runner) Execute(ctx context.Context, prompt string) error {
    // ... setup ...

    // Track metadata during streaming
    processor := stream.New(os.Stdout, os.Stderr, executor, stream.Options{
        ShowThinking:   r.options.ShowThinking,
        ThinkingFormat: r.options.ThinkingFormat,
        Quiet:          r.options.Quiet,
        OnThinking: func(sig, content string) {
            r.thinking = append(r.thinking, stream.ThinkingBlock{
                Signature: sig,
                Content:   content,
            })
        },
        OnToolCall: func(name, input string) {
            r.toolCalls = append(r.toolCalls, stream.ToolCallSummary{
                Name:  name,
                Input: input,
            })
        },
    })

    // ... process stream ...

    // Format final output
    result := stream.OutputResult{
        Content:       processor.GetContent(),
        Thinking:      r.thinking,
        ToolCalls:     r.toolCalls,
        FilesModified: r.filesModified,
        TokensUsed:    finalUsage,
        Cost:          calculateCost(finalUsage, model),
    }

    output, err := stream.FormatOutput(r.options.OutputFormat, result)
    if err != nil {
        return err
    }

    if len(output) > 0 {
        fmt.Fprint(os.Stdout, string(output))
    }

    return nil
}
```

## Phase 3: Performance Optimization (Week 3)

### Goal
2-3x faster cold start, 30% faster execution.

### Tasks

#### 3.1 Lazy LSP Initialization
```go
// internal/executor/lsp.go
type LazyLSP struct {
    config  map[string]config.LSPConfig
    clients map[string]*lsp.Client
    mu      sync.RWMutex
}

func (l *LazyLSP) GetClient(ctx context.Context, lang string) (*lsp.Client, error) {
    // Check cache first
    l.mu.RLock()
    client, ok := l.clients[lang]
    l.mu.RUnlock()

    if ok {
        return client, nil
    }

    // Initialize on first use
    l.mu.Lock()
    defer l.mu.Unlock()

    // Double-check after acquiring write lock
    if client, ok := l.clients[lang]; ok {
        return client, nil
    }

    cfg, ok := l.config[lang]
    if !ok {
        return nil, fmt.Errorf("no LSP config for %s", lang)
    }

    client, err := lsp.NewClient(ctx, cfg)
    if err != nil {
        return nil, err
    }

    l.clients[lang] = client
    return client, nil
}
```

#### 3.2 Headless-Specific Prompt
**File:** `internal/prompt/headless.md`

```markdown
You are Crush running in non-interactive headless mode. Execute the user's request completely and autonomously.

# Execution Mode

- Single-shot: Complete the entire task before returning
- No follow-up: User cannot respond, so be thorough
- Autonomous: Make decisions without asking
- Concise: Minimize unnecessary explanation

# Tool Usage

You have access to all standard Crush tools. Use them freely without permission checks:
- view, edit, write: File operations
- bash: Execute commands
- grep, glob: Search
- ls: Directory listing

# Output Format

- Stream content directly to stdout
- Tool execution details go to stderr (if not in quiet mode)
- Thinking/reasoning goes to stderr (if --show-thinking is enabled)
- Be concise in explanations unless detail is critical

# Environment

<env>
Working directory: {{.WorkingDir}}
Platform: {{.Platform}}
</env>

Execute the task completely and verify your work before finishing.
```

#### 3.3 Parallel Read-Only Tool Execution
```go
// internal/executor/parallel.go
func (e *DirectExecutor) ExecuteBatch(ctx context.Context, calls []message.ToolCall) ([]message.ToolResult, error) {
    // Check if all are read-only
    allReadOnly := true
    for _, call := range calls {
        tool := e.tools[call.Name]
        if tool != nil && !isReadOnlyTool(tool.Info().Name) {
            allReadOnly = false
            break
        }
    }

    if !allReadOnly {
        // Execute sequentially
        return e.executeSequential(ctx, calls)
    }

    // Execute in parallel
    results := make([]message.ToolResult, len(calls))
    var wg sync.WaitGroup
    errChan := make(chan error, len(calls))

    for i, call := range calls {
        wg.Add(1)
        go func(idx int, tc message.ToolCall) {
            defer wg.Done()
            result, err := e.Execute(ctx, tc)
            if err != nil {
                errChan <- err
                return
            }
            results[idx] = result
        }(i, call)
    }

    wg.Wait()
    close(errChan)

    if err := <-errChan; err != nil {
        return nil, err
    }

    return results, nil
}

func isReadOnlyTool(name string) bool {
    readOnly := map[string]bool{
        "view": true,
        "grep": true,
        "glob": true,
        "ls":   true,
    }
    return readOnly[name]
}
```

## Phase 4: Polish & Production (Week 4)

### Tasks

#### 4.1 Error Handling
```go
// internal/runner/errors.go
type ErrorHandler struct {
    maxRetries int
    retryDelay time.Duration
}

func (h *ErrorHandler) HandleProviderError(ctx context.Context, err error) error {
    if isContextError(err) {
        return err // Don't retry cancellation
    }

    if isRateLimitError(err) {
        return h.retryWithBackoff(ctx, err)
    }

    if isNetworkError(err) {
        return h.retryWithBackoff(ctx, err)
    }

    return err // Fatal error
}
```

#### 4.2 Comprehensive Testing
```go
// internal/runner/runner_test.go
func TestRunner_Execute(t *testing.T) {
    tests := []struct {
        name    string
        prompt  string
        options Options
        want    string
        wantErr bool
    }{
        {
            name:   "basic execution",
            prompt: "list files",
            want:   "main.go\nREADME.md\n",
        },
        {
            name:   "with thinking",
            prompt: "explain code",
            options: Options{ShowThinking: true, ThinkingFormat: "json"},
            want:   `{"type":"extended_thinking"`,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test implementation
        })
    }
}
```

#### 4.3 Documentation
- API reference
- Examples
- Migration guide from `crush run`
- CI/CD integration examples

#### 4.4 Benchmarks
```go
// benchmark_test.go
func BenchmarkColdStart(b *testing.B) {
    for i := 0; i < b.N; i++ {
        runner, _ := runner.New(cfg, opts)
        runner.Execute(ctx, "echo hello")
    }
}

func BenchmarkFirstToken(b *testing.B) {
    // Measure time to first content output
}
```

## Final Checklist

- [ ] Zero database usage
- [ ] No pub/sub system
- [ ] Direct stdout/stderr streaming
- [ ] Thinking exposure with multiple formats
- [ ] Lazy LSP initialization
- [ ] Parallel read-only tool execution
- [ ] Headless-specific prompt
- [ ] Error handling and retries
- [ ] Comprehensive tests (>80% coverage)
- [ ] Performance benchmarks
- [ ] Documentation complete
- [ ] CI/CD integration examples
