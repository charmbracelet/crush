# ADR-002: Hierarchical Manager/Agent Spawning

## Status

Proposed

## Context

Orchestra needs to coordinate multiple AI processes. Options include:

1. **Flat** - All agents equal, no hierarchy
2. **Hierarchical** - Managers spawn managers spawn agents
3. **Peer-to-peer** - Agents communicate directly

We need clear responsibility boundaries, traceability, and isolation.

## Decision

**Use hierarchical spawning: Orchestrator → Managers → Agents.**

```
Orchestrator (root)
├── Spec Manager
├── Design Manager
├── Construction Manager
├── Phase Manager
│   └── Task Manager
│       ├── Implementer Agent
│       └── Verifier Agent
```

Each level:
- Has isolated context and responsibility
- Can spawn children
- Tracks child status
- Reports to parent

## Consequences

### Positive

- Clear responsibility boundaries
- Context isolation per role
- Traceable parent-child relationships
- Natural progress aggregation (children → parent)
- Easier debugging (know which manager owns what)

### Negative

- More complex process management
- Potential for deep hierarchies
- Communication latency through levels

### Mitigations

- Limit hierarchy depth (max 3 levels)
- Broadcast messages for cross-tree communication
- Direct GitHub sync bypasses hierarchy for visibility
