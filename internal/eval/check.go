package eval

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// CheckResult is what check.sh produced.
type CheckResult struct {
	// Exit is the script's exit code; -1 when the script could not be
	// executed at all (missing, not executable, timed out).
	Exit   int
	Detail map[string]any // parsed EVAL_JSON line, if any
	Stdout string
	Stderr string
	// Err is set when the harness broke rather than the check judging:
	// cannot execute, or timed out. Distinct from a non-zero exit —
	// infra errors never count against the model.
	Err error
}

// RunCheck executes the trajectory's check.sh with cwd = workdir, on
// the working tree as the agent left it. The runner exports
// EVAL_WORKDIR and EVAL_TRAJECTORY_DIR for checks needing oracles.
func RunCheck(ctx context.Context, traj *Trajectory, trajDir, workdir string, env []string) CheckResult {
	// The check chdirs into the workdir; a relative trajDir would
	// resolve against it, so absolutize first.
	if abs, err := filepath.Abs(trajDir); err == nil {
		trajDir = abs
	}
	timeout := time.Duration(traj.Check.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	script := filepath.Join(trajDir, traj.Check.Script)
	cmd := exec.CommandContext(ctx, "bash", script)
	// Kill the whole process group — a timed-out check's children
	// (go run, spawned servers, bound ports) must not outlive the
	// workdir and poison later runs.
	checkProcAttr(cmd)
	cmd.Cancel = func() error { return killCheckGroup(cmd) }
	cmd.Dir = workdir
	if env == nil {
		env = os.Environ()
	}
	// Strip inherited EVAL_* — an exported EVAL_WORKDIR in the
	// operator's env would otherwise shadow ours (first-match wins).
	cmdEnv := make([]string, 0, len(env)+2)
	for _, kv := range env {
		if strings.HasPrefix(kv, "EVAL_") {
			continue
		}
		cmdEnv = append(cmdEnv, kv)
	}
	cmdEnv = append(cmdEnv,
		"EVAL_WORKDIR="+workdir,
		"EVAL_TRAJECTORY_DIR="+trajDir,
	)
	cmd.Env = cmdEnv
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	res := CheckResult{Exit: -1}
	err := cmd.Run()
	res.Stdout = stdout.String()
	res.Stderr = stderr.String()
	res.Detail = parseEvalJSON(res.Stdout)

	if ctx.Err() != nil {
		// Any context-done is a harness error — timeout or parent
		// cancel. A killed check's ExitError is not a model outcome;
		// recording it as `fail` would pollute baselines.
		if ctx.Err() == context.DeadlineExceeded {
			res.Err = fmt.Errorf("check timed out after %s", timeout)
		} else {
			res.Err = fmt.Errorf("check interrupted: %w", ctx.Err())
		}
		return res
	}
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			res.Exit = exitErr.ExitCode()
			return res
		}
		res.Err = fmt.Errorf("check could not execute: %w", err)
		return res
	}
	res.Exit = 0
	return res
}

// parseEvalJSON returns the last EVAL_JSON {...} line's object. A
// malformed line never changes the verdict; it just leaves Detail nil
// — a malformed *final* match does not fall back to an earlier
// parseable one. Trailing output may follow the matching line.
func parseEvalJSON(stdout string) map[string]any {
	var last string
	for line := range strings.Lines(stdout) {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "EVAL_JSON "); ok {
			last = rest
		}
	}
	if last == "" {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(last), &m); err != nil {
		return nil
	}
	return m
}
