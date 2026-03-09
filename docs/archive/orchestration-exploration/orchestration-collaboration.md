# Collaborative Orchestration with GitHub

## Overview

Extend the orchestration system to support multiple human developers collaborating with AI agents. GitHub becomes the shared workspace and coordination layer.

---

## The Vision

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         GITHUB REPO                                      │
│                                                                          │
│  Issues ──── Tasks/Tickets (T-001, T-002, ...)                          │
│  Projects ── Kanban boards (shared visualization)                       │
│  Branches ── Agent workspaces (task/T-001/coder, task/T-002/reviewer)   │
│  PRs ────── Agent handoffs, reviews, merges                             │
│  Actions ── Trigger agents, run tests, CI/CD                            │
│  Webhooks ─ Real-time notifications to all developers                   │
│                                                                          │
│  .orchestra/                                                             │
│  ├── ledgers/           # Shared task state                             │
│  ├── agents/            # Agent configurations                          │
│  └── sessions/          # Active session metadata                       │
└─────────────────────────────────────────────────────────────────────────┘
           │                              │
           │                              │
           ▼                              ▼
   ┌───────────────┐              ┌───────────────┐
   │  DEVELOPER A  │              │  DEVELOPER B  │
   │  (You)        │              │  (Bangladesh) │
   │               │              │               │
   │  crush CLI    │              │  crush CLI    │
   │  Local agents │              │  Local agents │
   │  Sees all     │              │  Sees all     │
   └───────────────┘              └───────────────┘
```

---

## GitHub-Native Architecture

### 1. Issues as Tickets

```yaml
# GitHub Issue #42 becomes Ticket T-042
---
title: "Implement user authentication API"
labels: [task, P1, backend]
assignees: [coder-agent]  # Agent assignment
milestone: "v1.0"
---

## Task Description
Implement REST API endpoints for user authentication...

## Acceptance Criteria
- [ ] POST /auth/login
- [ ] POST /auth/register
- [ ] POST /auth/refresh

## Agent Assignment
- **Planner**: @planner-agent
- **Coder**: @coder-agent
- **Reviewer**: @reviewer-agent

## Progress
🤖 Coder is working on this (branch: `task/T-042/coder`)
```

### 2. Branches as Agent Workspaces

```
main
├── task/T-042-auth-api/           # Task integration branch
│   ├── task/T-042/planner         # Planner's analysis
│   ├── task/T-042/coder           # Coder's implementation
│   ├── task/T-042/reviewer        # Reviewer's notes
│   └── task/T-042/tester          # Tester's tests
│
├── task/T-043-payment-system/     # Another task (Developer B working)
│   ├── task/T-043/planner
│   └── task/T-043/coder
│
└── dev/alice/feature-x            # Developer A's own work
```

### 3. Pull Requests as Handoffs

```markdown
# PR: Implement auth endpoints (T-042)

🤖 **Agent**: coder
📋 **Task**: #42
🔀 **Handoff to**: reviewer

## Summary
Implemented JWT-based authentication:
- Login endpoint with token generation
- Refresh token flow
- Input validation

## Changes
- `internal/auth/jwt.go` (new)
- `internal/handlers/auth.go` (new)
- `go.mod` (added jwt dependency)

## Requested Review
@reviewer-agent please review for:
- Security vulnerabilities
- Error handling
- Code style

---
*This PR was created by 🤖 coder agent*
*Turn: 7 | Tokens: 4,521 | Duration: 3m*
```

### 4. GitHub Actions for Agent Triggers

```yaml
# .github/workflows/agent-trigger.yml
name: Trigger Agent

on:
  issue_comment:
    types: [created]

jobs:
  trigger:
    if: contains(github.event.comment.body, '/agent')
    runs-on: ubuntu-latest
    steps:
      - name: Parse agent command
        id: parse
        run: |
          # Parse: /agent coder start
          # Parse: /agent planner analyze
          echo "agent=$(echo '${{ github.event.comment.body }}' | cut -d' ' -f2)" >> $GITHUB_OUTPUT
          echo "action=$(echo '${{ github.event.comment.body }}' | cut -d' ' -f3)" >> $GITHUB_OUTPUT

      - name: Trigger Crush Agent
        run: |
          curl -X POST "${{ secrets.CRUSH_WEBHOOK_URL }}" \
            -H "Authorization: Bearer ${{ secrets.CRUSH_TOKEN }}" \
            -d '{
              "agent": "${{ steps.parse.outputs.agent }}",
              "action": "${{ steps.parse.outputs.action }}",
              "issue": "${{ github.event.issue.number }}",
              "repo": "${{ github.repository }}"
            }'
```

### 5. Shared State in `.orchestra/`

```yaml
# .orchestra/sessions/active.yaml
# Pushed to GitHub, synced across all developers

sessions:
  - id: sess-001
    task: T-042
    status: active
    orchestrator: alice  # Who started it
    agents:
      - name: planner
        status: completed
        branch: task/T-042/planner
        commits: 3
      - name: coder
        status: active
        branch: task/T-042/coder
        worktree: /home/alice/.cache/crush/worktrees/T-042/coder

  - id: sess-002
    task: T-043
    status: active
    orchestrator: bob    # Developer B started this
    agents:
      - name: planner
        status: active
        branch: task/T-043/planner
```

---

## Multi-Developer Workflow

### Scenario: You and Your Friend Collaborating

```
DEVELOPER A (You)                    DEVELOPER B (Friend)
       │                                    │
       │ 1. crush task create               │
       │    "Auth API"                      │
       │    → Creates GitHub Issue #42      │
       │                                    │
       │ 2. crush task start T-042          │
       │    → Spawns planner, coder         │
       │    → Creates worktrees locally     │
       │    → Pushes branches to GitHub     │
       │                                    │
       │                    3. crush status (sees your task)
       │                       → Pulls .orchestra/ state
       │                       → Sees T-042 is active
       │                                    │
       │                    4. crush task create "Payment"
       │                       → Creates GitHub Issue #43
       │                                    │
       │                    5. crush task start T-043
       │                       → Spawns agents locally
       │                       → Pushes to GitHub
       │                                    │
       │ 6. crush status (sees both tasks)  │
       │    → Real-time sync via git pull   │
       │                                    │
       │ 7. Agent coder creates PR          │
       │    → GitHub notifies both          │
       │                                    │
       │                    8. Reviews PR
       │                       → Human review + agent review
       │                       → Approves/requests changes
       │                                    │
       │ 9. Agent merges, completes task    │
       │    → Updates issue status          │
       │    → Closes #42                    │
       │                                    │
       ▼                                    ▼
```

---

## GitHub Integration Points

### 1. Issue Commands (Bot)

Comment on any issue to trigger agents:

```
/agent planner analyze     # Start planner on this issue
/agent coder start         # Start coder
/agent review              # Request review from reviewer agent
/agent status              # Show current agent status
/agent stop                # Stop all agents on this task
/agent handoff @reviewer   # Hand off to specific agent
```

### 2. PR Review by Agents

```yaml
# .github/workflows/agent-review.yml
name: Agent Code Review

on:
  pull_request:
    types: [opened, synchronize]

jobs:
  review:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Trigger Reviewer Agent
        run: |
          crush agent run reviewer \
            --pr ${{ github.event.pull_request.number }} \
            --repo ${{ github.repository }} \
            --output review.md

      - name: Post Review Comment
        uses: actions/github-script@v7
        with:
          script: |
            const fs = require('fs');
            const review = fs.readFileSync('review.md', 'utf8');
            await github.rest.issues.createComment({
              owner: context.repo.owner,
              repo: context.repo.repo,
              issue_number: context.issue.number,
              body: `## 🤖 Agent Review\n\n${review}`
            });
```

### 3. Webhook Notifications

```go
// In Crush, subscribe to GitHub webhooks
type GitHubWebhookHandler struct {
    orchestrator *Orchestrator
}

func (h *GitHubWebhookHandler) HandlePush(event PushEvent) {
    // Someone pushed to an agent branch
    if strings.HasPrefix(event.Branch, "task/") {
        // Sync state
        h.orchestrator.SyncFromGit()

        // Notify local UI if relevant
        if h.orchestrator.IsLocalTask(extractTaskID(event.Branch)) {
            h.orchestrator.NotifyUI(StateUpdate)
        }
    }
}

func (h *GitHubWebhookHandler) HandlePR(event PREvent) {
    // Agent created PR or PR was reviewed
    if event.Action == "opened" && isAgentPR(event.PR) {
        // Agent work ready for review
        h.orchestrator.HandleAgentHandoff(event.PR)
    }
}
```

### 4. GitHub Projects Integration

```
┌─────────────────────────────────────────────────────────────────────────┐
│  GitHub Project: "Development Team Board"                                │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  BACKLOG          IN PROGRESS           IN REVIEW           DONE        │
│  ────────         ───────────           ─────────           ────        │
│                                                                          │
│  ┌──────────┐     ┌──────────────┐     ┌──────────────┐     ┌─────────┐│
│  │ #44 Add  │     │ #42 Auth API │     │ #42 Auth API │     │ #40 DB  ││
│  │ rate     │     │              │     │ (PR #45)     │     │ setup   ││
│  │ limiting │     │ 🤖 coder     │     │              │     │         ││
│  │          │     │ 👤 alice     │     │ 👤 bob       │     │ ✓       ││
│  │ P3       │     │ ████████░░   │     │ reviewing    │     │ 2d ago  ││
│  └──────────┘     └──────────────┘     └──────────────┘     └─────────┘│
│                                                                          │
│  ┌──────────┐     ┌──────────────┐                           ┌─────────┐│
│  │ #45 Add  │     │ #43 Payment  │                           │ #41 API ││
│  │ 2FA      │     │ system       │                           │ docs    ││
│  │          │     │              │                           │         ││
│  │ P2       │     │ 🤖 planner   │                           │ ✓       ││
│  │          │     │ 👤 bob       │                           │ 1w ago  ││
│  └──────────┘     └──────────────┘                           └─────────┘│
│                                                                          │
│  Legend: 🤖 = AI Agent working    👤 = Human developer                  │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## Collaboration Features

### 1. Real-Time Presence

```yaml
# .orchestra/presence.yaml (auto-updated, git-ignored or frequent commits)

developers:
  - name: alice
    status: active
    current_task: T-042
    last_seen: 2024-01-15T10:45:00Z
    watching: [T-042, T-043]

  - name: bob
    status: active
    current_task: T-043
    last_seen: 2024-01-15T10:44:00Z
    watching: [T-042, T-043]
```

### 2. Chat/Comments on Agent Work

```markdown
# In GitHub Issue #42 comments

👤 alice: @coder-agent can you also add rate limiting?

🤖 coder-agent: @alice Sure! Adding rate limiting to the auth endpoints.
               Working on it now. Will update this thread when done.

🤖 coder-agent: ✅ Rate limiting added in commit a1b2c3d
               - 100 req/min per IP
               - 1000 req/min per user
               Please review when ready.

👤 bob: Looks good! One question: should we use Redis for distributed
       rate limiting since we might have multiple instances?

🤖 planner-agent: @bob Good point. I'll analyze the trade-offs and
                  update the plan.
```

### 3. Conflict Resolution Between Developers

```
SCENARIO:
- Alice's coder-agent modifies auth.go
- Bob's coder-agent also modifies auth.go (different task)
- Both try to merge to main

RESOLUTION:
1. GitHub blocks second merge (conflict)
2. Orchestrator detects conflict
3. Options:
   a) Auto-merge with LLM resolution
   b) Create "conflict resolution" task
   c) Notify both developers to coordinate
```

### 4. Shared Agent Pool

```yaml
# .orchestra/config.yaml

team:
  name: "Dev Team"

  developers:
    - name: alice
      github: alicecoder
      role: lead
    - name: bob
      github: bobdev
      role: developer

  # Shared agents - anyone can use
  agents:
    - name: planner
      model: glm-4-plus
      max_concurrent: 2  # Allow 2 parallel planner sessions

    - name: coder
      model: claude-3.5-sonnet
      max_concurrent: 3

    - name: reviewer
      model: o3-mini
      max_concurrent: 2

  # Rate limiting per developer
  limits:
    alice:
      max_agents: 5
      max_parallel_tasks: 2
    bob:
      max_agents: 3
      max_parallel_tasks: 2
```

---

## CLI Commands for Collaboration

```bash
# Start a task (creates GitHub issue if needed)
crush task start "Auth API" --assign-agent=coder

# Check status of all tasks (pulls latest from GitHub)
crush status --all

# See what your collaborator is working on
crush status --developer=bob

# Join an existing task (collaborate on same task)
crush task join T-042

# Watch a task (receive notifications)
crush task watch T-042

# Hand off to collaborator
crush task handoff T-042 --to=bob

# Request review from human
crush task request-review T-042 --from=bob

# See activity feed
crush activity --since=1h

# Chat on task (posts to GitHub issue)
crush task comment T-042 "Can we also add rate limiting?"
```

---

## Setup Guide

### 1. Initialize Collaborative Repo

```bash
# In your shared GitHub repo
crush orchestra init --github

# Creates:
# - .orchestra/ directory
# - .github/workflows/agent-*.yml
# - GitHub App / OAuth setup
```

### 2. Connect Developer Machines

```bash
# Developer A (you)
crush login --github
crush orchestra connect your-org/your-repo

# Developer B (friend)
crush login --github
crush orchestra connect your-org/your-repo
```

### 3. Configure Agents

```yaml
# .orchestra/team.yaml (committed to repo)
name: "Dev Team"

agents:
  - name: planner
    cli: crush
    model: glm-4-plus

  - name: coder
    cli: crush
    model: claude-3.5-sonnet

  - name: reviewer
    cli: codex
    model: o3-mini
```

### 4. Start Collaborating

```bash
# You start a task
crush task create "Implement auth API" --priority=P1
crush task start T-042

# Your friend sees it
crush status
# Output:
# T-042  Auth API          active    🤖 coder (alice)    40%
# T-043  Payment system    backlog   -                   -

# Your friend starts their own task
crush task create "Payment system" --priority=P2
crush task start T-043

# Both can see each other's progress
crush dashboard  # Opens shared GitHub Projects view
```

---

## Benefits

| Feature | Benefit |
|---------|---------|
| **Shared visibility** | Both developers see all agent activity |
| **No conflicts** | Git worktrees + branches = isolated work |
| **Audit trail** | Every action tracked in git/GitHub |
| **Asynchronous** | Work across timezones naturally |
| **Human review** | Agents create PRs, humans approve |
| **Parallel work** | Multiple tasks, multiple developers |
| **Native GitHub** | Use familiar tools (Issues, Projects, PRs) |

---

## Architecture Summary

```
┌────────────────────────────────────────────────────────────────────────┐
│                        GITHUB (Source of Truth)                         │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐     │
│  │  Issues  │ │ Projects │ │ Branches │ │   PRs    │ │ Actions  │     │
│  │ (Tickets)│ │ (Visual) │ │(Worktrees)│ │(Handoffs)│ │(CI/CD)   │     │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘ └──────────┘     │
│                          │                                             │
│                   .orchestra/                                          │
│                   (shared state)                                       │
└──────────────────────────┬─────────────────────────────────────────────┘
                           │
           ┌───────────────┴───────────────┐
           │                               │
           ▼                               ▼
   ┌───────────────┐               ┌───────────────┐
   │    CRUSH      │               │    CRUSH      │
   │   (Alice)     │               │    (Bob)      │
   │               │               │               │
   │ ┌───────────┐ │               │ ┌───────────┐ │
   │ │Orchestrat │ │               │ │Orchestrat │ │
   │ │    or     │ │               │ │    or     │ │
   │ └───────────┘ │               │ └───────────┘ │
   │               │               │               │
   │ worktrees/    │               │ worktrees/    │
   │  └─ T-042/    │               │  └─ T-043/    │
   │     ├─ coder/ │               │     ├─ planner│
   │     └─ review │               │     └─ coder/ │
   └───────────────┘               └───────────────┘
```

---

## Implementation Phases

**Phase 1: Basic GitHub Sync**
- [ ] Push/pull `.orchestra/` state to GitHub
- [ ] Create issues from tasks
- [ ] Basic presence tracking

**Phase 2: PR Integration**
- [ ] Agents create PRs
- [ ] Human review workflow
- [ ] Merge automation

**Phase 3: Real-Time Collaboration**
- [ ] Webhook notifications
- [ ] Live presence indicators
- [ ] Cross-developer visibility

**Phase 4: GitHub Actions**
- [ ] CI/CD on agent branches
- [ ] Automated testing
- [ ] Agent review bot
