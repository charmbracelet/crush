package eval

import (
	"context"
	"encoding/json"
	"math/rand/v2"
	"os"
	"path/filepath"
	"testing"

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
