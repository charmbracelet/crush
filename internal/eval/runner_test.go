package eval

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// --- bands ---

func recFor(id, model, baseKey string, outcome Outcome, at time.Time) RunRecord {
	return RunRecord{
		TrajectoryID: id,
		Outcome:      outcome,
		StartedAt:    at,
		BaselineKey:  baseKey,
		Env:          Env{ModelResolved: model, ContentHash: "h"},
	}
}

func TestBands_WindowAndAssign(t *testing.T) {
	t.Parallel()
	b := &Bands{Entries: map[string]BandEntry{}}
	now := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	// 25 passes then demotion would need a bad characterization; a
	// clean stable requires two consecutive good characterizations.
	var recs []RunRecord
	for i := range 25 {
		recs = append(recs, recFor("t", "m", "cfg", OutcomePass, now.Add(time.Duration(i)*time.Hour)))
	}
	b.Recompute("t", recs, "h", now, "m", "cfg")
	require.Equal(t, BandMid, b.Band("t")) // First good char → streak 1.
	b.Recompute("t", recs, "h", now.Add(time.Hour), "m", "cfg")
	require.Equal(t, BandStable, b.Band("t")) // Second good → promote.

	// Enough fails to push the windowed p̂ below the stable floor
	// demotes immediately — one bad characterization suffices.
	for i := range 5 {
		recs = append(recs, recFor("t", "m", "cfg", OutcomeFail, now.Add(time.Duration(100+i)*time.Hour)))
	}
	b.Recompute("t", recs, "h", now.Add(110*time.Hour), "m", "cfg")
	require.Equal(t, BandMid, b.Band("t"))
}

func TestBands_NeverPassedTrailing(t *testing.T) {
	t.Parallel()
	b := &Bands{Entries: map[string]BandEntry{}}
	now := time.Now()
	var recs []RunRecord
	for i := range NeverPassedFails {
		recs = append(recs, recFor("t", "m", "c", OutcomeFail, now.Add(time.Duration(i)*time.Minute)))
	}
	b.Recompute("t", recs, "h", now, "m", "c")
	require.Equal(t, BandQuarantined, b.Band("t"))
	require.Equal(t, ReasonNeverPassed, b.Entries["t"].QuarantineReason)

	// With a lifetime pass, the same streak is rot, not never_passed.
	b2 := &Bands{Entries: map[string]BandEntry{}}
	recs2 := append([]RunRecord{recFor("t", "m", "c", OutcomePass, now.Add(-time.Hour))}, recs...)
	b2.Recompute("t", recs2, "h", now, "m", "c")
	require.NotEqual(t, ReasonNeverPassed, b2.Entries["t"].QuarantineReason)
}

func TestBands_ContentHashChangeResets(t *testing.T) {
	t.Parallel()
	b := &Bands{Entries: map[string]BandEntry{}}
	now := time.Now()
	var recs []RunRecord
	for i := range 10 {
		recs = append(recs, recFor("t", "m", "c", OutcomePass, now.Add(time.Duration(i)*time.Minute)))
	}
	b.Recompute("t", recs, "h", now, "m", "c")
	b.Recompute("t", recs, "h", now, "m", "c")
	require.Equal(t, BandStable, b.Band("t"))

	// Corpus revision change → baselines discarded, back to
	// uncharacterized pending re-characterization.
	b.Recompute("t", recs, "h2", now, "m", "c")
	require.Equal(t, BandUncharacterized, b.Band("t"))
	require.Empty(t, b.Entries["t"].Baselines)
}

func TestBands_SuspectCheckAlternation(t *testing.T) {
	t.Parallel()
	b := &Bands{Entries: map[string]BandEntry{}}
	now := time.Now()
	var recs []RunRecord
	for i := range 10 {
		o := OutcomePass
		if i%2 == 0 {
			o = OutcomeFail
		}
		recs = append(recs, recFor("t", "m", "c", o, now.Add(time.Duration(i)*time.Minute)))
	}
	b.Recompute("t", recs, "h", now, "m", "c")
	require.Equal(t, BandQuarantined, b.Band("t"))
	require.Equal(t, ReasonSuspectCheck, b.Entries["t"].QuarantineReason)
}

func TestBands_BaselineKeyedByConfig(t *testing.T) {
	t.Parallel()
	b := &Bands{Entries: map[string]BandEntry{}}
	now := time.Now()
	var recs []RunRecord
	for i := range 6 {
		recs = append(recs, recFor("t", "m", "cfgA", OutcomePass, now.Add(time.Duration(i)*time.Minute)))
	}
	for i := range 6 {
		recs = append(recs, recFor("t", "m", "cfgB", OutcomeFail, now.Add(time.Duration(60+i)*time.Minute)))
	}
	b.Recompute("t", recs, "h", now, "m", "cfgA")
	require.Equal(t, 6, b.Baseline("t", "m", "cfgA").Passes)
	require.Equal(t, 0, b.Baseline("t", "m", "cfgB").Passes)
	require.Equal(t, 6, b.Baseline("t", "m", "cfgB").N)
}

// --- quarantine ---

func quarantineRunner(t *testing.T) (*Runner, string) {
	t.Helper()
	root := newEvalDir(t)
	return &Runner{EvalDir: root, QuarantineRepeats: 3, WorkParent: t.TempDir()}, root
}

func TestQuarantine_CleanFailTrajectory(t *testing.T) {
	t.Parallel()
	r, root := quarantineRunner(t)
	dir := writeTrajectory(t, filepath.Join(root, "corpus"), "t1", nil)
	tr, err := LoadTrajectory(dir)
	require.NoError(t, err)
	reason, err := r.Quarantine(context.Background(), tr, dir)
	require.NoError(t, err)
	require.Empty(t, reason)
}

func TestQuarantine_Vacuous(t *testing.T) {
	t.Parallel()
	r, root := quarantineRunner(t)
	// expect fail, but the check always exits 0.
	dir := writeTrajectory(t, filepath.Join(root, "corpus"), "t1", map[string]any{
		"check_script_body": "#!/bin/bash\nexit 0\n",
	})
	tr, err := LoadTrajectory(dir)
	require.NoError(t, err)
	reason, err := r.Quarantine(context.Background(), tr, dir)
	require.NoError(t, err)
	require.Equal(t, ReasonVacuous, reason)
}

func TestQuarantine_Flaky(t *testing.T) {
	t.Parallel()
	r, root := quarantineRunner(t)
	// Alternating pass/fail on the same state → flaky.
	dir := writeTrajectory(t, filepath.Join(root, "corpus"), "t1", map[string]any{
		// Fresh materialization per rep — the flip marker lives in
		// the shared work parent so it alternates across reps.
		"check_script_body": `#!/bin/bash
f="$(dirname "$EVAL_WORKDIR")/.flip"
if [ -f "$f" ]; then rm "$f"; exit 0; else touch "$f"; exit 1; fi
`,
	})
	tr, err := LoadTrajectory(dir)
	require.NoError(t, err)
	reason, err := r.Quarantine(context.Background(), tr, dir)
	require.NoError(t, err)
	require.Equal(t, ReasonFlaky, reason)
}

func TestQuarantine_CounterexamplePassIsVacuous(t *testing.T) {
	t.Parallel()
	r, root := quarantineRunner(t)
	// Pass-guard whose counterexample doesn't break the check.
	dir := writeTrajectory(t, filepath.Join(root, "corpus"), "t1", map[string]any{
		"check":             map[string]any{"script": "check.sh", "expect_start_state": "pass"},
		"check_script_body": "#!/bin/bash\nexit 0\n",
	})
	patch := `diff --git a/hello.txt b/hello.txt
index ce01362..0000000
--- a/hello.txt
+++ /dev/null
@@ -1 +0,0 @@
-hi
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "counterexample.patch"), []byte(patch), 0o644))
	tr, err := LoadTrajectory(dir)
	require.NoError(t, err)
	reason, err := r.Quarantine(context.Background(), tr, dir)
	require.NoError(t, err)
	require.Equal(t, ReasonVacuous, reason)
}

// --- experiment end-to-end with a fake driver ---

// fakeRunner inspects the generated .crush.json: the flag under test
// flips whether the marker file gets written. That exercises the whole
// arm-config → materialize → check pipeline.
type fakeRunner struct {
	flag string
}

func (f fakeRunner) Run(_ context.Context, workdir string, _ []string, _ Budget) RunResult {
	data, _ := os.ReadFile(filepath.Join(workdir, ".crush.json"))
	var doc struct {
		Options map[string]any `json:"options"`
	}
	_ = json.Unmarshal(data, &doc)
	// The marker is written when the flag is OFF — so a treatment arm
	// enabling the flag collapses while control (flag off) passes.
	if on, _ := doc.Options[f.flag].(bool); !on {
		_ = os.WriteFile(filepath.Join(workdir, "fixed.marker"), []byte("x"), 0o644)
	}
	return RunResult{Steps: 3, ModelResolved: "mock/m", Tokens: TokenUsage{Input: 10, Output: 5}}
}

func experimentFixture(t *testing.T, r *Runner, root string, flag string) {
	t.Helper()
	// One stable trajectory with a deep baseline + one mid.
	writeTrajectory(t, filepath.Join(root, "corpus"), "stable-t", map[string]any{
		"coverage": map[string]any{"min_steps": 1},
	})
	writeTrajectory(t, filepath.Join(root, "corpus"), "mid-t", nil)

	manifest := &FlagsManifest{Defaults: map[string]any{flag: false}}
	require.NoError(t, os.WriteFile(filepath.Join(root, "flags.json"),
		[]byte(fmt.Sprintf(`{"flag_defaults":{%q:false}}`, flag)), 0o644))

	bands := &Bands{SchemaVersion: 1, Entries: map[string]BandEntry{
		"stable-t": {
			Band:        BandStable,
			Baselines:   map[string]map[string]BaselineCounts{"mock/m": {manifest.keyWith(map[string]any{flag: false}, map[string]any{"$temperature": "0"}): {Passes: 29, N: 30}}},
			ContentHash: mustHash(t, filepath.Join(root, "corpus", "stable-t")),
		},
		"mid-t": {Band: BandMid, ContentHash: mustHash(t, filepath.Join(root, "corpus", "mid-t"))},
	}}
	require.NoError(t, bands.Save(root))
}

func mustHash(t *testing.T, dir string) string {
	t.Helper()
	h, err := ContentHash(dir)
	require.NoError(t, err)
	return h
}

// The arm gate end to end: a run whose check passes but whose arm
// coverage starves lands inconclusive with coverage_scope "arm", and
// a trajectory-coverage miss records "trajectory".
func TestExecuteRun_ArmCoverageGate(t *testing.T) {
	t.Parallel()
	root := newEvalDir(t)
	trajDir := writeTrajectory(t, filepath.Join(root, "corpus"), "gate-t", nil)
	tr, err := LoadTrajectory(trajDir)
	require.NoError(t, err)
	r := &Runner{
		EvalDir:    root,
		Driver:     fakeRunner{flag: "debug"},
		WorkParent: t.TempDir(),
		RNG:        rand.New(rand.NewPCG(1, 2)),
	}
	exp := &Experiment{Name: "exp1", Model: "mock/m", Temperature: ptr(0.0)}
	manifest := &FlagsManifest{Defaults: map[string]any{"debug": false}}
	off := Arm{Config: ArmConfig{Options: map[string]any{"debug": false}}}

	// Flag off → fakeRunner writes the marker → check passes; the arm
	// demands more steps than the driver's fixed 3 → inconclusive.
	starving := off
	starving.Coverage = Coverage{"min_steps": 100}
	rec, err := r.ExecuteRun(context.Background(), exp, tr, trajDir, ArmTreatment, starving, manifest, 1, "inv")
	require.NoError(t, err)
	require.Equal(t, OutcomeInconclusive, rec.Outcome)
	require.Equal(t, "arm", rec.CheckDetail["coverage_scope"])
	require.Equal(t, "min_steps", rec.CheckDetail["coverage_key"])

	// Same run shape under a starving trajectory predicate → the
	// trajectory scope is recorded instead.
	tr.Coverage = Coverage{"min_steps": 100}
	rec, err = r.ExecuteRun(context.Background(), exp, tr, trajDir, ArmControl, off, manifest, 1, "inv")
	require.NoError(t, err)
	require.Equal(t, OutcomeInconclusive, rec.Outcome)
	require.Equal(t, "trajectory", rec.CheckDetail["coverage_scope"])
	require.Equal(t, "min_steps", rec.CheckDetail["coverage_key"])

	// No coverage anywhere → pass, no scope recorded.
	tr.Coverage = nil
	rec, err = r.ExecuteRun(context.Background(), exp, tr, trajDir, ArmControl, off, manifest, 1, "inv")
	require.NoError(t, err)
	require.Equal(t, OutcomePass, rec.Outcome)
	_, ok := rec.CheckDetail["coverage_scope"]
	require.False(t, ok)
}

func TestRunExperiment_CatastrophicFires(t *testing.T) {
	t.Parallel()
	root := newEvalDir(t)
	flag := "debug"
	r := &Runner{
		EvalDir:        root,
		Driver:         fakeRunner{flag: flag},
		WorkParent:     t.TempDir(),
		PermReplicates: 500,
		RNG:            rand.New(rand.NewPCG(7, 8)),
	}
	experimentFixture(t, r, root, flag)

	// Treatment turns the flag ON → fakeRunner writes the marker →
	// check passes; control leaves it off → fails. To make treatment
	// COLLAPSE instead, invert: treatment has flag false? Simpler:
	// flip which arm enables the marker.
	exp := &Experiment{
		Name: "exp1", Model: "mock/m", Temperature: ptr(0.0),
		Corpus:            []string{"*"},
		RunsPerTrajectory: map[Band]int{BandStable: 3, BandMid: 3, BandUncharacterized: 3},
		Arms: map[string]Arm{
			// Control turns the flag on (marker written → pass);
			// treatment leaves it off (fail) — treatment collapses.
			"control":   {Config: ArmConfig{Options: map[string]any{flag: false}}},
			"treatment": {Config: ArmConfig{Options: map[string]any{flag: true}}},
		},
	}

	rep, err := r.RunExperiment(context.Background(), exp)
	require.NoError(t, err)
	require.NotEmpty(t, rep.Catastrophic)
	require.Contains(t, rep.Catastrophic[0], "stable-t")

	// Records were written for both arms.
	recs, err := r.LoadExperimentRecords("exp1")
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(recs), 12) // 2 trajs × 2 arms × 3.
}

func TestRunExperiment_DiffuseAlarmOnUniformShift(t *testing.T) {
	t.Parallel()
	root := newEvalDir(t)
	flag := "debug"
	r := &Runner{
		EvalDir:        root,
		Driver:         fakeRunner{flag: flag},
		WorkParent:     t.TempDir(),
		PermReplicates: 2000,
		RNG:            rand.New(rand.NewPCG(3, 4)),
	}
	// All mid-band trajectories.
	for i := range 6 {
		id := fmt.Sprintf("mid-%d", i)
		writeTrajectory(t, filepath.Join(root, "corpus"), id, nil)
	}
	require.NoError(t, os.WriteFile(filepath.Join(root, "flags.json"),
		[]byte(fmt.Sprintf(`{"flag_defaults":{%q:false}}`, flag)), 0o644))
	bands := &Bands{SchemaVersion: 1, Entries: map[string]BandEntry{}}
	for i := range 6 {
		id := fmt.Sprintf("mid-%d", i)
		bands.Entries[id] = BandEntry{Band: BandMid, ContentHash: mustHash(t, filepath.Join(root, "corpus", id))}
	}
	require.NoError(t, bands.Save(root))

	exp := &Experiment{
		Name: "exp2", Model: "mock/m", Temperature: ptr(0.0),
		Corpus:            []string{"band:mid"},
		RunsPerTrajectory: map[Band]int{BandMid: 5},
		Arms: map[string]Arm{
			"control":   {Config: ArmConfig{Options: map[string]any{flag: false}}},
			"treatment": {Config: ArmConfig{Options: map[string]any{flag: true}}},
		},
	}
	rep, err := r.RunExperiment(context.Background(), exp)
	require.NoError(t, err)
	require.Less(t, rep.DiffuseP, 0.05)
	require.Empty(t, rep.Catastrophic)
}

func TestWriteArmConfig_CollisionAndContent(t *testing.T) {
	t.Parallel()
	wd := t.TempDir()
	exp := &Experiment{Model: "hyper/x", Temperature: ptr(0.0)}
	arm := Arm{Config: ArmConfig{Options: map[string]any{"flag_a": true}}}
	require.NoError(t, WriteArmConfig(wd, exp, arm, &FlagsManifest{Defaults: map[string]any{}}))

	rc, err := os.ReadFile(filepath.Join(wd, ".crushrc"))
	require.NoError(t, err)
	require.Contains(t, string(rc), "model large hyper/x")
	require.Contains(t, string(rc), "--temperature 0")

	js, err := os.ReadFile(filepath.Join(wd, ".crush.json"))
	require.NoError(t, err)
	require.Contains(t, string(js), `"flag_a": true`)
	require.Contains(t, string(js), `"disable_metrics": true`)

	// A pre-existing config is an error, not a silent override.
	wd2 := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(wd2, "crushrc"), []byte("x"), 0o644))
	require.Error(t, WriteArmConfig(wd2, exp, arm, &FlagsManifest{Defaults: map[string]any{}}))
}

func ptr[T any](v T) *T { return &v }

func TestBands_SuspectCheckIgnoresArmCorrelation(t *testing.T) {
	t.Parallel()
	b := &Bands{Entries: map[string]BandEntry{}}
	now := time.Now()
	// A real treatment effect — control passes, treatment fails every
	// interleaved round — must NOT read as a flaky check: each arm is
	// individually constant.
	var recs []RunRecord
	for i := range 12 {
		c := recFor("t", "m", "c", OutcomePass, now.Add(time.Duration(2*i)*time.Minute))
		c.Arm = "control"
		tr := recFor("t", "m", "c", OutcomeFail, now.Add(time.Duration(2*i+1)*time.Minute))
		tr.Arm = "treatment"
		recs = append(recs, c, tr)
	}
	b.Recompute("t", recs, "h", now, "m", "c")
	require.NotEqual(t, ReasonSuspectCheck, b.Entries["t"].QuarantineReason)
}

func TestBands_StaleModelBaselineDoesNotHoldStable(t *testing.T) {
	t.Parallel()
	b := &Bands{Entries: map[string]BandEntry{}}
	now := time.Now()
	// Deep baseline under the OLD model; the current pin has nothing.
	var recs []RunRecord
	for i := range 25 {
		recs = append(recs, recFor("t", "old/model", "cfg", OutcomePass, now.Add(time.Duration(i)*time.Minute)))
	}
	b.Recompute("t", recs, "h", now, "old/model", "cfg")
	b.Recompute("t", recs, "h", now, "old/model", "cfg")
	require.Equal(t, BandStable, b.Band("t"))

	// Re-pin: current condition is a different model — the stale
	// baseline must not hold the band.
	b.Recompute("t", recs, "h", now, "new/model", "cfg")
	require.Equal(t, BandUncharacterized, b.Band("t"))
}

func TestCheckRequires(t *testing.T) {
	t.Parallel()
	missing := CheckRequires(&Trajectory{
		Requires: Requires{Tools: []string{"definitely-not-a-real-binary-xyz"}},
	})
	require.Equal(t, []string{"tool:definitely-not-a-real-binary-xyz"}, missing)

	require.Empty(t, CheckRequires(&Trajectory{
		Requires: Requires{Tools: []string{"go"}, OS: []string{runtime.GOOS}},
	}))
	require.NotEmpty(t, CheckRequires(&Trajectory{
		Requires: Requires{OS: []string{"plan9"}},
	}))
}

func TestWriteArmConfig_MergesJSONConfig(t *testing.T) {
	t.Parallel()
	exp := &Experiment{Model: "hyper/x"}
	arm := Arm{Config: ArmConfig{Options: map[string]any{"flag_a": true}}}

	// A start-state .crush.json merges: its keys survive, arm wins.
	wd := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(wd, ".crush.json"),
		[]byte(`{"options":{"fixture_key":"keep","flag_a":false},"other":"x"}`), 0o644))
	require.NoError(t, WriteArmConfig(wd, exp, arm, &FlagsManifest{Defaults: map[string]any{}}))
	raw, err := os.ReadFile(filepath.Join(wd, ".crush.json"))
	require.NoError(t, err)
	require.Contains(t, string(raw), `"fixture_key": "keep"`)
	require.Contains(t, string(raw), `"flag_a": true`)
	require.Contains(t, string(raw), `"other": "x"`)

	// crush.json is lower precedence than .crush.json — allowed.
	wd2 := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(wd2, "crush.json"), []byte(`{}`), 0o644))
	require.NoError(t, WriteArmConfig(wd2, exp, arm, &FlagsManifest{Defaults: map[string]any{}}))
}

func TestBands_PinSpellingVsResolved(t *testing.T) {
	t.Parallel()
	b := &Bands{Entries: map[string]BandEntry{}}
	now := time.Now()
	// Records stamped with the pin's alias spelling but a canonical
	// resolved model — banding must follow the resolved key.
	var recs []RunRecord
	for i := range 25 {
		r := recFor("t", "canonical/x", "cfg", OutcomePass, now.Add(time.Duration(i)*time.Minute))
		r.Env.ModelPin = "alias/x"
		recs = append(recs, r)
	}
	b.Recompute("t", recs, "h", now, "alias/x", "cfg")
	b.Recompute("t", recs, "h", now, "alias/x", "cfg")
	require.Equal(t, BandStable, b.Band("t"))

	// A different pin's records never count toward this condition.
	var other []RunRecord
	for i := range 25 {
		r := recFor("t", "canonical/x", "cfg", OutcomePass, now.Add(time.Duration(i)*time.Minute))
		r.Env.ModelPin = "other/x"
		other = append(other, r)
	}
	b2 := &Bands{Entries: map[string]BandEntry{}}
	b2.Recompute("t", other, "h", now, "alias/x", "cfg")
	require.Equal(t, BandUncharacterized, b2.Band("t"))
}

func TestRunExperiment_AllSkippedFailsClosed(t *testing.T) {
	t.Parallel()
	root := newEvalDir(t)
	r := &Runner{EvalDir: root, Driver: fakeRunner{flag: "debug"}, WorkParent: t.TempDir()}
	require.NoError(t, os.WriteFile(filepath.Join(root, "flags.json"),
		[]byte(`{"flag_defaults":{"debug":false}}`), 0o644))
	writeTrajectory(t, filepath.Join(root, "corpus"), "needs-missing", map[string]any{
		"requires": map[string]any{"tools": []string{"definitely-not-a-real-binary-xyz"}},
	})

	exp := &Experiment{
		Name: "exp-skip", Model: "mock/m", Corpus: []string{"*"}, Temperature: ptr(0.0),
		RunsPerTrajectory: map[Band]int{BandUncharacterized: 2},
		Arms: map[string]Arm{
			"control":   {Config: ArmConfig{Options: map[string]any{"debug": false}}},
			"treatment": {Config: ArmConfig{Options: map[string]any{"debug": true}}},
		},
	}
	rep, err := r.RunExperiment(context.Background(), exp)
	require.Error(t, err) // Fail closed — no PASS on zero samples.
	require.NotEmpty(t, rep.Skipped)
}

// fakeRunnerFail fails every run regardless of arm — the
// model-drift/rot case the coincidence detector exists for.
type fakeRunnerFail struct{}

func (fakeRunnerFail) Run(_ context.Context, _ string, _ []string, _ Budget) RunResult {
	return RunResult{Steps: 3, ModelResolved: "mock/m"}
}

func TestRunExperiment_CoincidentCollapse(t *testing.T) {
	t.Parallel()
	root := newEvalDir(t)
	flag := "debug"
	r := &Runner{
		EvalDir:        root,
		Driver:         fakeRunnerFail{},
		WorkParent:     t.TempDir(),
		PermReplicates: 500,
		RNG:            rand.New(rand.NewPCG(7, 8)),
	}
	experimentFixture(t, r, root, flag)

	exp := &Experiment{
		Name: "exp-coincident", Model: "mock/m", Temperature: ptr(0.0),
		Corpus:            []string{"*"},
		RunsPerTrajectory: map[Band]int{BandStable: 3, BandMid: 3, BandUncharacterized: 3},
		Arms: map[string]Arm{
			"control":   {Config: ArmConfig{Options: map[string]any{flag: false}}},
			"treatment": {Config: ArmConfig{Options: map[string]any{flag: true}}},
		},
	}
	rep, err := r.RunExperiment(context.Background(), exp)
	require.NoError(t, err)
	// Both arms failed vs the same deep baseline — the gate must
	// attribute rot/drift, not the flag.
	require.NotEmpty(t, rep.Coincident)
	require.Empty(t, rep.Catastrophic)
}

// fakeRunnerResolved reports a fixed resolved projection regardless
// of arm — the flag under test no-ops.
type fakeRunnerResolved struct {
	resolved map[string]any
	pass     bool
}

func (f fakeRunnerResolved) Run(_ context.Context, workdir string, _ []string, _ Budget) RunResult {
	if f.pass {
		_ = os.WriteFile(filepath.Join(workdir, "fixed.marker"), []byte("x"), 0o644)
	}
	return RunResult{Steps: 3, ModelResolved: "mock/m", ResolvedOptions: f.resolved}
}

// Identical resolved projections under differing arm intents →
// noop-flag alarm: the pairing is a guaranteed null.
func TestRunExperiment_NoopFlagAlarm(t *testing.T) {
	t.Parallel()
	root := newEvalDir(t)
	r := &Runner{
		EvalDir:    root,
		Driver:     fakeRunnerResolved{resolved: map[string]any{"debug": false}, pass: true},
		WorkParent: t.TempDir(),
		RNG:        rand.New(rand.NewPCG(1, 2)),
	}
	writeTrajectory(t, filepath.Join(root, "corpus"), "stable-t", nil)
	require.NoError(t, os.WriteFile(filepath.Join(root, "flags.json"),
		[]byte(`{"flag_defaults":{"debug":false}}`), 0o644))
	bands := &Bands{SchemaVersion: 1, Entries: map[string]BandEntry{
		"stable-t": {Band: BandMid, ContentHash: mustHash(t, filepath.Join(root, "corpus", "stable-t"))},
	}}
	require.NoError(t, bands.Save(root))

	exp := &Experiment{
		Name: "noop-exp", Model: "mock/m", Temperature: ptr(0.0),
		Arms: map[string]Arm{
			"control":   {Config: ArmConfig{Options: map[string]any{"debug": false}}},
			"treatment": {Config: ArmConfig{Options: map[string]any{"debug": true}}},
		},
		Corpus:            []string{"*"},
		RunsPerTrajectory: map[Band]int{BandMid: 3},
	}
	rep, err := r.RunExperiment(t.Context(), exp)
	require.NoError(t, err)
	require.Contains(t, rep.NoopFlags, "stable-t")
	require.True(t, rep.Fired(0.05))
}

// A manifest key that isn't a config.Options field must be rejected —
// otherwise the flag silently no-ops and both arms resolve identically.
func TestLoadFlagsManifest_RejectsUnknownKeys(t *testing.T) {
	t.Parallel()
	root := newEvalDir(t)
	require.NoError(t, os.WriteFile(filepath.Join(root, "flags.json"),
		[]byte(`{"flag_defaults":{"notebook_stub_superseeded":false}}`), 0o644))
	_, err := LoadFlagsManifest(root)
	require.ErrorContains(t, err, "not a config.Options key")
}

// Resolved-vs-intent key divergence: the record's resolved
// baseline_key must be what banding joins on — manifest intent keys
// would orphan every record under a different namespace.
func TestRecomputeAll_ResolvedKeyJoins(t *testing.T) {
	t.Parallel()
	root := newEvalDir(t)
	writeTrajectory(t, filepath.Join(root, "corpus"), "t1", nil)
	require.NoError(t, os.WriteFile(filepath.Join(root, "flags.json"),
		[]byte(`{"flag_defaults":{"debug":false}}`), 0o644))
	manifest, err := LoadFlagsManifest(root)
	require.NoError(t, err)

	// Resolved says true where intent says false — stands in for the
	// nil-vs-default divergence on non-materialized options.
	resolved := map[string]any{"debug": true}
	resolvedKey := manifest.BaselineKey(resolved)
	intentKey := manifest.BaselineKey(nil)
	require.NotEqual(t, intentKey, resolvedKey)

	r := &Runner{EvalDir: root}
	hash := mustHash(t, filepath.Join(root, "corpus", "t1"))
	base := time.Now()
	for i := range 30 {
		require.NoError(t, r.appendRecord(RunRecord{
			Experiment: "char", TrajectoryID: "t1", Arm: "baseline",
			BaselineKey: resolvedKey, ResolvedOptions: resolved,
			Outcome: OutcomePass, StartedAt: base.Add(time.Duration(i) * time.Second),
			Env: Env{ModelPin: "mock/m", ModelResolved: "mock/m", ContentHash: hash},
		}))
	}

	bands := &Bands{Entries: map[string]BandEntry{}}
	corpus, err := LoadCorpus(root)
	require.NoError(t, err)
	require.NoError(t, r.RecomputeAll(bands, corpus, manifest, "mock/m", "0"))

	entry := bands.Entry("t1")
	// The baseline accumulated under the RESOLVED key — intent key
	// must stay empty, and 30/30 passes move the band off
	// uncharacterized (stable itself needs a promotion streak).
	require.Equal(t, 30, entry.Baselines["mock/m"][resolvedKey].N)
	require.Zero(t, entry.Baselines["mock/m"][intentKey].N)
	require.Equal(t, BandMid, entry.Band)
}

// Harness invariants can't be arm-overridden — data_directory
// relocation would break telemetry/session-DB paths.
func TestWriteArmConfig_RejectsHarnessInvariants(t *testing.T) {
	t.Parallel()
	wd := t.TempDir()
	exp := &Experiment{Model: "hyper/x", Temperature: ptr(0.0)}
	manifest := &FlagsManifest{Defaults: map[string]any{"data_directory": "/x"}}
	err := WriteArmConfig(wd, exp, Arm{Config: ArmConfig{Options: map[string]any{"data_directory": "evil"}}}, manifest)
	require.ErrorContains(t, err, "harness manages")
	err = WriteArmConfig(t.TempDir(), exp, Arm{Config: ArmConfig{Options: map[string]any{"disable_metrics": false}}}, manifest)
	require.ErrorContains(t, err, "harness manages")
}

// A fixture pinning a manifest flag — in EITHER config file — keys
// the trajectory under a foreign condition forever.
func TestWriteArmConfig_RejectsFixtureManifestFlags(t *testing.T) {
	t.Parallel()
	manifest := &FlagsManifest{Defaults: map[string]any{"auto_lsp": true}}
	exp := &Experiment{Model: "hyper/x", Temperature: ptr(0.0)}
	arm := Arm{Config: ArmConfig{Options: map[string]any{"auto_lsp": false}}}

	wd := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(wd, ".crush.json"),
		[]byte(`{"options":{"auto_lsp":false}}`), 0o644))
	require.ErrorContains(t, WriteArmConfig(wd, exp, arm, manifest), "manifest flag")

	// crush.json too — lower precedence, same trap.
	wd2 := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(wd2, "crush.json"),
		[]byte(`{"options":{"auto_lsp":false}}`), 0o644))
	require.ErrorContains(t, WriteArmConfig(wd2, exp, arm, manifest), "manifest flag")
}

// Experiments must pin temperature — unpinned arms land in the
// "default" temp cell no characterized baseline joins.
func TestValidateExperiment_RequiresTemperature(t *testing.T) {
	t.Parallel()
	exp := &Experiment{
		Name: "x", Model: "mock/m", Corpus: []string{"*"},
		RunsPerTrajectory: map[Band]int{BandMid: 1},
		Arms: map[string]Arm{
			ArmControl:   {},
			ArmTreatment: {},
		},
	}
	require.ErrorContains(t, ValidateExperiment(exp), "temperature")
}
