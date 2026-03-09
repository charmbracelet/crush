# Orchestra: Spec-Driven AI Framework Research

**Status: Research (Complete)**

Research on spec-driven AI frameworks that informed the Orchestra vision. See [future/orchestra-design.md](../future/orchestra-design.md) for the design.

---

## Overview

Research conducted February 2026 on spec-driven AI coding frameworks. This document summarizes 8 frameworks and extracts key patterns for Orchestra.

## Frameworks Analyzed

### 1. GitHub Spec Kit

**Repository**: github/github/spec-kit

**Key Innovation**: Constitution-guided development with slash commands

**Spec Format**: Markdown templates

**Features**:
- `/spec` slash command creates structured spec
- Constitution file guides AI behavior
- Integration with GitHub Copilot
- Human-in-the-loop at every step

**Patterns to Adopt**:
- Constitution file for AI behavior constraints
- Slash commands for spec operations
- Deep GitHub integration

---

### 2. OpenSpec

**Repository**: Fission-AI/OpenSpec

**Key Innovation**: Delta specs with ADDED/MODIFIED/REMOVED annotations

**Spec Format**: Markdown + Gherkin

**Features**:
- Filesystem-based specs
- Delta tracking for changes
- Gherkin acceptance criteria
- Automated test generation

**Patterns to Adopt**:
- Delta spec format for change tracking
- Filesystem as storage (YAML files)
- Gherkin-style acceptance criteria

---

### 3. cc-sdd (Spec-Driven Development)

**Repository**: gotalab/cc-sdd

**Key Innovation**: EARS requirements syntax, 8 AI provider support

**Spec Format**: Markdown + JSON state

**Features**:
- EARS syntax (Easy Approach to Requirements Syntax)
- Phase gates with human approval
- 8 AI providers supported
- State machine for workflow

**Patterns to Adopt**:
- EARS syntax for requirements
- Phase gates before proceeding
- Multi-provider support

---

### 4. AI-DLC (AI Development Lifecycle)

**Repository**: ai-dlc/ai-dlc

**Key Innovation**: Hats/roles system, backpressure over prescription

**Spec Format**: Markdown + YAML frontmatter

**Features**:
- "Hats" define agent roles
- Backpressure: AI asks for clarification vs guessing
- Three flows: Simple, FIRE, AI-DLC
- Configurable verbosity

**Patterns to Adopt**:
- Role-based agent system
- Backpressure (ask vs assume)
- Multiple workflow options

---

### 5. specs.md

**Repository**: specs-md/specs

**Key Innovation**: Three pluggable flows with unified spec format

**Spec Format**: Markdown

**Features**:
- Simple Flow: spec → implement
- FIRE Flow: spec → research → implement
- AI-DLC Flow: full lifecycle
- Pluggable architecture

**Patterns to Adopt**:
- Pluggable workflow phases
- Research step before implementation
- Unified spec format across flows

---

### 6. Autospec

**Repository**: autospec/autospec

**Key Innovation**: Context isolation for 80% cost reduction

**Spec Format**: YAML-first

**Features**:
- Aggressive context pruning
- Only relevant spec sections in prompt
- 80% cost reduction claimed
- Incremental spec building

**Patterns to Adopt**:
- Context isolation per role/phase
- Only include relevant spec sections
- Cost-aware architecture

---

### 7. Shotgun

**Repository**: shotgun-sh/shotgun

**Key Innovation**: Research-first, tree-sitter codebase indexing

**Spec Format**: Markdown

**Features**:
- Tree-sitter for codebase understanding
- Research phase before planning
- Generates understanding documents
- Incremental spec refinement

**Patterns to Adopt**:
- Research phase as first step
- Codebase indexing for context
- Understanding documents

---

### 8. Liatrio SDD

**Repository**: liatrio-labs/spec-driven-workflow

**Key Innovation**: Proof artifacts, context verification markers

**Spec Format**: Markdown

**Features**:
- Proof artifacts required for completion
- Context verification markers in specs
- Automated verification pipeline
- Enterprise-focused

**Patterns to Adopt**:
- Proof artifacts (evidence of completion)
- Verification markers in specs
- Automated completion verification

---

## Pattern Synthesis

### Core Patterns

| Pattern | Source | Orchestra Application |
|---------|--------|----------------------|
| **Delta Specs** | OpenSpec | Track spec changes with ADDED/MODIFIED/REMOVED |
| **EARS Syntax** | cc-sdd | Structured requirement format with "When... shall..." |
| **Proof Artifacts** | Liatrio | Evidence required before task marked complete |
| **Context Isolation** | Autospec | Only relevant spec sections per agent |
| **Research Phase** | Shotgun | Understand codebase before implementing |
| **Role System** | AI-DLC | Managers and agents have defined roles |
| **Phase Gates** | cc-sdd | Human approval before phase transitions |
| **Constitution** | GitHub Spec Kit | Constraints file for AI behavior |

### Spec Format Recommendation

Based on research, Orchestra should use:

```yaml
# YAML-first for machine readability
id: my-feature
version: 1.0.0

requirements:
  functional:
    - id: FR-001
      description: "When user submits form, system shall validate input"
      # EARS-style: When [trigger], system shall [behavior]
      acceptance_criteria:
        - Given valid input, When submitted, Then accepted
        # Gherkin-style criteria
      priority: P0
      
non_functional:
  - id: NFR-001
    category: performance
    description: "Response time shall be under 200ms"
    verification: automated_load_test
    # Proof artifact requirement

entities:
  - name: User
    fields:
      - name: email
        type: string
        constraints: [unique, email_format]
        # Delta-aware field definitions
```

### Workflow Recommendation

```
1. RESEARCH (Shotgun)
   └── Understand codebase with tree-sitter indexing
   
2. SPEC (YAML-first)
   └── Define requirements with EARS syntax
   
3. DESIGN (AI-DLC roles)
   └── Architecture, ADRs, blueprints
   
4. CONSTRUCTION (Autospec context isolation)
   └── Generate Prisma, OpenAPI, TypeScript
   
5. PHASES (cc-sdd gates)
   └── Break into phases with human approval
   
6. TASKS (OpenSpec deltas)
   └── Granular tasks with delta tracking
   
7. IMPLEMENT (GitHub Spec Kit integration)
   └── Agents work in worktrees, PRs for review
   
8. VERIFY (Liatrio proof artifacts)
   └── Evidence required for completion
```

### Cost Optimization

From Autospec research:
- Full spec in every prompt = expensive
- Only relevant sections = 80% cost reduction
- Implementation: Each manager/agent receives filtered spec

```go
// Pseudocode
func GetContextForRole(spec Spec, role Role) SpecSection {
    switch role {
    case "design-manager":
        return spec.Requirements + spec.Entities
    case "construction-manager":
        return spec.Entities + spec.APIEndpoints
    case "implementer-agent":
        return spec.GetTaskRelevantSections(taskID)
    }
}
```

## Open Questions

1. **Spec Versioning**: How to handle spec version history? Git tags? Changelog files?
2. **Partial Specs**: Can specs reference other specs? Import/include mechanism?
3. **Spec Validation**: JSON Schema validation? Custom validators?
4. **Conflict Resolution**: When two phases propose conflicting spec changes?

## References

- [OpenSpec](https://github.com/Fission-AI/OpenSpec)
- [GitHub Spec Kit](https://github.com/github/spec-kit)
- [cc-sdd](https://github.com/gotalab/cc-sdd)
- [Shotgun](https://github.com/shotgun-sh/shotgun)
- [Liatrio SDD](https://github.com/liatrio-labs/spec-driven-workflow)
- [Autospec](https://github.com/autospec/autospec)
- [AI-DLC](https://github.com/ai-dlc/ai-dlc)
- [specs.md](https://github.com/specs-md/specs)

## Changelog

| Version | Date | Changes |
|---------|------|---------|
| 0.1.0 | 2026-02-21 | Initial research summary |
