// Package eval implements the golden-trajectory harness described in
// docs/design/EVAL_HARNESS.md: a fixed, rerun-able corpus of tasks — each a
// start state, a prompt sequence, and a deterministic end-state check —
// plus the paired-experiment machinery that gates changes on corpus
// outcomes instead of live traffic.
//
// The package owns the corpus format (trajectory.json, experiment.json,
// run records, bands.json), the quarantine procedure, and the runner.
// Agent runs are driven through the crush binary itself so both arms of
// an experiment exercise the same build under test.
package eval

import "time"

// Outcome is the per-run verdict.
//
// Precedence when several apply: error > timeout > fail >
// inconclusive > pass. Derailment (error, timeout) preempts the check;
// a check-fail is a real outcome whether or not coverage fired;
// coverage unmet converts pass to inconclusive only.
type Outcome string

const (
	OutcomePass         Outcome = "pass"
	OutcomeFail         Outcome = "fail"
	OutcomeError        Outcome = "error"
	OutcomeTimeout      Outcome = "timeout"
	OutcomeInconclusive Outcome = "inconclusive"
)

// Conclusive reports whether the outcome counts toward pass-rate
// estimates: excluded classes (error, inconclusive) are non-samples.
func (o Outcome) Conclusive() bool {
	return o == OutcomePass || o == OutcomeFail || o == OutcomeTimeout
}

// Band is the characterization state of a trajectory.
type Band string

const (
	BandStable          Band = "stable"
	BandMid             Band = "mid"
	BandUncharacterized Band = "uncharacterized"
	BandQuarantined     Band = "quarantined"
)

// QuarantineReason enumerates why a trajectory is quarantined.
type QuarantineReason string

const (
	ReasonFlaky         QuarantineReason = "flaky"
	ReasonVacuous       QuarantineReason = "vacuous"
	ReasonMiscalibrated QuarantineReason = "miscalibrated"
	ReasonSuspectCheck  QuarantineReason = "suspect_check"
	ReasonNeverPassed   QuarantineReason = "never_passed"
)

// Arm names are load-bearing across ExecuteRun callers, gate pairing,
// and the condition-key join — const, not literals.
const (
	ArmControl   = "control"
	ArmTreatment = "treatment"
	// ArmBaseline is the characterize/smoke arm name — runs under the
	// empty arm, i.e. the true default condition.
	ArmBaseline = "baseline"
)

// CharacterizeExperiment is the reserved experiment name for genesis
// and re-characterization runs — non-comparison samples that flow
// through the same run-record pipeline.
const CharacterizeExperiment = "_characterize"

// Trajectory is the authored spec: what the task is.
type Trajectory struct {
	ID            string     `json:"id"`
	SchemaVersion int        `json:"schema_version"`
	Origin        Origin     `json:"origin"`
	StartState    StartState `json:"start_state"`
	Task          Task       `json:"task"`
	Check         Check      `json:"check"`
	Coverage      Coverage   `json:"coverage,omitempty"`
	Requires      Requires   `json:"requires,omitempty"`
	Budget        Budget     `json:"budget,omitempty"`
}

// Origin records what a trajectory guards.
type Origin struct {
	Kind     string `json:"kind"` // regression | production | synthetic
	Source   string `json:"source,omitempty"`
	Scrubbed *bool  `json:"scrubbed,omitempty"`
}

// StartState describes materialization.
type StartState struct {
	Kind       string   `json:"kind"` // fixture | git
	FixtureDir string   `json:"fixture_dir,omitempty"`
	Repo       string   `json:"repo,omitempty"`
	Ref        string   `json:"ref,omitempty"`
	Setup      []string `json:"setup,omitempty"`
}

// Task is the fixed prompt sequence.
type Task struct {
	Turns []string `json:"turns"`
}

// Check is the scoring-function contract.
type Check struct {
	Script           string `json:"script"`             // relative to trajectory dir, e.g. "check.sh"
	ExpectStartState string `json:"expect_start_state"` // fail | pass
	TimeoutSeconds   int    `json:"timeout_seconds,omitempty"`
}

// Coverage is the closed predicate grammar over run-record fields:
// keys are <op>_<field> where op is min|max and field is a dotted
// run-record path (steps, tokens.output, stub_stats.boundary_advances,
// stub_stats.kinds.deleted, recalls.entry). A run that misses any
// predicate is inconclusive (passes only; fails stand).
type Coverage map[string]float64

// Requires declares environment preconditions. Declared, not enforced:
// an undeclared dependency surfaces as error, which is the detection
// path.
type Requires struct {
	Network *bool    `json:"network,omitempty"` // non-provider egress allowed
	LSP     []string `json:"lsp,omitempty"`
	Tools   []string `json:"tools,omitempty"`
	OS      []string `json:"os,omitempty"`
}

// Budget bounds a derailed run, trajectory-wide.
type Budget struct {
	RunTimeoutSeconds int `json:"run_timeout_seconds,omitempty"`
	MaxSteps          int `json:"max_steps,omitempty"`
}

// Experiment is a paired comparison: arms × corpus slice.
type Experiment struct {
	Name              string         `json:"name"`
	Model             string         `json:"model"` // provider/model, e.g. "hyper/deepseek-v4-pro-0813"
	Temperature       *float64       `json:"temperature,omitempty"`
	Corpus            []string       `json:"corpus"` // globs or "band:<name>"
	RunsPerTrajectory map[Band]int   `json:"runs_per_trajectory"`
	Arms              map[string]Arm `json:"arms"`
}

// Arm is a generated config fragment plus an optional arm-scoped
// coverage block. Config is an options-fragment only: providers, MCPs,
// and LSPs can't vary between arms. Coverage applies only to this
// arm's runs, after the trajectory's shared predicates — it is where
// flag-gated firing assertions live (e.g. a treatment arm that enables
// stubbing can demand min_stub_stats.results so a run where the
// mechanism never fired lands inconclusive instead of passing as
// evidence of nothing), and its grammar reaches the flag-dependent
// call_metrics fields trajectory coverage excludes.
type Arm struct {
	Config   ArmConfig `json:"config"`
	Coverage Coverage  `json:"coverage,omitempty"`
}

// ArmConfig carries the options delta under test.
type ArmConfig struct {
	Options map[string]any `json:"options,omitempty"`
}

// RunRecord is one append-only results/*.jsonl line.
type RunRecord struct {
	Experiment   string `json:"experiment"`
	TrajectoryID string `json:"trajectory_id"`
	Arm          string `json:"arm"`
	// Invocation scopes records to one RunExperiment call — re-running
	// an experiment under a new build must not pool records into the
	// gate's arm samples (the "same build both arms" invariant).
	Invocation  string         `json:"invocation,omitempty"`
	RunIndex    int            `json:"run_index"` // attempt index, sparse under resampling
	Outcome     Outcome        `json:"outcome"`
	CheckDetail map[string]any `json:"check_detail,omitempty"`
	// Bounded tails of check.sh output — the first forensic stop on
	// failure is what the check actually said.
	CheckStdout string      `json:"check_stdout,omitempty"`
	CheckStderr string      `json:"check_stderr,omitempty"`
	StartedAt   time.Time   `json:"started_at"`
	DurationS   float64     `json:"duration_s"`
	Steps       int         `json:"steps"`
	Tokens      TokenUsage  `json:"tokens"`
	StubStats   StubStats   `json:"stub_stats"`
	Recalls     Recalls     `json:"recalls"`
	Checkpoints Checkpoints `json:"checkpoints"`
	SessionDB   string      `json:"session_db,omitempty"`
	// Workdir is the materialized run directory — recorded so a
	// post-hoc `crush eval analyze` on the artifact can anchor relative
	// call paths correctly (the directory itself is deleted).
	Workdir string `json:"workdir,omitempty"`
	// SessionDBIncomplete marks a raw-copy fallback snapshot — the WAL
	// tail may be missing, so call_metrics underreports.
	SessionDBIncomplete bool `json:"session_db_incomplete,omitempty"`
	// CallMetrics is the post-run sequence analysis of SessionDB —
	// populated between preserveSessionDB and record append so
	// min_call_metrics.* predicates can read it during CoverageMet.
	CallMetrics *CallMetrics `json:"call_metrics,omitempty"`
	// CallMetricsError records analyzer failure instead of silently
	// absent metrics — inconclusive-by-absence and analyzer-broke are
	// operationally different and must not conflate.
	CallMetricsError string `json:"call_metrics_error,omitempty"`
	// BaselineKey is the hash of the run's effective config over the
	// flag projection — which baseline condition this run counts
	// toward. Computed at run time so merged experiments' treatment
	// arms self-seed post-flip baselines.
	BaselineKey string `json:"baseline_key,omitempty"`
	// ResolvedOptions is the child's report of what each manifest flag
	// actually resolved to — arm intent can silently no-op on a
	// renamed/shadowed option; resolved state is the truth.
	ResolvedOptions map[string]any `json:"resolved_options,omitempty"`
	Env             Env            `json:"env"`
}

// TokenUsage mirrors fantasy.Usage for the record.
type TokenUsage struct {
	Input      int64 `json:"input"`
	Output     int64 `json:"output"`
	CacheRead  int64 `json:"cache_read"`
	CacheWrite int64 `json:"cache_write"`
}

// StubStats mirrors the agent's per-session stubbing telemetry.
type StubStats struct {
	Invalidations    int   `json:"invalidations"`
	Results          int   `json:"results"`
	SavedBytes       int64 `json:"saved_bytes"`
	BoundaryAdvances int   `json:"boundary_advances"`
	// Kinds splits Results by stub kind, keyed by the kind's
	// telemetry label — the superseded kind (the empty string on the
	// mark itself) is "superseded" here. It backs the
	// stub_stats.kinds.<kind> coverage predicates.
	Kinds map[string]int `json:"kinds,omitempty"`
}

// Recalls mirrors notebook.Stats' recall split.
type Recalls struct {
	Result int `json:"result"`
	Entry  int `json:"entry"`
	Empty  int `json:"empty"`
	Cross  int `json:"cross"`
}

// Checkpoints mirrors the checkpoint telemetry split: written counts
// committed checkpoint entries, rendered counts prefix renders that
// included one — the "checkpoint present at render" predicate field.
type Checkpoints struct {
	Written  int `json:"written"`
	Rendered int `json:"rendered"`
}

// Env is the forensic record: when a trajectory rots, the diff between
// last-green and first-red env blocks is the first place to look.
type Env struct {
	CrushSHA string `json:"crush_sha"`
	// ModelPin is the experiment's model string as spelled in the
	// pin; ModelResolved is what the run actually resolved to. They
	// can differ (aliases, normalization) — the pin defines the
	// condition, the resolved form is the baseline storage key.
	ModelPin      string `json:"model_pin,omitempty"`
	ModelResolved string `json:"model_resolved"`
	// Small/summary resolve from ambient config — recorded so a
	// compaction-flag experiment can audit which summarizer ran.
	ModelSmall   string `json:"model_small,omitempty"`
	ModelSummary string `json:"model_summary,omitempty"`
	// Temperature is part of the run's condition — a characterize at
	// temp 0 vs an experiment at model-default are different
	// conditions the baseline key must not merge. "default" marks
	// an unpinned temperature.
	Temperature string `json:"temperature,omitempty"`
	Go          string `json:"go"`
	OS          string `json:"os"`
	ContentHash string `json:"content_hash"`
}

// BaselineCounts are the integer cells Fisher's exact needs; p̂ is
// derived.
type BaselineCounts struct {
	Passes  int    `json:"passes"`
	N       int    `json:"n"`
	LastRun string `json:"last_run,omitempty"`
}

// PHat derives the baseline pass rate.
func (b BaselineCounts) PHat() float64 {
	if b.N == 0 {
		return 0
	}
	return float64(b.Passes) / float64(b.N)
}

// BandEntry is per-trajectory characterization state. Baselines are
// keyed, not singleton: baselines[model][baselineConfigHash] → counts,
// because control-equivalent is experiment-relative.
type BandEntry struct {
	Band              Band             `json:"band"`
	QuarantineReason  QuarantineReason `json:"quarantine_reason,omitempty"`
	ContentHash       string           `json:"content_hash"`
	LastCharacterized string           `json:"last_characterized,omitempty"`
	// PromotionStreak implements the asymmetric hysteresis: a
	// non-stable band promotes after PromotionStreakRequired
	// consecutive good characterizations.
	PromotionStreak int                                  `json:"promotion_streak,omitempty"`
	Baselines       map[string]map[string]BaselineCounts `json:"baselines,omitempty"` // model -> config hash -> counts
}

// Bands is bands.json: generated characterization state.
type Bands struct {
	SchemaVersion int                  `json:"_schema_version"`
	Entries       map[string]BandEntry `json:"-"` // flattened at (un)marshal time
}
