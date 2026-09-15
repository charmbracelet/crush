# Vision

An agent harness that helps the user create a good product.

A good product requires two things:

- **Good planning and documentation** — intent captured before
  execution, and a development process that produces durable docs,
  not just code.
- **Good execution** — the model gets the _correct_ context, not
  the _complete_ context.

And one control principle:

- **Bounded autonomy** — the agent runs free between human
  confirmations. Each gate bounds the expectation delta between
  what the user wants and what gets built.

Scope: built for a single technical user driving the loop.
Multi-user and team semantics — whose confirmations, whose
revision rates — are a different design, not a later parameter.

## Why bounded, not maximal, autonomy

Autonomous development is good — but a long autonomous run drifts
toward common, general practice. Left unconfirmed, the delta
between expectation and implementation compounds until correcting
it costs more than the task did. The human touch is what makes a
product specific rather than generic — so confirmation gates are
not only error-prevention; they are the points where the user's
taste enters the artifact.

The rule: autonomy is fine _as long as the user confirms the
changes_. Deterministic at the boundaries, autonomous inside them.

**User attention is the scarce resource, and it prices every
gate.** The optimization is not "gate where drift is likely" but
_maximize taste per confirmation_ — fire where the user's input
changes the artifact most, not most often. This also names the
honest dependency: "good product" is proxied by _the user
confirms the changes_ — satisfaction at confirmation time, which
assumes an engaged reviewer. Gate fatigue is what erodes that
engagement, so the system must spend attention deliberately or it
trains the user to stop paying it.

**Gate calibration is measurable, not just felt.** A gate is
mis-placed if it fires too late (drift already compounded) or too
often (autonomy isn't actually autonomous). The proxy signal:
_post-checkpoint revision rate_ — how often a user's next action
after confirming a checkpoint is to revise or roll back something
inside the segment they just confirmed. Rising revision rate on a
given class of checkpoint means gates of that class are placed too
coarsely (too much drifted before the ask); near-zero revision
rate alongside high gate frequency means they're placed too
tightly (confirmation fatigue, autonomy not being used). Neither
extreme is free — the target is a stable, low, non-zero revision
rate per checkpoint class. Three caveats the calibration spec must
carry: a revision can mean the agent drifted _or_ the user's
intent moved — the metric measures delta, not whose delta, so
trajectory context (was the revision inside the confirmed
segment's scope?) is part of the signal; near-zero revision is
ambiguous — well-placed gates or a user who stopped reviewing —
so it needs a companion signal (late revisions outside the
immediate window); and any feedback loop that tightens gates on
rising revisions can oscillate, so adjustment needs damping and
hysteresis, not instant response. Cold start: with no revision
history, gate sensitivity defaults to conservative high-frequency
placement and widens as data accumulates. (Full calibration spec
to come, alongside the ambiguity taxonomy that decides gate
placement — referential, architectural delta, domain/taste, blast
radius.)

## Why correct context, not complete context

A good execution harness feeds the LLM the right input: the right
codebase slices, the right history, a good prompt. Bigger context
does not mean better output — a large context full of noise makes
the result worse, and every noisy byte is re-sent on every step of
the loop.

The trap is optimizing the wrong number. Total cost is

```
Σ requests (context-per-request × model-cost-multiplier)
```

Prompt caching discounts context-per-request while two other
factors are the hidden multipliers: **request count** and **which
model answers**. A large cached history that causes three extra
distraction-loops — a request cycle that re-derives or re-confirms
something already established in context, driven by noise rather
than new information — costs more than a smaller, fresh, correct
context that finishes in fewer requests. Likewise, routing
classification-shaped work to a cheap model and reasoning-shaped
work to a capable one changes the multiplier directly, independent
of context size. The metric that matters is **tokens-to-done per
task** (cost-normalized across model tiers), not cache-hit rate or
raw token count.

## What this means for the harness

| Axis                           | Mechanism                                                                                                                                                                                                                                                                                                                                                                                                                                                    |
| ------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Planning & documentation       | **Documentation is an output, not a side effect.** Checkpoints, plan items, and edge firings are designed to be the raw material for durable product documentation — decision logs, architecture notes, "why this exists." Outputs are compacted — high-leverage decisions surface, implementation noise doesn't — or docs become a swamp nobody reads. (Dedicated spec to come — currently the least mature axis; it must name who reads what, or it rots.) |
| Bounded autonomy               | **Ask at commitment points.** Ambiguous referents and large scopes are where expectation deltas are born — confirm before the first write, not after the last. Calibrated against post-checkpoint revision rate, above.                                                                                                                                                                                                                                      |
| Correct, not complete, context | **Prune noise, preserve truth.** Stubs, boundaries, and relevance selection cut what the model re-reads; everything cut names its recovery path — and that label is user-visible, not just a debug log, so the cut is auditable by the person it affects, not only the maintainer.                                                                                                                                                                           |

## How we'll know this is working

- **Context**: tokens-to-done per task (cost-normalized across
  model tiers), measured against the golden-trajectory corpus,
  tracked over time as harness changes ship.
- **Autonomy**: post-checkpoint revision rate per gate class,
  trending down and stabilizing — low, non-zero.
- **Planning & documentation**: reviewable doc-artifact output per
  checkpoint (decision logs, architecture notes) exists and is
  read, not just generated.

None of these are gut checks — each ties back to a mechanism
already in the harness or already planned, and each should be
falsifiable by the eval harness before being trusted in
production.

## Where the work lives

| Pillar                | Docs                                                                                                                              |
| --------------------- | --------------------------------------------------------------------------------------------------------------------------------- |
| Correct context       | `docs/design/CONTEXT_NOTEBOOK.md`, `TOOL_RESULT_PRUNING.md`, `CONTEXT_PREFETCH.md`, `SEMANTIC_INDEX.md`, `PROMPT_OPTIMIZATION.md` |
| Bounded autonomy      | `docs/design/RUN_EDGES.md`, `PLAN_ARTIFACT.md`, `TURN_CONTEXT.md`, `VERIFICATION_LOOPS.md`                                        |
| Session knowledge     | `docs/design/SESSION_KNOWLEDGE.md`, `NOTEBOOK_QUALITY.md`, `INTRA_TURN_BOUNDARIES.md`                                             |
| Parallelism           | `docs/design/BACKGROUND_SUBAGENTS.md`, `SWARM_DEDUP.md`                                                                           |
| Measurement           | `docs/design/EVAL_HARNESS.md`, `CONTEXT_WINDOW_SAFETY.md`                                                                         |
| Design-space analysis | `docs/design/HARNESS_TOPOLOGY.md`                                                                                                 |
