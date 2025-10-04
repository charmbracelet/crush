# Cliffy Documentation

> **Status Update:** This directory originally contained design docs for "crush-headless" (a planning-phase name). Cliffy has now been **implemented** with some differences from the original design. Documents have been updated to reflect current reality.

## Current Implementation (v0.1.0)

Cliffy is a fast, headless AI coding assistant optimized for one-off tasks and parallel execution. It's a fork of Crush with all interactive/persistent components removed.

**What Works:**
- ✅ Single and multi-task execution
- ✅ Parallel task processing (volley mode)
- ✅ Smart retry with exponential backoff
- ✅ JSON output and NDJSON tool traces
- ✅ Token tracking and cost calculation
- ✅ Progress indicators and stats
- ✅ Preset system for model/agent configs
- ✅ Context files and shared context
- ✅ Shell completions

**What's Planned:**
- See [ROADMAP.md](./ROADMAP.md) for unimplemented features

## Documentation Structure

### [Architecture](./architecture.md) ⚠️ UPDATED
**For developers** - Current system design:
- ✅ Reflects actual implementation (Volley + Runner dual-path)
- Component breakdown
- Execution flow
- Memory model
- Concurrency patterns

### [Roadmap](./ROADMAP.md) ⭐ NEW
**For contributors** - Features and improvements:
- What's implemented vs planned
- Priority levels and status
- References to code locations
- How to contribute

### [Implementation Guide](./implementation-guide.md) ⚠️ HISTORICAL
**Original design doc** - Preserved for reference:
- Shows original "crush-headless" design
- **Not the actual implementation**
- Marked as historical document
- Useful for understanding design evolution

### [Performance Analysis](./performance-analysis.md) 📊
**For optimization work** - Benchmarks vs Crush:
- Cliffy vs Crush performance comparisons
- Cold start times
- Memory usage
- Token overhead
- Real-world scenarios

### [API Specification](./api-specification.md) ⚠️ PARTIALLY OUTDATED
**For users** - CLI reference:
- ⚠️ Originally written for "crush-headless" design
- Some features described don't exist (e.g., full diff mode)
- Check `cliffy --help` for current accurate flags
- JSON schemas still relevant

### [Model Selection & Reasoning](./model-selection.md)
**For users** - Model selection:
- `--fast` vs `--smart` flags
- `--model` for specific models
- `--preset` for curated configs
- Cost and performance trade-offs

### [Fork Strategy](./fork-strategy.md)
**For maintainers** - Keeping up with Crush:
- Sync strategy recommendations
- Component overlap analysis
- Update procedures
- Risk mitigation

## Quick Reference

### Usage Examples

```bash
# Single task
cliffy "analyze auth.go for security issues"

# Multiple tasks in parallel (volley mode)
cliffy "analyze auth.go" "analyze db.go" "analyze api.go"

# With thinking visible for debugging
cliffy --show-thinking "why is this test failing?"

# JSON output for CI/CD
cliffy --output-format json "review this diff" | jq

# Parallel execution with progress
cliffy --verbose "task1" "task2" "task3"

# Tool trace export for monitoring
cliffy --emit-tool-trace "task" 2>tools.ndjson
```

### Performance vs Crush

| Metric | Crush | Cliffy | Improvement |
|--------|-------|--------|-------------|
| Cold start | 800ms | ~200ms | **4x faster** |
| Title gen | 1.5s | 0s (skipped) | **Eliminated** |
| Memory | ~50MB | ~15MB | **3x less** |
| DB writes | 50+ | 0 | **Zero I/O** |

### Key Features

✅ **Implemented:**
- Zero persistence (no database)
- Parallel task execution (volley mode)
- Smart retry with exponential backoff
- JSON output and NDJSON tool traces
- Token tracking and cost calculation
- Progress indicators
- Preset system

📋 **Planned:**
- Full streaming in single-task mode
- Diff output mode
- Enhanced thinking formatter
- See [ROADMAP.md](./ROADMAP.md) for details

## Actual File Structure

**Current Cliffy structure:**

```
cliffy/
├── cmd/cliffy/
│   ├── main.go              # CLI entry point
│   ├── doctor.go            # Health check command
│   ├── init.go              # Setup command
│   ├── preset.go            # Preset management
│   └── volley.go            # Volley helpers
├── internal/
│   ├── agent/               # Agent system
│   ├── volley/              # Multi-task scheduler
│   ├── runner/              # Single-task executor
│   ├── output/              # Formatters
│   ├── preset/              # Preset system
│   ├── config/              # Configuration
│   ├── llm/                 # LLM providers and tools
│   ├── lsp/                 # LSP integration
│   └── message/             # In-memory message store
└── docs/                    # This documentation
```

**Shared from Crush:**
- LLM providers (Anthropic, OpenAI, Gemini, etc.)
- All tools (bash, edit, view, grep, etc.)
- LSP integration
- Config system
- Utility modules

## Key Improvements Over Crush

### 1. Zero Persistence
- No SQLite database
- No session storage
- Zero disk I/O during execution
- Faster startup, lower memory

### 2. Parallel Execution
- Volley mode for multiple tasks
- Worker pool with configurable concurrency
- Smart retry with exponential backoff
- Real-time progress tracking

### 3. Thinking Visibility
- `--show-thinking` flag exposes reasoning
- Useful for debugging model behavior
- Can output to stderr for logging
- Optional JSON format

### 4. Automation-Friendly
- JSON output for parsing
- NDJSON tool traces for monitoring
- Exit codes for scripting
- Token usage and cost tracking

## Questions & Support

- **Current Architecture:** See [architecture.md](./architecture.md)
- **Planned Features:** See [ROADMAP.md](./ROADMAP.md)
- **Performance Data:** See [performance-analysis.md](./performance-analysis.md)
- **Improvement Ideas:** See [../IMPROVEMENT_REPORT.md](../IMPROVEMENT_REPORT.md)

---

**Status:** v0.1.0 - Implemented and functional
**Goal Achieved:** Fast, headless, parallel execution with zero persistence
