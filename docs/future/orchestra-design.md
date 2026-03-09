# Orchestra: Spec-Driven Multi-Agent Development System

**Status: Future Vision**

This document describes a larger system that uses the dispatch system as a component. The dispatch system handles agent-to-agent communication; Orchestra adds spec-driven workflows, phases, and GitHub integration.

For the dispatch system itself, see [multi-agent-orchestration.md](multi-agent-orchestration.md).

---

## Overview

Orchestra is a spec-driven development system where multiple AI agents collaborate under human oversight to build software. The system uses GitHub as its backbone for persistence, visibility, and collaboration.

**Key Principle**: A single spec document serves as the source of truth. All work—design, construction, phases, tasks, and implementation—flows from and traces back to this spec. When the spec changes, updates cascade downstream automatically.

## Dogfooding Note

This system will be built using its own spec-driven methodology. We will:
1. Write a spec for Orchestra itself
2. Manually follow the spec → design → construction → phases → tasks → implement workflow
3. Use this process to validate and refine the methodology

The spec for Orchestra lives at `specs/orchestra.yaml` and will be updated as we learn.

---

## Architecture

### High-Level Flow

```
┌─────────────────────────────────────────────────────────────────────────┐
│                           SPEC DOCUMENT                                  │
│                      (Single Source of Truth)                            │
│                                                                          │
│  YAML file defining: requirements, entities, API endpoints, constraints │
│  Any phase can propose edits → cascade updates downstream               │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                    ┌───────────────┼───────────────┐
                    ▼               ▼               ▼
            ┌─────────────┐ ┌─────────────┐ ┌─────────────┐
            │   DESIGN    │ │ BLUEPRINTS  │ │CONSTRUCTION │
            │   MANAGER   │ │  (Human     │ │   MANAGER   │
            │             │ │  Readable)  │ │             │
            │ Architecture│ │             │ │ Prisma,     │
            │ ADRs        │ │ README.md   │ │ OpenAPI,    │
            │ Patterns    │ │ Arch.md     │ │ TypeScript  │
            └─────────────┘ │ Domain.md   │ └─────────────┘
                            └─────────────┘
                                    │
                                    ▼
                            ┌─────────────┐
                            │    PHASE    │
                            │   MANAGER   │
                            │             │
                            │ Foundation  │
                            │ Core        │
                            │ Features    │
                            │ Polish      │
                            └──────┬──────┘
                                   │
                                   ▼
                            ┌─────────────┐
                            │    TASK     │
                            │   MANAGER   │
                            │             │
                            │ Granular    │
                            │ work items  │
                            │ w/ traceability
                            └──────┬──────┘
                                   │
                    ┌──────────────┼──────────────┐
                    ▼              ▼              ▼
            ┌─────────────┐ ┌─────────────┐ ┌─────────────┐
            │IMPLEMENTER  │ │IMPLEMENTER  │ │  VERIFIER   │
            │   Agent     │ │   Agent     │ │   Agent     │
            │             │ │             │ │             │
            │ Writes code │ │ Writes code │ │ Tests, lint │
            │ In worktree │ │ In worktree │ │ Proof check │
            └─────────────┘ └─────────────┘ └─────────────┘
```

### The Orchestrator

A single root process (`orchestrator`) owns the entire system:

- Spawns and monitors all managers
- Maintains GitHub connection
- Runs the visibility dashboard (TUI)
- Handles graceful shutdown
- Aggregates activity events

---

## Managers

Managers are long-running processes with specific responsibilities. Each manager:

1. Has an isolated role and context
2. Can spawn child managers or agents
3. Communicates via messages (mirrored to GitHub)
4. Can propose upstream changes (e.g., spec edits)
5. Reports status to the orchestrator

### Manager Types

| Manager | Responsibility | Spawns | GitHub Artifacts |
|---------|---------------|--------|------------------|
| **Spec Manager** | Guardian of the spec. Validates changes, triggers cascade. | — | `specs/*.yaml`, spec change issues |
| **Design Manager** | Creates architecture, ADRs, patterns. Reviews spec changes. | — | ADRs as Discussions, comments |
| **Construction Manager** | Generates technical artifacts (Prisma, OpenAPI, TS). | — | PRs with generated files |
| **Phase Manager** | Breaks work into phases with dependencies. Tracks progress. | Task Manager | Milestones, Project board |
| **Task Manager** | Creates granular tasks from phases. Assigns agents. | Implementer, Verifier agents | Issues (one per task) |

### Manager Interface

```go
type Manager interface {
    // Identity
    ID() string
    Role() string
    
    // Lifecycle
    Start(ctx context.Context) error
    Stop() error
    
    // Communication
    Send(msg Message) error
    Receive(msg Message) error
    
    // Hierarchy
    Spawn(role string) (Agent, error)
    Children() []Agent
    
    // Visibility
    Status() ManagerStatus
}
```

---

## Agents

Agents are short-lived processes that execute specific tasks. Each agent:

1. Works on a single task
2. Operates in an isolated git worktree
3. Has limited scope (implement, verify, review)
4. Reports progress to its parent (Task Manager)
5. Creates proof artifacts before completion

### Agent Types

| Agent | Role | Input | Output |
|-------|------|-------|--------|
| **Implementer** | Writes code | Task definition, design docs | Code, tests, proof artifact |
| **Verifier** | Validates work | Implementation PR | Pass/fail, coverage report |
| **Reviewer** | Human proxy | PR, context | Review comments, approval |

### Agent Interface

```go
type Agent interface {
    // Identity
    ID() string
    Role() string
    
    // Assignment
    Assign(task Task) error
    
    // Lifecycle
    Start(ctx context.Context) error
    Stop() error
    
    // Git isolation
    Branch() string
    Worktree() string
    
    // Visibility
    Status() AgentStatus
    Progress() float64  // 0-100
    Activity() []ActivityEvent
}
```

---

## GitHub as Backbone

GitHub provides persistence, visibility, and collaboration. All orchestra concepts map to GitHub entities.

### Entity Mapping

| Orchestra | GitHub | Purpose |
|-----------|--------|---------|
| Spec | `specs/*.yaml` files | Version controlled source of truth |
| Spec Change Proposal | Draft PR on spec file | Review changes before merge |
| Phase | Milestone | Groups related tasks |
| Task | Issue | Individual work item |
| Task Assignment | Issue Assignee | Who's working on it |
| Agent Work | Branch + PR | `task/T-005/short-description` |
| Agent Progress | Issue Comment | Status updates, blockers |
| Proof Artifact | PR Description Checklist | Evidence of completion |
| Review | PR Review | Approval or changes requested |
| Manager Discussion | GitHub Discussion | Cross-team communication |
| Activity Log | Issue/PR timeline | Visible history |

### Labels

```
# Task labels
task
task/ready
task/in-progress
task/blocked
task/review
task/completed

# Phase labels
phase/1-foundation
phase/2-core
phase/3-features
phase/4-polish

# Agent labels
agent/implementer
agent/verifier
agent/reviewer

# Spec labels
spec-change
spec-impact:breaking
spec-impact:high
spec-impact:low
```

---

## Communication

### Message Types

All communication between managers and agents uses typed messages:

```go
type Message struct {
    ID        string
    From      string    // sender ID
    To        string    // receiver ID or "broadcast"
    Type      MessageType
    Payload   any
    Timestamp time.Time
    
    // GitHub mirror
    GitHubURL string  // Link to issue/PR/comment
}

type MessageType string

const (
    // Task lifecycle
    MsgTaskAssigned    MessageType = "task.assigned"
    MsgTaskStarted     MessageType = "task.started"
    MsgTaskProgress    MessageType = "task.progress"
    MsgTaskBlocked     MessageType = "task.blocked"
    MsgTaskComplete    MessageType = "task.complete"
    
    // Spec changes
    MsgSpecPropose     MessageType = "spec.propose"
    MsgSpecApproved    MessageType = "spec.approved"
    MsgSpecCascade     MessageType = "spec.cascade"
    
    // Review
    MsgReviewRequest   MessageType = "review.request"
    MsgReviewApprove   MessageType = "review.approve"
    MsgReviewChanges   MessageType = "review.changes"
    
    // Clarification
    MsgClarifyRequest  MessageType = "clarify.request"
    MsgClarifyResponse MessageType = "clarify.response"
)
```

### Message Flow Example

```
Spec edited (human)
    │
    ├─► Spec Manager detects change
    │   │
    │   ├─► GitHub: Create Issue "Spec Change: password policy"
    │   │
    │   └─► Message: MsgSpecCascade → Design Manager
    │
    ▼
Design Manager
    │
    ├─► Analyzes impact
    ├─► GitHub: Comment on Issue with impact analysis
    │
    └─► Message: MsgSpecCascade → Construction Manager
    │
    ▼
Construction Manager
    │
    ├─► Regenerates openapi.yaml, types.ts
    ├─► GitHub: Create PR "Update artifacts for password policy"
    │
    └─► Message: MsgSpecCascade → Task Manager
    │
    ▼
Task Manager
    │
    ├─► Identifies affected tasks: T-004, T-005
    ├─► GitHub: Comment on T-004, T-005 issues
    │   "Spec changed. Verify implementation against new criteria."
    │
    └─► Message: MsgTaskBlocked → implementer-02 (if T-005 active)
```

---

## Cascade System

When the spec changes, updates flow downstream:

### Cascade Levels

| Level | Trigger | Action |
|-------|---------|--------|
| **Blueprints** | Spec semantic change | Regenerate architecture.md, domain-model.md, etc. |
| **Construction** | Entity/endpoint change | Regenerate Prisma, OpenAPI, TypeScript |
| **Phases** | Requirement change | Re-evaluate phase scope, add/remove phases |
| **Tasks** | Requirement/acceptance change | Update task definitions, flag affected tasks |
| **Implementation** | Task criteria change | Notify active agents, pause for review |

### Impact Levels

```go
type ImpactLevel string

const (
    ImpactBreaking  ImpactLevel = "breaking"   // API contract change, entity removal
    ImpactHigh      ImpactLevel = "high"       // New requirement, field addition
    ImpactMedium    ImpactLevel = "medium"     // Validation change, constraint update
    ImpactLow       ImpactLevel = "low"        // Documentation, comment change
)
```

### Cascade Rules

```yaml
# Example cascade rules
rules:
  - path: "entities.*.fields"
    impact: high
    affects: [construction, tasks]
    
  - path: "api_endpoints.*"
    impact: high
    affects: [construction, tasks]
    
  - path: "requirements.functional.*.acceptance_criteria"
    impact: medium
    affects: [tasks]
    
  - path: "description"
    impact: low
    affects: [blueprints]
```

---

## Visibility Dashboard

The orchestrator runs a TUI dashboard showing all activity:

### Dashboard Sections

```
┌─────────────────────────────────────────────────────────────────────────┐
│  CRUSH ORCHESTRA                              Project: auth-system      │
│  v1.2.0 · 3 active agents · 2 pending reviews                           │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  MANAGERS                                           Status   Activity    │
│  ─────────────────────────────────────────────────────────────────────  │
│  📋 Spec Manager                                    idle     2m ago     │
│  🏗️  Design Manager                                 idle     1h ago     │
│  🔧 Construction Manager                            idle     3h ago     │
│  📊 Phase Manager                                   active   now        │
│  ✅ Task Manager                                    active   now        │
│                                                                          │
│  ACTIVE AGENTS                                                           │
│  ─────────────────────────────────────────────────────────────────────  │
│                                                                          │
│  🤖 implementer-01                    Task: T-005                       │
│     Branch: task/T-005/auth-login     Status: ⚡ working                │
│     Worktree: worktrees/T-005/        Progress: ████████░░ 80%         │
│     Last: "Writing login handler"     Activity: 30s ago                 │
│                                                                          │
│  🤖 verifier-01                       Task: T-003                       │
│     Branch: task/T-003/user-model     Status: 🔍 reviewing             │
│     Last: "Running test suite"        Activity: 1m ago                  │
│                                                                          │
│  PHASES                                                                  │
│  ─────────────────────────────────────────────────────────────────────  │
│  ✓ Phase 1: Foundation                         100%  3/3 tasks          │
│  ⚡ Phase 2: Core Auth                          65%   5/8 tasks         │
│  ○ Phase 3: OAuth Integration                  0%    0/4 tasks         │
│                                                                          │
│  RECENT ACTIVITY                                                         │
│  ─────────────────────────────────────────────────────────────────────  │
│  2m ago  implementer-01 → T-005: Wrote login handler                   │
│  5m ago  verifier-01 → T-003: All tests passing ✓                      │
│  10m ago task-manager: Assigned T-007 to implementer-02                │
│                                                                          │
│  [a] Activity  [p] Phases  [t] Tasks  [m] Managers  [s] Spec  [q] Quit  │
└─────────────────────────────────────────────────────────────────────────┘
```

### Activity Stream

All events are logged with context:

```go
type ActivityEvent struct {
    ID        string
    Timestamp time.Time
    Source    string    // manager or agent ID
    Type      string    // "task.progress", "spec.change", etc.
    Message   string
    Metadata  map[string]any
    
    // GitHub link
    GitHubURL string
}
```

---

## File Structure

```
project/
├── specs/
│   ├── auth-system.yaml              # Source of truth
│   └── changelogs/
│       └── 2026-02-21-password.yaml  # Change history
│
├── .orchestra/
│   ├── config.yaml                   # Project config
│   │
│   ├── state/                        # Manager/Agent state
│   │   ├── orchestrator.yaml
│   │   ├── managers/
│   │   │   ├── spec-manager.yaml
│   │   │   ├── design-manager.yaml
│   │   │   ├── construction-manager.yaml
│   │   │   ├── phase-manager.yaml
│   │   │   └── task-manager.yaml
│   │   └── agents/
│   │       ├── impl-001.yaml
│   │       └── verifier-001.yaml
│   │
│   ├── messages/                     # Message log
│   │   └── 2026-02-21.jsonl
│   │
│   └── activity/                     # Activity log
│       └── 2026-02-21.jsonl
│
├── blueprints/                       # Human-readable docs
│   └── auth-system/
│       ├── README.md
│       ├── architecture.md
│       ├── domain-model.md
│       ├── api-contracts.md
│       └── user-stories.md
│
├── construction/                     # Generated artifacts
│   └── auth-system/
│       ├── prisma/
│       │   └── schema.prisma
│       ├── api/
│       │   └── openapi.yaml
│       └── contracts/
│           └── types.ts
│
├── phases/                           # Phase definitions
│   └── auth-system/
│       ├── phase-1-foundation.yaml
│       ├── phase-2-core.yaml
│       └── phase-3-oauth.yaml
│
├── tasks/                            # Task definitions
│   ├── T-001.yaml
│   ├── T-002.yaml
│   └── ...
│
└── worktrees/                        # Agent worktrees
    ├── T-005/
    │   └── impl-001/
    └── T-007/
        └── impl-002/
```

---

## Spec Format

The spec is a YAML document with the following structure:

```yaml
id: auth-system
name: User Authentication System
version: 1.2.0
status: active
owner: team-backend
created: 2026-02-21

description: |
  Implement user authentication with JWT tokens and OAuth providers.

goals:
  - Secure user authentication
  - Multiple auth providers
  - Session management

non_goals:
  - Two-factor authentication (future)
  - Enterprise SSO (future)

constraints:
  technical:
    - PostgreSQL database
    - Go backend
    - JWT with RS256
  business:
    - Launch by Q2 2026
    - Pass security audit

requirements:
  functional:
    - id: FR-001
      description: User can register with email and password
      priority: P0
      acceptance_criteria:
        - Email validation (RFC 5322)
        - Password 12+ chars with complexity requirements
        - Confirmation email sent
        - Duplicate email rejected
    
    - id: FR-002
      description: User can login with email and password
      priority: P0
      acceptance_criteria:
        - Returns JWT access token (15 min expiry)
        - Returns refresh token (7 day expiry)
        - Rate limited: 5 attempts per minute

  non_functional:
    - id: NFR-001
      category: security
      description: All passwords hashed with bcrypt
      details: Cost factor 12

entities:
  - name: User
    description: Registered user
    fields:
      - name: id
        type: UUID
        description: Unique identifier
      - name: email
        type: string
        description: User email (unique)
      - name: password_hash
        type: string
        description: Bcrypt hash

api_endpoints:
  - path: /auth/register
    method: POST
    requirement: FR-001
    description: Register new user
    request:
      email: string
      password: string
    response:
      user: User
      message: string
    errors:
      - code: EMAIL_EXISTS
        message: Email already registered

dependencies:
  internal:
    - database-pool
    - email-service
  external:
    - google-oauth
    - github-oauth
```

---

## Implementation Plan

We will build Orchestra using the spec-driven methodology it defines:

### Phase 1: Foundation

1. **Spec System** (this doc)
   - [ ] Define types in `internal/orchestra/types.go`
   - [ ] Spec parser/validator in `internal/orchestra/spec/`
   - [ ] Spec manager skeleton

2. **Core Types**
   - [ ] Role interface
   - [ ] Manager interface
   - [ ] Agent interface
   - [ ] Message types

3. **Orchestrator Skeleton**
   - [ ] Process management
   - [ ] Config loading
   - [ ] Basic state persistence

### Phase 2: GitHub Integration

1. **GitHub Client**
   - [ ] Authentication (App/Token)
   - [ ] Issue/PR operations
   - [ ] Label management
   - [ ] Comment posting

2. **Entity Mapping**
   - [ ] Spec ↔ File
   - [ ] Phase ↔ Milestone
   - [ ] Task ↔ Issue
   - [ ] Work ↔ Branch/PR

### Phase 3: Managers

1. **Spec Manager**
   - [ ] Load/validate spec
   - [ ] Detect changes
   - [ ] Trigger cascade

2. **Design Manager**
   - [ ] Blueprint generation
   - [ ] ADR creation

3. **Construction Manager**
   - [ ] Prisma generator
   - [ ] OpenAPI generator
   - [ ] TypeScript generator

4. **Phase Manager**
   - [ ] Phase CRUD
   - [ ] Progress tracking
   - [ ] Dependency resolution

5. **Task Manager**
   - [ ] Task CRUD
   - [ ] Agent assignment
   - [ ] Status tracking

### Phase 4: Agents

1. **Implementer Agent**
   - [ ] Worktree isolation
   - [ ] Task execution
   - [ ] Progress reporting

2. **Verifier Agent**
   - [ ] Test running
   - [ ] Lint checking
   - [ ] Proof validation

### Phase 5: Visibility

1. **Activity System**
   - [ ] Event logging
   - [ ] Activity stream

2. **Dashboard TUI**
   - [ ] Manager status view
   - [ ] Agent status view
   - [ ] Phase/Task progress
   - [ ] Activity feed

### Phase 6: Cascade

1. **Change Detection**
   - [ ] Spec diff
   - [ ] Impact analysis

2. **Cascade Execution**
   - [ ] Regenerate artifacts
   - [ ] Notify affected agents
   - [ ] Update task status

---

## Open Questions

1. **Agent Process Model**: Should agents be subprocesses, goroutines, or separate CLI invocations?
   - *Leaning toward*: Separate CLI invocations for isolation, managed by Task Manager

2. **GitHub Auth**: GitHub App vs Personal Access Token?
   - *Leaning toward*: Support both, App for teams, PAT for individuals

3. **Spec Change Approval**: Should spec changes require human approval before cascade?
   - *Leaning toward*: Yes, Draft PR → Review → Merge triggers cascade

4. **Parallel Agents**: How many agents can work simultaneously?
   - *Leaning toward*: Configurable per project, default 3

5. **Agent Model Selection**: Can different agents use different LLM models?
   - *Leaning toward*: Yes, per-role model configuration

---

## References

- [OpenSpec](https://github.com/Fission-AI/OpenSpec) - Delta specs, filesystem-based
- [GitHub Spec Kit](https://github.com/github/spec-kit) - Constitution-guided, slash commands
- [cc-sdd](https://github.com/gotalab/cc-sdd) - EARS syntax, phase gates
- [Shotgun](https://github.com/shotgun-sh/shotgun) - Research-first, codebase indexing
- [Liatrio SDD](https://github.com/liatrio-labs/spec-driven-workflow) - Proof artifacts

---

## Changelog

| Version | Date | Changes |
|---------|------|---------|
| 0.1.0 | 2026-02-21 | Initial design document |
