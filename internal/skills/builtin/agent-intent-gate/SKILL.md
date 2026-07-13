---
name: agent-intent-gate
description: Use before acting when the user intent may be research-only, implementation, configuration, deployment, SSH, or automation work. Helps small or local models avoid premature edits and hallucinated actions.
---

# Agent Intent Gate

Classify the request before using tools or changing files.

## Intent Classes

- Inquiry: the user asks what, why, whether, explain, compare, or research. Read and answer. Do not edit files.
- Directive: the user explicitly asks to change, add, fix, run, commit, install, deploy, or configure. Act within scope.
- Approval-sensitive directive: deploy, push, commit, overwrite installed binaries, rotate secrets, delete files, change remote services, or change global config. Confirm when the user has not clearly approved that specific action.
- Continuation or correction: the user interrupts or changes direction. Treat the newest message as steering the active task.

## Operating Rules

1. Restate the operative intent internally in one short phrase.
2. If it is an inquiry, gather evidence and answer without edits.
3. If it is a directive, inspect the relevant files or runtime first.
4. If the request touches unknown repos or remote machines, prefer read-only inspection before writes.
5. If the user says "research first", "wait", "do not initialize", or similar, do not perform side-effecting setup.
6. If you are about to exceed the clear scope, stop and ask one narrow question.

## Anti-Hallucination Rules

- Never claim a file, command, endpoint, model capability, or test result exists without reading or running it.
- Prefer primary sources: local code, official docs, live API metadata, or direct command output.
- When you infer, say it is an inference.
- If a model repeats or loops, stop repeating and summarize the known state once.
