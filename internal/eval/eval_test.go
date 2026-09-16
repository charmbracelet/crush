package eval

import (
	"context"
	"encoding/json"
	"math/rand/v2"
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

// writeTrajectory scaffolds corpus/<id>/ with a spec, check.sh, and
// fixture. Returns the trajectory dir.
func writeTrajectory(t *testing.T, corpusRoot, id string, spec map[string]any) string {
	t.Helper()
	if spec == nil {
		spec = map[string]any{}
	}
	dir := filepath.Join(corpusRoot, id)
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "fixture"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "fixture", "hello.txt"), []byte("hi\n"), 0o644))
	if _, ok := spec["check"]; !ok {
		spec["check"] = map[string]any{"script": "check.sh", "expect_start_state": "fail"}
	}
	if _, ok := spec["check_script_body"]; !ok {
		spec["check_script_body"] = "#!/bin/bash\ntest -f fixed.marker\n"
	}
	body := spec["check_script_body"].(string)
	delete(spec, "check_script_body")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "check.sh"), []byte(body), 0o755))

	if _, ok := spec["origin"]; !ok {
		spec["origin"] = map[string]any{"kind": "synthetic"}
	}
	if _, ok := spec["start_state"]; !ok {
		spec["start_state"] = map[string]any{"kind": "fixture", "fixture_dir": "fixture"}
	}
	if _, ok := spec["task"]; !ok {
		spec["task"] = map[string]any{"turns": []string{"fix it"}}
	}
	spec["id"] = id
	spec["schema_version"] = 1
	data, err := json.Marshal(spec)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "trajectory.json"), data, 0o644))
	return dir
}

func newEvalDir(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "corpus"), 0o755))
	return root
}

// --- trajectory validation ---

func TestValidateTrajectory_PassGuardRequiresCounterexample(t *testing.T) {
	t.Parallel()
	root := newEvalDir(t)
	dir := writeTrajectory(t, filepath.Join(root, "corpus"), "t1", map[string]any{
		"check": map[string]any{"script": "check.sh", "expect_start_state": "pass"},
	})
	tr, err := LoadTrajectory(dir)
	require.Error(t, err)
	require.Nil(t, tr)
	require.Contains(t, err.Error(), "counterexample.patch is required")
}

func TestValidateTrajectory_CounterexampleOnFailIsError(t *testing.T) {
	t.Parallel()
	root := newEvalDir(t)
	dir := writeTrajectory(t, filepath.Join(root, "corpus"), "t1", nil)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "counterexample.patch"), []byte("x"), 0o644))
	_, err := LoadTrajectory(dir)
	require.Error(t, err)
	require.Contains(t, err.Error(), "counterexample.patch on a \"fail\" trajectory")
}

func TestValidateTrajectory_RegressionRequiresReference(t *testing.T) {
	t.Parallel()
	root := newEvalDir(t)
	dir := writeTrajectory(t, filepath.Join(root, "corpus"), "t1", map[string]any{
		"origin": map[string]any{"kind": "regression"},
	})
	_, err := LoadTrajectory(dir)
	require.Error(t, err)
	require.Contains(t, err.Error(), "reference.patch is required")
}

func TestValidateTrajectory_ProductionRequiresScrubbed(t *testing.T) {
	t.Parallel()
	root := newEvalDir(t)
	dir := writeTrajectory(t, filepath.Join(root, "corpus"), "t1", map[string]any{
		"origin": map[string]any{"kind": "production"},
	})
	_, err := LoadTrajectory(dir)
	require.Error(t, err)
	require.Contains(t, err.Error(), "scrubbed")
}

func TestValidateTrajectory_GitNetworkConflict(t *testing.T) {
	t.Parallel()
	root := newEvalDir(t)
	dir := writeTrajectory(t, filepath.Join(root, "corpus"), "t1", map[string]any{
		"start_state": map[string]any{"kind": "git", "repo": "https://example.com/r.git", "ref": "abc"},
		"requires":    map[string]any{"network": false},
	})
	_, err := LoadTrajectory(dir)
	require.Error(t, err)
	require.Contains(t, err.Error(), "network=false")
}

func TestValidateTrajectory_OK(t *testing.T) {
	t.Parallel()
	root := newEvalDir(t)
	dir := writeTrajectory(t, filepath.Join(root, "corpus"), "t1", nil)
	tr, err := LoadTrajectory(dir)
	require.NoError(t, err)
	require.Equal(t, "t1", tr.ID)
}

// --- content hash ---

func TestContentHash_ScopesToRevision(t *testing.T) {
	t.Parallel()
	root := newEvalDir(t)
	dir := writeTrajectory(t, filepath.Join(root, "corpus"), "t1", nil)
	h1, err := ContentHash(dir)
	require.NoError(t, err)
	h2, err := ContentHash(dir)
	require.NoError(t, err)
	require.Equal(t, h1, h2)

	// Editing the check changes the hash — samples scored by the old
	// function are stale.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "check.sh"), []byte("#!/bin/bash\nexit 0\n"), 0o755))
	h3, err := ContentHash(dir)
	require.NoError(t, err)
	require.NotEqual(t, h1, h3)
}

// --- check.sh contract ---

func TestRunCheck_PassFailAndEvalJSON(t *testing.T) {
	t.Parallel()
	root := newEvalDir(t)
	dir := writeTrajectory(t, filepath.Join(root, "corpus"), "t1", map[string]any{
		"check_script_body": "#!/bin/bash\necho 'EVAL_JSON {\"why\":\"marker missing\"}'\nexit 1\n",
	})
	tr, err := LoadTrajectory(dir)
	require.NoError(t, err)
	wd := t.TempDir()
	res := RunCheck(context.Background(), tr, dir, wd, nil)
	require.NoError(t, res.Err)
	require.Equal(t, 1, res.Exit)
	require.Equal(t, "marker missing", res.Detail["why"])

	// Malformed EVAL_JSON never changes the verdict.
	dir2 := writeTrajectory(t, filepath.Join(root, "corpus"), "t2", map[string]any{
		"check_script_body": "#!/bin/bash\necho 'EVAL_JSON {broken'\nexit 0\n",
	})
	tr2, err := LoadTrajectory(dir2)
	require.NoError(t, err)
	res2 := RunCheck(context.Background(), tr2, dir2, wd, nil)
	require.Equal(t, 0, res2.Exit)
	require.Nil(t, res2.Detail)
}

func TestRunCheck_TimeoutIsError(t *testing.T) {
	t.Parallel()
	root := newEvalDir(t)
	dir := writeTrajectory(t, filepath.Join(root, "corpus"), "t1", map[string]any{
		"check":             map[string]any{"script": "check.sh", "expect_start_state": "fail", "timeout_seconds": 1},
		"check_script_body": "#!/bin/bash\nsleep 5\n",
	})
	tr, err := LoadTrajectory(dir)
	require.NoError(t, err)
	res := RunCheck(context.Background(), tr, dir, t.TempDir(), nil)
	require.Error(t, res.Err)
	require.Contains(t, res.Err.Error(), "timed out")
}

// --- coverage ---

func TestCoverage_MinMaxAndClosedGrammar(t *testing.T) {
	t.Parallel()
	rec := &RunRecord{Steps: 6}
	rec.StubStats.BoundaryAdvances = 2
	met, err := CoverageMet(Coverage{"min_steps": 3, "min_stub_stats.boundary_advances": 1}, rec)
	require.NoError(t, err)
	require.True(t, met)

	met, err = CoverageMet(Coverage{"min_stub_stats.boundary_advances": 3}, rec)
	require.NoError(t, err)
	require.False(t, met)

	_, _, err = ParseCoverageKey("min_steps")
	require.NoError(t, err)
	_, _, err = ParseCoverageKey("min_bogus")
	require.Error(t, err)
	_, _, err = ParseCoverageKey("gte_steps")
	require.Error(t, err)
}

func TestCoverage_StubKinds(t *testing.T) {
	t.Parallel()
	rec := &RunRecord{}
	rec.StubStats.Kinds = map[string]int{"deleted": 2, "superseded": 1}

	// Every declared kind is a valid coverage field — a new kind that
	// missed registration fails here.
	for _, kind := range message.StubKinds() {
		_, _, err := ParseCoverageKey("min_stub_stats.kinds." + kind.String())
		require.NoError(t, err, "kind %q must be a valid coverage field", kind)
	}

	met, err := CoverageMet(Coverage{"min_stub_stats.kinds.deleted": 1}, rec)
	require.NoError(t, err)
	require.True(t, met)

	// The empty-string superseded mark spells "superseded" in the
	// kinds map — and is a valid predicate.
	met, err = CoverageMet(Coverage{"min_stub_stats.kinds.superseded": 1}, rec)
	require.NoError(t, err)
	require.True(t, met)

	// Kinds absent from the map count as 0.
	met, err = CoverageMet(Coverage{"min_stub_stats.kinds.rerun": 1}, rec)
	require.NoError(t, err)
	require.False(t, met)

	// An unknown kind is still rejected by the closed grammar.
	_, _, err = ParseCoverageKey("min_stub_stats.kinds.bogus")
	require.Error(t, err)
}

func TestCoverage_ArmScoped(t *testing.T) {
	t.Parallel()

	// The arm grammar reaches the flag-dependent call_metrics fields
	// trajectory coverage excludes — asserting "the model called map"
	// is legal only where the arm itself fixes project_index.
	_, _, err := ParseArmCoverageKey("min_call_metrics.map_calls_ok")
	require.NoError(t, err)
	_, _, err = ParseCoverageKey("min_call_metrics.map_calls_ok")
	require.Error(t, err)

	// Shared fields work in both grammars.
	_, _, err = ParseArmCoverageKey("min_stub_stats.results")
	require.NoError(t, err)

	rec := &RunRecord{CallMetrics: &CallMetrics{MapCalls: 3, MapCallsOK: 2}}
	met, err := ArmCoverageMet(Coverage{"min_call_metrics.map_calls": 1}, rec)
	require.NoError(t, err)
	require.True(t, met)

	met, err = ArmCoverageMet(Coverage{"min_call_metrics.map_calls_ok": 3}, rec)
	require.NoError(t, err)
	require.False(t, met)

	// Same fail-closed rule as trajectory coverage: absent analysis
	// starves call_metrics predicates, even max_ ones.
	met, err = ArmCoverageMet(Coverage{"max_call_metrics.map_result_bytes": 1024}, &RunRecord{})
	require.NoError(t, err)
	require.False(t, met)

	// Unknown fields stay rejected.
	_, _, err = ParseArmCoverageKey("min_call_metrics.bogus")
	require.Error(t, err)
}

func TestValidateExperiment_ArmCoverage(t *testing.T) {
	t.Parallel()
	temp := 0.0
	exp := &Experiment{
		Name:              "x",
		Model:             "p/m",
		Temperature:       &temp,
		Corpus:            []string{"*"},
		RunsPerTrajectory: map[Band]int{BandUncharacterized: 1},
		Arms: map[string]Arm{
			ArmControl:   {},
			ArmTreatment: {Coverage: Coverage{"min_call_metrics.map_calls": 1}},
		},
	}
	require.NoError(t, ValidateExperiment(exp))

	exp.Arms[ArmTreatment] = Arm{Coverage: Coverage{"min_bogus_field": 1}}
	require.Error(t, ValidateExperiment(exp))
}

func TestValidateExperiment_ArmCoverageStarvation(t *testing.T) {
	t.Parallel()
	temp := 0.0
	exp := &Experiment{
		Name:              "x",
		Model:             "p/m",
		Temperature:       &temp,
		Corpus:            []string{"*"},
		RunsPerTrajectory: map[Band]int{BandUncharacterized: 1},
		Arms:              map[string]Arm{ArmControl: {}, ArmTreatment: {}},
	}

	// min_ on a flag-gated field where the arm sets the flag off —
	// every run starves, so this is a load error.
	exp.Arms[ArmControl] = Arm{
		Config:   ArmConfig{Options: map[string]any{"notebook_stub_superseded": false}},
		Coverage: Coverage{"min_stub_stats.results": 1},
	}
	require.Error(t, ValidateExperiment(exp))

	// Same predicate on the arm that enables the flag is the intended
	// firing assertion — legal.
	exp.Arms[ArmControl] = Arm{}
	exp.Arms[ArmTreatment] = Arm{
		Config:   ArmConfig{Options: map[string]any{"notebook_stub_superseded": true}},
		Coverage: Coverage{"min_stub_stats.results": 1},
	}
	require.NoError(t, ValidateExperiment(exp))

	// The question tool is interactive-only — min_ predicates on
	// question_* starve in headless runs regardless of flags.
	exp.Arms[ArmTreatment] = Arm{Coverage: Coverage{"min_call_metrics.question_calls": 1}}
	require.Error(t, ValidateExperiment(exp))

	// max_ is a bound, not a firing assertion — still legal.
	exp.Arms[ArmTreatment] = Arm{Coverage: Coverage{"max_call_metrics.question_calls": 0}}
	require.NoError(t, ValidateExperiment(exp))

	// map_calls_ok needs the flag; an arm that sets it off starves.
	exp.Arms[ArmTreatment] = Arm{
		Config:   ArmConfig{Options: map[string]any{"project_index": false}},
		Coverage: Coverage{"min_call_metrics.map_calls_ok": 1},
	}
	require.Error(t, ValidateExperiment(exp))

	// map_calls itself is NOT guarded — tool-not-found attempts count,
	// so flag-off arms can measure unprompted map reach.
	exp.Arms[ArmTreatment] = Arm{
		Config:   ArmConfig{Options: map[string]any{"project_index": false}},
		Coverage: Coverage{"min_call_metrics.map_calls": 1},
	}
	require.NoError(t, ValidateExperiment(exp))

	// wrong_pointer_events needs a successful map call — zero on
	// flag-off arms, so min_ starves there.
	exp.Arms[ArmTreatment] = Arm{
		Config:   ArmConfig{Options: map[string]any{"project_index": false}},
		Coverage: Coverage{"min_call_metrics.wrong_pointer_events": 1},
	}
	require.Error(t, ValidateExperiment(exp))
}

func TestValidateArmCoverageResolved(t *testing.T) {
	t.Parallel()
	manifest := &FlagsManifest{Defaults: map[string]any{
		"notebook_stub_superseded": false,
		"notebook_enabled":         true,
		"project_index":            false,
	}}
	exp := &Experiment{Arms: map[string]Arm{
		ArmControl:   {},
		ArmTreatment: {},
	}}

	// Firing assertion with NO config: the flag resolves to the
	// manifest's false default — silent starvation, now an error.
	exp.Arms[ArmTreatment] = Arm{Coverage: Coverage{"min_stub_stats.results": 1}}
	require.Error(t, ValidateArmCoverageResolved(exp, manifest))

	// Enabled by the arm — resolves on.
	exp.Arms[ArmTreatment] = Arm{
		Config:   ArmConfig{Options: map[string]any{"notebook_stub_superseded": true}},
		Coverage: Coverage{"min_stub_stats.results": 1},
	}
	require.NoError(t, ValidateArmCoverageResolved(exp, manifest))

	// Enabled by manifest default — resolves on without arm config.
	manifest.Defaults["project_index"] = true
	exp.Arms[ArmTreatment] = Arm{Coverage: Coverage{"min_call_metrics.map_result_bytes": 1}}
	require.NoError(t, ValidateArmCoverageResolved(exp, manifest))
}

func TestValidateTrajectory_FlagGatedMinRejected(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "fixture"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "check.sh"), []byte("#!/bin/sh\nexit 1\n"), 0o755))
	mk := func(cov Coverage) *Trajectory {
		return &Trajectory{
			ID: "x", SchemaVersion: 1,
			Origin:     Origin{Kind: "synthetic"},
			StartState: StartState{Kind: "fixture", FixtureDir: "fixture"},
			Task:       Task{Turns: []string{"do it"}},
			Check:      Check{Script: "check.sh", ExpectStartState: "fail"},
			Coverage:   cov,
		}
	}

	// A shared min_ on a flag-gated field starves the flag-off arm —
	// load-time error, not a surprise at run time.
	problems := ValidateTrajectory(mk(Coverage{"min_stub_stats.results": 1}), dir)
	require.NotEmpty(t, problems)
	require.Contains(t, problems[0], "flag-gated")

	problems = ValidateTrajectory(mk(Coverage{"min_recalls.entry": 1}), dir)
	require.NotEmpty(t, problems)

	// max_ bounds both arms legitimately — still legal unscoped.
	problems = ValidateTrajectory(mk(Coverage{"max_stub_stats.results": 10}), dir)
	require.Empty(t, problems)

	// Flag-invariant fields unaffected.
	problems = ValidateTrajectory(mk(Coverage{"min_steps": 1}), dir)
	require.Empty(t, problems)
}

// --- run telemetry ---

// TestRunTelemetry_StubKinds pins the telemetry contract end to end:
// the child's stub_stats.kinds object parses, and per-turn kind deltas
// sum into the trajectory totals like the other counters.
func TestRunTelemetry_StubKinds(t *testing.T) {
	t.Parallel()
	writeTel := func(kinds map[string]int) runTelemetry {
		doc := map[string]any{
			"session_id": "s1",
			"steps":      3,
			"stub_stats": map[string]any{
				"invalidations":     1,
				"results":           2,
				"saved_bytes":       900,
				"boundary_advances": 1,
				"kinds":             kinds,
			},
		}
		path := filepath.Join(t.TempDir(), "tel.json")
		data, err := json.Marshal(doc)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(path, data, 0o644))
		tel, err := readTelemetry(path)
		require.NoError(t, err)
		return tel
	}

	var res RunResult
	res.addTurnTelemetry(writeTel(map[string]int{"superseded": 1, "stale": 1}))
	// A mid-run turn may emit no kinds at all — sparse map or absent.
	res.addTurnTelemetry(writeTel(nil))
	res.addTurnTelemetry(writeTel(map[string]int{"deleted": 2}))

	require.Equal(t, 9, res.Steps)
	require.Equal(t, 6, res.StubStats.Results)
	require.Equal(t, map[string]int{"superseded": 1, "stale": 1, "deleted": 2}, res.StubStats.Kinds)
}

// --- stats ---

func TestFisherExactCollapse_KnownValues(t *testing.T) {
	t.Parallel()
	// 0/3 vs 3/3 → one-sided p = 0.05.
	require.InDelta(t, 0.05, FisherExactCollapse(3, 3, 0, 3), 1e-9)
	// 0/3 vs 29/30 → ≈ 7.33e-4.
	require.InDelta(t, 0.000733, FisherExactCollapse(3, 3, 1, 30), 1e-5)
	// 0/3 vs 19/20 → ≈ 0.00226.
	require.InDelta(t, 0.002259, FisherExactCollapse(3, 3, 1, 20), 1e-5)
	// No failures in current arm → p = 1.
	require.InDelta(t, 1.0, FisherExactCollapse(0, 3, 5, 30), 1e-9)
}

func TestPermutationP_DetectsShift(t *testing.T) {
	t.Parallel()
	rng := rand.New(rand.NewPCG(1, 2))

	// Uniform degradation: treatment fails everywhere, control passes.
	var pairs []ArmPair
	for range 10 {
		pairs = append(pairs, ArmPair{
			Control:   []bool{true, true, true, true, true},
			Treatment: []bool{false, false, false, false, false},
		})
	}
	p := PermutationP(pairs, 2000, rng)
	require.Less(t, p, 0.001)

	// Null-ish: arms identical → p should be well above 0.05.
	var nullPairs []ArmPair
	for range 10 {
		nullPairs = append(nullPairs, ArmPair{
			Control:   []bool{true, false, true, true, false},
			Treatment: []bool{true, false, true, true, false},
		})
	}
	p = PermutationP(nullPairs, 2000, rng)
	require.Greater(t, p, 0.05)
}
