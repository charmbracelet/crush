package eval

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// Materialize builds a fresh workdir for a run inside parentDir and
// returns its path. Every run gets a new tempdir — never the corpus or
// the real repo.
// env supplies the setup commands' environment — pass the runner's
// pinned env so e.g. `go mod download` populates the pinned module
// cache, not the operator's real HOME.
func Materialize(ctx context.Context, traj *Trajectory, trajDir, parentDir string, env []string) (string, error) {
	workdir, err := os.MkdirTemp(parentDir, "eval-run-*")
	if err != nil {
		return "", fmt.Errorf("create workdir: %w", err)
	}

	switch traj.StartState.Kind {
	case "fixture":
		src := filepath.Join(trajDir, traj.StartState.FixtureDir)
		if err := copyTree(src, workdir); err != nil {
			return "", fmt.Errorf("copy fixture: %w", err)
		}
	case "git":
		if err := gitCheckout(ctx, traj.StartState.Repo, traj.StartState.Ref, workdir); err != nil {
			return "", err
		}
	default:
		return "", fmt.Errorf("unknown start_state kind %q", traj.StartState.Kind)
	}

	for _, cmdLine := range traj.StartState.Setup {
		if err := runShell(ctx, workdir, cmdLine, env); err != nil {
			return "", fmt.Errorf("setup %q: %w", cmdLine, err)
		}
	}
	return workdir, nil
}

// gitCheckout clones repo and checks out ref. A local source may use
// --shared (objects shared via alternates); --shared is meaningless —
// and wrong to emit — for remote URLs.
// gitCheckout deliberately runs under the ambient environment, not
// the pinned eval env: git credentials (ssh keys, credential helpers,
// .gitconfig) live in the real HOME, and cloning is materialization —
// not part of the measured run.
func gitCheckout(ctx context.Context, repo, ref, dest string) error {
	args := []string{"clone", "--quiet"}
	if isLocalRepo(repo) {
		args = append(args, "--shared")
	}
	args = append(args, repo, dest)
	if out, err := exec.CommandContext(ctx, "git", args...).CombinedOutput(); err != nil {
		return fmt.Errorf("git clone %s: %w: %s", repo, err, out)
	}
	if out, err := exec.CommandContext(ctx, "git", "-C", dest, "checkout", "--quiet", ref).CombinedOutput(); err != nil {
		return fmt.Errorf("git checkout %s: %w: %s", ref, err, out)
	}
	return nil
}

func isLocalRepo(repo string) bool {
	return strings.HasPrefix(repo, "/") || strings.HasPrefix(repo, ".") ||
		strings.HasPrefix(repo, "file://")
}

// ApplyPatch applies a patch file to a workdir (git apply; works in
// non-git dirs with --unsafe-paths off since our patches are relative).
func ApplyPatch(ctx context.Context, workdir, patchPath string) error {
	data, err := os.ReadFile(patchPath)
	if err != nil {
		return fmt.Errorf("read patch: %w", err)
	}
	cmd := exec.CommandContext(ctx, "git", "apply", "--whitespace=nowarn")
	cmd.Dir = workdir
	cmd.Stdin = strings.NewReader(string(data))
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git apply: %w: %s", err, out)
	}
	return nil
}

// WriteArmConfig drops the arm's generated config into the workdir.
//
// The model pin goes to .crushrc via the `model` builtin (top of the
// directory precedence order, so it overrides anything a fixture or
// cloned repo carries). Options go to .crush.json: the `option`
// builtin's key set is closed and flags without a builtin path
// (notebook_*) can only be expressed in JSON. Collision is per-key:
// a start-state shell config (.crushrc/crushrc) would shadow the JSON
// arm's same-named keys — an error. A start-state .crush.json merges
// with the arm options winning; crush.json is lower precedence than
// .crush.json outright, so it never shadows.
func WriteArmConfig(workdir string, exp *Experiment, arm Arm, manifest *FlagsManifest) error {
	for _, name := range []string{".crushrc", "crushrc"} {
		if fileExists(filepath.Join(workdir, name)) {
			return fmt.Errorf("workdir already carries %s — a shell config shadows the .crush.json arm; fixtures must not ship crush shell config", name)
		}
	}

	// .crushrc: model pin identical for every arm — options-only
	// constrains what may *differ* between arms, not what may be set.
	var rc strings.Builder
	if exp.Model != "" {
		fmt.Fprintf(&rc, "model large %s", exp.Model)
		if exp.Temperature != nil {
			fmt.Fprintf(&rc, " --temperature %v", *exp.Temperature)
		}
		rc.WriteString("\n")
	}
	if err := os.WriteFile(filepath.Join(workdir, ".crushrc"), []byte(rc.String()), 0o644); err != nil {
		return fmt.Errorf("write .crushrc: %w", err)
	}

	// .crush.json: the options delta plus harness invariants (metrics
	// and provider auto-update off — identical for both arms).
	options := map[string]any{
		"disable_metrics":              true,
		"disable_provider_auto_update": true,
		// Harness state (crush.db, logs) beside the workdir, not in
		// it: check.sh sees the tree exactly as the agent left it.
		"data_directory": DataDirFor(workdir),
	}
	for k, v := range arm.Config.Options {
		// Harness invariants an arm must not override — data dir
		// relocation breaks telemetry/session-DB paths and litters
		// the tree checks observe; metrics/auto-update re-enable
		// nondeterministic side effects in measured runs.
		switch k {
		case "data_directory", "disable_metrics", "disable_provider_auto_update":
			return fmt.Errorf("arm sets %q which the harness manages — remove it from the experiment", k)
		}
		options[k] = v
	}
	doc := map[string]any{"options": options}
	// A start-state .crush.json merges under the arm: the fixture's
	// keys survive, the arm's win — same per-key rule the loader
	// applies across config layers.
	jsonPath := filepath.Join(workdir, ".crush.json")
	if fileExists(jsonPath) {
		raw, err := os.ReadFile(jsonPath)
		if err != nil {
			return fmt.Errorf("read existing .crush.json: %w", err)
		}
		var existing map[string]any
		if err := json.Unmarshal(raw, &existing); err != nil {
			// A corrupt fixture config must not be silently clobbered.
			return fmt.Errorf("existing .crush.json does not parse: %w", err)
		}
		if opts, ok := existing["options"].(map[string]any); ok {
			// A fixture pinning a manifest flag keys this trajectory's
			// runs under a foreign condition — every characterization
			// lands in a cell the gate never reads.
			for k := range opts {
				if _, declared := manifest.Defaults[k]; declared {
					return fmt.Errorf("existing .crush.json sets manifest flag %q — the fixture would pin a flag under test; remove it or drop the flag from flags.json", k)
				}
			}
			for k, v := range options {
				opts[k] = v
			}
			existing["options"] = opts
		} else {
			existing["options"] = options
		}
		doc = existing
	}
	// crush.json merges at lower precedence — still an error when it
	// pins a manifest flag (same foreign-condition trap).
	lowPath := filepath.Join(workdir, "crush.json")
	if fileExists(lowPath) {
		raw, err := os.ReadFile(lowPath)
		if err != nil {
			return fmt.Errorf("read existing crush.json: %w", err)
		}
		var existing map[string]any
		if err := json.Unmarshal(raw, &existing); err != nil {
			return fmt.Errorf("existing crush.json does not parse: %w", err)
		}
		if opts, ok := existing["options"].(map[string]any); ok {
			for k := range opts {
				if _, declared := manifest.Defaults[k]; declared {
					return fmt.Errorf("existing crush.json sets manifest flag %q — the fixture would pin a flag under test; remove it or drop the flag from flags.json", k)
				}
			}
		}
	}
	data, err := json.MarshalIndent(doc, "", "\t")
	if err != nil {
		return fmt.Errorf("marshal arm config: %w", err)
	}
	if err := os.WriteFile(jsonPath, data, 0o644); err != nil {
		return fmt.Errorf("write .crush.json: %w", err)
	}
	return nil
}

// CheckRequires reports which declared environment preconditions this
// machine can't honor — the pre-flight half of `requires`. A missing
// `go` or `jq` must surface as a skip-report, not as check `error`
// outcomes masquerading as flakiness. `network` has no cheap probe;
// its constraint is enforced at load (local-path git rules), not here.
func CheckRequires(t *Trajectory) []string {
	var missing []string
	for _, tool := range append(t.Requires.Tools, t.Requires.LSP...) {
		if _, err := exec.LookPath(tool); err != nil {
			missing = append(missing, "tool:"+tool)
		}
	}
	if len(t.Requires.OS) > 0 {
		ok := false
		for _, osname := range t.Requires.OS {
			if osname == runtime.GOOS {
				ok = true
				break
			}
		}
		if !ok {
			missing = append(missing, "os:"+runtime.GOOS)
		}
	}
	return missing
}

// BaselineConfigHash keys a baseline by the run's effective config over
// the declared flag projection — the context-management flag set
// experiments may touch. Unrelated option churn must not rotate
// baselines, and extending the projection re-keys every baseline (a
// third darkness trigger alongside re-pins and flips).
func BaselineConfigHash(projection []string, effective map[string]any) string {
	keys := make([]string, 0, len(projection))
	for _, k := range projection {
		if _, ok := effective[k]; ok {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		v, _ := json.Marshal(effective[k])
		fmt.Fprintf(&b, "%s=%s;", k, v)
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:8])
}

// runShell executes a setup command in the workdir via bash under
// the pinned eval environment (nil env inherits os.Environ).
func runShell(ctx context.Context, dir, cmdLine string, env []string) error {
	cmd := exec.CommandContext(ctx, "bash", "-c", cmdLine)
	cmd.Dir = dir
	cmd.Env = env
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%w: %s", err, out)
	}
	return nil
}

// copyTree recursively copies src into dst.
func copyTree(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode())
	})
}

// DataDirFor locates the run's .crush state beside the workdir so the
// checked tree carries no harness litter.
func DataDirFor(workdir string) string {
	return filepath.Join(filepath.Dir(workdir), filepath.Base(workdir)+".crush-data")
}
