# Git-Backed Orchestration Design

## Overview

Use git worktrees and branches as the foundational infrastructure for multi-agent orchestration. Each agent works in an isolated worktree, git history becomes the communication/audit layer, and tickets are git branches.

---

## Why Git-Backed?

| Problem | Git Solution |
|---------|--------------|
| Agent isolation | Worktrees (each agent gets own directory) |
| State tracking | Commits = checkpoints |
| Communication | Push/pull/merge between worktrees |
| Tickets/tasks | Branches per task |
| Audit trail | Full git history |
| Conflict resolution | Standard merge/rebase workflow |
| Rollback | `git reset` / `git revert` |
| Review process | Pull requests between agent branches |
| Parallel work | Multiple worktrees from same base |

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         ORCHESTRATION REPO                               │
│  .orchestra/                                                             │
│  ├── config.yaml          # Team configuration                           │
│  ├── ledgers/             # Task & progress state                        │
│  │   ├── task-ledger.json                                             │
│  │   └── progress-ledger.json                                          │
│  ├── agents/              # Agent state tracking                         │
│  │   ├── planner.yaml                                                  │
│  │   ├── coder.yaml                                                    │
│  │   └── reviewer.yaml                                                 │
│  └── logs/                # Orchestration logs                           │
│                                                                          │
│  .git/                                                                   │
│  └── branches: main, task/auth-api, task/auth-api/planner, etc.         │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
            ┌───────────────────────┼───────────────────────┐
            ▼                       ▼                       ▼
   ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
   │  worktree/      │    │  worktree/      │    │  worktree/      │
   │  planner/       │    │  coder/         │    │  reviewer/      │
   │                 │    │                 │    │                 │
   │  branch:        │    │  branch:        │    │  branch:        │
   │  task/auth/     │    │  task/auth/     │    │  task/auth/     │
   │  planner        │    │  coder          │    │  reviewer       │
   │                 │    │                 │    │                 │
   │  [PLANNER       │    │  [CODER         │    │  [REVIEWER      │
   │   working here] │    │   working here] │    │   working here] │
   └─────────────────┘    └─────────────────┘    └─────────────────┘
```

---

## Git Workflow for Agents

### Branch Strategy

```
main
│
└── task/auth-api (task branch - aggregates all agent work)
    │
    ├── task/auth-api/planner (planner's work)
    │   └── commits: plan updates, fact discoveries
    │
    ├── task/auth-api/coder (coder's work)
    │   └── commits: implementation changes
    │
    ├── task/auth-api/reviewer (reviewer's work)
    │   └── commits: review notes, requested changes
    │
    └── task/auth-api/tester (tester's work)
        └── commits: test files, test results
```

### Worktree Lifecycle

```go
type WorktreeManager struct {
    repoRoot    string
    worktreeDir string // ~/.cache/crush/worktrees/
}

// Create isolated workspace for agent
func (m *WorktreeManager) CreateAgentWorktree(agent *Agent, taskBranch string) (*Worktree, error) {
    worktreePath := filepath.Join(m.worktreeDir, taskBranch, agent.Name)

    // Create task branch if not exists
    m.git("branch", "-c", taskBranch, taskBranch+"/"+agent.Name)

    // Create worktree
    m.git("worktree", "add", worktreePath, taskBranch+"/"+agent.Name)

    return &Worktree{
        Path:      worktreePath,
        Branch:    taskBranch + "/" + agent.Name,
        AgentName: agent.Name,
    }, nil
}

// Agent done - merge work back
func (m *WorktreeManager) FinalizeWorktree(w *Worktree) error {
    // Push agent's commits
    m.gitInWorktree(w, "push", "origin", w.Branch)

    // Create PR or merge to task branch
    m.git("checkout", taskBranch)
    m.git("merge", w.Branch, "--no-ff", "-m", fmt.Sprintf("Merge %s's work", w.AgentName))

    // Remove worktree
    m.git("worktree", "remove", w.Path)

    return nil
}
```

---

## Tickets as Branches

### Ticket Structure

```go
type Ticket struct {
    ID          string    `json:"id"`           // T-001
    Title       string    `json:"title"`
    Description string    `json:"description"`
    Branch      string    `json:"branch"`       // task/auth-api
    Status      string    `json:"status"`       // backlog, active, review, done
    Assignee    string    `json:"assignee"`     // agent name or "unassigned"
    Priority    int       `json:"priority"`     // P0-P3
    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`

    // Stored in branch description or .orchestra/tickets/
}

// Create ticket = create branch with metadata
func (m *WorktreeManager) CreateTicket(title, description string) (*Ticket, error) {
    ticketID := generateTicketID() // T-001, T-002, etc.
    branchName := fmt.Sprintf("ticket/%s-%s", ticketID, slugify(title))

    // Create orphan branch with ticket metadata
    m.git("checkout", "--orphan", branchName)

    // Store ticket metadata as git notes or in .orchestra/tickets/T-001.yaml
    ticket := &Ticket{
        ID:          ticketID,
        Title:       title,
        Description: description,
        Branch:      branchName,
        Status:      "backlog",
        CreatedAt:   time.Now(),
    }
    m.saveTicketMetadata(ticket)

    // Return to main
    m.git("checkout", "main")

    return ticket, nil
}
```

### Ticket States → Git Operations

| Ticket State | Git Operation |
|--------------|---------------|
| `backlog` | Branch exists, no worktrees |
| `active` | Agent worktree created from branch |
| `in_progress` | Agent has commits on branch |
| `review` | PR created from agent branch to task branch |
| `done` | Merged to task branch, worktree removed |
| `blocked` | Marked in ticket metadata |

---

## Commits as Messages

### Commit Convention

```
<type>(<agent>): <subject>

[optional body]

Agent: <agent-name>
Task: <task-id>
Turn: <turn-number>

<footer>
```

### Examples

```bash
# Planner discovers a fact
git commit -m "fact(planner): Project uses Gin framework

Discovered during initial codebase analysis.

Agent: planner
Task: T-001
Turn: 3"

# Coder implements feature
git commit -m "feat(coder): Implement JWT validation middleware

- Add ValidateToken function in internal/auth/jwt.go
- Add middleware in internal/middleware/auth.go
- Add config for JWT secret

Agent: coder
Task: T-001
Turn: 7"

# Reviewer requests changes
git commit -m "review(reviewer): Request changes on auth middleware

Missing error handling for expired tokens.
Line 45: Should return 401, not 500.

Agent: reviewer
Task: T-001
Turn: 12"

# Handoff communication
git commit -m "handoff(coder): Ready for review

Completed login endpoint implementation.
Ready for code review.

Agent: coder -> reviewer
Task: T-001
Turn: 10"
```

### Parsing Commit Trail

```go
type CommitMessage struct {
    Type     string    // feat, fix, fact, review, handoff
    Agent    string    // who made it
    Subject  string    // what happened
    Body     string    // details
    TaskID   string    // associated task
    Turn     int       // orchestration turn
    HandoffTo string   // if handoff, who to
}

func ParseCommit(msg string) *CommitMessage {
    // Parse conventional commit format with agent metadata
    // ...
}

// Get agent communication history
func (m *WorktreeManager) GetAgentHistory(taskBranch string) ([]CommitMessage, error) {
    commits, _ := m.gitLog(taskBranch, "--format=%B%n---COMMIT---")

    var messages []CommitMessage
    for _, commit := range strings.Split(commits, "---COMMIT---") {
        if msg := ParseCommit(commit); msg != nil {
            messages = append(messages, *msg)
        }
    }
    return messages, nil
}
```

---

## Agent Workspace Structure

### Worktree Layout

```
~/.cache/crush/worktrees/
└── task-auth-api/
    ├── planner/
    │   ├── .git          # Points to main repo
    │   ├── .orchestra/
    │   │   └── agent-state.yaml
    │   └── (project files - planner's view)
    │
    ├── coder/
    │   ├── .git
    │   ├── .orchestra/
    │   │   └── agent-state.yaml
    │   └── (project files - coder's view)
    │
    └── reviewer/
        ├── .git
        ├── .orchestra/
        │   └── agent-state.yaml
        └── (project files - reviewer's view)
```

### Agent State File

```yaml
# .orchestra/agent-state.yaml (in each worktree)
agent:
  name: coder
  role: Software Developer
  model: claude-3.5-sonnet

task:
  id: T-001
  title: Implement REST API for user authentication
  branch: task/auth-api

state:
  status: active
  turn: 7
  started_at: 2024-01-15T10:30:00Z
  last_activity: 2024-01-15T10:45:00Z

work:
  commits: 3
  files_changed: 5
  lines_added: 247
  lines_removed: 12

handoff:
  from: planner
  instruction: "Implement /auth/login endpoint based on the plan"
```

---

## Coordination via Git

### Scenario: Planner → Coder Handoff

```
1. PLANNER (in worktree/planner/)
   └── git commit -m "handoff(planner): Plan ready for implementation"
   └── git push origin task/auth-api/planner

2. ORCHESTRATOR
   └── Detects new commit on planner branch
   └── Parses handoff message, extracts target: coder
   └── Updates ledger: next_agent = coder
   └── Creates worktree for coder (if not exists)
   └── Notifies coder with planner's commit as context

3. CODER (in worktree/coder/)
   └── git pull origin task/auth-api (gets planner's work)
   └── Starts implementation
   └── git commit -m "feat(coder): Implement login endpoint"
```

### Scenario: Parallel Work + Merge

```
           task/auth-api (integration branch)
          /              |              \
   planner/         coder/          tester/
      │                │                │
   plan.yml        auth.go          auth_test.go
      │                │                │
      └────────────────┴────────────────┘
                         │
                    merge to task/auth-api
```

```go
// Merge agent work to task branch
func (m *WorktreeManager) MergeAgentWork(taskBranch string, agentName string) error {
    agentBranch := taskBranch + "/" + agentName

    // Switch to task branch
    m.git("checkout", taskBranch)

    // Merge with strategy
    _, err := m.git("merge", agentBranch,
        "--no-ff",
        "-m", fmt.Sprintf("Merge %s's work into %s", agentName, taskBranch),
    )

    if err != nil {
        // Handle conflicts - notify orchestrator
        return &MergeConflictError{
            Branch: agentBranch,
            Files:  m.getConflictedFiles(),
        }
    }

    return nil
}
```

### Scenario: Conflict Resolution

```
1. CODER and REVIEWER both modify same file
2. Merge fails with conflict
3. ORCHESTRATOR detects conflict

   ┌─────────────────────────────────────────┐
   │ ⚠️ MERGE CONFLICT DETECTED              │
   │                                         │
   │ File: internal/handlers/auth.go         │
   │                                         │
   │ CONFLICTING AGENTS:                     │
   │ ┌─────────────┐ ┌─────────────┐        │
   │ │ 💻 CODER    │ │ 🔍 REVIEWER │        │
   │ │ +45 -12     │ │ +3 -1       │        │
   │ └─────────────┘ └─────────────┘        │
   │                                         │
   │ [Auto-resolve] [Manual] [Escalate]     │
   └─────────────────────────────────────────┘

4. Resolution options:
   a) Auto-resolve: LLM merges the conflict
   b) Manual: User resolves in UI
   c) Escalate: Create resolution task for senior agent
```

---

## Git-Backed Ledger

### Task Ledger in Git

```yaml
# .orchestra/ledgers/task-ledger.yaml
version: 1
task:
  id: T-001
  title: Implement REST API for user authentication
  created_at: 2024-01-15T10:00:00Z
  created_by: user

facts:
  - id: F001
    content: "Project uses Go 1.21 with Gin framework"
    discovered_by: planner
    discovered_at: 2024-01-15T10:15:00Z
    commit: a1b2c3d

  - id: F002
    content: "PostgreSQL database already configured"
    discovered_by: planner
    discovered_at: 2024-01-15T10:16:00Z
    commit: d4e5f6g

plan:
  - id: P1
    description: "Setup project structure"
    assignee: planner
    status: completed
    commit: g7h8i9j

  - id: P2
    description: "Design user schema"
    assignee: planner
    status: completed
    commit: k1l2m3n

  - id: P3
    description: "Implement auth endpoints"
    assignee: coder
    status: in_progress
    started_at: 2024-01-15T10:30:00Z

  - id: P4
    description: "Review implementation"
    assignee: reviewer
    status: pending

  - id: P5
    description: "Write tests"
    assignee: tester
    status: pending
```

### Progress Ledger in Git

```yaml
# .orchestra/ledgers/progress-ledger.yaml
version: 1
current:
  agent: coder
  turn: 12
  started_at: 2024-01-15T10:30:00Z

history:
  - turn: 12
    agent: coder
    action: commit
    message: "feat(coder): Implement login endpoint"
    commit: x1y2z3a
    timestamp: 2024-01-15T10:45:00Z

  - turn: 11
    agent: planner
    action: handoff
    target: coder
    message: "Plan ready, starting implementation"
    commit: b4c5d6e
    timestamp: 2024-01-15T10:29:00Z

  - turn: 10
    agent: planner
    action: fact
    content: "JWT library already in dependencies"
    commit: f7g8h9i
    timestamp: 2024-01-15T10:25:00Z

stalls:
  count: 1
  max: 3
  last_stall:
    turn: 8
    reason: "Waiting for external API documentation"
    resolved: true
```

---

## CLI Commands

```bash
# Initialize orchestra in repo
crush orchestra init

# Create a task/ticket
crush orchestra task create "Implement auth API" --priority=P1

# Start agents on a task
crush orchestra start T-001 --team=dev-team.yaml

# View status
crush orchestra status T-001

# View agent worktrees
crush orchestra worktrees

# Merge specific agent's work
crush orchestra merge T-001 coder

# Resolve conflicts
crush orchestra resolve T-001 --auto

# View history/timeline
crush orchestra log T-001

# Cleanup completed task
crush orchestra cleanup T-001
```

---

## Benefits

### 1. Full Audit Trail
Every action is a git commit:
- Who did what (agent)
- When (timestamp)
- Why (commit message)
- What changed (diff)

### 2. Native Rollback
```bash
# Undo last agent action
git revert HEAD

# Reset to before coder's changes
git reset --hard HEAD~3

# Branch off from any point
git checkout -b recovery-point HEAD~5
```

### 3. Parallel Work
Multiple agents work simultaneously in separate worktrees without conflicts.

### 4. Natural Review Process
```bash
# See what coder changed
git diff task/auth-api..task/auth-api/coder

# Review as PR
gh pr create --base task/auth-api --head task/auth-api/coder
```

### 5. External Integration
- GitHub/GitLab issues sync with tickets
- CI/CD on agent branches
- Standard git tooling works

### 6. Persistence & Recovery
If orchestrator crashes:
```bash
# Resume from last state
crush orchestra resume T-001
# Reads ledger from .orchestra/ledgers/
# Recreates worktrees from branches
```

---

## Implementation

### Core Types

```go
type GitBackedOrchestrator struct {
    repoRoot      string
    worktreeMgr   *WorktreeManager
    ledgerManager *LedgerManager
    ticketManager *TicketManager
}

type Worktree struct {
    Path      string
    Branch    string
    AgentName string
    TaskID    string
    Status    WorktreeStatus
}

type Ticket struct {
    ID          string
    Title       string
    Branch      string
    Status      TicketStatus
    Assignee    string
    Priority    int
    Metadata    map[string]any
}

type AgentCommit struct {
    Hash      string
    Agent     string
    Type      string // feat, fix, handoff, fact, review
    Subject   string
    Body      string
    TaskID    string
    Turn      int
    Timestamp time.Time
}
```

### Git Operations

```go
func (g *GitBackedOrchestrator) InitializeTask(task *Ticket) error {
    // Create task branch
    g.git("checkout", "-b", task.Branch)

    // Create .orchestra directory
    os.MkdirAll(".orchestra/ledgers", 0755)
    os.MkdirAll(".orchestra/agents", 0755)

    // Initialize ledgers
    g.ledgerManager.InitTaskLedger(task)
    g.ledgerManager.InitProgressLedger(task)

    // Commit orchestra setup
    g.git("add", ".orchestra")
    g.git("commit", "-m", fmt.Sprintf("chore(orchestra): Initialize task %s", task.ID))

    return nil
}

func (g *GitBackedOrchestrator) SpawnAgent(agent *Agent, task *Ticket) (*Worktree, error) {
    worktree, err := g.worktreeMgr.CreateAgentWorktree(agent, task.Branch)
    if err != nil {
        return nil, err
    }

    // Write agent state file
    statePath := filepath.Join(worktree.Path, ".orchestra", "agent-state.yaml")
    g.writeAgentState(statePath, agent, task)

    return worktree, nil
}

func (g *GitBackedOrchestrator) RecordAgentAction(agent string, action AgentAction) error {
    // Commit the action
    commitMsg := formatCommitMessage(agent, action)
    g.gitInWorktree(agent, "add", "-A")
    g.gitInWorktree(agent, "commit", "-m", commitMsg)

    // Update progress ledger
    g.ledgerManager.RecordTurn(agent, action)

    return nil
}

func (g *GitBackedOrchestrator) SyncToTask(taskID string, agentName string) error {
    taskBranch := g.getTaskBranch(taskID)
    agentBranch := taskBranch + "/" + agentName

    // Fetch agent's latest
    g.git("fetch", ".", agentBranch)

    // Merge into task branch
    g.git("checkout", taskBranch)
    _, err := g.git("merge", agentBranch, "--no-ff")

    return err
}
```

---

## Updated Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         CRUSH ORCHESTRATOR                               │
│                                                                          │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐    │
│  │   Router    │  │   Ledger    │  │  Worktree   │  │   Ticket    │    │
│  │  (AI-driven)│  │  Manager    │  │  Manager    │  │  Manager    │    │
│  └─────────────┘  └─────────────┘  └─────────────┘  └─────────────┘    │
│         │               │                 │               │             │
│         └───────────────┴─────────────────┴───────────────┘             │
│                                  │                                      │
│                         ┌────────▼────────┐                             │
│                         │   Git Backend   │                             │
│                         │  (orchestrator) │                             │
│                         └─────────────────┘                             │
└─────────────────────────────────────────────────────────────────────────┘
                                   │
         ┌─────────────────────────┼─────────────────────────┐
         ▼                         ▼                         ▼
  ┌──────────────┐         ┌──────────────┐         ┌──────────────┐
  │   worktree   │         │   worktree   │         │   worktree   │
  │   /planner   │         │   /coder     │         │   /reviewer  │
  │              │         │              │         │              │
  │ branch:      │         │ branch:      │         │ branch:      │
  │ task/T-001/  │         │ task/T-001/  │         │ task/T-001/  │
  │ planner      │         │ coder        │         │ reviewer     │
  │              │         │              │         │              │
  │ git commits  │         │ git commits  │         │ git commits  │
  │ = messages   │         │ = messages   │         │ = messages   │
  └──────────────┘         └──────────────┘         └──────────────┘
         │                         │                         │
         └─────────────────────────┴─────────────────────────┘
                                   │
                         merge to task/T-001
                                   │
                                   ▼
                            ┌──────────────┐
                            │  task/T-001  │
                            │ (integration)│
                            └──────────────┘
```

---

## Next Steps

1. **Implement WorktreeManager** - Create/remove worktrees for agents
2. **Implement TicketManager** - Create tickets as branches
3. **Implement GitBackedOrchestrator** - Coordinate via git operations
4. **Update visualization** - Show git branch/tree in UI
5. **Handle conflicts** - Auto-resolve with LLM, manual escalation
