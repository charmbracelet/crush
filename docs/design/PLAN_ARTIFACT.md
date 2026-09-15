# Plan Artifact — Typed Plan Items

> **Status:** Spec. Split from `HARNESS_TOPOLOGY.md` — the typed
> plan object the harness can check.
>
> **Depends on:** nothing — `session.Todos` exist today and the
> gate reads them. `phase-confirm` (#39) lands first and renders
> the flat list; this doc is what upgrades it.
> **Ship when:** the confirmation needs structure flat todos can't
> carry — evidence binding ("item done ⇔ its checks green"), open
> items blocking the gate — or fan-out needs `DependsOn` dispatch
> units.
> **Measured by:** one definition of done (unified gate trigger),
> eval trajectory assertions on plan state, replan-turn quality.

## Goal

Give the harness a plan it can _check_: typed items with
dependencies and evidence, so "is this done" and "may this execute"
are structural questions about an artifact — not self-reported
model judgment. The model keeps writing plans; the harness gains
the ability to verify them.

## Problem

`session.Todo` is `{Content, Status, ActiveForm}`
(`session.go:34-38`) — flat strings, unordered, no dependencies,
model-managed. The todos gate (`verify_gate.go:533`) can ask "are
items open?" and nothing more structured. `phase-confirm` (#39)
can render the list to a human but cannot _check_ it — no evidence
binding, no "which item is unresolved", no structural confidence.
`BACKGROUND_SUBAGENTS.md` needs a dispatch unit; today the dispatch
spec is a prompt string the model composes inline in the `agent`
tool call, with no handle the harness can track, dedup against, or
verify.

## Design

`session.Todo` → `PlanItem{ID, Content, DependsOn []ID, Status,
Evidence []string}`:

- `DependsOn` is the dispatch unit for fan-out: independent
  subtrees are what a background subagent may be handed, and the
  join edge (`BACKGROUND_SUBAGENTS.md`) reads "outstanding" off
  the same structure.
- `Evidence` binds an item to check names — an item is `completed`
  only when its checks resolved green. This unifies today's two
  gate triggers (failed checks, open todos) into one definition of
  done instead of two scans of the same run. Index paths are a
  second evidence kind: an item bound to `internal/agent/` lets a
  repair/replan edge render the current symbols of the files it
  names (`CONTEXT_PREFETCH.md`) into the prompt — the plan carries
  its own map.
- **Structural confidence.** `phase-confirm` graduates from
  "render the list, ask yes/no" to a checkable gate: every item
  must bind evidence (files or checks); items the model marks
  open block the transition. Executor confidence becomes a
  property of the artifact — the harness inspects the plan, it
  never asks the model to self-report a percentage.
- Persistence: todos already ride the session row; the durable win
  is a `plan` notebook entry type so the plan survives compaction
  as ground truth — today the summary has to re-derive what the
  plan was from rendered todo tool calls.
- The model still writes the plan (the `todos` tool graduates, or
  a `plan` tool replaces it with a migration shim reading old
  `session.Todos`). The harness gains structure to _check_; it
  does not gain a planner that replaces the model. That is the
  side of the line the production tools landed on, and it keeps
  prompt-mode cost at zero for tasks too small to plan.

## Migration

- `todos` graduates vs. `plan` replaces — decide at implementation;
  either way a shim reads legacy `session.Todos` so in-flight
  sessions don't orphan.
- The plan format is model-visible — expect `bands.json`
  re-characterization after merge (same confound class as the #39
  clause: corpus authored under the old surface).

## Non-goals

- No planner model — the model writes the plan; the harness checks.
- No mandatory planning — `PlanItem` must never make a one-line
  fix pay a planning turn. The plan-free path stays below the same
  scope threshold `phase-confirm` uses.
- No DAG evaluation — `DependsOn` is a dispatch/readiness input,
  not a scheduler; the loop kernel still routes.

## PR ordering

1. `PlanItem` schema + `todos`/`plan` tool + migration shim.
2. Gate reads the typed list — unify failed-checks and open-items
   into one done-definition.
3. `phase-confirm` upgrade: structural check — evidence-bound
   items required, open items block.
4. `plan` notebook entry type — compaction-surviving ground truth.
