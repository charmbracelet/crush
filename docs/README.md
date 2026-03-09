# Crush Documentation

## Quick Navigation

| What You Want | Where To Go |
|---------------|-------------|
| What's implemented now | [implemented/](implemented/) |
| What we're building next | [active-design/](active-design/) |
| Future vision | [future/](future/) |
| Research notes | [research/](research/) |
| Architecture decisions | [adr/](adr/) |
| **External review materials** | [forReview/](forReview/) |

## Status Summary

### Implemented

- **Dispatch MVP**: Crush can dispatch tasks to registered workers via the `dispatch_task` tool
- **Worker Registry**: CLI commands to manage workers (`crush agents`)
- **Database**: SQLite schema for dispatch documents

[→ See implemented/](implemented/)

### Active Design

- **Dispatch API Server**: REST API with session-based dispatch and long-poll command delivery
- **Sessions**: The fundamental unit - worker_type + machine + directory
- **Handler**: One per machine, manages multiple sessions, polls for commands
- **Ack Protocol**: Claim/Start/Finish for reliable command execution
- **Crash Recovery**: Process fingerprint (PID + start time) + atomic state file writes
- **Redirect Lineage**: Track redirect history with parent_id

[→ See active-design/](active-design/)

### Future

- **Multi-Agent Orchestration**: Full session-based dispatch system with distributed workers
- **Orchestra**: Spec-driven development system with phases, tasks, and blueprints

[→ See future/](future/)

## Architecture At A Glance

```
SERVER                          MACHINE (laptop-A)
  │                                 │
  │                                 ▼
  │                           ┌───────────────┐
  │                           │   HANDLER     │
  │                           │ (one per      │
  │                           │  machine)     │
  │◄───────────────────── long-poll ──────────┤
  │  GET /machine/{id}/commands?wait=30       │
  │                           │               │
  │                           │  Sessions:    │
  │                           │  - goose-fe   │
  │                           │  - crush-fe   │
  │                           │               │
  │──────────────────────────────────────►    │
  │  [{action: spawn, session: ..., command}] │
  │                           │               │
  │                           ▼               │
  │                      WORKER               │
  │                    (ephemeral)            │
  │                           │               │
  │◄────────────────────── POST claim ────────┤
  │◄────────────────────── POST start ────────┤
  │◄────────────────────── POST finish ───────┤
  │◄────────────────────── POST activity ─────┤
```

**Key decisions:**
- **Sessions**: Fundamental unit = worker_type + machine + directory
- **Directory constraint**: One session per worker type per directory
- **Long-polling** (not SSE) - works through firewalls
- **One handler per machine** - manages multiple sessions
- **Handler is dumb executor** - server crafts commands
- **Claim/Start/Finish** - three-phase ack for reliability
- **Process fingerprint** - PID + start time prevents killing wrong process

## Folder Structure

```
docs/
├── README.md              # This file
├── implemented/           # What works NOW
│   └── dispatch-mvp.md
├── active-design/         # What we're building NEXT
│   └── dispatch-api.md
├── future/                # Vision documents
│   ├── multi-agent-orchestration.md
│   └── orchestra-design.md
├── research/              # Investigation notes
│   ├── cli-agent-interoperability-research.md
│   └── orchestra-research.md
├── adr/                   # Architecture Decision Records
│   └── ...
├── forReview/             # External review materials
│   ├── README.md          # Instructions for reviewers
│   └── *-review.md        # Reviewer feedback
└── archive/               # Old/outdated docs
```

## For External Reviewers

If you're reviewing this architecture for the first time:

1. Start with [forReview/README.md](forReview/README.md)
2. It explains the project from zero context
3. Asks specific review questions
4. Drop your `{name}-review.md` in the `forReview/` directory

## Review History

| Round | Date | Reviewers | Status |
|-------|------|-----------|--------|
| 1 | Mar 5, 2026 | Codex, Gemini CLI | Feedback incorporated |
| 2 | Mar 5, 2026 | Codex, Gemini CLI | Feedback incorporated |
