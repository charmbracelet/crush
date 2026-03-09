# ADR-004: Spec as Single Source of Truth

## Status

Proposed

## Context

We need a way to define what to build and trace all work back to requirements. Options include:

1. **Code-first** - Tests/specs extracted from code
2. **Spec-first** - Define spec, then generate everything
3. **Hybrid** - Specs and code evolve together

We need traceability from implementation to requirements.

## Decision

**Spec is the single source of truth. All work traces back to it.**

Flow:
```
Spec (YAML)
    ↓
Blueprints (generated docs)
    ↓
Construction (Prisma, OpenAPI, TypeScript)
    ↓
Phases (milestones)
    ↓
Tasks (issues)
    ↓
Implementation (code)
```

When spec changes:
1. Cascade system detects changes
2. Impact analysis determines affected artifacts
3. Downstream artifacts regenerated/flagged
4. Active agents notified

## Consequences

### Positive

- Full traceability (code → task → phase → spec → requirement)
- Automatic updates when requirements change
- Clear contract before implementation
- Easier code reviews (check against spec)

### Negative

- Spec maintenance overhead
- Risk of spec-code drift if cascade fails
- Requires spec quality discipline

### Mitigations

- Spec validation on every change
- Automated cascade with human review gates
- Proof artifacts verify implementation matches spec
