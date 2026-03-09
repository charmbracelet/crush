# ADR-003: Agent Process Model

## Status

Proposed

## Context

Agents need to execute work with isolation. Options include:

1. **Goroutines** - Lightweight, shared memory, in-process
2. **Subprocesses** - OS-level isolation, separate memory
3. **CLI invocations** - Fresh process per task, maximum isolation

We need agent isolation, error containment, and resource management.

## Decision

**Use CLI invocations managed by Task Manager.**

Each agent:
- Spawned as separate `crush agent start --task T-005` invocation
- Works in isolated git worktree
- Exits when task completes
- State persisted to `.orchestra/state/agents/`

Rationale:
- Maximum isolation (crash doesn't affect orchestrator)
- Fresh context per task (no memory leaks)
- Easy debugging (run agent standalone)
- Resource cleanup on exit

## Consequences

### Positive

- Crash isolation
- Memory isolation
- Easy to debug individual agents
- Can run agents on different machines (future)
- Natural resource cleanup

### Negative

- Higher startup cost per task
- No shared memory (must use files/GitHub)
- More process management complexity

### Mitigations

- Pool of pre-warmed agent processes (future optimization)
- Use files and GitHub for state sharing
- Task Manager handles lifecycle
