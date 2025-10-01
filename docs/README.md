# Crush-Headless Documentation

This directory contains comprehensive documentation for the proposed **crush-headless** project - an optimized fork of Crush designed specifically for non-interactive, one-off execution.

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

### Current Problems with `crush run -q -y`

1. **Slow:** 800ms cold start + 1.5s title generation = **2.3s before first token**
2. **Wasteful:** 50+ DB writes for one-off task that never needs history
3. **Opaque:** Extended thinking is captured but **never shown to users**
4. **Heavy:** 50MB memory, full SQLite + TUI infrastructure for simple streaming

### Headless Solution

```bash
# 200ms cold start, no title gen, direct streaming
crush-headless "fix the type errors"

# With thinking visible for debugging
crush-headless --show-thinking "why is this test failing?" 2>thinking.log

# JSON output for CI/CD
crush-headless --output-format=json "review this diff" | jq '.content'
```

### Performance Targets

| Metric | Current | Target | Improvement |
|--------|---------|--------|-------------|
| Cold start | 800ms | 200ms | **4x faster** |
| First token | 2500ms | 600ms | **4x faster** |
| Memory | 50MB | 12MB | **4x less** |
| Code | 100% | 35% | **65% simpler** |

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
