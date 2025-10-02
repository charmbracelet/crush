# Cliffy Observability Documentation

Documentation for enhancing cliffy's observability and developer experience.

---

## Overview

This directory contains research, planning, and implementation guides for cliffy's observability improvements. The goal is to transform cliffy from "fast but opaque" to "fast and trustworthy" through enhanced tool logging, progress feedback, and better error reporting.

---

## Documents

### 1. [Current State](./current-state.md)
**Purpose**: Status report of what's been completed in Sprint 1

**Contents**:
- ✅ Completed infrastructure (metadata collection, verbosity system, formatting)
- ❌ What's not working yet (display pipeline)
- Architecture analysis and findings
- Testing results
- Next steps and decisions needed

**Read this first** to understand where we are.

---

### 2. [Streaming Architecture](./streaming-architecture.md)
**Purpose**: Detailed research on streaming event implementation

**Contents**:
- Current event flow architecture
- Two implementation options (simple vs streaming)
- Why streaming is recommended
- Detailed technical analysis
- Benefits and trade-offs
- Timeline estimates (4-6 hours)

**Read this** to understand the architectural approach.

---

### 3. [Implementation Guide](./implementation-guide.md)
**Purpose**: Step-by-step implementation instructions

**Contents**:
- 4 implementation phases with code examples
- Testing checkpoints after each phase
- Common issues and solutions
- Performance considerations
- Success criteria

**Use this** as your implementation roadmap.

---

## Quick Start

### For Implementers

1. **Read**: [Current State](./current-state.md) - Understand what exists
2. **Research**: [Streaming Architecture](./streaming-architecture.md) - Understand the approach
3. **Implement**: [Implementation Guide](./implementation-guide.md) - Follow step-by-step
4. **Test**: Run tests at each checkpoint
5. **Complete**: Verify success criteria

### For Decision Makers

**Key Decision**: Option A (simple, 2-3h) vs Option B (streaming, 4-6h)

**Recommendation**: Option B (streaming) because:
- Real-time feedback during execution
- Solves "is it stuck?" problem
- Enables progress events (Phase 3 of plan)
- Uses existing streaming infrastructure
- Higher UX value

See [Streaming Architecture - Two Implementation Options](./streaming-architecture.md#two-implementation-options) for details.

---

## Project Context

### Source Documents

This work is based on:
- **CLIFFY-FEEDBACK.md** - Extensive user feedback from actual usage
- **OBSERVABILITY-PLAN.md** - Comprehensive 9-phase implementation plan

### Sprint Structure

**Sprint 1** (Partial - 80% complete):
- ✅ Tool metadata infrastructure
- ✅ Three-tier verbosity system
- ✅ Tool trace formatting
- ⏳ Display wiring (next step)

**Sprint 2** (Planned):
- Streaming progress events
- Diff preview for modifications

**Sprint 3** (Planned):
- Smart output formatting
- JSON output improvements

---

## Architecture Summary

### Current Flow (Broken)
```
Tool executes → Metadata captured ✅
    ↓
Agent stores in messages ✅
    ↓
Agent emits ONE final event ⚠️
    ↓
Volley waits for final event ⚠️
    ↓
Output displays result ❌ (no tool traces)
```

### Streaming Flow (Goal)
```
Tool executes → Metadata captured
    ↓
Tool event emitted → Agent sends to channel
    ↓
Volley processes event → Display trace in real-time
    ↓
... more tool events ...
    ↓
Final response event → Display result
```

---

## Key Technical Decisions

### 1. Event Channel through Context
Pass event channel via `context.WithValue()` to make it available in tool execution loop.

**Why**: Avoids threading channel through 5+ function signatures.

### 2. Non-blocking Send
Use `select`/`default` when emitting events.

**Why**: Prevents deadlock if channel is full.

### 3. Buffered Channel (10 events)
Increase from size 1 to size 10.

**Why**: Accommodate multiple tool events + final response.

### 4. Progress Function via Context
Pass progress function to tools via context (Phase 4).

**Why**: Avoids circular dependency (tools can't import agent).

### 5. Display to Stderr
Tool traces print to `os.Stderr`, results to `os.Stdout`.

**Why**: Standard practice, allows separate redirection.

---

## Testing Strategy

### Checkpoint Testing

**Phase 1**: Add debug log when emitting event
- Verify: "Tool event emitted" in logs

**Phase 2**: Add debug log when receiving event
- Verify: "Tool trace received" in logs

**Phase 3**: Remove debug logs, test end-to-end
- Verify: [TOOL] lines appear in output

### Integration Tests

```bash
# Single tool
./bin/cliffy "summarize README.md"

# Multiple tools
./bin/cliffy "find all Go files and count them"

# Quiet mode
./bin/cliffy --quiet "same task"

# Parallel execution
./bin/cliffy "task1" "task2" "task3"
```

---

## Success Metrics

### User Experience
- [x] Tool metadata captured (Sprint 1 ✅)
- [ ] Tool traces displayed in real-time
- [ ] `--quiet` provides clean output
- [ ] Progress feedback on long operations (Phase 4)
- [ ] No "is it stuck?" anxiety

### Performance
- [ ] <50ms overhead per task
- [ ] No deadlocks or hangs
- [ ] No dropped events

### Code Quality
- [ ] No circular dependencies
- [ ] Clean error handling
- [ ] Consistent patterns
- [ ] Well-documented

---

## Estimated Effort

| Phase | Description | Effort |
|-------|-------------|--------|
| 1 | Emit tool events from agent | 1.5h |
| 2 | Process events in volley | 1.5h |
| 3 | Display tool traces | 1h |
| 4 | Progress events (optional) | 2h |
| **Total** | **Streaming implementation** | **4-6h** |

---

## Related Documentation

### In Repository
- `OBSERVABILITY-PLAN.md` - Full 9-phase plan (32 hours)
- `CLIFFY-FEEDBACK.md` - Original user feedback
- `CLAUDE.md` - Project architecture overview

### External Resources
- [Go Context Best Practices](https://go.dev/blog/context)
- [Channel Patterns in Go](https://go.dev/doc/effective_go#channels)

---

## Common Questions

### Q: Why not extract from message history (Option A)?
**A**: Works but misses opportunity for real-time feedback and progress events. Streaming is only slightly more complex but much higher value.

### Q: What if streaming has issues?
**A**: Can revert to Option A (post-execution display) with minimal changes. See [Implementation Guide - Rollback Plan](./implementation-guide.md#rollback-plan).

### Q: Will this slow down cliffy?
**A**: No. Overhead is <20ms per tool, negligible compared to LLM latency (seconds).

### Q: What about parallel execution?
**A**: Each task has separate event channel, fully thread-safe.

### Q: Can we skip Phase 4 (progress)?
**A**: Yes! Phases 1-3 are standalone. Phase 4 is optional enhancement.

---

## Next Steps

1. **Review documents** in order (Current State → Architecture → Guide)
2. **Decide on approach** (Option A vs B)
3. **Start implementation** following guide
4. **Test at checkpoints** to catch issues early
5. **Complete Sprint 1** with working tool traces

---

## Feedback & Questions

For questions about:
- **Architecture**: See `streaming-architecture.md`
- **Implementation**: See `implementation-guide.md`
- **Current status**: See `current-state.md`
- **Original plan**: See `../OBSERVABILITY-PLAN.md`

---

**Last Updated**: 2025-10-01
**Status**: Ready for implementation
**Estimated Start**: Phases 1-3 (4 hours)
