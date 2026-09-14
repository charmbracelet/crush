package eval

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// DefaultEvalDir is where the corpus lives relative to the repo root.
const DefaultEvalDir = "eval"

// Validation collects every trajectory spec violation instead of
// failing on the first — corpus review wants the full list.
type Validation struct {
	TrajectoryID string
	Problems     []string
}

func (v Validation) Error() string {
	return fmt.Sprintf("trajectory %s: %s", v.TrajectoryID, strings.Join(v.Problems, "; "))
}

// LoadTrajectory reads and validates corpus/<id>/trajectory.json.
// trajDir is the trajectory's corpus directory.
func LoadTrajectory(trajDir string) (*Trajectory, error) {
	path := filepath.Join(trajDir, "trajectory.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read trajectory spec: %w", err)
	}
	var t Trajectory
	if err := json.Unmarshal(data, &t); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if problems := ValidateTrajectory(&t, trajDir); len(problems) > 0 {
		return nil, Validation{TrajectoryID: t.ID, Problems: problems}
	}
	return &t, nil
}

// ValidateTrajectory checks the spec rules that are decidable at load
// time. trajDir is used to confirm referenced files exist.
func ValidateTrajectory(t *Trajectory, trajDir string) []string {
	var problems []string

	if t.ID == "" {
		problems = append(problems, "id is required")
	}
	if t.SchemaVersion != 1 {
		problems = append(problems, fmt.Sprintf("schema_version must be 1, got %d", t.SchemaVersion))
	}

	switch t.Origin.Kind {
	case "regression", "production", "synthetic":
		if t.Origin.Kind == "production" && (t.Origin.Scrubbed == nil || !*t.Origin.Scrubbed) {
			problems = append(problems, "origin.scrubbed must be true for production trajectories")
		}
		// A regression carrying any source reference is a real-world
		// artifact — same scrubbing requirement as production.
		if t.Origin.Kind == "regression" && t.Origin.Source != "" && (t.Origin.Scrubbed == nil || !*t.Origin.Scrubbed) {
			problems = append(problems, "origin.scrubbed must be true for regression trajectories with a real source")
		}
	case "":
		problems = append(problems, "origin.kind is required")
	default:
		problems = append(problems, fmt.Sprintf("origin.kind %q is not regression|production|synthetic", t.Origin.Kind))
	}

	switch t.StartState.Kind {
	case "fixture":
		if t.StartState.FixtureDir == "" {
			problems = append(problems, "start_state.fixture_dir is required for fixture kind")
		} else if _, err := os.Stat(filepath.Join(trajDir, t.StartState.FixtureDir)); err != nil {
			problems = append(problems, fmt.Sprintf("fixture dir %q: %v", t.StartState.FixtureDir, err))
		}
	case "git":
		if t.StartState.Repo == "" || t.StartState.Ref == "" {
			problems = append(problems, "start_state.repo and .ref are required for git kind")
		}
	case "":
		problems = append(problems, "start_state.kind is required (fixture|git)")
	default:
		problems = append(problems, fmt.Sprintf("start_state.kind %q is not fixture|git", t.StartState.Kind))
	}

	// A git start state under network:false must resolve repo to a
	// local source — the clone itself is egress.
	// Coverage must be achievable within budget — a min_steps
	// predicate above the step cap is permanently inconclusive and
	// burns attempts to the starvation cap forever.
	for key, v := range t.Coverage {
		op, field, err := ParseCoverageKey(key)
		if err != nil {
			problems = append(problems, fmt.Sprintf("coverage %q: %v", key, err))
			continue
		}
		if op == "min" && field == "steps" && t.Budget.MaxSteps > 0 && int(v) > t.Budget.MaxSteps {
			problems = append(problems, fmt.Sprintf("coverage min_steps=%v exceeds budget.max_steps=%d — permanently inconclusive", v, t.Budget.MaxSteps))
		}
	}
	if t.Requires.Network != nil && !*t.Requires.Network && t.StartState.Kind == "git" {
		repo := t.StartState.Repo
		if strings.HasPrefix(repo, "http://") || strings.HasPrefix(repo, "https://") || strings.HasPrefix(repo, "git@") {
			problems = append(problems, "requires.network=false conflicts with remote git start_state (use a local path, file:// URL, or pre-seeded mirror)")
		}
	}

	if len(t.Task.Turns) == 0 {
		problems = append(problems, "task.turns must contain at least one prompt")
	} else {
		for i, turn := range t.Task.Turns {
			if strings.TrimSpace(turn) == "" {
				problems = append(problems, fmt.Sprintf("task.turns[%d] is empty", i))
			}
		}
	}

	if t.Check.Script == "" {
		problems = append(problems, "check.script is required")
	} else if _, err := os.Stat(filepath.Join(trajDir, t.Check.Script)); err != nil {
		problems = append(problems, fmt.Sprintf("check script %q: %v", t.Check.Script, err))
	}

	hasRef := fileExists(filepath.Join(trajDir, "reference.patch"))
	hasCounter := fileExists(filepath.Join(trajDir, "counterexample.patch"))

	switch t.Check.ExpectStartState {
	case "fail":
		// The strong form: the check must fail on the start state.
		if hasCounter {
			problems = append(problems, "counterexample.patch on a \"fail\" trajectory is a validation error")
		}
		if t.Origin.Kind == "regression" && !hasRef {
			// The fix diff is known by construction for a regression.
			problems = append(problems, "reference.patch is required for origin.kind=regression")
		}
	case "pass":
		// Required, not optional: without it a pass-guard accrues
		// p̂=1.0, joins stable, and can never fire — false coverage.
		if !hasCounter {
			problems = append(problems, "counterexample.patch is required for expect_start_state=pass")
		}
	case "":
		problems = append(problems, "check.expect_start_state is required (fail|pass)")
	default:
		problems = append(problems, fmt.Sprintf("check.expect_start_state %q is not fail|pass", t.Check.ExpectStartState))
	}

	if t.Budget.MaxSteps < 0 || t.Budget.RunTimeoutSeconds < 0 {
		problems = append(problems, "budget values must be non-negative")
	}

	return problems
}

// LoadCorpus discovers trajectories under evalDir/corpus. Validation
// failures are aggregated: one bad trajectory shouldn't hide the rest.
func LoadCorpus(evalDir string) (map[string]*Trajectory, error) {
	corpusDir := filepath.Join(evalDir, "corpus")
	entries, err := os.ReadDir(corpusDir)
	if err != nil {
		return nil, fmt.Errorf("read corpus dir: %w", err)
	}
	out := make(map[string]*Trajectory)
	var problems []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		t, err := LoadTrajectory(filepath.Join(corpusDir, e.Name()))
		if err != nil {
			problems = append(problems, err.Error())
			continue
		}
		if t.ID != e.Name() {
			problems = append(problems, fmt.Sprintf("trajectory %s: id %q does not match directory name", e.Name(), t.ID))
			continue
		}
		out[t.ID] = t
	}
	if len(problems) > 0 {
		return out, fmt.Errorf("corpus validation failed:\n  %s", strings.Join(problems, "\n  "))
	}
	return out, nil
}

// LoadExperiment reads an experiment definition.
func LoadExperiment(path string) (*Experiment, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read experiment: %w", err)
	}
	var e Experiment
	if err := json.Unmarshal(data, &e); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if err := ValidateExperiment(&e); err != nil {
		return nil, err
	}
	return &e, nil
}

// ValidateExperiment checks the experiment definition.
func ValidateExperiment(e *Experiment) error {
	if e.Name == "" {
		return fmt.Errorf("experiment name is required")
	}
	if e.Name == CharacterizeExperiment {
		return fmt.Errorf("experiment name %q is reserved for characterization runs", CharacterizeExperiment)
	}
	if e.Model == "" {
		return fmt.Errorf("experiment model pin is required (provider/model)")
	}
	if len(e.Arms) != 2 {
		return fmt.Errorf("analysis is defined for two arms, got %d", len(e.Arms))
	}
	if _, ok := e.Arms[ArmControl]; !ok {
		return fmt.Errorf("arms must include \"control\", got %v", sortedKeys(e.Arms))
	}
	if _, ok := e.Arms[ArmTreatment]; !ok {
		return fmt.Errorf("arms must include \"treatment\", got %v", sortedKeys(e.Arms))
	}
	// Temperature is a run condition hashed into baseline keys —
	// characterize/smoke always pin it, so an unpinned experiment's
	// control records land in a "default"-temp cell no baseline can
	// join, silently darkening the catastrophic tier.
	if e.Temperature == nil {
		return fmt.Errorf("temperature must be pinned — unpinned arms never join characterized baselines")
	}
	if len(e.Corpus) == 0 {
		return fmt.Errorf("corpus selector is required")
	}
	for _, sel := range e.Corpus {
		if sel == "band:"+string(BandQuarantined) {
			return fmt.Errorf("band:quarantined is never selectable — re-validating needs characterize mode, not an experiment arm")
		}
	}
	for band, n := range e.RunsPerTrajectory {
		switch band {
		case BandStable, BandMid, BandUncharacterized:
		case BandQuarantined:
			return fmt.Errorf("runs_per_trajectory cannot target quarantined band")
		default:
			return fmt.Errorf("runs_per_trajectory: unknown band %q", band)
		}
		if n <= 0 {
			return fmt.Errorf("runs_per_trajectory[%s] must be > 0", band)
		}
	}
	return nil
}

// SelectCorpus resolves the corpus selector against loaded trajectories
// and band state. Quarantined trajectories are excluded even under "*".
func SelectCorpus(corpus map[string]*Trajectory, bands *Bands, selectors []string) ([]*Trajectory, error) {
	return selectCorpus(corpus, bands, selectors, false)
}

// selectCorpus with includeQuarantined=true is used by the quarantine
// pass itself — a quarantined trajectory must be selectable for
// re-validation; it just can't be gated on.
func selectCorpus(corpus map[string]*Trajectory, bands *Bands, selectors []string, includeQuarantined bool) ([]*Trajectory, error) {
	want := make(map[string]bool)
	for _, sel := range selectors {
		if rest, ok := strings.CutPrefix(sel, "band:"); ok {
			band := Band(rest)
			for id := range corpus {
				if bands.Band(id) == band && (band != BandQuarantined || includeQuarantined) {
					want[id] = true
				}
			}
			continue
		}
		// Glob over trajectory ids.
		matched := false
		for id := range corpus {
			ok, err := filepath.Match(sel, id)
			if err != nil {
				return nil, fmt.Errorf("corpus selector %q: %w", sel, err)
			}
			if ok {
				matched = true
				want[id] = true
			}
		}
		if !matched && sel != "*" {
			return nil, fmt.Errorf("corpus selector %q matched no trajectories", sel)
		}
	}
	var out []*Trajectory
	for id, t := range corpus {
		if bands.Band(id) == BandQuarantined && !includeQuarantined {
			continue
		}
		if want[id] {
			out = append(out, t)
		}
	}
	slices.SortFunc(out, func(a, b *Trajectory) int { return strings.Compare(a.ID, b.ID) })
	return out, nil
}

// ContentHash scopes samples to the corpus revision: p̂ is conditional
// on check.sh, trajectory.json, the fixture, and the patches as much as
// on the model. Any change re-keys the baseline like a re-pin.
func ContentHash(trajDir string) (string, error) {
	h := sha256.New()
	var files []string
	err := filepath.WalkDir(trajDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("walk trajectory dir: %w", err)
	}
	slices.Sort(files)
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			return "", fmt.Errorf("hash %s: %w", f, err)
		}
		rel, _ := filepath.Rel(trajDir, f)
		h.Write([]byte(rel))
		h.Write([]byte{0})
		h.Write(data)
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))[:16], nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}
