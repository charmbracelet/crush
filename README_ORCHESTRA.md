# Orchestra Discovery Space

This repository is the **research and design workspace** for [Crush Orchestra](./design/README.md) — a spec-driven multi-agent development system being developed by [Charmbracelet](https://charm.land).

## What is Orchestra?

Orchestra is a system where multiple AI agents collaborate under human oversight to build software. It uses:

- **Specs as single source of truth** — YAML documents define what to build
- **Hierarchical managers** — Coordinators that spawn and supervise agents
- **GitHub as backbone** — Issues, PRs, and Milestones for visibility
- **Git worktree isolation** — Each agent works in isolation
- **Cascade system** — Spec changes flow downstream automatically

See [design/README.md](./design/README.md) for the full vision.

## Repository Structure

```
orchestra/
├── research/           # Analysis of existing frameworks and patterns
│   ├── frameworks/     # Deep dives into 8 spec-driven AI systems
│   └── patterns/       # Extracted patterns to adopt
│
├── design/             # Orchestra design documents
│   ├── README.md       # What is Orchestra?
│   ├── orchestra.md    # Full system design
│   └── adr/            # Architecture Decision Records
│
├── specs/              # Spec examples and Orchestra's own spec
│   ├── orchestra.yaml  # Orchestra defined using Orchestra
│   └── examples/       # Example specs for other systems
│
└── archive/            # Superseded exploration documents
```

## Research

We analyzed 8 existing spec-driven AI frameworks:

| Framework | Key Innovation |
|-----------|----------------|
| [OpenSpec](./research/frameworks/openspec.md) | Delta specs with ADDED/MODIFIED/REMOVED |
| [GitHub Spec Kit](./research/frameworks/github-spec-kit.md) | Constitution-guided development |
| [cc-sdd](./research/frameworks/cc-sdd.md) | EARS requirements syntax |
| [AI-DLC](./research/frameworks/ai-dlc.md) | Role-based "hats" system |
| [specs.md](./research/frameworks/specs-md.md) | Pluggable workflow phases |
| [Autospec](./research/frameworks/autospec.md) | Context isolation (80% cost reduction) |
| [Shotgun](./research/frameworks/shotgun.md) | Research-first with tree-sitter |
| [Liatrio SDD](./research/frameworks/liatrio-sdd.md) | Proof artifacts |

See [research/README.md](./research/README.md) for synthesis and recommendations.

## Design Status

Orchestra is in **active design**. Key decisions documented in ADRs:

- [ADR-001: GitHub as Backbone](./design/adr/001-github-backbone.md)
- [ADR-002: Hierarchical Spawning](./design/adr/002-hierarchical-spawning.md)
- [ADR-003: Agent Process Model](./design/adr/003-agent-process-model.md)
- [ADR-004: Spec as Single Source of Truth](./design/adr/004-spec-single-source.md)
- [ADR-005: Git Worktree Isolation](./design/adr/005-worktree-isolation.md)

## The Crush CLI

This workspace also contains the [Crush CLI](https://github.com/charmbracelet/crush) codebase — the terminal-based AI assistant that Orchestra will be built into. The CLI is production-ready; Orchestra is the new capability being designed.

---

Part of [Charm](https://charm.land).
