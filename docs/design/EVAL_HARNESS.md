# Eval Harness — Golden Trajectories & Paired Outcome Gating

> **Status:** Spec. Covers the corpus format (trajectory spec, check
> contract, experiment definition, run record, banding state) and the
> quarantine procedure. The runner itself is a later doc; the schema
> here is what it must honor.

## Goal

A fixed, rerun-able corpus of tasks — each a start state, a prompt
sequence, and a deterministic end-state check — that measures
_behavioral_ regressions in the agent before they ship. The corpus
replaces live-traffic acceptance gates ("watch edit-failure rate
after flipping the flag") with paired corpus comparisons run
pre-merge: same trajectories, same pinned model, flag on vs. flag
off. Detection becomes a controlled experiment instead of a
production observation, and every change on the context-management
roadmap gets its gate from the same artifact instead of re-deriving
"correct" per change.

## Problem

VCR cassettes assert request-shape equality: any prompt or
message-list change breaks matching, and a re-recorded cassette only
proves the shape changed — it says nothing about whether task
outcomes got better or worse (`PROMPT_OPTIMIZATION.md:1212`).
`cassette_patch_test.go` exists solely to rewrite request bodies
after a prompt change; that is the brittleness made literal.

A golden trajectory asserts the orthogonal property: given a fixed
start state and task, a checkable fact about the end state holds.
The path — tool-call order, prompt internals, stub strategy — is
free to vary. That is exactly the freedom every plan in this
directory needs.

## Layout

```
eval/
  corpus/
    <trajectory-id>/
      trajectory.json      spec (immutable)
      check.sh             the scoring function
      fixture/             optional synthetic start state
      reference.patch      known-good diff (required for regression)
      counterexample.patch known-bad diff (required for pass-guards)
  flags.json               declared flag projection + defaults — arm
                           options must be manifest keys; baseline keys
                           hash this projection
  experiments/
    <name>.json            a paired comparison (arms × corpus slice)
  results/
    <experiment>/<run>.jsonl   run records, append-only
  bands.json               characterization state (mutable, generated)
```

Two artifacts are deliberately separate: `trajectory.json` is
authored and reviewed (what the task is); `bands.json` is generated
and revised (what we've measured about it). Mixing them would make
every characterization pass a diff to reviewed files.

## trajectory.json

```json
{
	"id": "fix-nil-map-write",
	"schema_version": 1,
	"origin": {
		"kind": "regression | production | synthetic",
		"source": "PR #27 / bug report / session id — what this guards",
		"scrubbed": true
	},
	"start_state": {
		"kind": "fixture | git",
		"fixture_dir": "fixture/",
		"repo": "https://...",
		"ref": "<commit sha>",
		"setup": ["go mod download"]
	},
	"task": {
		"turns": ["the user prompt, verbatim", "optional follow-up"]
	},
	"check": {
		"script": "check.sh",
		"expect_start_state": "fail",
		"timeout_seconds": 300
	},
	"coverage": {
		"min_stub_stats.boundary_advances": 2
	},
	"requires": {
		"network": false,
		"lsp": ["gopls"],
		"tools": ["go", "rg"],
		"os": ["linux", "darwin"]
	},
	"budget": {
		"run_timeout_seconds": 900,
		"max_steps": 40
	}
}
```

- **`start_state`.** `fixture` copies `fixture/` into a fresh tempdir
  (the `createSimpleGoProject` precedent, `common_test.go:193`);
  `git` clones `repo` and checks out `ref` — a local mirror MAY back
  the clone via `--reference` for speed (`--shared` only applies to
  local sources and is wrong for `https://` remotes). `setup` runs
  once after materialization, before the agent. Every run gets a new
  tempdir — never the real repo.
- **`task.turns`.** Array, not a single prompt: long-session
  degradation (multiple boundary moves, ratchet resets) is a stated
  target and needs multi-turn replay. A single-prompt trajectory is
  the one-element case. Follow-up turns must be **path-agnostic** —
  "also add a test for X," never "yes, go ahead" or "now fix the
  test you broke" — because the agent's route isn't known at author
  time. Turns that react to the run require scripting the runner
  doesn't have (see Non-goals). Sequencing: turn _i_+1 is sent when
  turn _i_'s run completes **and any harness-initiated follow-ups
  settle** — the verify gate (and edge transitions generally,
  `HARNESS_TOPOLOGY.md`) can enqueue a forced turn at the run
  boundary; sending turn _i_+1 before the edge queue drains
  interleaves it with the repair turn and corrupts both. A turn
  that errors aborts the
  trajectory as `error`. Checks see only the final tree — a turn-1
  failure the agent self-heals by the end is invisible; authors
  wanting intermediate assertions need separate trajectories.
- **`origin`.** `scrubbed` must be `true` — a value, not just a
  present field — when `kind` is `production`, and for `regression`
  whenever `source` is a real session or bug report — the same
  secret-carrying pipeline (see Risks). For a fully synthetic
  trajectory it may be omitted. (`source`, not `ref` —
  `start_state.ref` already owns the commit-sha meaning.)
- **`check.expect_start_state`.** `"fail"` is the default and the
  strong form: the check must fail on the start state (the bug is
  real, the function doesn't exist yet). `"pass"` is allowed for
  non-regression guards ("don't break X") but is weaker — it cannot
  detect a vacuous check — so `counterexample.patch` is REQUIRED
  for `"pass"` trajectories: without it a pass-guard accrues
  p̂=1.0, joins `stable`, consumes catastrophic-tier runs, and can
  never fire (false coverage), so a pass-guard without one is a
  validation error at author time. Symmetric: `reference.patch` is
  required for `origin.kind: regression` — the fix diff is known
  by construction — else an always-fail check (`exit 1`) passes
  quarantine and lands as a permanently-failing trajectory. A
  check that passes on start state when declared `"fail"` is
  itself invalid. A `counterexample.patch` on a `"fail"`
  trajectory is a validation error, not ignored.
- **`coverage`.** Optional predicate declaring that the run actually
  exercised the guarded mechanism — evaluated from run-record
  telemetry (`stubStats.BoundaryAdvances`, `agent.go:1825`), not
  the tree. A run that doesn't meet coverage is `inconclusive`:
  not a pass (nothing was demonstrated), not a fail (nothing went
  wrong) — a non-sample excluded from `p̂` and every gate. This
  closes the runtime-vacuity hole: mechanism-targeted trajectories
  (e.g., boundary moves under a shrunk `notebook_raw_token_budget`)
  would otherwise pass on runs where the trigger never fired.
  Coverage-starved trajectories alarm like corpus shrinkage. The
  predicate grammar is closed — comparisons against run-record
  fields only (`stub_stats`, `recalls`, `steps`, `tokens`,
  `call_metrics`;
  `edge_firings` joins the grammar when named transitions land —
  `HARNESS_TOPOLOGY.md` promises them as assertable checkpoints),
  no arbitrary expressions — and coverage must be achievable within
  `budget`: a trajectory that can't reach its mechanism inside
  `max_steps` is permanently inconclusive. `stub_stats` carries a
  per-kind split — `stub_stats.kinds.<kind>` for `superseded`,
  `modified`, `deleted`, `duplicate`, `rerun`, `stale`, sparse in
  the record (only kinds that fired appear) — so a trajectory
  authored to trigger a specific kind can assert it fired instead
  of grepping message metadata. Like every `stub_stats.*` field the
  counters only exist once stubbing ran, so per-kind predicates are
  safe on stubbing-enabled arms only.
  `call_metrics.*` is the sequence-analysis record: after each run
  the preserved `session_db` is replayed into an ordered
  `tool_calls[]` (joined to results by call ID, ordered
  `created_at, rowid`) and reduced to per-call metrics on the
  record — `requests`, `calls`, `first_write_index`,
  `first_write_attempt_index`, `requests_to_first_edit`,
  `discovery_calls_before_write`, `files_viewed`,
  `read_files_rows`, `edit_failures` (cause-bucketed),
  `rereads{,_same_turn,_cross_turn}`, `canceled_calls`,
  `interrupted_calls`, `truncated_calls`, `view_directory_errors`,
  plus the flag-dependent forensics
  (`map_*`, `question_*`, `wrong_pointer_events`) that can never
  be predicates — `map` isn't registered in a `project_index`-off
  arm and `question` isn't registered headless, so those fields
  are absent-by-construction in one arm and same-arm-invariance is
  the registration rule. The analyzer runs inside `ExecuteRun`
  between `preserveSessionDB` and record append, so predicates are
  populated before `CoverageMet` reads them; an analyzer failure
  lands as `call_metrics_error` on the record —
  inconclusive-by-absence and analyzer-broke stay distinguishable.
  Semantics: `requests` counts billed requests — the finish-only
  canceled-turn placeholder is excluded, but a mid-stream cancel
  (real parts + `finish{canceled}`) still counts. Discovery and
  rereads count _attempts_ — the gate measures roundtrips spent
  before acting, so a failed read is a spent discovery attempt that
  just never joins the seen-set; but a re-view of a seen path with a
  different `offset`/`limit` is legitimate paging, not a reread —
  only a re-read of the same window counts. Window keys are the
  _requested_ window resolved to the tool's defaults
  (`offset=0`/`limit<=0` alias to the unpaged head), so a
  contained-window re-read of a short file still undercounts —
  conservative, never fabricated. The discovery set is
  enumerated: grep/glob/ls, the LSP read tools, sourcegraph, agent
  delegation, and view/read of unseen paths. Excluded deliberately:
  `recall`/`notebook_search` (notebook_enabled-gated — counting
  them would make this registered metric flag-variant),
  `fetch`/`agentic_fetch`/`web_fetch`/`web_search`/`download`
  (external fetching, not codebase discovery), and `map` (the
  metric measures what map replaces). The discovery cutoff is
  `first_write_attempt_index` — a canceled write placeholder keeps
  its tool name, so the window closes when the model tried to act,
  not only when a write landed; `requests_to_first_edit` anchors on
  the same attempt, since the gate measures time-to-action.
  `read_files_rows` is a loose bound on `files_viewed`, not an
  equality — the tracker also records writes and keys rows on the
  raw param path, so `read_files_rows >= files_viewed` is expected.
  The axes overlap deliberately: a
  pre-write `view`-on-directory lands in both
  `view_directory_errors` and `discovery_calls_before_write`.
  `crush eval analyze <session_db>` runs the same pass standalone
  and backfills old artifacts; the record carries `workdir` so a
  post-hoc analyze can anchor relative call paths. Known blind
  spots, both bash-side: discovery through `cat`/`find`/`rg`/`go doc`
  is invisible to tool-name classification so
  `discovery_calls_before_write` undercounts systematically, and
  mutations through `sed -i`/redirects/`download` are equally
  invisible so a bash-only mutating run shows `first_write_index=-1`.
  `inconclusive` does not
  consume a `runs_per_trajectory` slot: the runner resamples to N
  conclusive runs with an attempts cap (~2N) before flagging the
  trajectory coverage-starved. `error` resamples identically — a
  provider outage isn't data either — and exhaustion there flags
  the trajectory `error`-saturated (infrastructure, not coverage),
  a distinct alarm. Both exhaustion states are alarm labels on the
  experiment summary, not persisted trajectory states — the
  trajectory itself isn't broken.
- **`budget`.** Cost containment for derailed runs; both bounds are
  trajectory-wide — `max_steps` counts agent steps across the whole
  trajectory (`AgentResult` steps), not user turns, and
  `run_timeout_seconds` bounds the whole trajectory likewise. A run that hits either
  bound records `timeout` — a real failure mode, counted against
  pass rate but tagged separately for diagnosis (`steps` and
  `duration_s` say which bound). Exception: a run that timed out
  _while_ its API calls were already erroring classifies as
  `error` — the model didn't produce the outcome, the transport
  did. The carve-out needs a measurable rule — e.g., zero
  successful provider calls in the trailing window, or errors on
  over half of calls; the runner doc fixes the threshold — this
  doc requires the classification be deterministic.
- **`requires`.** Environment preconditions; the runner skips (and
  reports) trajectories whose environment can't honor them — the
  environment-rot surface, made explicit instead of silent.
  `network: false` means no _non-provider_ egress — the LLM call
  always needs the network; the flag constrains materialization,
  setup, fixtures, and check (a `git` start state under
  `network: false` must resolve `repo` to a local path/`file://`
  or a pre-seeded mirror — the clone itself is egress). `tools`
  declares binaries the check/agent need — a missing `go` or `jq`
  surfaces as a skip-report, not a check `error` masquerading as
  flakiness. Declared, not enforced — sandboxing is the runner's
  choice; an undeclared dependency surfaces as `error` in a
  network-free CI, which is the detection path.

## Check contract

`check.sh` runs with cwd = the materialized workdir **after** the
agent run, against the working tree as the agent left it (not HEAD —
the agent may commit; untracked files count too — the tree is the
outcome either way).

- The runner exports `EVAL_WORKDIR` (the materialized workdir, also
  cwd) and `EVAL_TRAJECTORY_DIR` (the corpus dir) — checks needing
  oracles or helper assets reference `$EVAL_TRAJECTORY_DIR`.
- Exit 0 → `pass`; non-zero → `fail`; cannot execute → `error`.
- stdout/stderr are captured into the run record. The last line
  matching `EVAL_JSON {"key": ...}` (trailing output may follow it)
  is parsed into the record's `check_detail` — most valuable on
  failure. A malformed EVAL_JSON line never changes the verdict;
  it just leaves `check_detail` absent.
- `error` is distinct from `fail`: the check ran and judged vs. the
  harness broke. Infra errors never count against the model.
- Checks must be deterministic and hermetic. `check.timeout_seconds`
  bounds them; a timed-out check is an `error`, not a `fail`.
- The tree contains the arm's generated config — `.crushrc` (model
  pin) and `.crush.json` (options delta) are harness materialization,
  identical modulo arm options and part of the run's environment, not
  the agent's output. A `git status --porcelain`-style tree-equality
  check must ignore them (`.crush/` data lives outside the tree).

The two-directional validity rule: for `expect_start_state: "fail"`,
the check MUST fail on unmodified start state and MUST pass on
start state + `reference.patch`. For `"pass"` trajectories with a
`counterexample.patch`, the symmetric rule: pass on start state,
fail on start + counterexample. Both runs are agent-free — this is
the quarantine pass, below.

## experiment.json — the arm structure

```json
{
	"name": "stub-superseded-flip",
	"model": "hyper/deepseek-v4-pro-0813",
	"temperature": 0,
	"corpus": ["*"],
	"runs_per_trajectory": {"stable": 3, "mid": 15, "uncharacterized": 5},
	"arms": {
		"control": {"config": {"options": {"notebook_stub_superseded": false}}},
		"treatment": {"config": {"options": {"notebook_stub_superseded": true}}}
	}
}
```

An arm is a generated config fragment written to `.crushrc` in the
materialized workdir — top of the directory precedence order
(`.crushrc` > `crushrc` > `.crush.json` > `crush.json`,
`load.go:957`), so it overrides any config a fixture or cloned repo
already carries. Two constraints follow because the merge is
per-key, not per-file (`load.go:1041`): a start state carrying
`.crushrc` collides with the arm at the same path — fixtures must
not ship one, or the runner appends to it — and a `crush.json`
fallback arm loses on any key a start-state shell config also
sets, so JSON arms are valid only when no start-state shell
config touches the flag under test. `crush.json` is the fallback
because the `option` builtin's key set is closed
(`options.go:195`) — flags without a builtin path (`notebook_*`
today) can't be expressed in crushrc until a generic passthrough
lands — while JSON itself is deprecated per AGENTS.md; the corpus
will outlive the format either way. Arm `config` is an
options-fragment only — providers, MCPs, and LSPs can't vary
between arms. The flag under test is
ordinary config, so A/B needs no code plumbing — but both arms
MUST run the same build under test (the
control-as-coincidence-detector logic depends on it), and the
runner isolates the run from user-level config (pinned
`HOME`/`XDG_CONFIG_HOME`): the global layers merge into every run
(`load.go:946`) and would otherwise leak a dev laptop's
options/MCPs/models into results. Credentials come from the eval
environment, never the corpus.

Agent subprocesses inherit the operator's full credential environment
(provider keys, `GITHUB_TOKEN`, `AWS_*`, `SSH_AUTH_SOCK`) — pinning is
on `HOME`/`XDG` config, not on secrets. `origin.kind: production`
prompts replay verbatim; run experiments that replay real prompts in a
credential-scoped environment or accept that exposure as the runner's
documented posture.

An arm may also carry `coverage` — predicates applied only to that
arm's runs, evaluated after the trajectory's shared coverage at the
same gate point (pass → `inconclusive`, and the run's
`check_detail.coverage_scope` records which scope starved it —
`"trajectory"` or `"arm"`). This is where flag-gated firing
assertions live: a treatment arm enabling `notebook_stub_superseded`
asserts `min_stub_stats.results: 1` so a run where stubbing never
fired is a non-sample, not evidence of nothing. The arm grammar is
the trajectory grammar plus the flag-dependent `call_metrics` fields
(`map_*`, `question_*`, `wrong_pointer_events`, `read_files_rows`) —
inside an arm scope flag-dependence is the point, not a footgun.
Firing assertions are for arms where the mechanism firing is
_required_ evidence; they are wrong where firing is the measured
signal — the `project-index` experiment's treatment arm carries no
`map_*` predicate because map non-adoption is itself a datum, not a
non-sample.

Experiment validation catches the arm-level starvation traps it can
see: `min_` over `stub_stats.*`/`recalls.*`/`call_metrics.map_*` on an
arm that explicitly sets the gating option `false`, and `min_` over
`call_metrics.question_*` on any arm (the question tool is
interactive-only — headless runs never register it). The
mirror-image guard lives in trajectory validation: a shared `min_`
over `stub_stats.*` or `recalls.*` is a load error because it would
starve the arm where the flag is off — move it to arm coverage.
`max_` stays legal unscoped (bounding both arms is meaningful). Runs
of an arm with no coverage block — `baseline` characterize runs
always — see trajectory predicates only.

`corpus` selects trajectory ids by glob (`["*"]` = everything) or
`"band:<name>"` for a band slice — `"band:stable"` is how the smoke
tier expresses its corpus. `quarantined` is excluded even under
`*` — and `band:quarantined` is never selectable: re-validating a
rotted trajectory needs agent runs, which is a runner mode
(characterize), not an experiment arm.

`model` + `temperature` pin at the experiment level —
`temperature` is required (unpinned arms land in a `default`-
temperature baseline cell no characterize output joins); run
characterize at the temperature your experiments pin — self-seeding
heals a mismatch, but the first run's catastrophic tier stays dark.
`temperature` via `SessionAgentCall.Temperature` (`agent.go:94`);
`SessionAgentCall` carries no model field, so `model` rides the
generated `.crushrc` as a `model` builtin identical in both arms
— options-only constrains what may _differ_ between arms, not
what may be set — or runner-side agent construction. Both arms
run the same pinned model interleaved in time —
alternating per run, per-trajectory batch at coarsest, auditable
via `started_at` in the run record — so provider-side drift can't
alias into the comparison: it lands on both arms equally and
cancels in the pairing. Two honest edges: pairing cancels
_time-correlated_ drift only — a provider change that interacts
with the flag under test doesn't cancel — and interleaving is
itself load-bearing: it's what makes within-trajectory runs
approximately exchangeable, which the permutation test assumes.

## Run record (results/\*.jsonl)

```json
{
	"experiment": "stub-superseded-flip",
	"trajectory_id": "fix-nil-map-write",
	"arm": "treatment",
	"run_index": 3,
	"outcome": "pass | fail | error | timeout | inconclusive",
	"check_detail": {},
	"started_at": "2026-09-13T22:00:00Z",
	"duration_s": 142,
	"steps": 6,
	"tokens": {"input": 0, "output": 0, "cache_read": 0, "cache_write": 0},
	"stub_stats": {
		"invalidations": 1,
		"results": 4,
		"saved_bytes": 12803,
		"boundary_advances": 2,
		"kinds": {"superseded": 3, "stale": 1}
	},
	"recalls": {"result": 0, "entry": 0, "empty": 0, "cross": 0},
	"session_db": "results/<experiment>/artifacts/<trajectory_id>-<arm>-<run_index>.db",
	"env": {"crush_sha": "...", "model_resolved": "...", "go": "1.25", "os": "darwin", "content_hash": "..."}
}
```

Sources, all existing: `fantasy.AgentResult` (turns/steps/usage)
from `agent.Run`; `stubStats` per session (`stubs.go:60`); recall
counts from `notebook.Stats` (`notebook.go:103` — the
`ResultRecalls`/`EntryRecalls`/`EmptyRecalls`/`CrossRecalls` split,
already wired into step telemetry at `telemetry.go:85`), which is
the sufficiency breakdown that matters — a flat count can't
separate "stub lost needed content" from "notebook entry was thin."
`session_db` is preserved per run under `results/<experiment>/` —
the failed-run debugging artifact is the full message/tool trace,
free. `started_at` is not decorative: it makes the arm interleave
auditable (the drift-cancellation claim depends on it) and answers
"which runs sat inside the provider outage." `env` is the forensic
record for environment rot: when a trajectory rots, the diff
between last-green and first-red env blocks is the first place to
look. `experiment` is `"_characterize"` (reserved sentinel) for
genesis and re-characterization runs — non-comparison samples flow
through the same record pipeline. `run_index` is the attempt index,
not the conclusive index — sparse under resampling, since
`inconclusive`/`error` attempts consume indices but not slots.
Runs execute the full product surface — verify gate, edges,
arm-config hooks — so `steps` counts gate-forced retries and the
end-state check correctly credits self-repair (that is what
ships; a regression the gate reliably heals never reaches users).

Outcome precedence when several apply: `error` (transport) >
`timeout` (budget) > `fail` > `inconclusive` (coverage unmet) >
`pass` — derailment is a real outcome regardless of whether the
mechanism fired (a `timeout` run never reaches `check.sh` — the
bound preempts the verdict), and coverage unmet converts `pass` to
`inconclusive` only: a check-fail is a real outcome whether or not
the mechanism fired. Coverage exists to catch vacuous passes — a
flag that suppresses boundaries AND breaks the task would produce
fails classified inconclusive, invisible in p̂, if `inconclusive`
outranked `fail`.

Exclusions are only safe if class membership is orthogonal to the
arm — and for mechanism flags it isn't by construction: a flag
that suppresses boundary advances lands in `inconclusive`, a flag
that inflates prompts into provider 400s lands in `error`, and
resampling to N conclusive then conditions each arm on a
differently-selected subsample. So the experiment summary carries
per-arm counts of every excluded class, and a significant
arm-differential in `inconclusive`-or-`error` rate is a first-class
signal — alarmed like a gate, not absorbed. The differential is
tested per trajectory — Fisher's exact on the excluded-class
counts, for symmetry with the catastrophic tier — as a third
multiple-comparison family with its own correction; corpus-pooling
it would be insensitive to one trajectory going all-inconclusive.
(The arm-side differential is the counterpart of the
trajectory-side coverage-starved alarm.)

## bands.json — characterization state

```json
{
	"_schema_version": 1,
	"fix-nil-map-write": {
		"band": "stable | mid | uncharacterized | quarantined",
		"quarantine_reason": "flaky | vacuous | miscalibrated | suspect_check | never_passed",
		"content_hash": "<sha of check.sh + trajectory.json + fixture/ + patches>",
		"last_characterized": "2026-09-10",
		"baselines": {
			"<model>": {
				"<baseline-config-hash>": {"passes": 29, "n": 30, "last_run": "2026-09-10"}
			}
		}
	}
}
```

Baseline stats are **keyed, not singleton**: `baselines` maps
model → config-hash → counts (nested — model names contain `/`),
because control-equivalent is experiment-relative — post-A-flip,
experiment-B's control runs `{A:on, B:off}` are baseline for B but
not for A. The config hash covers a **projection of the options
experiments may touch** (the context-management flag set —
a declared corpus-level manifest, not "whatever any experiment
happens to set": extending the list re-keys every baseline, a
third darkness trigger alongside re-pins and flips), not the
whole options map — unrelated option churn shouldn't rotate
baselines. Baseline keys bound non-signal variance only:
`crush_sha` is deliberately absent — the cross-build delta is the
thing being measured, and keying on it would darken the tier on
every commit; the rolling window and the control arm carry
build drift instead. Band assignment reads the key matching the
current default condition. Fisher needs integer cells, so
`passes`/`n` are stored and `p̂` is derived. `content_hash` scopes samples to the corpus revision:
p̂ is conditional on `check.sh`, the fixture, and the patches as
much as on the model — a check fix invalidates samples scored by
the old function (stricter manufactures regressions; looser hides
them), so a hash change triggers re-characterization like a re-pin.
Within a key, `passes`/`n` count **control-equivalent runs only**
— treatment runs can't pollute the baseline the catastrophic tier
tests against, and a re-pin starts a new key (see Risks). The
baseline window is rolling — `min(K runs, D days)` (K and D are
characterization constants, not per-trajectory fields):
a count bound alone retains arbitrarily old runs in quiet periods
and under-bounds `crush_sha` drift — and `n` in the record is the
windowed count, since that's what eligibility consumes.
Thresholds carry hysteresis — but asymmetric: demotion on one bad
characterization, promotion on two consecutive good ones, so a
`stable` trajectory producing a suspect characterization doesn't
keep catastrophic eligibility through the confirmation window.
Band assignments freeze at experiment
start, so a hovering trajectory can't flap `runs_per_trajectory`
out from under an in-flight experiment. The boundary positions
themselves (what `p̂` and `n` make `stable`) belong to the
characterization doc, not this schema — the record only needs
enough to recompute.

Bands are **provisional, not ground truth**: the N=5 genesis pass
has a uselessly wide CI, so a true-p=0.75 trajectory can draw 5/5
and misroute to `stable`. Bands are revised from accumulated run
records — every paired comparison adds samples as a byproduct — and
the `stable` band gets re-characterized every N full-corpus cycles
rather than trusted from genesis.

## Quarantine procedure (precedes all gating)

Per trajectory, no agent runs involved:

```
materialize start_state
for m in 1..M:                      # M ≈ 5
    run check.sh → must all agree with expect_start_state
if reference.patch exists:
    re-materialize; apply reference.patch
    for m in 1..M:
        run check.sh → must all pass
if counterexample.patch exists:
    re-materialize; apply counterexample.patch   # pass-guards only
    for m in 1..M:
        run check.sh → must all fail
```

Verdicts: inconsistent results → `quarantined` (flaky check);
pass-on-start where `fail` expected → `quarantined` (vacuous check);
fail-on-reference → `quarantined` (check doesn't recognize a known-
good state — mis-calibrated, worse than flaky because it's
confidently wrong); pass-on-counterexample → `quarantined`
(vacuous pass-guard — the symmetric counterpart of pass-on-start:
the check can't see bad). Both vacuous directions share
`quarantine_reason: "vacuous"` — the quarantine report records
which state failed. Quarantined trajectories are excluded from every
gate until the check is fixed and re-validated. M=5 is a floor,
not a guarantee — a check wrong 10% of the time sails through at
~0.9⁵≈0.59 per state; `suspect_check` (Risks) is the empirical net
for what quarantine misses. For `regression`
trajectories this pass IS the artifact: "fails on old, passes on
new" is both the validity check and the reason the trajectory
exists — corpus-building and validation are one pass, not two.

## Gating (what the schema anticipates, not what it implements)

- **Pairing is at trajectory level.** Provider APIs offer no usable
  seed; the shared-variance cancellation comes from trajectory
  identity (same task difficulty hits both arms), not shared
  randomness. Analysis: per-trajectory rate difference `d_t =
p̂_treat − p̂_ctrl`, paired permutation test over `{d_t}`.
- **Two-tier gate.** (a) Catastrophic: a `stable`-band trajectory
  collapsing to ~0/N. The test is one-sided Fisher's exact of
  current arm vs. the trajectory's **accumulated baseline** —
  `bands.json` p̂ over control-equivalent runs under the same model
  pin, where control-equivalent means the run's effective config
  matches the **baseline condition** — flag-off semantics: genesis
  runs, and arms that set only the flag under test to its
  pre-experiment default. (Not the build's literal defaults —
  those shift at a flip, and post-flip `flag: false` is no longer
  default yet is still baseline.) — and EXCLUDES the current
  experiment's own control arm, which stays clean as the
  coincidence detector rather than double-dipping into the
  denominator it helps interpret. The detector is collapse-grade
  only — at N=3, control 2/3 against a 29/30 baseline is p≈0.18,
  so moderate baseline drift slips through silently and inflates
  the apparent treatment effect; the rolling window is the
  mitigation. Bonferroni across the FULL
  stable band, not the eligible subset — correcting across
  "eligible" would be circular, since eligibility is defined by
  reaching that α. The within-experiment arm comparison CANNOT
  work here: 0/3 vs 3/3 is p=0.05 one-sided (0.10 two-sided),
  never clearing a corrected α≈0.0017. Baseline-vs-current does
  fire (0/3 vs ~29/30 historical ≈ p=7e-4). Eligibility is
  computed, not a fixed n: a trajectory qualifies when a 0/N
  result would reach corrected significance against its baseline —
  a function of baseline n, p̂, arm N, and the corrected α (i.e.,
  stable-band size at freeze) (p̂=0.95, n=20: 0/3 gives p≈2.3e-3 >
  1.7e-3 — does not fire even on total collapse; and at
  stable-band N=3 the tier effectively protects only near-pristine
  baselines — 0/3 vs 28/30 ≈ p=1.8e-3 still misses — so a p̂=0.95
  trajectory is not collapse-protected) — and
  is frozen at experiment start alongside band assignments. Until
  eligible, a trajectory's runs are characterization, not gating.
  This trades pairing for power, so it inherits band-staleness
  risk — accepted, because the control arm is the coincidence
  detector: if control collapses too, suspect trajectory
  rot/model drift, not the change under test. (b) Diffuse:
  permutation test over `{d_t}` restricted to mid +
  uncharacterized bands — within each trajectory, pool both arms'
  runs and re-split into arms of the observed sizes (runs are
  exchangeable under the null; no seeds needed), not a sign-flip
  on `d_t` alone. The corpus statistic is the mean of `d_t`
  (equivalently the sum, for fixed T); each replicate independently
  re-splits ALL trajectories simultaneously and recomputes it —
  the null is corpus-level, not per-trajectory p-values — and the
  two tiers are separate multiple-comparison families. `d_t`
  variances differ ~3× across bands (mid at N=15 vs uncharacterized
  at N=5) — valid under permutation but suboptimal; a precision-
  weighted statistic is a cheap refinement the runner may adopt.
  Stable-band `d_t`'s are excluded
  not because their grid is coarse — uncharacterized at N=5 is
  equally coarse — but because they're near-degenerate at the
  ceiling (p̂≈1 ⇒ d*t≈0 almost surely), which dilutes the signal
  pairing exists to surface. The same degeneracy applies at the
  floor — p̂≈0 ⇒ d_t≈0 — so never-passing trajectories (beyond-
  model task, or an always-fail check) route to `quarantined` with
  `quarantine_reason: "never_passed"` rather than entering the
  diffuse pool via `uncharacterized`. Threshold: zero passes in
  the trailing ≥10 conclusive runs, counted per baseline key —
  genesis 0/5 doesn't qualify (a real p=0.3 task gets ejected
  ~17% of the time at 0/5; 0/10 drops it to ~3%), so it stays
  `uncharacterized` and dilutes the pool for at most a cycle —
  deliberately, since early ejection costs more signal than the
  dilution does. A trajectory \_with* lifetime passes that hits
  the same streak is rot, not beyond-model — that routes through
  `suspect_check`/environment alarms, not `never_passed`.
- **Smoke tier is a different rule, not a weaker one.** Stable-band
  only (`"band:stable"` corpus), gated on the catastrophic pattern.
  A single failure at p=0.95, N=5 is a 23% false alarm; smoke
  catches collapses, the nightly run catches drift. Its N is its
  own knob (5 in that example), independent of the nightly
  experiment's `runs_per_trajectory` — smoke is a collapse alarm,
  not a sample for `p̂`. It fires on the observed collapse pattern
  (0/N on a stable trajectory — strict; the 23% false-alarm math
  above is why it's not ≥1 failure), not the eligibility-gated
  baseline Fisher — smoke must work where baselines are thin.

## Benefit, quantified

Three separable wins, each with its own math. Numbers below are
illustrative but the formulas are the decision rule.

### 1. Exposure cost of the alternative

Today, acceptance criteria like `TOOL_RESULT_PRUNING.md`'s
"edit-failure rate flat or better" are measured on live traffic —
the regression has to reach users before it can be measured. To
detect a baseline failure rate f₀ shifting to f₁ on live sessions
(one-proportion test, α=0.05, 80% power):

```
n = (z_α·√(f₀(1−f₀)) + z_β·√(f₁(1−f₁)))² / (f₁−f₀)²
```

- f₀=5% → f₁=7%: **n ≈ 820 sessions** — ~820 users run the
  regressed build before the shift is even detectable.
- f₀=5% → f₁=10% (a doubling): **n ≈ 150 sessions**.

The corpus equivalent: T=30 trajectories × N=10 runs × 2 arms =
600 agent runs, zero user exposure, and re-runnable for the next
change at marginal compute cost only.

### 2. Pairing detects systematicity, not just magnitude

A diffuse regression — many trajectories each degrading slightly —
is the dangerous case and the hardest to see in aggregate. Suppose
the treatment arm drops every mid-band trajectory's p by 0.10:

- **Pooled comparison** (all runs as one Bernoulli sample): the
  aggregate diff is −0.10 with pooled SE ≈ √(2·p̄(1−p̄)/(N·T)) —
  borderline at affordable N·T.
- **Paired test** (per-trajectory d*t = p̂_treat − p̂_ctrl): under
  the null, sign(d_t) is a fair coin. Twenty trajectories all
  landing negative is p = 2⁻²⁰ — but that's the extremity of the
  observed pattern under the null, not detection probability:
  under a uniform −0.10 shift at T=20, N=15, the mean-d_t
  permutation has ~70% power (a sign gate ~45%), so T/N must be
  provisioned for the effect size that matters. A real regression
  is \_systematic*, and sign-consistency across trajectories is the
  signal pooled statistics structurally discard.

This is why the gate is a permutation test over `{d_t}` rather than
a z-test on pooled pass rates: the information lives in the
per-trajectory pattern, not the sum.

### 3. Amortization and compounding coverage

Cost per decision:

- Live measurement: n_live user-exposed sessions + wall-time for
  organic traffic, **re-paid in full for every change**.
- Corpus: 2·T·N compute runs, **flat per change forever**.

The roadmap already queues ≥4 decisions against this corpus
(`notebook_stub_superseded` flip — TOOL_RESULT_PRUNING acceptance;
pressure-ladder triggers — CONTEXT_WINDOW_SAFETY; compaction —
NOTEBOOK_QUALITY; prompt rewrites — PROMPT_OPTIMIZATION). Corpus
build cost amortizes over all of them; the first comparison repays
it. And coverage compounds: every regression fix deposits a
permanent trajectory, so the corpus's protected surface grows
linearly with project history while cost per check stays flat.
Live measurement has no memory — it can never accumulate.

## Non-goals

- **Model-path replay.** No VCR anywhere near the LLM calls —
  cassettes freeze the path, which is the thing that must vary.
  Non-LLM HTTP (sourcegraph, downloads) may still be replayed for
  hermeticity, optionally.
- **Path assertions.** Nothing here inspects tool-call order or
  prompt shape; that's what cassettes already cover.
- **Per-trajectory diffuse gating.** Unaffordable at useful N; the
  gate is corpus-level by design.
- **Multi-arm experiments.** The schema permits >2 arms; the
  analysis is defined for two. Multi-arm needs its own correction
  story — defer until needed.
- **Reactive/scripted turns.** `task.turns` is fixed at author time.
  Turns conditional on what the agent did (branching dialogs,
  "approve the plan" responses) need a turn-runner with agent-state
  visibility — a real feature, not a schema field.

## Risks

- **Corpus composition drift.** All-stable corpora catch only
  collapses; a deliberate mid-band fraction is load-bearing.
  Curation tracks band mix against a soft `mid` floor (e.g., ≥30%)
  so the composition is auditable, not vibes.
- **Out-of-corpus regressions.** The gate covers the corpus and
  nothing else — live-traffic telemetry stays on as the tripwire
  for surface no trajectory protects. The harness replaces live
  measurement _as the gate_, not as observability.
- **Environment rot.** Git SHAs don't pin toolchains. `requires` +
  `env` blocks bound it; residual risk is quarantined trajectories
  silently shrinking the corpus — alarm on corpus-size drop, not
  just on failures.
- **Agent-state-conditioned check flakiness.** Quarantine exercises
  two agent-free states; checks flaky only on agent-produced states
  (port binding, races keyed to implementation choices) pass
  quarantine, then silently depress `p̂` and masquerade as `mid`
  band. Mitigation: consecutive within-arm pass/fail alternation
  flags the check — recorded as `quarantined` with
  `quarantine_reason: "suspect_check"` (the reason field is its
  schema home, not a fifth band) — instead of absorbing into the
  estimate.
- **Pinned-model shelf life.** A pinned dated model has a shorter
  life than the corpus: when the provider deprecates or pulls it,
  every experiment breaks at once — and re-pointing `model` is not
  the whole fix, because baselines are model-conditional. The `p̂`
  history the catastrophic tier tests against was generated by the
  old model; a re-pin means re-characterizing, treating prior
  history as a different trajectory's data. Consequence worth
  naming: after a re-pin, no baseline is eligible — the
  catastrophic tier is dark until re-characterization completes.
- **Every merged flip darkens the tier, not just re-pins.** The
  keyed-baseline consequence is routine, not exceptional: when
  flag A's experiment passes and the flip merges, the shipped
  default becomes `{A:on}` — so experiment B's baseline condition
  `{A:on, B:off}` is a _new_ key with zero history, and the same
  is true for every other pending experiment. The darkness is
  partially self-seeding — the merged experiment's treatment arm
  runs are exactly control-equivalent samples under the new
  default and join its baseline key — but that's only N per
  trajectory, still short of eligibility. Two alternatives if the
  dark window proves too long: a canonical reference config (all
  experimental flags off, independent of shifting defaults — keeps
  baselines warm but lets control diverge from what users run), or
  continuous `_characterize` runs against the current default so
  the new key arrives pre-populated.
- **Production-derived prompts carry secrets.** `origin.scrubbed`
  is a required field, not a courtesy — CI runs make the corpus
  leave the machine.
