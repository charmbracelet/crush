---
name: personal-agent-workflow
description: Use when the user is designing, running, or asking for a full personal AI agent workflow, OpenClaw-like orchestration, Telegram/provider routing, long-running automation, or careful multi-step agent behavior.
---

# Personal Agent Workflow

Use a deliberate agent loop. Do not optimize every turn for speed. Fast response is useful only when the task is simple, read-only, or already fully understood.

## Core Loop

1. Classify intent: inquiry, directive, approval-sensitive directive, or correction.
2. Load relevant standing context: project rules, global instructions, MCP/server instructions, and matching skills.
3. Gather evidence from the real source: files, APIs, docs, browser, logs, or runtime state.
4. Decide the smallest safe plan that can satisfy the user.
5. Act only when the user intent is a directive.
6. Verify with the narrowest meaningful check.
7. Report what changed, what was verified, and what remains uncertain.

## Speed Policy

- Simple factual reply: answer directly.
- Read-only research: gather sources first, then answer.
- Code/config edit: inspect first, edit second, verify third.
- Remote/server change: read-only check first, then apply with rollback awareness.
- Deployment, push, commit, delete, credential, or binary replacement: require explicit user approval for that specific action unless already given.

## Terminal And Long-Running Work

- Use normal Bash for finite commands that need truthful exit status: tests, builds, git, config validation, and one-shot inspection.
- Use the `tmux` tool for persistent shells, REPLs, dev servers, log streams, and commands that need interactive shell startup across turns.
- Capture tmux output before deciding what happened. A visible pane is not proof of success unless it shows a completed successful result.
- Clean up tmux sessions when they are no longer useful.

## Model Escalation

- Current Crush model selection is explicit: choose from the model menu or use `crush run -m provider/model` for one-off runs.
- Do not claim automatic fallback to a larger model unless the code implements it and the run logs show it happened.
- A future auto-escalation feature must avoid replaying side-effectful tool calls, preserve session state, and disclose when escalation occurs.

## Provider and Channel Expansion

When adding Telegram, Slack, web, CLI, or other providers:

- Keep channel handling outside the model prompt when possible.
- Route messages through a gateway that adds source, user, permissions, attachments, and urgency metadata.
- Keep action tools behind MCP/tools with clear schemas.
- Use hooks for deterministic policy gates.
- Use skills for repeatable workflows.
- Use memory only for durable user/project facts, not current external truth.

## Small Model Discipline

For local or small models:

- Prefer short always-on rules and load detailed skills only when relevant.
- Force evidence before claims.
- Use checklists internally for multi-step work.
- Stop repeated reasoning and summarize once.
- Use deterministic hooks for things the model should not decide probabilistically.
