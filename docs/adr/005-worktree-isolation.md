# ADR-005: Git Worktree Isolation

## Status

Proposed

## Context

Multiple agents may work on different tasks simultaneously. Options include:

1. **Single branch** - All agents work on same branch
2. **Multiple branches** - Each agent has own branch, shared worktree
3. **Git worktrees** - Each agent has own branch AND own worktree

We need isolation, conflict avoidance, and parallel work.

## Decision

**Each agent works in an isolated git worktree.**

Structure:
```
project/
├── (main worktree)
├── worktrees/
│   ├── T-005/
│   │   └── impl-001/    # Implementer agent for task T-005
│   └── T-007/
│       └── impl-002/    # Implementer agent for task T-007
```

Branch naming: `task/{task-id}/{agent-id}`

## Consequences

### Positive

- True parallel work without conflicts
- Each agent has clean working directory
- Easy cleanup (delete worktree)
- Independent git operations per agent
- Can review agent's work in isolation

### Negative

- Disk space usage (multiple checkouts)
- Worktree management complexity
- Shared files may cause issues (rare)

### Mitigations

- Clean up worktrees after task completion
- Limit concurrent agents (configurable, default 3)
- Document worktree lifecycle in agent
