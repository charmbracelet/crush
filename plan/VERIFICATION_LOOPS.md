# Verification Loops — Ground-Truth Gates on Task Completion

> **Status:** Implemented (PR #28). Scope delivered: `verifyingTool`
> decorator + `verification` metadata, `verify` crushrc builtin,
> end-of-turn gate with bounded retries, notebook `verified` ground
> truth. Deferred: `bash`-redirection/MCP-write coverage, gate-check
> parallelism, the Measurement section's telemetry counters.

Independent of the notebook selection work (`NOTEBOOK_QUALITY.md`)
but composes with stubbing (`TOOL_RESULT_PRUNING.md`) and intra-turn
boundaries (`INTRA_TURN_BOUNDARIES.md`, implemented — PRs #24, #25).
Scope: `agent.go` run loop, a new post-execution tool decorator, a
structured diagnostics accessor, `internal/notebook`, and a `verify`
builtin in crushrc.

## Problem

The agent loop treats the model's text output as evidence of task
completion. "Tests pass" is a token sequence, not a fact — the harness
has no way to distinguish a model that fixed something from a model
that produced fluent text claiming it did. This is the self-report
trust gap, and it is structurally different from the context-
management problems the rest of this directory covers: pruning and
stubbing manage what the model _sees_; verification manages whether
what the model _believes_ is true.

Everything downstream inherits this gap — with one precision:
`Succeeded` is already tool-grounded (`result != nil &&
!result.IsError`, `classify.go:60`); it records whether the tool ran
cleanly, not whether the task succeeded. The self-report gap lives
in `#decision` entries, entry prose claiming completion, and
anything promoted to cross-session memory. `Succeeded` and
verification-pass are also different axes — an edit can land
cleanly and still fail its check (see PR 4).

The general shape, regardless of domain:

1. **Claim** — the model asserts done/fixed/true.
2. **Check** — a deterministic external process evaluates the world
   (compiler, test runner, linter, type checker, LSP diagnostics).
3. **Ground truth signal** — the check's own output (exit code, diff,
   diagnostic set), not the model's interpretation of it.
4. **Gate** — the loop does not reach "done," and the notebook does
   not record success, until the signal confirms it.

The critical property: the gate reads the check's output directly.
Letting the model run `go test` and report "passed" moves the trust
gap one level down instead of closing it — the model can misread
output, or the harness can trust the summary over the exit code.

## What exists

- **Inline post-edit diagnostics — already shipped.** `edit.go:97-101`,
  `write.go:166-170`, and `multiedit.go:105-110` each call
  `notifyLSPs` + `getDiagnostics` after a successful mutation and
  append the raw output to the tool result. `bash.go:572`
  (`lspDiagnosticsForFailure`) appends project diagnostics to failed
  build/test commands. The model _already_ receives unmediated check
  output in-context. What is missing is the structured half: no
  pass/fail is computed, nothing is recorded, nothing gates on it.
- **Structured diagnostics are available underneath the string.**
  `getDiagnostics` (`diagnostics.go:130-180`) returns a formatted,
  sorted, truncated-at-10 string — unusable for a delta. But
  `client.GetDiagnostics()` (`diagnostics.go:139`) is
  `map[location][]protocol.Diagnostic`; a new accessor
  `fileDiagnostics(path) []protocol.Diagnostic` gives the delta a
  stable input while `getDiagnostics` keeps formatting the appended
  text.
- **Gate trigger site — and a field trap.** `agent.Stream` returns
  `fantasy.AgentResult`, but `Response.FinishReason` is NOT the
  terminal step's reason: `finalResponse` back-scans to the last
  step with non-blank text (fantasy `agent.go:341-351`). A clean
  `Stop` ending whose last step is textless reports an earlier
  `ToolCalls` step (false negative — gate skips a clean turn); a
  provider that reports `Stop` on tool-call steps — tolerated at
  fantasy `agent.go:552` — lets a `StopTurn`-halted turn report
  `Stop` (false positive — gate retries after a hook halt). The
  trigger is `result.Steps[len-1].Response.FinishReason` ==
  `FinishReasonStop` AND no `StopTurn` in the terminal step's tool
  results, with a `result != nil` guard for the Stream error path.
  The stored message reason can't substitute: `OnStepFinish` maps
  `StopTurn` endings to `FinishReasonEndTurn` too
  (`agent.go:1195-1206`). Enum note: `fantasy.FinishReasonStop` at
  this layer; `FinishReasonEndTurn` is the message-level name it's
  mapped to at `agent.go:1179-1180`.
- **Capture point:** `OnToolResult` (`agent.go:1154`) creates the
  `message.Tool` row and sets `IsError`.
- **Injection point:** `PrepareStep` (`agent.go:948`) rewrites
  `prepared.Messages` before every step — now also the per-step
  notebook rebuild/splice site from the intra-turn work; a gate
  that injects messages must compose with that splice.
- **Turn-end detection:** `OnStepFinish` (`agent.go:1171`) sees each
  step's finish reason.
- **Loop control:** `StopWhen` (`agent.go:1228`) holds the
  auto-summarize condition and `hasRepeatedToolCalls`
  (`loop_detection.go` — window 10, max 5 _identical_ calls). Three
  caveats: `StopWhen` can only stop the loop, never force another
  step; the detector misses verify-thrash because each retrying edit
  has a different signature; and it is per-`Stream` — `steps` resets
  on every recursive `a.Run`, so thrash across gate retries is
  invisible even with identical signatures. `VerificationAttempts`
  is the only bound.
- **Post-run pipeline — and an ordering trap.** The post-run
  goroutine spawns at `agent.go:1406`, re-lists messages, runs
  `flagPrunableToolResults`, then `generateRunEndSegments`
  (per-segment generation + per-segment mem0 sync). Segment
  coverage is one-shot: `GenerateSegmentEntries` commits entries +
  the processed marker in one transaction (`segments.go:207`), and
  a `SegmentProcessed` row is never regenerated
  (`notebook_segments.go:692`). The goroutine fires ~50 lines
  BEFORE the dequeue-adjacent position a gate would naturally take —
  gate outcome writes must be ordered before generation or the
  notebook permanently misses them. See The gate.
- **Never-stub for free:** every pass in `flagPrunableToolResults`
  skips `tr.IsError` (`stubs.go:236,249,290,338,381,430,473,525`).
  Note the mechanism distinction: gate outcomes land in `Metadata`,
  not in an error-flagged result — and stubbing only marks rendered
  content superseded, never touches stored `Metadata`, so
  `classifyEvents` sees the verification outcome regardless of stub
  state. The outcome survives either way; it just isn't carried by
  `IsError`.
- **Structured metadata precedent:** `ToolResult.Metadata`
  (`message/content.go:123`) already carries `{"hook":...}` via
  `mergeHookMetadata` (`hooked_tool.go:132`). A `"verification"` key
  is the same move — and must use the same `sjson.SetRaw` merge so it
  composes with, not clobbers, the hook key.
- **Post-hoc metadata precedent:** `mergeSupersededMarks`
  (`stubs.go:541-571`) does a read-modify-write whole-message Update
  to annotate stored tool results after the fact — exactly what
  landing a gate outcome on a stored edit result requires.
- **Orphaned results are inert:** a `message.Tool` result with no
  matching tool call is dropped at render and invisible to
  `classifyEvents` — `findToolResult` matches by `ToolCallID`
  (`classify.go:81`). A synthetic result row stores data nothing
  reads.
- **Re-entry machinery — and its scope trap.** Re-entry is literally
  `return a.Run(ctx, firstQueuedMessage)` (`agent.go:1563`) under the
  `sessionMu`/`BeginAccepted` handoff (`agent.go:1475-1551`), which
  already solves the dequeue→re-register cancel window.
  `skipRunComplete` (`agent.go:1523`) + `outerOwesRunComplete`
  (`agent.go:1533-1561`) already implement "don't publish a premature
  terminal event when the same RunID continues." **Corollary: any
  counter scoped to a single `Run` invocation resets on every gate
  retry** — the attempt budget must be threaded through
  `SessionAgentCall` (`agent.go:79`, add e.g. `VerificationAttempts
int`; the gate builds the follow-up call, so it self-propagates)
  rather than held in `Run` locals or session-keyed state.
- **The queue is not FIFO-until-run-end.** `drainQueueForStep`
  (`agent.go:528-553`) _folds_ queued calls that carry no `RunID`
  into the running turn mid-stream — they never reach the end-of-run
  dequeue. Only `RunID`-bearing calls stay queued — a drain-point
  truth: a non-`RunID` prompt submitted _during_ gate checks (the
  session still reads busy — `activeRequests.Del` hasn't run) lands
  behind the prepended retry at dequeue and folds into the retry
  turn's first drain. The retry still runs first, but the fold
  merges the user prompt into the verification-repair turn; if the
  model answers it and drops the fix, the gate fires again (budget
  permitting). Two consequences for the gate: (a) ordering
  contention at gate time is only against `RunID` calls queued
  before the last drain, and (b) a gate retry enqueued without a
  `RunID` while any run is active can be folded into a later run's
  step instead of gating — carrying the caller's `RunID` (the rule
  under The gate) doubles as the fold exemption.
- **Folded turns widen the gate's scan scope — resolved to
  whole-run.** One `Stream` can span multiple user turns (folded
  mid-run prompts each start one). The boundary is free:
  `preTurnMsgCount` is already captured (`agent.go:841`), so
  `msgs[preTurnMsgCount:]` — or equivalently the in-memory
  `result.Steps` tool results — is exactly this-run scope, folded
  turns included. Whole-run is the conservative default and needs
  no extra machinery.
- **Cancel-mark droppability is a feature, state it.** The dequeue
  filter (`agent.go:1487`) drops queued calls with `acceptSeq == 0`
  or `<=` the cancel mark. A gate retry enqueued that way is dropped
  on user cancel — the desired semantics (a cancel should kill
  retries) — and when it carries the caller's `RunID` the drop is
  not even silent: `publishCanceledQueueDrops` emits the cancelled
  `RunComplete` the waiter needs.
- **Post-run pipeline (updated for intra-turn merge):** the
  post-run goroutine now runs `generateRunEndSegments` — per-segment
  entry generation plus per-segment mem0 sync — instead of
  turn-grain `GenerateEntries`. `classifyEvents` is shared, so
  `EntryInput.Verified` flows through unchanged; each gate retry is
  a new turn, which also closes the previous open segment — every
  failed gate gets its own segment coverage for free.
- **Path/tool classification:** `writeToolNames`, `readToolNames`,
  `commandToolNames`, `toolCallFilePath` (`stubs.go:28-98`),
  `normalizedPath` (`stubs.go:147`).
- **Schema precedent:** `succeeded` and `error_headline` columns
  (migrations `20260911000000`, `20260912000000`) — per-entry
  ground-truth fields are an established pattern.
- **Config machinery:** crushrc builtins via `shell.RegisterBuiltin`
  - `ConfigBuilder` (`internal/shellconfig/`) — a `verify` builtin
    naming project check commands slots in by convention.

## Correction to the motivating memo

The memo assumes "a small model at 0.17x" classifies tool calls as
significant/trivial for the notebook. It does not:
`isSignificant`/`classifyEvents` (`notebook/classify.go:23-77`) is a
pure heuristic — tool name plus a 1000-byte read threshold. The
small model only writes entry prose (`generator.go:54`). For
verification this is the better design anyway: the edit-class →
check mapping should be deterministic, not an LLM call whose
borderline judgments drift under exactly the conditions verification
exists to catch.

## Design

### Where the check runs

A new `verifyingTool` decorator, sibling to `hookedTool`, applied at
the coordinator level — but PR 1 is a **migration, not an addition**:
the inline `notifyLSPs`/`getDiagnostics` calls move out of
`edit`/`write`/`multiedit` and into the decorator, which (a) appends
the raw output to `resp.Content` (the `result.Context` precedent,
`hooked_tool.go:98-103`) and (b) merges a `"verification"` metadata
key via the `sjson.SetRaw` pattern (so the `"hook"` key survives
either decorator ordering). Moving the calls matters: decorator
append + existing tool append would duplicate diagnostics in every
edit result.

Alternative considered: running the check inside `OnToolResult`.
Rejected — the decorator sees the `fantasy.ToolResponse` before
message conversion and can annotate content and metadata in one
place; `OnToolResult` would need to re-derive the call's inputs.

Sub-agent caveat, sharpened: sub-agent edits land in a **child
session** (`CreateTaskSession`, `coordinator.go:1840`) — the parent
gate's end-of-turn scan can't see them at all. But the gate fires
inside the sub-agent's own `Run` automatically (same code path), so
the only real decision is decorator wiring on the sub-agent toolset
(`wrapToolsWithHooks` skips `isSubAgent` at `hooked_tool.go:32` —
don't inherit that exemption blindly). A second gap: budget
exhaustion inside a sub-agent surfaces only through the task tool
result's text — the "surfaced, never silent" terminal state needs a
propagation line into the parent transcript or the parent records
an unverified clean.

### The pre-existing-diagnostics problem

`getDiagnostics` returns the file's _full_ diagnostic set plus
project diagnostics — as a formatted string. Two consequences:

1. **Baseline, not absolute — and not file-scoped.** Naively setting
   `passed = (error count == 0)` fails forever on a file that had
   errors before the edit — the gate would force re-entries on
   errors the edit didn't cause, the exact thrash generator this
   design exists to avoid. The decorator wraps the call, so snapshot
   diagnostics before `inner.Run`, snapshot after, and set `passed`
   on the **delta** — no new errors. Scope the delta to the whole
   `GetDiagnostics()` map, not just the edited file: a signature
   change breaks _callers_, and a file-scope `passed` stays green on
   the classic compile break. Baseline-over-project already excludes
   pre-existing errors elsewhere; the residual risk is settle noise
   from unrelated files, which is budget-bounded like the rest.
2. **The delta needs structured input.** Diffing the formatted
   string is unstable (sort order, 10-entry truncation, decoration).
   Compute the delta over `[]protocol.Diagnostic` from a new
   `fileDiagnostics(path)` accessor reading
   `client.GetDiagnostics()`; `getDiagnostics` stays purely for
   rendered output.

Settle caveat: the pre-edit snapshot can catch diagnostics still
settling from a _prior_ edit, making a fixed-then-recounted error
look "new" → false fail. `WaitForDiagnostics` before the baseline
snapshot mitigates; residual false fails are budget-bounded.
Never-opened caveat: a file no LSP client has opened yields an
empty baseline, so the first edit's diagnostics all read as new —
false fail on first touch. The baseline must `OpenFileOnDemand` +
`WaitForDiagnostics` before snapshotting, not just read
`GetDiagnostics`.

### Check selection by edit class

Deterministic classification on the touched path — biased toward
verifying, since a false negative (skipping a needed check) costs a
silently wrong "done" while a false positive costs one lint run:

| Edit class                  | Check                         | Runs      |
| --------------------------- | ----------------------------- | --------- |
| Source file, LSP handles it | diagnostics delta             | decorator |
| Any file mutation           | + every configured verify cmd | gate      |
| Go file on a tested path    | + targeted test (same pkg)    | gate      |

Declared verify commands are not a compile-check fallback — a user who
configures `make check` is declaring a project gate, and a `go.mod` or
`Makefile` edit breaks builds as readily as source does. They apply to
every mutation the write tools make; the diagnostics delta stays
LSP-gated and the package test stays `.go`-gated.

The Runs column is the spec: the decorator runs only what is free
inline — the diagnostics delta (already inline today). Anything that
spawns a build/test/lint process defers to the gate, once per turn
rather than per edit; the decorator records it `pending` in the
`"verification"` metadata — which is what "never ran" means to the
trigger below, distinct from _unverified_ (no check applies at all).

"Targeted test" = tests in the same package/directory as the changed
file, using the `normalizedPath` signal. Full suite is a periodic or
pre-completion check, not a per-edit tax — running it after every
edit re-introduces the cost problem the classifier exists to avoid.

Config surface: a `verify` builtin in crushrc (`shell.RegisterBuiltin`

- `ConfigBuilder`) naming the project's build/test/lint commands.
  No config → LSP-only verification, `passed` unset (recorded as
  _unverified_, distinct from _failed_). Optional: make `unverified`
  visible in-context — a one-line suffix on the tool result lets
  the model hedge instead of silently claiming done. Cheap;
  debatable.

Coverage gap to state plainly: `bash` redirections mutate files
constantly and carry no path signal — they escape the edit-class
table entirely, more often than MCP writes do. The observed-mutation
pass (`stubs.go` pass 2) detects them for _flagging_, but the gate's
"never ran" case can't see them either. Out of scope for v1; worth a
follow-up that diffs the filetracker read-set after bash calls.

### The gate

`StopWhen` cannot express "don't end the turn" — it only stops. The
gate lives at the run boundary, after `agent.Stream` returns and its
error path, and carries TWO ordering constraints:

- **Before the notebook goroutine spawn (`agent.go:1406`) — a
  flush, not just a position.** The post-run pipeline commits
  segment coverage transactionally and never regenerates — a gate
  outcome that lands after `generateRunEndSegments` read the result
  metadata is permanently absent from the covering segment's
  entries. Position alone doesn't close this: `messages.Update`
  debounces non-terminal writes (`defaultUpdateDebounce` = 33ms;
  `shouldFlushNow` fires on finish/tool-call/reasoning transitions —
  a metadata-only update to a `Role: Tool` message hits none), and
  `Get`/`List` read storage only, never the buffer. The gate's
  writes therefore need `a.messages.FlushAll(ctx)` before the
  `a.notebookEnabled` block — the deferred `FlushAll` at Run return
  is too late, the goroutine has already Listed. The same flush
  protects the reverse clobber: `flagPrunableToolResults` rewrites
  whole messages from its listed snapshot, and `mergeSupersededMarks`
  unions only `Superseded` — a flag write from a pre-gate snapshot
  drops `"verification"` outright. And the overlap is cross-run:
  the goroutine is detached (`context.WithoutCancel`,
  `agent.go:1396`), so run N's flag pass can still be rewriting a
  message while run N+1's gate writes it — unioning `Metadata` into
  `mergeSupersededMarks` is REQUIRED, not belt-and-suspenders, and
  the gate's own write must union stored `Superseded` for the same
  reason. Narrow the window further: read pending/failed from
  `result.Steps[i]` tool results' `ClientMetadata` — already in
  memory — and reserve the storage write for outcomes. Residual hole either way: mid-run segment
  generation can cover the failed edit's segment before the run
  ends — those entries carry decorator-level outcomes only, and the
  gate outcome is recorded by the retry turn's own segment instead.
  State it; don't fix it.
- **Before the queue dequeue at `agent.go:1477`.** If the user
  queued follow-ups during the run, the gate retry must prepend —
  verification settles before queued prompts run. The alternative —
  user prompts first — risks the gate never firing if the next
  prompt changes the task. Prepend is the defensible default. Note
  the queue is no longer FIFO-until-run-end (see
  `drainQueueForStep` above): prompts queued before the last drain
  are `RunID`-bearing, but a non-`RunID` prompt submitted after it —
  during the final step's tail or the gate checks — can also be
  sitting there; prepend puts the retry ahead regardless. The gate
  call itself must not be foldable: carrying a `RunID` is the
  exemption.

Trigger policy: fire only when the run ended on a clean model
stop — terminal step `FinishReasonStop` read off
`result.Steps[len-1]` (NOT `result.Response.FinishReason` — see the
field trap under What exists) with no `StopTurn` among the terminal
step's tool results. That exempts hook halts, permission denials,
and question-tool stops — all `StopTurn`-carrying `ToolCalls`
endings — plus non-`Stop` terminations (`Length`, `Other`,
`Unknown` — the run didn't cleanly claim done). `result` is nil
when `Stream` errors; guard before indexing steps.

On trigger, the scan sorts this run's write-tool results into three
states: _failed_ or _pending_ (a check was selected but deferred)
proceed to the gate; _unverified_ (no check applies — no LSP, no
config) is recorded and warned, never re-entered — otherwise the
gate thrashes on exotic stacks. With pending or failed checks:

1. Run the pending check(s) harness-side. `sessionAgent` holds no
   executor today — inject the check registry + shell handle from
   the coordinator (`internal/shell` exposes `RunAndCapture`
   independent of the bash tool). Either way the check runs without
   the bash tool's permission prompt: crushrc-declared `verify`
   commands are implicitly user-approved, a trust boundary worth
   stating in the builtin's docs (the `hook` builtin already shows
   how much unmediated shell surface a config file can carry). State
   a second boundary in the same docs: whether `verify` commands
   still pass the bash tool's banned-command blocking
   (`blockFuncs`, `bash.go:165`) — calling the shell package
   directly skips it unless wired in. And a third: `verify`
   commands run once per gate turn and once per retry — the builtin
   docs should require side-effect-safe commands.
2. **Land the outcome on stored metadata**, not in a synthetic tool
   result — an orphan `message.Tool` row is dropped at render and
   invisible to `findToolResult`. Read-modify-write the originating
   edit result's `Metadata` via the `mergeSupersededMarks` pattern so
   `classifyEvents` picks it up through the normal path. Same write
   when a gate re-run _passes_ — otherwise the edit keeps stale
   `failed` metadata and the notebook records
   `#verification-failed` on a turn that ended clean.
   (Alternative: emit a synthetic `verify` call+result pair so it
   renders — heavier, and the metadata update is needed anyway for
   staleness.) No by-tool-call-id message query exists — locating
   the stored row is a `List` + `ToolCallID` scan (the post-run
   goroutine already pays that `List`; share the snapshot) or a new
   sqlc query.
3. Enqueue a follow-up prompt containing the raw check output through
   `messageQueue` under `sessionMu`, prepended ahead of queued
   `RunID` calls — the dequeue at `agent.go:1477` then picks it as
   `firstQueuedMessage`, and cancel-mark handling plus the
   dequeue→re-register window come free. A bespoke direct `a.Run`
   call would have to reimplement both.
4. Each gate retry is a new turn — a new user message containing
   the check output and its own notebook entries; each failed gate
   is real history worth recording. But the retry's `RunID` is
   load-bearing, not cosmetic: every `Run` publishes exactly one
   `RunComplete{RunID}` (deferred at `agent.go:889-921`), and
   `outerOwesRunComplete` suppresses the outer publish only when a
   queued call carries the SAME `RunID`. A `RunID`-less retry lets
   the outer turn publish `RunComplete{orig}` at `agent.go:1561`
   before verification settles — `crush run` exits mid-gate and
   never reports the verified outcome. Rule: the retry inherits the
   caller's `RunID` when present (one terminal event after
   settlement; non-foldable; a cancel-drop still publishes a
   cancelled `RunComplete` via `publishCanceledQueueDrops`), none
   otherwise. And the retry is a CLONE of the caller's
   `SessionAgentCall` — `Prompt` replaced, `VerificationAttempts`
   incremented — not a from-scratch call: the struct carries
   `ProviderOptions`, all five sampling params, `NonInteractive`
   (which gates the `TypeAgentFinished` publish at
   `agent.go:1455`), and `OnAuthRefresh`; dropping them silently
   changes the retry turn's model parameters.

Check execution rules: (a) every check runs under a per-check
`context.WithTimeout` derived from the run ctx — a user cancel
aborts a hung check rather than waiting out the timeout; a hung
`go test` otherwise blocks `TypeAgentFinished` (`agent.go:1456`),
the deferred `RunComplete`, and the queued retry indefinitely;
timeout records `failed` with a duration headline. (b) Dedup pending checks by check identity — N edits in
one package produce N identical targeted-test pendings; the gate
runs each once. (c) Satisfy pending checks from observed bash runs:
if the model already ran the configured command this turn, its exit
code already answered the check — re-running is the design's largest
recurring waste. Note the verdict is NOT the result type: bash and
job_output report a non-zero exit as a text response (`bash.go`),
never an error result — the gate reads `exit_code`/`done` from
`BashResponseMetadata`/`JobOutputResponseMetadata` on the step's
`ClientMetadata` (a still-running backgrounded command has `done`
absent and is not a verdict). Exact command match only (a prefix
match invites `cmd && rm -rf` trickery), and only when the observed
run postdates the LAST write it covers — a test pass before the
last edit verifies nothing. (d) Truncate the retry prompt's check
output via `tools.TruncateHeadTail`/`MaxOutputLength` — a raw test
log can be megabytes.

`shouldSummarize` composes safely by ordering: the continuation only
re-queues when the stopped assistant message still has tool calls —
co-occurring with a gate trigger needs a provider that reports
`Stop` on tool-call steps. In that case the continuation appends
_behind_ the prepended retry: the retry runs first on summarized
context, the continuation follows as its own `RunID` turn.

UX affordance: the checks run synchronously inside `Run`, so
`TypeAgentFinished` (`agent.go:1456`) and the deferred `RunComplete`
don't fire until the gate settles — correct for `crush run`, but an
interactive session shows a bare spinner through `go test ./...`.
Emit a verify-in-progress notify/state so the TUI can say _why_ the
session is still busy. Adjacent flap: `TypeAgentFinished` publishes
before the dequeue — a queued retry means the session emits
finished→busy on every gate retry; suppress the notify when a retry
is prepended, or the TUI flickers per attempt.

**Retry budget via `SessionAgentCall.VerificationAttempts`** — the
gate builds the follow-up call, so incrementing the field there
self-propagates across the recursive `a.Run` boundary where any
`Run`-local counter would reset. On budget exhaustion the terminal
state is honest: "couldn't get this passing after N tries, last
failure: <headline>" — surfaced, never converted into a silent
pass. "Surfaced" needs a named channel — and the obvious one is
broken: `RunComplete.Text` reads `currentAssistant.Content()`
(`agent.go:905`), so a newly appended message never reaches it and
`crush run` exits with the last pre-gate text. Write the
exhaustion line into `currentAssistant` (it IS the final assistant
message of the run — semantically honest) or carry the text to the
publish path explicitly. A bare toast or notify is invisible to
non-interactive callers either way.

Why structural, not a prompt instruction: "always run tests before
claiming done" in the system prompt degrades like every other
in-context instruction — under compaction, in long sessions, or when
the model judges a check unnecessary. The lesson of the pruning work
applies directly: instructions in context are lossy under pressure;
harness mechanisms (the universal cap, the invariants file) are not.
The gate is control flow — `if !check_passed { continue }` — not a
request.

### Notebook honesty

`EntryInput` gains a verification field populated from tool-result
metadata during `classifyEvents`; `storeEntry` persists it alongside
`Succeeded`. Aggregation rule for entries covering multiple results:
**worst-wins** — `failed` > `unverified` > `verified`, with the
4-state metadata mapping down: `pending` counts as `unverified` — a
hook halt can strand pending checks on results whose gate never
fires, and pending is not a persisted verdict. The generator
stays free to summarize _what happened_ but the outcome claim comes
from the field: entries get a `#verified` / `#unverified` /
`#verification-failed` tag injected structurally (not left to
`extractTags` on generated prose), and the generator prompt states
outcome only as given. A `#decision` entry claiming success on an
unverified edit is the exact failure mode this closes.

Ordering dependency: the tags are only as good as the metadata at
generation time. Decorator outcomes (PR 1) sit on the result before
any generation — safe at any position. Gate outcomes are not: the
gate's write must land before the covering segment's generation
commits (the gate-before-goroutine ordering above), or the entry
keeps a missing/stale verdict forever — coverage is one-shot.

Metadata shape note: an edit can run more than one check
(diagnostics + targeted test), so `"verification"` is a list of
`{check, state}` where state is `passed` / `failed` / `pending` /
`unverified` — the `pending` entries are the gate's work list. Don't
let two writes race a scalar.

## PRs

### PR 1: Extract inline diagnostics into `verifyingTool` + metadata

- New decorator absorbs the `notifyLSPs`/`getDiagnostics` calls from
  `edit`/`write`/`multiedit` (and optionally generalizes
  `lspDiagnosticsForFailure` for `bash`). No double-append: the tools
  stop appending when the decorator owns it.
- New diagnostics accessor over `client.GetDiagnostics()` for delta
  computation — the project-scope delta wants the whole map, and
  `fileDiagnostics` already names a local inside `getDiagnostics`
  (`diagnostics.go:135`), so pick another name; `getDiagnostics`
  remains for rendered output only.
- Baseline snapshot before `inner.Run` — `OpenFileOnDemand` +
  `WaitForDiagnostics` first, or a never-opened file reads an empty
  baseline and the first edit false-fails; `passed` = no new errors
  on the project-scope delta (see the caveats above); metadata
  merged via `sjson.SetRaw`.
- Preserve the `IsError` early-return the tools have today
  (`edit.go:91`, `multiedit.go:100`): no diagnostics append and no
  verification write on a failed mutation.
- Marginal cost is small, not zero: the post-edit LSP calls happen
  today, but the baseline adds a _pre_-edit `WaitForDiagnostics` +
  snapshot per mutation — bounded by the settle timeout.
- Decide here whether the decorator wraps the sub-agent toolset too
  — `wrapToolsWithHooks` skips `isSubAgent`, and `buildTools` has
  separate call paths; deliberate choice, not an inherited
  exemption.
- Verify: unit test — edit introducing a type error surfaces
  diagnostics inside the edit's own tool result _and_ sets
  `passed:false`; edit on a file with pre-existing errors but a clean
  delta sets `passed:true`; first edit of a never-opened file does
  not false-fail; edit breaking a _caller_ in another file sets
  `passed:false`; hook + verify decorators stacked produce both
  metadata keys.

### PR 2: Check registry + `verify` builtin

- Edit-class table → check dispatch, including the decorator/gate
  split: diagnostics delta and cheap syntax run per-edit in the
  decorator; process-spawning checks are recorded `pending` in the
  `"verification"` metadata for the gate to collect. `verify`
  builtin for project build/test/lint commands; same-package test
  selection via `normalizedPath`.
- Verify: table-driven tests over (file class, config presence,
  LSP availability) → selected check.

### PR 3: End-of-turn gate + retry budget

- Post-`Stream` inspection on the terminal step —
  `result.Steps[len-1].Response.FinishReason == FinishReasonStop`
  with no `StopTurn` in its tool results — positioned BEFORE the
  notebook goroutine spawn; read pending/failed from the steps'
  `ClientMetadata` in memory; run the pending checks (per-check
  timeout, dedup by identity, satisfy-from-observed-bash, output
  truncated); write outcomes onto originating results' stored
  metadata (`mergeSupersededMarks` pattern — union stored
  `Superseded` AND `Metadata` so it can't clobber a concurrent
  `flagPrunableToolResults` update); `a.messages.FlushAll(ctx)`
  BEFORE the `a.notebookEnabled` block so the post-run `List`
  observes the verdicts; prepend the follow-up under `sessionMu`
  ahead of queued `RunID` prompts, inheriting the caller's `RunID`
  (non-foldable, suppresses the premature `RunComplete`);
  `SessionAgentCall.VerificationAttempts` carries the budget across
  the recursive `a.Run` boundary; each retry is its own turn, a
  clone of the caller's call with `Prompt` replaced; budget
  exhaustion writes into `currentAssistant` so it surfaces via
  `RunComplete.Text` for `crush run`.
- Verify: scripted provider test — model ends turn after a broken
  edit; run continues with check output; loop terminates at budget;
  stored metadata reflects the final outcome, not the intermediate
  failure; queued user prompt still runs after the gate settles; a
  `RunID`-waiting caller observes exactly one `RunComplete`, after
  the last retry; a `StopTurn`-halted turn does not trigger the
  gate; an edit with no applicable check records `unverified` and
  does not re-enter; a user prompt submitted during the gate folds
  into the retry turn, never ahead of it; the post-run goroutine's
  `List` observes the flushed verdicts; a `verify` command already
  run successfully via bash this turn is not re-run; a hung check
  times out as `failed`.

### PR 4: Notebook ground truth

- `EntryInput.Verified` with worst-wins aggregation, migration for a
  `verified` column — a separate axis from `Succeeded`: mechanical
  tool success and check-pass differ (an edit can land cleanly and
  fail its check), so reuse would conflate them. Structural
  verification tags, generator prompt constraint.
- Verify: entry for a failed-verification edit carries
  `#verification-failed` regardless of generated prose; a gate
  re-run that passes rewrites the stored outcome before generation —
  the test asserts the ordering, since coverage is one-shot.

## Measurement

Per session: verification runs by check type, gate re-entries,
budget exhaustions, `#verification-failed` entry count, and —
the real signal — decision/edit entries claiming completion whose
metadata disagrees (should be structurally zero).

## Non-goals

- Prompt-level "please run tests" nudges — the point of the design
  is that they don't work.
- Verifying mutations with no path signal — `bash` redirections and
  MCP writes escape the edit-class table. Observed-mutation flagging
  detects them after the fact; gate participation needs a follow-up
  (e.g. diffing the filetracker read-set around bash calls). Note that
  `lsp_rename`/`lsp_replace_symbol` ARE covered — they mutate via
  workspace edits (the canonical caller-breaker) and sit in the
  decorator's wrap set; `lsp_rename` gets a project-wide post-mutation
  refresh rather than anchor-file notify.
- Full-suite runs per edit — explicitly a periodic/pre-completion
  check only.
- Turn-state machine / durable task objects — the gate is a run
  boundary check, not a workflow engine.

## Risks

- **Diagnostics lag:** LSP publish is async — `WaitForDiagnostics`
  before the baseline snapshot and again post-edit, both with
  bounded timeouts; on timeout record _unverified_, not _passed_.
  Residual settle races (a prior edit's diagnostics arriving late)
  can false-fail — budget-bounded, logged, not silent.
- **No LSP, no config:** the common case on exotic stacks — the
  honest output is "unverified," and the gate should treat it as
  non-blocking (warn, don't re-enter) or verification becomes a
  thrash generator.
- **Re-entry loops:** `VerificationAttempts` is the only thing
  between a gate and an infinite edit→fail→edit cycle; re-entry
  must ride the `sessionMu`/`BeginAccepted` handoff or the cancel
  window reopens.
- **Metadata staleness:** any path that writes `passed` must also
  have a path that rewrites it — a gate pass after an earlier fail
  leaves a lie in the DB otherwise. The gate's read-modify-write
  also races `flagPrunableToolResults` whole-message Updates on the
  same message — the write must union stored `Superseded` marks, not
  just set its own key.
- **Coverage ordering:** segment generation commits coverage in one
  shot and never regenerates. The gate-before-goroutine ordering
  keeps run-end coverage honest, but mid-run segment closes can
  still predate the gate — accepted: decorator outcomes are already
  on those results, and the gate outcome lands on the retry turn's
  own segment entries.
- **Queue starvation:** prepending gate retries ahead of user
  follow-ups is correct but means N budgeted retries delay the
  user's queued prompts by N turns — acceptable at small budgets,
  worth a metric.
- **Cost:** each gate re-entry is a full turn of model tokens. The
  classifier bias toward verifying applies to _cheap_ checks; an
  expensive check should require a stronger trigger than a file
  extension.
