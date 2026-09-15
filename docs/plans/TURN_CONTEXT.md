# Turn Context — Per-Turn Augmentation & Ambiguity Handling

> **Status:** Spec. Covers why "improve the prompt" should mean
> _augment_ (add context), not _rewrite_ (rephrase), the tail-position
> cache constraint that decides where augmented context lands, and the
> ordered PR list. Interacts with `EVAL_HARNESS.md` (arm axis),
> `CONTEXT_NOTEBOOK.md` (candidate source), and
> `NOTEBOOK_QUALITY.md` (retrieval-signal reuse).

## Problem

User prompt quality is uncertain. Two failure modes, different fixes:

1. **Poor form** — rambling or unclear phrasing ("it doesn't work,
   can you look"). Rewriting helps here.
2. **Missing referents** — "fix the bug" (which bug?), "update the
   config" (which one?), "use the usual pattern" (what pattern?).

The second mode is more common and more damaging — a run that
executes confidently on the wrong referent burns a whole turn of
tool calls. It is also the mode a rewriter **cannot** fix: a rewriter
only rephrases what exists; it cannot supply a referent the user
never typed. Only two mechanisms supply missing information —
retrieved context, or asking the user. Prompt _form_ is a weak
lever; prompt _information content_ is the strong one.

Consequence for scope: turn context = augmentation + (optionally)
clarification. Silent rewriting is rejected (see Non-goals).

## What exists

- `preparePrompt` (`agent.go:1764`) — deterministic history render:
  notebook boundary split, tool-result adjacency re-emission, stub
  substitution. No relevance pass over the _current_ user prompt
  beyond `maybeAutoInject`.
- `maybeAutoInject` (`agent.go:2005`) + `notebookRelevanceRefs`
  (`agent.go:1938`) — injects full notebook entries whose `file:`
  tags match paths in the latest user message. Notebook-mode only;
  ref extraction requires a slash in the path
  (`extractExplicitFilePaths`, `agent.go:1967` — the "fix auth.go"
  bare-name miss documented in `NOTEBOOK_QUALITY.md`).
- `buildSelectionInput` (`notebook_segments.go:777`) — working set
  from `filetracker.ListRecentReadFiles` + live-path stats. Already
  the session's "what is the user probably looking at" signal;
  currently consumed only by notebook selection.
- `SelectedModelTypeSmall` (`coordinator.go:1154`) and
  `SelectedModelTypeSummary` — the small-model slots; title
  generation (`agent.go:2329+`) shows the pattern for a cheap
  auxiliary call.
- `question` tool (`internal/agent/tools/question.md`) — the
  clarification mechanism exists; policy suppresses it
  (`BE AUTONOMOUS`, coder.md.tpl:7, `decision_making` 112-124).
- `SearchMem0` (`notebook/mem0.go:78`) — cross-session memory,
  reachable only via `recall("cross:...")`. Never auto-injected.
- Per-project control: `context_paths` (`config.go:378`) +
  `defaultContextPaths` (`config.go:28-45`) render into
  `<project_context>` (coder.md.tpl:314-324); `GlobalContextPaths`
  into `<user_preferences>`. Per-project `crushrc` already scopes
  MCP/LSP/skills/permissions. What is _not_ per-project: the base
  template itself (`coder.md.tpl` is embedded; built once per agent
  at `coordinator.go:901-907`).

## Cache constraint (decides the design)

Anthropic-style prefix caching is positional: a byte change at
position _k_ kills every breakpoint after _k_. Current breakpoints:
last built-in tool + last tool (`agent.go:802-815`), last system
message, and the moving tail (`agent.go:1008-1037`).

Therefore augmented context goes at the **tail**: appended as a
system message at the end of `Messages`, immediately before the
user prompt fantasy appends. The entire history prefix still
cache-reads; the blob rides in the tail that is new anyway (the
user prompt is new regardless). Injection anywhere earlier —
system prompt, mid-history, notebook prefix — rebills the full
history every turn.

Two corollaries:

- **Compute once per user turn.** The blob must be byte-identical
  across the turn's steps or each `PrepareStep` breaks the moving
  tail breakpoint. Fantasy accumulates returned messages, so
  injecting into `history` at Run time (after the `preparePrompt`
  call, `agent.go:923`) carries it through every step for free —
  same property `prefixFingerprint`/`prefixCache`
  (`notebook_segments.go:845-855`) maintains for the prefix side.
- **Ephemeral, not persisted.** The blob lives in the request, not
  the message DB. Next turn it is gone; a fresh one is derived.
  Facts the model actually used land in its reply and (notebook
  mode) in entries — the transcript is not polluted with stale
  retrieval output. Do **not** fold it into the persisted user
  message: that bakes ephemeral context into history and resends
  it forever.

This also makes augmentation orthogonal to notebook mode: it works
identically in legacy-summary and no-notebook sessions because it
never touches the history pipeline.

## Proposal

### PR 0: Option + measurement surface (before any behavior)

- `options.turn_context = off | session | semantic`
  (default `off`). Values name the _scope of context_ drawn on,
  not the mechanism: `session` = deterministic session signals,
  `semantic` = meaning-based retrieval. Every later PR is dead
  code without it; the enum reserves the arm axis
  `experiment.json` needs (`EVAL_HARNESS.md:165`).
- Corpus slice: author `task.turns` that are deliberately
  underspecified against fixtures where the referent _is_
  discoverable ("fix the broken test" where exactly one test
  fails; "add validation" where one form exists). Turns stay
  path-agnostic per the trajectory contract — vagueness about
  _referent_, not about _response_.
- Telemetry: per-turn augmented bytes, section counts, latency of
  the context step (0 for `session`), plus `cache_read` delta in
  the run record — the harness already captures token fields
  (`EVAL_HARNESS.md:193`).

### PR 1: Session-signal augmentation (`turn_context = session`)

`buildTurnContext(ctx, sessionID, userPrompt) → string`, called in
`Run` after `preparePrompt`; non-empty output appends as
`fantasy.NewSystemMessage` (the `notebook_segments.go:1052`
convention) to `history` before `Stream`.

Candidate sources, each labeled and token-capped (target ≤ ~2K
total; a context blob that costs more than the ambiguity it
resolves is net-negative):

- **Working set**: `filetracker.ListRecentReadFiles` — files
  recently read/edited. "Fix the bug" usually means the file the
  user was just looking at. Recency-bound it (last N segments'
  worth) — the set is cumulative and degenerates without a bound
  (same caveat as `NOTEBOOK_QUALITY.md` PR 1).
- **Explicit refs**: `extractExplicitFilePaths` on the current
  prompt → matching notebook entries (extends `maybeAutoInject`'s
  idea to all sessions) + the files' existence/summary lines.
- **Session state**: active todos (`session.Todos`), git status
  delta, last failed command's error headline (`ErrorHeadline`
  already computed at classify.go:63).
- **Render shape**: `<turn_context>` sections, each labeled —
  silent unlabeled context is the known failure mode (invariant 4,
  `TOOL_RESULT_PRUNING.md`).

### PR 2: Per-project prompt slot

Per-project _knowledge_ already exists (context files). The gap
is narrower: a project cannot extend the base prompt with
behavioral directives that don't belong in a context file.

- `options.system_prompt_append`: file path, rendered into the
  template after `<project_context>` — same load path as
  `promptData`'s context loading (`prompt.go:188-189`).
- **Keep the base template fixed.** A variable base prompt makes
  corpus pass rates conditional on each fixture's prompt and
  confounds every arm comparison. Project knowledge goes in the
  context slot; the append slot is for org policy blocks and
  project-specific workflow rules only.

### PR 3: Semantic retrieval tier (`turn_context = semantic`)

Literal matching misses paraphrase relevance ("login" → auth
entries; "the crash" → the erroring command). The semantic tier
widens candidate recall behind the same tail blob:

- **FTS5 over notebook entries** (titles, tags, entry text) →
  candidate set. Requires a new `internal/db` migration +
  sqlc query. _Prerequisite check_: confirm the CGO-disabled
  sqlite driver compiles FTS5 before committing the schema.
- **mem0 as a candidate source** — `SearchMem0` today is
  tool-only; as a candidate it gets top-k filtered like
  everything else, still bounded by `mem0SearchMaxTokens`.
- **Small-model rerank**: `SelectedModelTypeSmall` scores
  candidates against the user prompt; top-K inject. This is the
  only added LLM call in the turn (~300ms–1s serial before first
  token) — which is why it is a tier, not the default.
- Determinism note: rerank output must still be computed once per
  turn and frozen; a re-rerank per step violates the
  once-per-turn invariant.

### PR 4: Ambiguity classifier → targeted clarification

- Small model scores **referent resolvability** of the prompt
  against the PR-1/PR-3 retrieved context: is the thing being
  asked about identifiable in what we already know?
- Below threshold → the run stops at one `question`-tool-style
  clarifying question instead of executing. The mechanism exists;
  only the routing is new. One focused question is cheaper than a
  derailed 20-step run.
- Opt-in (`options.ambiguity_clarification`, default off);
  `BE AUTONOMOUS` stays the default posture. Miscalibration is
  the risk — measure clarification rate vs. derailed-run rate on
  the vague-prompt corpus slice before considering default-on.

### PR 5: `/enhance` — opt-in rewrite (the only rewriting allowed)

- A command that rewrites the _editor contents_ with the small
  model before submit. User reviews the rewrite, edits, sends.
- Keeps determinism (nothing runs unless invoked), keeps the
  transcript honest (the sent text is what the user approved),
  costs zero latency when unused.
- Explicitly **not** an automatic per-turn rewrite — see
  Non-goals.

### PR 6 (free): sharper ambiguity clause in the base prompt

`decision_making` (coder.md.tpl:112-124) already says "make
reasonable assumptions, state them, proceed." Tighten it: when a
referent is ambiguous, enumerate in-context candidates, pick the
most probable, state the assumption in one line. Zero latency,
helps every prompt, and it's a `PROMPT_OPTIMIZATION.md`-style
edit the eval harness can gate on its own.

## Measurement

- `turn_context` arm on the vague-prompt corpus slice: pass-rate
  delta `off` vs `session` vs `semantic` — does the semantic tier
  earn its latency?
- Augmented bytes per turn vs. derailment proxy (clarification
  turns, user follow-ups of the form "no, I meant…" — measurable
  as correction-pattern trajectories in the corpus).
- `cache_read` must be flat vs. option off — the regression test
  for tail-position correctness.
- Clarification rate (PR 4 on): target band, not zero — zero
  means the classifier never fires (dead code); too high means
  it's nagging.

## Non-goals

- **Silent prompt rewriting.** When the rewriter guesses intent
  wrong, you pay latency _and_ corrupt intent _and_ the
  transcript no longer records what the user asked. All three
  failure costs at once.
- **Prefix or mid-history injection.** Covered under Cache
  constraint — this is the expensive version of the same feature.
- **Relevance-based history removal.** Dropping middle messages
  breaks tool-call/result adjacency (`agent.go:1821-1832`) — the
  strict-adjacency handling exists because providers hard-fail on
  it. Augmentation only _adds_; the boundary mechanism stays the
  only pruner.
- **Reactive/scripted turns.** Clarification is a real user turn;
  harness `task.turns` stays fixed at author time
  (`EVAL_HARNESS.md` non-goal).
- **Persisted augmentation.** The blob is per-turn ephemeral by
  design; persisting it conflates retrieval state with transcript.

## Risks

- **Augmentation bloat** — each source individually capped; total
  ≤ ~2K tokens. An oversized blob both costs tokens and dilutes
  the actual prompt.
- **Ephemeral-context inconsistency** — the model can reference
  blob content that is gone next turn. Accepted: facts in use
  land in the reply/notebook; the alternative (persisting) is
  worse. If sessions show the model confused by vanished context,
  the fallback is persisting a one-line provenance marker, not
  the blob.
- **Classifier miscalibration (PR 4)** — nagging clarifications
  are a worse UX failure than no feature. Opt-in flag + corpus
  measurement gate before any default change.
- **Small-model nondeterminism vs. eval variance** — rerank adds
  per-turn stochasticity the paired design must absorb. The
  `session` tier landing first means the `semantic` tier is only
  justified by a measurable pass-rate lift on the vague slice.
- **FTS5 availability** — CGO-disabled sqlite may not compile
  FTS5; verify before the migration, fall back to `LIKE`-scoped
  candidate queries (titles/tags only) if absent.
- **Per-turn staleness** — the blob is computed at Run start;
  mid-turn state changes don't refresh it. Correct by design
  (once-per-turn invariant); the failure mode is only that the
  model acts on slightly stale working-set info within a single
  turn — bounded and self-correcting via tool results.
