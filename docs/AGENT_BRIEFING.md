# Agent Briefing: Cliffy Development

## Context

You're working on **Cliffy** - a fast, headless fork of Crush optimized for one-off AI coding tasks. This is a fresh fork from `charmbracelet/crush` that will become `github.com/bwl/cliffy`.

## Project Status

- ✅ Forked from crush v0.10.4
- ✅ Documentation complete (in `docs/`)
- ⏳ **Next: Phase 1 implementation** (remove unwanted components, create cliffy entry point)

## Required Reading (in order)

**Start here:**
1. **`docs/crush-headless-overview.md`** - Read first! High-level vision, why cliffy exists
2. **`docs/architecture.md`** - System design, what to keep vs. remove
3. **`docs/implementation-guide.md`** - Phase-by-phase build instructions

**Reference as needed:**
4. **`docs/performance-analysis.md`** - What we're optimizing and why
5. **`docs/model-selection.md`** - Critical feature: per-task model selection
6. **`docs/api-specification.md`** - CLI flags and behavior
7. **`docs/fork-strategy.md`** - Long-term maintenance strategy

## Key Facts

### What Cliffy Is
- **Headless** AI coding assistant (no TUI, no interactive mode)
- **Fast** (4x faster cold start vs `crush run -q -y`)
- **Focused** on one-off tasks, CI/CD, automation
- **Transparent** (exposes LLM thinking/reasoning - crush doesn't!)
- **Flexible** (model selection per task, not per session)

### What Makes It Different from Crush
```
Crush = Interactive TUI + Sessions + History + Database
Cliffy = Direct streaming + Zero persistence + Speed
```

**Usage:**
```bash
# Crush
crush run -q -y "task"  # 800ms startup, 50+ DB writes, no thinking visible

# Cliffy (goal)
cliffy "task"           # 200ms startup, 0 DB writes, thinking visible
cliffy --fast "quick"   # Use cheap model
cliffy --smart "complex" # Use powerful model
cliffy --show-thinking "debug" # See reasoning
```

## Your Immediate Tasks

### Phase 1: Remove Unwanted Components

**Remove these directories:**
```bash
rm -rf internal/tui/           # Bubbletea UI
rm -rf internal/db/            # SQLite persistence
rm -rf internal/pubsub/        # Event system
rm -rf internal/session/       # Session management
rm -rf internal/permission/    # Permission prompts (auto-approve in cliffy)
rm -rf internal/history/       # File history tracking
rm -rf internal/app/           # App coordinator (too heavy)
rm -rf internal/event/         # Metrics/telemetry
rm -rf internal/format/        # Spinner (we'll create minimal version)
```

**Keep these directories:**
```bash
internal/llm/provider/    # Anthropic, OpenAI, Gemini clients
internal/llm/tools/       # All tools (bash, edit, view, grep, etc)
internal/llm/prompt/      # System prompts
internal/lsp/             # LSP integration
internal/config/          # Config loading
internal/shell/           # Persistent shell
internal/message/         # Message types
internal/fsext/           # File utilities
internal/diff/            # Diff generation
internal/csync/           # Concurrency utilities
internal/env/             # Environment utilities
internal/log/             # Logging
```

**Update go.mod:**
```bash
# Change module name
module github.com/bwl/cliffy

# Remove TUI dependencies:
# - github.com/charmbracelet/bubbles/v2
# - github.com/charmbracelet/bubbletea/v2
# - github.com/charmbracelet/lipgloss/v2
# - github.com/charmbracelet/glamour/v2
# - github.com/ncruces/go-sqlite3 (DB)
# - github.com/pressly/goose/v3 (DB migrations)
```

### Phase 2: Create Cliffy Entry Point

**File: `cmd/cliffy/main.go`**

Reference `docs/implementation-guide.md` section 1.3 for the basic structure.

**Key features:**
- Cobra CLI with flags: `--model`, `--reasoning`, `--fast`, `--smart`, `--show-thinking`, `--output-format`
- Direct streaming to stdout (no TUI)
- Thinking/reasoning visible via stderr (if `--show-thinking`)
- Cost awareness (`--show-cost`, `--max-cost`)

**File: `internal/runner/runner.go`**

Implements the core execution flow:
1. Load config
2. Select model based on flags
3. Initialize provider
4. Stream LLM response
5. Execute tools (no permission checks)
6. Loop until complete

See `docs/implementation-guide.md` sections 1.4-1.6 for code examples.

### Phase 3: Update All Imports

After removing components, you'll need to update imports throughout the codebase:

**Tools will need:**
- Remove permission service parameters (auto-approve in cliffy)
- Remove history service parameters (no file tracking)
- Update to use cliffy's module path

**Example:**
```go
// Old (in crush):
func NewBashTool(permissions permission.Service, cwd string, attr config.Attribution) BaseTool

// New (in cliffy):
func NewBashTool(cwd string, attr config.Attribution) BaseTool
```

## Critical Issues to Solve

### 1. **Thinking/Reasoning Exposure** (HIGH PRIORITY)

**Problem:** Crush captures extended thinking but never shows it to users!

**Location:** `internal/llm/agent/agent.go:709-714`
```go
case provider.EventThinkingDelta:
    assistantMsg.AppendReasoningContent(event.Thinking)
    return a.messages.Update(ctx, *assistantMsg)  // Stored but not printed!
```

**Solution:** In cliffy, stream thinking directly to stderr:
```go
case provider.EventThinkingDelta:
    if showThinking {
        fmt.Fprintf(os.Stderr, "[THINKING] %s", event.Thinking)
    }
```

See `docs/architecture.md` section on StreamingProcessor for full implementation.

### 2. **Permission System Removal**

Tools in `internal/llm/tools/` all take a `permission.Service` parameter. In cliffy:
- No interactive approval possible
- User explicitly ran the command = implied consent
- Remove permission checks entirely

### 3. **Model Selection**

Crush selects model per-session in TUI. Cliffy needs per-invocation selection via CLI flags.

See `docs/model-selection.md` for full specification.

## Architecture Overview

### Data Flow (Cliffy)

```
User invokes: cliffy --smart "debug race condition"
    ↓
main.go parses flags, loads config
    ↓
runner.Execute(ctx, prompt)
    ↓
provider.StreamResponse(messages, tools) → events channel
    ↓
for event := range events:
    case ThinkingDelta  → stderr (if --show-thinking)
    case ContentDelta   → stdout (streaming)
    case ToolUseStart   → execute tool immediately
    case Complete       → done
```

**No database, no pub/sub, no sessions, no UI.**

### Key Differences from Crush

| Aspect | Crush | Cliffy |
|--------|-------|--------|
| Entry | `cmd/crush/main.go` (TUI) | `cmd/cliffy/main.go` (CLI) |
| Mode | Interactive | One-shot |
| Storage | SQLite (sessions, messages) | None (ephemeral) |
| Events | Pub/sub to TUI | Direct stdout/stderr |
| Thinking | Hidden (stored in DB) | Visible (streamed) |
| Model | Selected in TUI per session | CLI flag per invocation |
| Permissions | Interactive prompts | Auto-approved |
| Speed | 800ms + title gen (1.5s) | 200ms target |

## Testing Your Changes

### Build Test
```bash
cd /path/to/cliffy
go build -o bin/cliffy ./cmd/cliffy
```

Should compile without errors.

### Smoke Test
```bash
./bin/cliffy "echo hello"
```

Should execute successfully (even if just stub implementation).

### Dependency Check
```bash
go mod tidy
go mod graph | grep -E "tui|bubbletea|lipgloss|sqlite"
```

Should return nothing (TUI deps removed).

## Common Issues

### Import Errors After Removal

After removing `internal/db/`, `internal/pubsub/`, etc., you'll see import errors in:
- `internal/llm/tools/*.go` - Remove permission/history params
- `internal/message/*.go` - May reference db/pubsub, simplify to types only
- `internal/config/*.go` - May reference session defaults, remove

**Strategy:** Start from the bottom (tools, message types) and work up (runner, main).

### Tool Construction

Crush tools are created with services they don't need in cliffy:

```go
// Crush
tools := []tools.BaseTool{
    tools.NewBashTool(permissions, cwd, attribution),
    tools.NewEditTool(lspClients, permissions, history, cwd),
    // ...
}

// Cliffy
tools := []tools.BaseTool{
    tools.NewBashTool(cwd, attribution),
    tools.NewEditTool(lspClients, cwd),  // Simplified
    // ...
}
```

You'll need to update tool constructors.

## Success Criteria

### Phase 1 Complete When:
- ✅ TUI/DB/pubsub/session components removed
- ✅ `go.mod` updated to `github.com/bwl/cliffy`
- ✅ TUI dependencies removed from go.mod
- ✅ Project compiles (`go build ./cmd/cliffy`)

### Phase 2 Complete When:
- ✅ `cliffy --help` shows flags
- ✅ `cliffy "echo hello"` executes
- ✅ `--show-thinking` flag streams to stderr
- ✅ `--model` flag overrides config

### Phase 3 Complete When:
- ✅ All tools work without permission service
- ✅ Streaming output works (stdout)
- ✅ Thinking visible (stderr when enabled)
- ✅ Performance: < 500ms cold start

## Getting Help

If you get stuck:

1. **Architecture questions:** See `docs/architecture.md`
2. **Implementation details:** See `docs/implementation-guide.md` (has code examples)
3. **Performance targets:** See `docs/performance-analysis.md`
4. **CLI behavior:** See `docs/api-specification.md`

## Current State Summary

**What exists:**
- Complete documentation (this repo, `docs/`)
- Fresh fork of crush v0.10.4
- All crush code intact (unmodified)

**What you need to do:**
1. Remove unwanted components (see lists above)
2. Create cliffy entry point (`cmd/cliffy/main.go`)
3. Create runner (`internal/runner/runner.go`)
4. Update tool constructors (remove permission/history)
5. Test build

**Estimated time:** 2-4 hours for Phase 1 completion

## Questions to Consider

1. **Do we remove components immediately or gradually?**
   - Recommendation: Remove immediately, fix import errors as you go
   - Faster to see what breaks than to tentatively remove

2. **Do we keep message types even though no DB?**
   - Yes! Message types are used by provider layer
   - Just remove the service/CRUD, keep the types

3. **What about LSP initialization?**
   - Keep it, but make it lazy (only init when tool needs it)
   - See `docs/performance-analysis.md` section on lazy LSP

4. **Should we remove ALL telemetry/metrics?**
   - Yes for now - focus on core functionality
   - Can add minimal metrics later if needed

## Final Note

**You're not modifying Crush, you're creating Cliffy.**

Be bold with removals. If something references sessions/DB/pubsub/TUI, and it's not in the "keep" list - remove it or stub it out.

The goal is a **lean, fast, focused** tool that does one thing well: execute AI coding tasks quickly with full transparency.

Good luck! 🚀
