# Cliffy ᕕ( ᐛ )ᕗ

Fast, focused AI coding assistant for one-off tasks. Cliffy zips in, executes your task, and gets back to ready position.

Headless fork of [Crush](https://github.com/charmbracelet/crush). No TUI, no database, no sessions. Just direct execution.

## Quick Start

### 1. Build Cliffy

```bash
go build -o bin/cliffy ./cmd/cliffy
```

### 2. Configure (first run)

Generate a starter config with the provider you want to use:

```bash
./bin/cliffy init
```

If you already have an OpenRouter key, set it first so the wizard can pick it up:

```bash
export CLIFFY_OPENROUTER_API_KEY="your-api-key-here"
```

Prefer to hand-edit config? Run `./bin/cliffy doctor` to validate it.

### 3. Run a Task

```bash
./bin/cliffy "list all Go files in this project"
```

## Features

- **Fast**: Cold start in 200ms. Ready when you are.
- **Transparent**: Shows LLM thinking when you need it.
- **Zero Persistence**: No database, no sessions, no cleanup needed.
- **Focused**: One task, done right, back to waiting.
- **CLI-First**: Built for scripts, automation, quick hits.

## Usage

```bash
# Basic usage - single task
cliffy "your task here"

# Multiple tasks in parallel (volley mode)
cliffy "analyze auth.go" "analyze db.go" "analyze api.go"

# Show LLM thinking/reasoning
cliffy --show-thinking "debug this function"

# Use different model
cliffy --model sonnet "complex refactoring task"
cliffy --fast "simple task"      # alias for --model small
cliffy --smart "complex task"    # alias for --model large

# Shared context across multiple tasks
cliffy --context "You are a security expert" \
  "review auth.go" \
  "review db.go" \
  "review api.go"

# Load context from file
cliffy --context-file security-rules.md \
  "review auth.go" \
  "review payment.go"

# Quiet mode (no tool logs)
cliffy --quiet "run tests"

# Structured output formats
cliffy --output-format json "analyze code"   # JSON blob with summary
cliffy --output-format diff "summarize changes"   # Only show tool diffs

# Verbose mode with detailed stats
cliffy --verbose "task1" "task2"

# Show token usage and timing stats
cliffy --stats "complex analysis"

# Emit tool traces as NDJSON (perfect for logging)
cliffy --emit-tool-trace --verbose "task1"

# Tune concurrency and models with presets
cliffy --preset sec-review --max-concurrent 2 "audit auth.go" "scan payments.go"

# Feed prompts from files or pipelines
cliffy --tasks-file prompts.txt
cat tasks.json | cliffy --json -

# Verify configuration health
cliffy doctor
```

### Shell Completions

Install shell completions for tab-completion of commands and flags:

```bash
# Bash (Linux)
cliffy completion bash | sudo tee /etc/bash_completion.d/cliffy

# Bash (macOS with Homebrew)
cliffy completion bash > $(brew --prefix)/etc/bash_completion.d/cliffy

# Zsh
cliffy completion zsh > "${fpath[1]}/_cliffy"
# Then reload: exec zsh

# Fish
cliffy completion fish > ~/.config/fish/completions/cliffy.fish

# PowerShell
cliffy completion powershell > cliffy.ps1
# Then add to your PowerShell profile
```

After installation, restart your shell or source the completion file. Tab-completion will then work for all flags and commands.

## Configuration

Configuration is stored in `~/.config/cliffy/cliffy.json` (or `$XDG_CONFIG_HOME/cliffy/cliffy.json`):

```json
{
  "models": {
    "large": {
      "provider": "openrouter",
      "model": "x-ai/grok-4-fast:free",
      "max_tokens": 4096
    },
    "small": {
      "provider": "openrouter",
      "model": "x-ai/grok-4-fast:free",
      "max_tokens": 2048
    }
  },
  "providers": {
    "openrouter": {
      "base_url": "https://openrouter.ai/api/v1",
      "api_key": "${CLIFFY_OPENROUTER_API_KEY}",
      "models": [
        {
          "id": "x-ai/grok-4-fast:free",
          "name": "Grok 4 Fast (Free)"
        }
      ]
    }
  },
  "options": {
    "debug": false,
    "data_directory": ".cliffy"
  }
}
```

You can also create a local `.cliffy.json` or `cliffy.json` in your project directory to override global settings. The `cliffy init` wizard writes the global config; `cliffy doctor` verifies keys, provider connectivity, and context files before you start a session.

### Task Input Options

Cliffy reads task prompts from the following sources (highest priority first):

1. `--tasks-file` – line-delimited text, or JSON when paired with `--json`
2. `-` (single hyphen) – STDIN stream, supporting text or JSON
3. Positional arguments – traditional CLI usage

Blank lines and lines beginning with `#` are ignored in text mode, so you can keep prompt libraries under version control without extra tooling.

### Presets

Presets bundle model size, tool access, context paths, and concurrency defaults:

- List available options: `cliffy preset list`
- Inspect details: `cliffy preset show sec-review`
- Apply to your project config: `cliffy preset apply fast-qa`
- Use ad-hoc: `cliffy --preset perf-analyze "profile rendering"`

Custom presets live under `internal/preset/presets/` as JSON if you want to author your own.

### Health Checks

- `cliffy init` — interactive first-run setup for credentials and defaults
- `cliffy doctor` — validates configs, provider connectivity, API keys, and context paths

Both commands exit non-zero on failure, so you can wire them into CI or bootstrap scripts without surprises.

## Crush-Headless Documentation

This directory also contains comprehensive documentation for the **crush-headless** project design.

## Documentation Structure

### [Overview](./crush-headless-overview.md)
**Start here** - High-level introduction covering:
- Why a separate headless binary?
- Key improvements over `crush run -q -y`
- Performance targets
- Quick usage examples

### [Architecture](./architecture.md)
**For developers** - Detailed system design:
- Component breakdown
- Execution flow
- Memory model
- Removed vs. shared components
- Concurrency patterns

### [Implementation Guide](./implementation-guide.md)
**For contributors** - Step-by-step build instructions:
- 4-phase implementation plan
- Code examples for each component
- Testing strategy
- Week-by-week milestones

### [Performance Analysis](./performance-analysis.md)
**For optimization work** - Deep dive into performance:
- Current overhead breakdown
- Optimization impact by component
- Real-world scenario comparisons
- Benchmark targets
- Cost analysis

### [API Specification](./api-specification.md)
**For users** - Complete CLI reference:
- All flags and options
- Input/output formats
- JSON schema
- Environment variables
- Usage examples

### [Model Selection & Reasoning](./model-selection.md)
**For users** - Critical for one-off tasks:
- Choose models per task (fast vs. smart)
- Control reasoning levels (none/low/medium/high)
- Cost awareness and budgets
- Provider switching
- Interactive model selection

### [Fork Strategy](./fork-strategy.md)
**For maintainers** - How to keep up with Crush:
- Sync vs. hard fork analysis
- Commit activity breakdown (82% shared components)
- Recommended: Structured sync via Go modules
- Implementation roadmap
- Risk mitigation

## Quick Reference

### Why Cliffy exists

Crush's `run -q -y` mode takes 2.3 seconds before first token. Writes 50+ database entries for tasks you never revisit. Hides extended thinking that could help debug issues.

Cliffy strips all that. Direct streaming, 200ms cold start, optional thinking output. No database writes, no session management, no waiting around.

### Performance

Benchmarked with identical configurations (same model, same settings):

| Task | Crush | Cliffy | Speedup |
|------|-------|--------|---------|
| List files | 7719ms | 6902ms | 1.11x |
| Count lines | 14278ms | 9243ms | 1.54x |

See `benchmark/` for full test suite and methodology.

## Implementation Phases

### Phase 1: Fork & Simplify (Week 1)
- Remove: DB, pub/sub, sessions, TUI, permissions
- Create: Direct streaming architecture
- **Target:** Working prototype, 50% code reduction

### Phase 2: Thinking Exposure (Week 2)
- Add: `--show-thinking`, `--thinking-format`
- Implement: JSON/text formatters
- **Target:** Full reasoning transparency

### Phase 3: Performance (Week 3)
- Add: Lazy LSP, parallel tools
- Optimize: Prompt, memory usage
- **Target:** 2-3x faster execution

### Phase 4: Polish (Week 4)
- Add: Error handling, tests, docs
- Verify: Production readiness
- **Target:** Ship it

## Key Design Decisions

### 1. Separate Binary (Not a Mode)
**Why?** Cleaner dependencies, smaller binary, different optimization targets.

### 2. Zero Persistence
**Why?** One-off execution implies no need for history. Can be added later as opt-in feature.

### 3. Thinking Exposed by Default
**Why?** Current limitation - thinking is captured but never shown. Critical for debugging.

### 4. Reuse Tool System
**Why?** Tools are the core value. Just remove permission layer, not the tools themselves.

## File Structure

Proposed structure for `crush-headless`:

```
crush-headless/
├── main.go
├── internal/
│   ├── runner/
│   │   ├── runner.go          # HeadlessRunner
│   │   └── runner_test.go
│   ├── stream/
│   │   ├── processor.go       # StreamingProcessor
│   │   ├── thinking.go        # ThinkingFormatter
│   │   └── output.go          # OutputFormatter
│   ├── executor/
│   │   ├── executor.go        # DirectToolExecutor
│   │   └── parallel.go        # ParallelExecutor
│   └── prompt/
│       └── headless.md        # Headless prompt
├── go.mod
└── README.md
```

**Shared from crush:**
- `internal/llm/provider/` (providers unchanged)
- `internal/llm/tools/` (tool implementations)
- `internal/config/` (minimal config loading)
- `internal/lsp/` (LSP clients)
- Utility modules

## Critical Missing Feature

### Extended Thinking Not Exposed

**Current state in Crush:**
```go
// agent.go:709-714
case provider.EventThinkingDelta:
    assistantMsg.AppendReasoningContent(event.Thinking)
    return a.messages.Update(ctx, *assistantMsg)
```

Thinking is **captured and stored in DB** but:

```go
// app.go:174-195 - RunNonInteractive()
fmt.Print(part)  // Only prints msg.Content().String()
```

**Thinking is NEVER printed to user!**

**Headless fix:**
```go
case EventThinkingDelta:
    if showThinking {
        stderr.Write(formatThinking(event.Thinking))
    }
```

This alone is a huge value-add for debugging and understanding model behavior.

## Cost Savings

### Token Optimization
- Current prompt: ~2500 tokens overhead (TUI instructions, session mgmt)
- Headless prompt: ~1000 tokens (focused, minimal)
- **Savings:** 1500 tokens × $0.003/1K = $0.0045 per call

At 1000 calls/day: **$135/month**

### Title Generation
- Current: Extra LLM call (~$0.0006 + 1.5s per run)
- Headless: Skip entirely
- **Savings:** $18/month + time

### Total
**~$150/month** at 1000 calls/day + significant performance improvement

## Next Steps

1. **Read the docs** in order (Overview → Architecture → Implementation → Performance → API)
2. **Prototype Phase 1** - Get basic streaming working
3. **Validate approach** - Compare performance with current `crush run`
4. **Iterate** - Implement phases 2-4
5. **Ship** - Release as separate binary

## Questions?

For implementation questions, see [Implementation Guide](./implementation-guide.md).

For performance specifics, see [Performance Analysis](./performance-analysis.md).

For API details, see [API Specification](./api-specification.md).

For architecture decisions, see [Architecture](./architecture.md).

---

**Status:** Documentation complete, implementation not started
**Target:** Separate binary, not a mode within Crush
**Goal:** 4x faster, 4x lighter, 100% thinking visible
