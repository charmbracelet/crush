---
name: goal-project-init
description: Use when starting a multi-step Goal mode coding objective that needs project discovery, planning, execution checkpoints, and verified completion.
license: MIT
metadata:
  inspiration: https://github.com/obra/superpowers
---

# Goal Project Initialization

Adapt the useful Superpowers development lifecycle to the current objective
without loading its global bootstrap or forcing every task through a large
workflow.

## 1. Understand

- Inspect the repository, its instruction files, and recent relevant changes.
- Restate the concrete outcome internally.
- Identify acceptance evidence before editing: tests, build output, diagnostics,
  file state, or observable behavior.
- Ask the user only when missing information would materially change the result.

## 2. Plan

- Use `todos` for a short plan when the objective has multiple meaningful steps.
- Keep steps independently verifiable and ordered by dependency.
- Do not create planning documents or branches unless the user requests them.

## 3. Execute

- Read before editing and follow existing project patterns.
- Make the smallest coherent change that advances the objective.
- After a failed attempt, use the new evidence before choosing another route.
- Keep working while the goal is active; a normal response is not completion.

## 4. Verify

- Run fresh, relevant verification before claiming success.
- Inspect the actual output and distinguish unrelated environmental failures.
- Call `goal_status` with `complete` only when the acceptance evidence passes.
- Call `goal_status` with `blocked` only when progress requires external input or
  state that cannot be obtained safely.
