package eval

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/version"
)

// Runner orchestrates quarantine, characterization, and experiments
// over a corpus directory.
type Runner struct {
	// EvalDir is the eval/ root (corpus/, experiments/, results/,
	// bands.json, flags.json live under it).
	EvalDir string
	// Driver executes agent runs; nil uses CrushRunner with the
	// current executable.
	Driver AgentRunner
	// Home is the pinned HOME for agent subprocesses. Created under
	// os.MkdirTemp when empty.
	Home string
	// AttemptsFactor bounds resampling: the runner samples to N
	// conclusive per arm with an attempts cap of AttemptsFactor*N
	// (~2N per the spec) before flagging the trajectory starved or
	// saturated.
	AttemptsFactor float64
	// PermReplicates is the diffuse-tier replicate count.
	PermReplicates int
	// Alpha is the per-family significance level before correction.
	Alpha float64
	// QuarantineRepeats is M — check.sh invocations per state.
	QuarantineRepeats int
	// WorkParent is where materialized workdirs are created. Empty
	// uses os.TempDir.
	WorkParent string
	RNG        *rand.Rand
	// Now is the clock, injectable for tests.
	Now func() time.Time
	// homeCreated marks Home as runner-allocated so Close can
	// remove it; a caller-provided Home is the caller's to clean.
	homeCreated bool
	// workParentChecked gates the once-per-runner repo-leak warning.
	workParentChecked bool
}

// Close removes the pinned-HOME tempdir when the runner created it.
func (r *Runner) Close() {
	if r.homeCreated && r.Home != "" {
		os.RemoveAll(r.Home)
		r.homeCreated = false
	}
}

func (r *Runner) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
}

func (r *Runner) rng() *rand.Rand {
	if r.RNG != nil {
		return r.RNG
	}
	return rand.New(rand.NewPCG(uint64(r.now().UnixNano()), 0))
}

func (r *Runner) driver() AgentRunner {
	if r.Driver != nil {
		return r.Driver
	}
	return CrushRunner{Home: r.home()}
}

func (r *Runner) home() string {
	if r.Home != "" {
		return r.Home
	}
	h, err := os.MkdirTemp("", "crush-eval-home-*")
	if err == nil {
		r.Home = h
		r.homeCreated = true
	}
	return r.Home
}

func (r *Runner) attemptsFactor() float64 {
	if r.AttemptsFactor > 0 {
		return r.AttemptsFactor
	}
	return 2.0
}

func (r *Runner) alpha() float64 {
	if r.Alpha > 0 {
		return r.Alpha
	}
	return 0.05
}

func (r *Runner) permReplicates() int {
	if r.PermReplicates > 0 {
		return r.PermReplicates
	}
	return 10000
}

func (r *Runner) workParent() string {
	if r.WorkParent != "" {
		return r.WorkParent
	}
	return os.TempDir()
}

// Quarantine runs the agent-free validation pass for one trajectory:
// the check must agree with expect_start_state on the start state, pass
// on start+reference (when present), and fail on start+counterexample
// (pass-guards). Returns the quarantine reason, or "" when the
// trajectory is clean.
func (r *Runner) Quarantine(ctx context.Context, traj *Trajectory, trajDir string) (QuarantineReason, error) {
	m := r.QuarantineRepeats
	if m <= 0 {
		m = 5
	}
	// Checks chdir into the workdir; a relative trajectory dir must be
	// resolved to absolute before it reaches the check's environment.
	trajDir, err := filepath.Abs(trajDir)
	if err != nil {
		return "", err
	}

	// check runs the check n times against a materialization and
	// reports whether results were consistent and what they said.
	check := func(patch string) (allPass, allFail bool, err error) {
		outs := map[bool]int{}
		for range m {
			// Fresh materialization per repetition — a check flaky
			// only across fresh trees (setup races, git-state drift)
			// must surface as flaky, not hide in a reused workdir.
			wd, err := Materialize(ctx, traj, trajDir, r.workParent(), r.checkEnv())
			if err != nil {
				return false, false, err
			}
			if patch != "" {
				if err := ApplyPatch(ctx, wd, patch); err != nil {
					os.RemoveAll(wd)
					return false, false, err
				}
			}
			res := RunCheck(ctx, traj, trajDir, wd, r.checkEnv())
			os.RemoveAll(wd)
			os.RemoveAll(DataDirFor(wd))
			if res.Err != nil {
				return false, false, res.Err
			}
			outs[res.Exit == 0]++
		}
		return outs[true] == m, outs[false] == m, nil
	}

	startPass, startFail, err := check("")
	if err != nil {
		return "", err
	}
	wantPass := traj.Check.ExpectStartState == "pass"

	// Inconsistent results → flaky check.
	if !startPass && !startFail {
		return ReasonFlaky, nil
	}
	// Pass on start where fail expected → vacuous check.
	if !wantPass && startPass {
		return ReasonVacuous, nil
	}
	// Fail on start where pass expected → also vacuous-in-reverse:
	// the check can't even see the good state.
	if wantPass && startFail {
		return ReasonVacuous, nil
	}

	ref := filepath.Join(trajDir, "reference.patch")
	if fileExists(ref) {
		pass, _, err := check(ref)
		if err != nil {
			return "", err
		}
		if !pass {
			// Fails on a known-good state — mis-calibrated,
			// confidently wrong, worse than flaky.
			return ReasonMiscalibrated, nil
		}
	}

	counter := filepath.Join(trajDir, "counterexample.patch")
	if fileExists(counter) {
		_, fail, err := check(counter)
		if err != nil {
			return "", err
		}
		if !fail {
			// Vacuous pass-guard: the check can't see bad.
			return ReasonVacuous, nil
		}
	}
	return "", nil
}

// ExecuteRun performs one trajectory run: materialize, arm config,
// agent turns, check, coverage — and classifies the outcome.
//
// Precedence: error (transport) > timeout (budget) > fail >
// inconclusive (coverage unmet) > pass. A timeout run never reaches
// check.sh — the bound preempts the verdict.
func (r *Runner) ExecuteRun(ctx context.Context, exp *Experiment, traj *Trajectory, trajDir, armName string, arm Arm, manifest *FlagsManifest, attempt int, inv string) (RunRecord, error) {
	rec := RunRecord{
		Experiment:   exp.Name,
		Invocation:   inv,
		TrajectoryID: traj.ID,
		Arm:          armName,
		RunIndex:     attempt,
		StartedAt:    r.now(),
		Env: Env{
			CrushSHA: crushSHA(),
			Go:       goToolchain(),
			OS:       runtime.GOOS,
		},
	}

	contentHash, err := ContentHash(trajDir)
	if err != nil {
		return rec, fmt.Errorf("content hash: %w", err)
	}
	rec.Env.ContentHash = contentHash
	// Temperature is a run condition — an unpinned temp and temp-0
	// are different baselines, invisible unless hashed.
	tempKey := temperatureKey(exp.Temperature)
	rec.Env.Temperature = tempKey
	rec.BaselineKey = manifest.keyWith(arm.Config.Options, map[string]any{"$temperature": tempKey})

	workdir, err := Materialize(ctx, traj, trajDir, r.workParent(), r.checkEnv())
	if err != nil {
		rec.Outcome = OutcomeError
		return rec, nil
	}
	// The data dir is a sibling, not inside the workdir — clean both.
	defer os.RemoveAll(workdir)
	defer os.RemoveAll(DataDirFor(workdir))

	if err := WriteArmConfig(workdir, exp, arm, manifest); err != nil {
		rec.Outcome = OutcomeError
		rec.CheckDetail = map[string]any{"harness": err.Error()}
		return rec, nil
	}

	rec.StartedAt = r.now()
	drv := r.driver()
	if cr, ok := drv.(CrushRunner); ok {
		cr.FlagKeys = flagNames(manifest)
		drv = cr
	}
	res := drv.Run(ctx, workdir, traj.Task.Turns, traj.Budget)
	rec.DurationS = r.now().Sub(rec.StartedAt).Seconds()
	rec.Steps = res.Steps
	rec.Tokens = res.Tokens
	rec.StubStats = res.StubStats
	rec.Recalls = res.Recalls
	rec.Checkpoints = res.Checkpoints
	rec.Env.ModelPin = exp.Model
	rec.Env.ModelResolved = res.ModelResolved
	// The baseline key hashes resolved config when the child reported
	// it — arm intent can silently no-op; resolved state is truth.
	if len(res.ResolvedOptions) > 0 {
		rec.ResolvedOptions = res.ResolvedOptions
		rec.BaselineKey = manifest.keyWith(res.ResolvedOptions, map[string]any{"$temperature": tempKey})
	}
	rec.Env.ModelSmall = res.ModelSmall
	rec.Env.ModelSummary = res.ModelSummary

	// Preserve the session DB — the failed-run debugging artifact is
	// the full message/tool trace, free. Attempted regardless of
	// whether telemetry reported a session: a turn-0 hard-kill writes
	// no telemetry but still leaves a DB worth keeping.
	dst, walSafe, err := r.preserveSessionDB(ctx, exp.Name, traj.ID, armName, inv, attempt, workdir)
	// Record the run's workdir unconditionally — the materialized dir is
	// deleted after the run, and the anchor is the only way a post-hoc
	// `eval analyze` can resolve relative call paths correctly.
	rec.Workdir = workdir
	if err != nil {
		// Record why the metrics are absent — indistinguishable from
		// "no metrics by design" otherwise.
		rec.CallMetricsError = fmt.Sprintf("session db not preserved: %v", err)
	} else {
		rec.SessionDB = dst
		rec.SessionDBIncomplete = !walSafe
		// Sequence analysis runs on the preserved artifact, not the
		// about-to-be-deleted source — `crush eval analyze <artifact>`
		// then reproduces exactly what the record carries.
		metrics, aerr := AnalyzeSessionDB(ctx, filepath.Join(r.EvalDir, dst), AnalyzeOptions{
			SessionID: res.SessionID,
			Workdir:   workdir,
			Turns:     traj.Task.Turns,
			// The producing host's conventions — rec.Env.OS — not the
			// analyzer's, in case artifacts are analyzed cross-platform.
			GOOS: rec.Env.OS,
		})
		if aerr != nil {
			rec.CallMetricsError = aerr.Error()
		} else {
			rec.CallMetrics = metrics
		}
	}

	switch {
	case res.Err != nil:
		// The model didn't produce the outcome; the transport did.
		rec.Outcome = OutcomeError
		rec.CheckDetail = map[string]any{"run_error": res.Err.Error()}
		return rec, nil
	case res.TimedOut || (traj.Budget.MaxSteps > 0 && res.Steps > traj.Budget.MaxSteps):
		rec.Outcome = OutcomeTimeout
		return rec, nil
	}

	chk := RunCheck(ctx, traj, trajDir, workdir, r.checkEnv())
	rec.CheckStdout = string(tail([]byte(chk.Stdout), 4096))
	rec.CheckStderr = string(tail([]byte(chk.Stderr), 4096))
	if chk.Err != nil {
		rec.Outcome = OutcomeError
		rec.CheckDetail = map[string]any{"check_error": chk.Err.Error()}
		return rec, nil
	}
	rec.CheckDetail = chk.Detail
	if chk.Exit != 0 {
		rec.Outcome = OutcomeFail
		return rec, nil
	}

	// Coverage masks passes only: a check-fail is a real outcome
	// whether or not the mechanism fired. Trajectory coverage is
	// flag-invariant by construction; the arm's own block is where
	// flag-gated firing assertions live (a treatment arm asserting
	// stubbing actually fired starves itself otherwise — the pairing
	// can't pass on evidence of nothing).
	met, failedKey, err := coverageMet(traj.Coverage, &rec, coverageFields)
	if err != nil {
		return rec, fmt.Errorf("coverage eval: %w", err)
	}
	scope := ""
	if !met {
		scope = "trajectory"
	} else if armMet, akey, aerr := coverageMet(arm.Coverage, &rec, armFields); aerr != nil {
		return rec, fmt.Errorf("arm coverage eval: %w", aerr)
	} else if !armMet {
		// An arm-coverage miss after a trajectory-coverage pass is
		// the firing assertion tripping, not generic undercoverage.
		scope, failedKey = "arm", akey
		met = false
	}
	if !met {
		// Record which scope and predicate starved the run so
		// forensics can tell a firing assertion from generic
		// undercoverage.
		if rec.CheckDetail == nil {
			rec.CheckDetail = map[string]any{}
		}
		rec.CheckDetail["coverage_scope"] = scope
		rec.CheckDetail["coverage_key"] = failedKey
		rec.Outcome = OutcomeInconclusive
		return rec, nil
	}
	rec.Outcome = OutcomePass
	return rec, nil
}

// preserveSessionDB snapshots the run's SQLite DB into
// results/<experiment>/artifacts/<trajectory>-<arm>-<inv>-<run_index>.db.
//
// The source is WAL-mode: a bare file copy drops the committed tail
// still sitting in crush.db-wal — on WaitDelay hard-kills (timeout
// runs) that tail is exactly the derailment trace the artifact exists
// for. VACUUM INTO produces a consistent single-file snapshot including
// un-checkpointed commits; a raw copy is the last-resort fallback and
// reports walSafe=false so the record can flag a possibly-truncated
// artifact.
func (r *Runner) preserveSessionDB(ctx context.Context, expName, trajID, arm, inv string, runIndex int, workdir string) (rel string, walSafe bool, err error) {
	src := filepath.Join(DataDirFor(workdir), "crush.db")
	if !fileExists(src) {
		return "", false, fmt.Errorf("no session db at %s", src)
	}
	dstDir := filepath.Join(r.EvalDir, "results", expName, "artifacts")
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return "", false, err
	}
	dst := filepath.Join(dstDir, fmt.Sprintf("%s-%s-%s-%d.db", trajID, arm, inv, runIndex))

	conn, err := db.ConnectReadOnly(ctx, src)
	if err == nil {
		// VACUUM INTO takes a string literal, not a bound parameter.
		_, err = conn.ExecContext(ctx, `VACUUM INTO '`+strings.ReplaceAll(dst, "'", "''")+`'`)
		conn.Close()
	}
	if err != nil {
		slog.Warn("VACUUM INTO failed, falling back to raw copy (WAL tail may be lost)",
			"src", src, "error", err)
		data, rerr := os.ReadFile(src)
		if rerr != nil {
			return "", false, rerr
		}
		if werr := os.WriteFile(dst, data, 0o644); werr != nil {
			return "", false, werr
		}
		rel, err = filepath.Rel(r.EvalDir, dst)
		return rel, false, err
	}
	rel, err = filepath.Rel(r.EvalDir, dst)
	return rel, true, err
}

// appendRecord writes one line to results/<experiment>/<traj>.jsonl.
func (r *Runner) appendRecord(rec RunRecord) error {
	dir := filepath.Join(r.EvalDir, "results", rec.Experiment)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(dir, rec.TrajectoryID+".jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	data, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	_, err = f.Write(append(data, '\n'))
	return err
}

// LoadRecords reads every run record for a trajectory across all
// experiments — the revision input for bands.json.
func (r *Runner) LoadRecords(trajectoryID string) ([]RunRecord, error) {
	return r.loadRecordsFiltered(func(rec RunRecord) bool { return rec.TrajectoryID == trajectoryID })
}

// LoadExperimentRecords reads one experiment's records.
func (r *Runner) LoadExperimentRecords(expName string) ([]RunRecord, error) {
	return r.loadRecordsFiltered(func(rec RunRecord) bool { return rec.Experiment == expName })
}

func (r *Runner) loadRecordsFiltered(match func(RunRecord) bool) ([]RunRecord, error) {
	root := filepath.Join(r.EvalDir, "results")
	var out []RunRecord
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".jsonl" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for line := range strings.Lines(string(data)) {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			var rec RunRecord
			if err := json.Unmarshal([]byte(line), &rec); err != nil {
				slog.Warn("Skipping unparseable record line — possible mid-file corruption", "path", path, "err", err)
				continue // Tolerate a torn final line.
			}
			if match(rec) {
				out = append(out, rec)
			}
		}
		return nil
	})
	if os.IsNotExist(err) {
		return nil, nil
	}
	return out, err
}

// RunExperiment executes a paired comparison: frozen bands, per-
// trajectory resampling to N conclusive per arm, arms interleaved in
// time so provider drift lands on both and cancels in the pairing.
// Returns the gate report.
func (r *Runner) RunExperiment(ctx context.Context, exp *Experiment) (Report, error) {
	// Programmatic callers bypass LoadExperiment's validation — the
	// corpus-shape and arm checks still apply.
	if err := ValidateExperiment(exp); err != nil {
		return Report{}, err
	}
	unlock, err := r.acquireLock()
	if err != nil {
		return Report{}, err
	}
	defer unlock()
	corpus, err := LoadCorpus(r.EvalDir)
	if err != nil {
		return Report{}, err
	}
	manifest, err := LoadFlagsManifest(r.EvalDir)
	if err != nil {
		return Report{}, err
	}
	if err := manifest.ValidateArmFlags(exp); err != nil {
		return Report{}, err
	}
	if err := ValidateArmCoverageResolved(exp, manifest); err != nil {
		return Report{}, err
	}
	bands, err := LoadBands(r.EvalDir)
	if err != nil {
		return Report{}, err
	}
	frozen := deepCopyBands(bands) // Band assignments freeze at experiment start.

	trajs, err := SelectCorpus(corpus, frozen, exp.Corpus)
	if err != nil {
		return Report{}, err
	}
	if len(trajs) == 0 {
		// Fail closed: an empty selection must not reach the gate —
		// a verdict over zero trajectories is a PASS on nothing.
		return Report{}, fmt.Errorf("corpus selection %v matched no runnable trajectories", exp.Corpus)
	}

	baselineKey := manifest.keyWith(exp.Arms[ArmControl].Config.Options,
		map[string]any{"$temperature": temperatureKey(exp.Temperature)})
	// Invocation scopes this call's records: re-running an experiment
	// under a new build must not pool stale records into the gate.
	inv := fmt.Sprintf("%s-%04x", r.now().UTC().Format("20060102T150405Z"), r.rng().Uint64()&0xffff)
	rep := Report{CatastrophicEligible: map[string]bool{}, DiffuseP: 1}

	runnable := 0
	requiresSkipped := 0
	for _, traj := range trajs {
		band := frozen.Band(traj.ID)
		n := exp.RunsPerTrajectory[band]
		if n == 0 {
			// Partial undercoverage must be visible — a selection
			// whose band isn't in runs_per_trajectory silently
			// measures nothing.
			rep.Skipped = append(rep.Skipped,
				fmt.Sprintf("%s: band %q has no runs_per_trajectory entry", traj.ID, band))
			continue
		}
		runnable++
		trajDir := filepath.Join(r.EvalDir, "corpus", traj.ID)
		trep := r.runTrajectory(ctx, exp, traj, trajDir, manifest, n, inv)
		rep.Starved = append(rep.Starved, trep.Starved...)
		rep.Saturated = append(rep.Saturated, trep.Saturated...)
		if trep.Skipped != "" {
			requiresSkipped++
			rep.Skipped = append(rep.Skipped, fmt.Sprintf("%s: %s", traj.ID, trep.Skipped))
		}
	}
	// Fail closed: a corpus whose every runnable trajectory skipped
	// would otherwise report PASS on zero samples. requiresSkipped
	// counts only requires-misses — band-uncovered entries share
	// rep.Skipped but aren't runnable.
	if runnable > 0 && requiresSkipped == runnable {
		return rep, fmt.Errorf("all %d runnable trajectories skipped requires pre-flight: %s",
			runnable, strings.Join(rep.Skipped, "; "))
	}
	// A selection whose bands are all absent from runs_per_trajectory
	// schedules nothing — a config error, not a clean gate.
	if len(trajs) > 0 && runnable == 0 {
		return rep, fmt.Errorf("no selected trajectory's band is covered by runs_per_trajectory")
	}
	// Cancellation aborts cleanly — don't evaluate the gate on a
	// partial record set and print a misleading verdict.
	if err := ctx.Err(); err != nil {
		return rep, err
	}

	// Gate on the frozen snapshot — the current experiment's own
	// control arm is excluded from baselines automatically because
	// bands were frozen before it ran.
	allRecords, err := r.LoadExperimentRecords(exp.Name)
	if err != nil {
		return rep, err
	}
	// Only this invocation's records feed the gate — earlier runs of
	// the same-named experiment were a different build's data.
	var records []RunRecord
	for _, rec := range allRecords {
		if rec.Invocation == inv {
			records = append(records, rec)
		}
	}
	// The gate's baseline key joins on the control arm's resolved
	// condition — what actually ran — falling back to intent only
	// when no control record carried a resolved key.
	if k := currentConditionKey(records, exp.Model, ArmControl); k != "" {
		baselineKey = k
	}
	gate := Evaluate(exp, frozen, baselineKey, records, corpusIDs(corpus), r.alpha(), r.permReplicates(), r.rng())
	rep.Coincident = gate.Coincident
	rep.NoopFlags = gate.NoopFlags
	rep.Catastrophic = gate.Catastrophic
	rep.CatastrophicEligible = gate.CatastrophicEligible
	rep.DiffuseP = gate.DiffuseP
	rep.ExcludedDifferential = gate.ExcludedDifferential
	rep.Smoke = gate.Smoke

	// Every paired comparison adds baseline samples as a byproduct —
	// recompute characterization state after gating so the frozen
	// snapshot stays clean for the coincidence-detector role.
	if err := r.RecomputeAll(bands, corpus, manifest, exp.Model, temperatureKey(exp.Temperature)); err != nil {
		slog.Warn("Failed to recompute bands", "error", err)
	}
	if err := bands.Save(r.EvalDir); err != nil {
		return rep, fmt.Errorf("save bands.json: %w", err)
	}
	return rep, nil
}

// trajReport carries per-trajectory alarm labels back to the
// experiment summary.
type trajReport struct {
	Starved   []string
	Saturated []string
	Skipped   string
}

// runTrajectory samples one trajectory to N conclusive runs per arm,
// alternating arms each round — per-run interleaving, the finest
// granularity, is what makes within-trajectory runs approximately
// exchangeable for the permutation test.
func (r *Runner) runTrajectory(ctx context.Context, exp *Experiment, traj *Trajectory, trajDir string, manifest *FlagsManifest, n int, inv string) trajReport {
	rep := trajReport{}
	if missing := CheckRequires(traj); len(missing) > 0 {
		// Environment pre-flight: a missing tool is a skip-report,
		// not an error outcome masquerading as flakiness.
		rep.Skipped = strings.Join(missing, ",")
		return rep
	}
	armNames := []string{ArmControl, ArmTreatment}
	maxAttempts := int(float64(n) * r.attemptsFactor())
	conclusive := map[string]int{ArmControl: 0, ArmTreatment: 0}
	attempts := map[string]int{ArmControl: 0, ArmTreatment: 0}
	excluded := map[string]map[Outcome]int{
		ArmControl: {}, ArmTreatment: {},
	}

	round := 0
	for conclusive[ArmControl] < n || conclusive[ArmTreatment] < n {
		if ctx.Err() != nil {
			// Stop cleanly on cancellation — don't materialize and
			// error-append up to ~2N records per remaining trajectory.
			return rep
		}
		// Alternate which arm leads each round — under monotonic
		// provider drift a fixed control-first order systematically
		// hands treatment the later sample and biases the pairing.
		lead := round % 2
		round++
		progressed := false
		for _, armName := range append(armNames[lead:], armNames[:lead]...) {
			if conclusive[armName] >= n || attempts[armName] >= maxAttempts {
				continue
			}
			progressed = true
			attempts[armName]++

			rec, err := r.ExecuteRun(ctx, exp, traj, trajDir, armName, exp.Arms[armName], manifest, attempts[armName], inv)
			if err != nil {
				slog.Warn("Run harness failed", "trajectory", traj.ID, "arm", armName, "error", err)
				rec = RunRecord{Experiment: exp.Name, TrajectoryID: traj.ID, Arm: armName, Invocation: inv, Outcome: OutcomeError}
				rec.RunIndex = attempts[armName]
			}
			if err := r.appendRecord(rec); err != nil {
				// The sample is lost — a full disk silently shrinking
				// N is worse than burning an attempt on an error.
				slog.Warn("Failed to append run record", "error", err)
				excluded[armName][OutcomeError]++
				continue
			}
			if rec.Outcome.Conclusive() {
				conclusive[armName]++
			} else {
				excluded[armName][rec.Outcome]++
			}
			slog.Info("Eval run", "trajectory", traj.ID, "arm", armName, "outcome", rec.Outcome,
				"conclusive", conclusive[armName], "attempts", attempts[armName])
		}
		if !progressed {
			break
		}
	}

	// Attempts-cap exhaustion is a distinct alarm per excluded class:
	// coverage-starved (inconclusive) vs error-saturated (infra).
	for _, armName := range armNames {
		if conclusive[armName] < n {
			if excluded[armName][OutcomeError] >= excluded[armName][OutcomeInconclusive] {
				rep.Saturated = append(rep.Saturated, fmt.Sprintf("%s/%s", traj.ID, armName))
			} else {
				rep.Starved = append(rep.Starved, fmt.Sprintf("%s/%s", traj.ID, armName))
			}
		}
	}
	return rep
}

// RecomputeAll rebuilds characterization state from accumulated run
// records — bands are revised, never trusted from genesis. model is
// the current pin; the default-condition baseline key comes from the
// manifest. Band assignment and the never_passed/suspect_check scans
// read only that (model, key) pair — a stale model's deep baseline
// must not hold a trajectory stable across a re-pin.
func (r *Runner) RecomputeAll(bands *Bands, corpus map[string]*Trajectory, manifest *FlagsManifest, model, temp string) error {
	// curKey joins banding to the records' own resolved keys — the
	// resolved projection can diverge from manifest intent on
	// non-materialized options, and keying the gate/bands off intent
	// would orphan every record under its real condition. Intent is
	// the fallback only until a control-condition record exists.
	perTraj := make(map[string][]RunRecord, len(corpus))
	var all []RunRecord
	for id := range corpus {
		records, err := r.LoadRecords(id)
		if err != nil {
			return err
		}
		perTraj[id] = records
		all = append(all, records...)
	}
	// The canonical default comes from baseline-arm records
	// (characterize/smoke run the empty arm — true defaults).
	// Control-arm records are the fallback for corpora that only
	// ever ran experiments; a non-default control's key is itself
	// correct for banding but must not redefine "default" when
	// baseline records exist.
	curKey := currentConditionKey(all, model, ArmBaseline)
	if curKey == "" {
		curKey = currentConditionKey(all, model, ArmControl)
	}
	if curKey == "" {
		// Bootstrap fallback — hashed with temperature so the intent
		// namespace stays coherent with what records would stamp.
		curKey = manifest.keyWith(nil, map[string]any{"$temperature": temp})
	}
	for id := range corpus {
		trajDir := filepath.Join(r.EvalDir, "corpus", id)
		hash, err := ContentHash(trajDir)
		if err != nil {
			return fmt.Errorf("content hash %s: %w", id, err)
		}
		bands.Recompute(id, perTraj[id], hash, r.now(), model, curKey)
	}
	return nil
}

// Characterize runs genesis/re-characterization: n runs per trajectory
// under the current default condition (empty arm options → the
// manifest's baseline key), then recompute.
func (r *Runner) Characterize(ctx context.Context, model string, temperature *float64, n int, selectors []string) error {
	unlock, err := r.acquireLock()
	if err != nil {
		return err
	}
	defer unlock()
	corpus, err := LoadCorpus(r.EvalDir)
	if err != nil {
		return err
	}
	manifest, err := LoadFlagsManifest(r.EvalDir)
	if err != nil {
		return err
	}
	bands, err := LoadBands(r.EvalDir)
	if err != nil {
		return err
	}
	trajs, err := SelectCorpus(corpus, bands, selectors)
	if err != nil {
		return err
	}
	if n <= 0 {
		n = GenesisRuns
	}
	exp := &Experiment{Name: CharacterizeExperiment, Model: model, Temperature: temperature}
	charInv := fmt.Sprintf("characterize-%s-%04x", r.now().UTC().Format("20060102T150405Z"), r.rng().Uint64()&0xffff)
	for _, traj := range trajs {
		if missing := CheckRequires(traj); len(missing) > 0 {
			slog.Warn("Skipping trajectory — unmet requires", "trajectory", traj.ID, "missing", missing)
			continue
		}
		trajDir := filepath.Join(r.EvalDir, "corpus", traj.ID)
		for i := range n {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			rec, err := r.ExecuteRun(ctx, exp, traj, trajDir, ArmBaseline, Arm{}, manifest, i+1, charInv)
			if err != nil {
				return fmt.Errorf("characterize %s: %w", traj.ID, err)
			}
			if err := r.appendRecord(rec); err != nil {
				return err
			}
			slog.Info("Characterize run", "trajectory", traj.ID, "outcome", rec.Outcome)
		}
	}
	if err := r.RecomputeAll(bands, corpus, manifest, model, temperatureKey(temperature)); err != nil {
		return err
	}
	return bands.Save(r.EvalDir)
}

// Smoke runs the smoke tier: every stable-band trajectory gets n runs
// under the current default condition, and any strict 0/N result is a
// collapse alarm. Smoke is not a sample for p̂ — its records flow
// through the _characterize pipeline anyway since they're
// baseline-eligible under current defaults.
func (r *Runner) Smoke(ctx context.Context, model string, temperature *float64, n int) ([]string, error) {
	unlock, err := r.acquireLock()
	if err != nil {
		return nil, err
	}
	defer unlock()
	corpus, err := LoadCorpus(r.EvalDir)
	if err != nil {
		return nil, err
	}
	manifest, err := LoadFlagsManifest(r.EvalDir)
	if err != nil {
		return nil, err
	}
	bands, err := LoadBands(r.EvalDir)
	if err != nil {
		return nil, err
	}
	trajs, err := SelectCorpus(corpus, bands, []string{"band:stable"})
	if err != nil {
		return nil, err
	}
	if len(trajs) == 0 {
		// An empty stable band must not report "smoke: clean" — zero
		// trajectories checked is a confidence hole, not a pass.
		return nil, fmt.Errorf("no stable-band trajectories to smoke — run characterize first")
	}
	if dropped := len(corpus) - len(trajs); dropped > 0 {
		slog.Warn("Smoke covers a shrunken corpus — corpus-size drop is an environment-rot signal",
			"stable", len(trajs), "corpus", len(corpus))
	}
	if n <= 0 {
		n = 5
	}
	exp := &Experiment{Name: CharacterizeExperiment, Model: model, Temperature: temperature}
	smokeInv := fmt.Sprintf("smoke-%s-%04x", r.now().UTC().Format("20060102T150405Z"), r.rng().Uint64()&0xffff)
	var alarms []string
	for _, traj := range trajs {
		if missing := CheckRequires(traj); len(missing) > 0 {
			slog.Warn("Skipping trajectory — unmet requires", "trajectory", traj.ID, "missing", missing)
			continue
		}
		trajDir := filepath.Join(r.EvalDir, "corpus", traj.ID)
		passes := 0
		conclusive := 0
		for i := range n {
			if ctx.Err() != nil {
				return alarms, ctx.Err()
			}
			rec, err := r.ExecuteRun(ctx, exp, traj, trajDir, ArmBaseline, Arm{}, manifest, i+1, smokeInv)
			if err != nil {
				return alarms, fmt.Errorf("smoke %s: %w", traj.ID, err)
			}
			if rec.Outcome.Conclusive() {
				conclusive++
				if rec.Outcome == OutcomePass {
					passes++
				}
			}
			if err := r.appendRecord(rec); err != nil {
				return alarms, err
			}
		}
		// Strict 0/N over conclusive runs — a single failure at
		// p=0.95, N=5 is a 23% false alarm; smoke catches collapses,
		// not drift. Require a conclusive majority: four errors and
		// one fail is an outage, not a collapse. Zero conclusive runs
		// is a provider outage, not a collapse.
		if passes == 0 && conclusive*2 > n {
			alarms = append(alarms, fmt.Sprintf("%s (0/%d conclusive)", traj.ID, conclusive))
		}
	}
	if err := r.RecomputeAll(bands, corpus, manifest, model, temperatureKey(temperature)); err != nil {
		return alarms, err
	}
	return alarms, bands.Save(r.EvalDir)
}

// QuarantineCorpus runs the quarantine pass over every corpus
// trajectory and records verdicts into bands.
func (r *Runner) QuarantineCorpus(ctx context.Context, selectors []string) (map[string]QuarantineReason, error) {
	unlock, err := r.acquireLock()
	if err != nil {
		return nil, err
	}
	defer unlock()
	corpus, err := LoadCorpus(r.EvalDir)
	if err != nil {
		return nil, err
	}
	bands, err := LoadBands(r.EvalDir)
	if err != nil {
		return nil, err
	}
	// includeQuarantined: this pass is how they get re-validated.
	trajs, err := selectCorpus(corpus, bands, selectors, true)
	if err != nil {
		return nil, err
	}
	verdicts := map[string]QuarantineReason{}
	for _, traj := range trajs {
		if missing := CheckRequires(traj); len(missing) > 0 {
			slog.Warn("Skipping quarantine — unmet requires", "trajectory", traj.ID, "missing", missing)
			verdicts[traj.ID] = QuarantineReason("skipped: " + strings.Join(missing, ", "))
			continue
		}
		trajDir := filepath.Join(r.EvalDir, "corpus", traj.ID)
		reason, err := r.Quarantine(ctx, traj, trajDir)
		if err != nil {
			return verdicts, fmt.Errorf("quarantine %s: %w", traj.ID, err)
		}
		verdicts[traj.ID] = reason
		e := bands.Entry(traj.ID)
		if h, err := ContentHash(trajDir); err == nil {
			e.ContentHash = h
		}
		if reason != "" {
			e.Band = BandQuarantined
			e.QuarantineReason = reason
		} else if e.Band == BandQuarantined {
			// Re-validated: back to uncharacterized pending
			// characterization runs.
			e.Band = BandUncharacterized
			e.QuarantineReason = ""
		}
		bands.Put(traj.ID, *e)
	}
	return verdicts, bands.Save(r.EvalDir)
}

// deepCopyBands snapshots band state for the experiment-start freeze.
func deepCopyBands(b *Bands) *Bands {
	out := &Bands{SchemaVersion: b.SchemaVersion, Entries: map[string]BandEntry{}}
	for k, v := range b.Entries {
		cp := v
		if v.Baselines != nil {
			cp.Baselines = map[string]map[string]BaselineCounts{}
			for m, byCfg := range v.Baselines {
				inner := map[string]BaselineCounts{}
				for c, counts := range byCfg {
					inner[c] = counts
				}
				cp.Baselines[m] = inner
			}
		}
		out.Entries[k] = cp
	}
	return out
}

// crushSHA resolves the build's VCS revision — version.Commit is a
// ldflags placeholder ("unknown") for plain `go build`, while
// ReadBuildInfo stamps vcs.revision for module builds. BuildID (exe
// mtime fingerprint) is the last resort.
func crushSHA() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, s := range info.Settings {
			if s.Key == "vcs.revision" && s.Value != "" {
				return s.Value
			}
		}
	}
	if version.Commit != "" && version.Commit != "unknown" {
		return version.Commit
	}
	return version.BuildID
}

// checkEnv gives check.sh the same hermeticity the agent subprocess
// gets — pinned HOME/XDG, parent's CRUSH_* stripped — so a check
// can't leak the operator's real config into outcomes.
func (r *Runner) checkEnv() []string {
	pinned := map[string]string{
		"HOME":            r.home(),
		"XDG_CONFIG_HOME": filepath.Join(r.home(), ".config"),
		"XDG_DATA_HOME":   filepath.Join(r.home(), ".local", "share"),
		"XDG_STATE_HOME":  filepath.Join(r.home(), ".local", "state"),
	}
	env := make([]string, 0, len(os.Environ()))
	for _, kv := range os.Environ() {
		k, _, _ := strings.Cut(kv, "=")
		if strings.HasPrefix(k, "CRUSH_") || strings.HasPrefix(k, "EVAL_") {
			continue
		}
		if _, overridden := pinned[k]; !overridden {
			env = append(env, kv)
		}
	}
	for k, v := range pinned {
		env = append(env, k+"="+v)
	}
	return env
}

// corpusIDs returns the corpus's trajectory id set — gates filter
// band tables to live corpus members.
func corpusIDs(corpus map[string]*Trajectory) map[string]bool {
	out := make(map[string]bool, len(corpus))
	for id := range corpus {
		out[id] = true
	}
	return out
}

// acquireLock is a mkdir-based mutual exclusion over the eval dir —
// bands.json/records are load-modify-append state that two concurrent
// `crush eval` invocations would otherwise clobber.
func (r *Runner) acquireLock() (func(), error) {
	dir := filepath.Join(r.EvalDir, ".eval-lock")
	pidFile := filepath.Join(dir, "pid")
	for range 100 {
		if err := os.Mkdir(dir, 0o755); err == nil {
			_ = os.WriteFile(pidFile, []byte(strconv.Itoa(os.Getpid())), 0o644)
			return func() { _ = os.RemoveAll(dir) }, nil
		}
		// Break a lock whose holder is dead — a killed `crush eval`
		// would otherwise wedge the dir forever.
		if data, err := os.ReadFile(pidFile); err == nil {
			if pid, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil && !pidAlive(pid) {
				_ = os.RemoveAll(dir)
				continue
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	return nil, fmt.Errorf("another eval process holds %s", dir)
}

// pidAlive reports whether a lock holder still runs. Signal 0 is a
// Unix existence probe; on other platforms assume alive — a stale
// lock there expires only by removal.
func pidAlive(pid int) bool {
	if runtime.GOOS == "windows" {
		// Signal(0) is unsupported — assume alive rather than
		// steal a live lock.
		return true
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return p.Signal(syscall.Signal(0)) == nil
}

// goToolchain records the `go` on PATH — the check script's toolchain
// — not the harness binary's runtime.Version(). Toolchain rot shows in
// env diffs; the binary's own version never changes.
func goToolchain() string {
	if out, err := exec.Command("go", "version").Output(); err == nil {
		return strings.TrimSpace(string(out))
	}
	return runtime.Version()
}

// temperatureKey renders the run's temperature condition for the
// baseline hash — "default" when unpinned.
func temperatureKey(t *float64) string {
	if t == nil {
		return "default"
	}
	return strconv.FormatFloat(*t, 'g', -1, 64)
}
