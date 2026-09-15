package eval

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// EvalTelemetryEnvVar names the file the agent subprocess writes its
// per-run telemetry to when set. It is the eval extraction path for
// numbers that live in-process: steps, usage, stub stats, recalls.
const EvalTelemetryEnvVar = "CRUSH_EVAL_TELEMETRY"

// EvalMaxStepsEnvVar caps a single `crush run`'s steps — the run-side
// enforcement of the trajectory-wide max_steps budget. The driver
// passes the remaining budget (plus one) each turn.
const EvalMaxStepsEnvVar = "CRUSH_EVAL_MAX_STEPS"

// EvalFlagsEnvVar carries the manifest flag names the subprocess
// should report resolved values for.
const EvalFlagsEnvVar = "CRUSH_EVAL_FLAGS"

// RunResult is what one trajectory run (all turns) produced.
type RunResult struct {
	Steps         int
	Tokens        TokenUsage
	StubStats     StubStats
	Recalls       Recalls
	SessionID     string
	ModelResolved string
	ModelSmall    string
	ModelSummary  string
	// ResolvedOptions is the child's report of what each manifest
	// flag resolved to — the truth the baseline key hashes.
	ResolvedOptions map[string]any
	// TimedOut is set when the run hit the trajectory's
	// run_timeout_seconds or max_steps budget.
	TimedOut bool
	// Err is set when the run failed in transport/agent machinery —
	// the model didn't produce the outcome.
	Err error
}

// AgentRunner executes the agent against a prepared workdir. The
// interface exists so tests drive the harness without a provider.
type AgentRunner interface {
	Run(ctx context.Context, workdir string, turns []string, budget Budget) RunResult
}

// CrushRunner drives runs through the crush binary under test —
// os.Executable(), so both arms provably run the same build. Each turn
// is a `crush run` subprocess in the materialized workdir with a pinned
// HOME/XDG: the global config layers merge into every run and would
// otherwise leak a dev laptop's options into results. Credentials come
// from the eval environment — they pass through.
type CrushRunner struct {
	Bin string // os.Executable() when empty is resolved at Run time
	// Home is the pinned HOME for subprocesses; XDG dirs are derived
	// from it.
	Home string
	// ExtraEnv entries override os.Environ for the subprocess — they
	// precede inherited vars so duplicates resolve to these values.
	// Harness-pinned keys (HOME, telemetry, flags) still win.
	ExtraEnv []string
	// FlagKeys are the manifest flag names the child reports resolved
	// values for, via CRUSH_EVAL_FLAGS.
	FlagKeys []string
}

// runTelemetry is the JSON the agent subprocess drops at
// CRUSH_EVAL_TELEMETRY.
type runTelemetry struct {
	SessionID string `json:"session_id"`
	Steps     int    `json:"steps"`
	Tokens    struct {
		Input      int64 `json:"input"`
		Output     int64 `json:"output"`
		CacheRead  int64 `json:"cache_read"`
		CacheWrite int64 `json:"cache_write"`
	} `json:"tokens"`
	StubStats struct {
		Invalidations    int            `json:"invalidations"`
		Results          int            `json:"results"`
		SavedBytes       int64          `json:"saved_bytes"`
		BoundaryAdvances int            `json:"boundary_advances"`
		Kinds            map[string]int `json:"kinds"`
	} `json:"stub_stats"`
	Recalls struct {
		Result int `json:"result"`
		Entry  int `json:"entry"`
		Empty  int `json:"empty"`
		Cross  int `json:"cross"`
	} `json:"recalls"`
	Model        string `json:"model"`
	ModelSmall   string `json:"model_small"`
	ModelSummary string `json:"model_summary"`
	// ResolvedOptions is the child's effective config projected onto
	// the manifest flags — what actually ran, not what the arm asked.
	ResolvedOptions map[string]any `json:"resolved_options"`
	Error           string         `json:"error,omitempty"`
}

// Run executes the trajectory's turns sequentially — each turn a
// `crush run` subprocess continuing the same session. Turn i+1 starts
// only after turn i's process exits, which is after its terminal
// RunComplete; gate retries share the run's correlation and suppress
// the premature completion event, so the subprocess boundary satisfies
// the "follow-ups settle" sequencing rule.
func (c CrushRunner) Run(ctx context.Context, workdir string, turns []string, budget Budget) RunResult {
	var res RunResult
	deadline := time.Now().Add(time.Duration(budget.RunTimeoutSeconds) * time.Second)
	if budget.RunTimeoutSeconds <= 0 {
		deadline = time.Now().Add(15 * time.Minute)
	}
	ctx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()

	var sessionID string
	for i, turn := range turns {
		// A multi-turn trajectory must continue the SAME session —
		// launching turn i+1 without --session silently degrades to a
		// fresh session and the check may still pass.
		if i > 0 && sessionID == "" {
			res.Err = fmt.Errorf("turn %d: session continuation lost — turn %d's telemetry had no session_id", i+1, i)
			return res
		}
		if budget.MaxSteps > 0 && res.Steps >= budget.MaxSteps {
			// Trajectory-wide budget already consumed — don't launch
			// the next turn at all.
			res.TimedOut = true
			return res
		}
		// Telemetry lives beside the workdir, not inside it: check.sh
		// must see the tree exactly as the agent left it — untracked
		// harness litter could flip a globbing check.
		tfile := filepath.Join(filepath.Dir(workdir), fmt.Sprintf(".eval-telemetry-%s-%d.json", filepath.Base(workdir), i))
		args := []string{"run", "--quiet"}
		if sessionID != "" {
			args = append(args, "--session", sessionID)
		}
		args = append(args, "--", turn)

		bin := c.Bin
		if bin == "" {
			bin, _ = os.Executable()
		}
		cmd := exec.CommandContext(ctx, bin, args...)
		// SIGINT (not Kill) on deadline/cancel: `crush run` translates
		// it into ctx cancellation, giving the child a beat to write
		// telemetry for the timeout/error carve-out. WaitDelay bounds
		// the grace before the hard kill.
		cmd.Cancel = func() error {
			if err := cmd.Process.Signal(os.Interrupt); err != nil {
				return cmd.Process.Kill()
			}
			return nil
		}
		cmd.WaitDelay = 10 * time.Second
		cmd.Dir = workdir
		cmd.Env = c.subprocessEnv(tfile, remainingSteps(budget, res.Steps))
		out, err := cmd.CombinedOutput()

		tel, telErr := readTelemetry(tfile)
		_ = os.Remove(tfile)
		res.addTurnTelemetry(tel)
		if tel.SessionID != "" {
			sessionID = tel.SessionID
			res.SessionID = sessionID
		}
		if tel.Model != "" {
			res.ModelResolved = tel.Model
		}
		if tel.ModelSmall != "" {
			res.ModelSmall = tel.ModelSmall
		}
		if tel.ModelSummary != "" {
			res.ModelSummary = tel.ModelSummary
		}
		if len(tel.ResolvedOptions) > 0 {
			res.ResolvedOptions = tel.ResolvedOptions
		}

		// Timeout/error carve-out: a run that hit the deadline while
		// its API calls were already erroring classifies as `error`,
		// not `timeout` — the child's graceful-cancel telemetry says
		// which. A clean cancellation (our own signal) is a timeout.
		if tel.Error != "" && !isCancellation(tel.Error) {
			res.Err = fmt.Errorf("agent run failed: %s", tel.Error)
			return res
		}
		if ctx.Err() == context.DeadlineExceeded || (budget.MaxSteps > 0 && res.Steps > budget.MaxSteps) {
			res.TimedOut = true
			return res
		}
		if ctx.Err() != nil {
			res.Err = ctx.Err()
			return res
		}
		if err != nil {
			res.Err = fmt.Errorf("crush run failed: %w: %s", err, tail(out, 4096))
			return res
		}
		if telErr != nil {
			// The child exited clean but the telemetry contract broke
			// — zeroed stats would read as coverage-starved
			// inconclusive instead of the error this is.
			res.Err = fmt.Errorf("telemetry unreadable after clean run: %w", telErr)
			return res
		}
		if tel.Error != "" {
			res.Err = fmt.Errorf("agent run failed: %s", tel.Error)
			return res
		}
	}
	return res
}

// addTurnTelemetry folds one turn's telemetry counters into the run
// totals. Per-turn counters reset with each fresh `crush run` process
// (the stats maps are in-memory per session, not rehydrated on
// --session resume), so the trajectory totals are the SUM of per-turn
// deltas, not the last turn's value — per-kind counts included.
func (res *RunResult) addTurnTelemetry(tel runTelemetry) {
	res.Steps += tel.Steps
	res.Tokens.Input += tel.Tokens.Input
	res.Tokens.Output += tel.Tokens.Output
	res.Tokens.CacheRead += tel.Tokens.CacheRead
	res.Tokens.CacheWrite += tel.Tokens.CacheWrite
	res.StubStats.Invalidations += tel.StubStats.Invalidations
	res.StubStats.Results += tel.StubStats.Results
	res.StubStats.SavedBytes += tel.StubStats.SavedBytes
	res.StubStats.BoundaryAdvances += tel.StubStats.BoundaryAdvances
	if len(tel.StubStats.Kinds) > 0 {
		if res.StubStats.Kinds == nil {
			res.StubStats.Kinds = make(map[string]int, len(tel.StubStats.Kinds))
		}
		for kind, n := range tel.StubStats.Kinds {
			res.StubStats.Kinds[kind] += n
		}
	}
	res.Recalls.Result += tel.Recalls.Result
	res.Recalls.Entry += tel.Recalls.Entry
	res.Recalls.Empty += tel.Recalls.Empty
	res.Recalls.Cross += tel.Recalls.Cross
}

// remainingSteps converts the trajectory-wide max_steps budget into
// the child process's per-run cap: the run may consume the remaining
// budget plus one step — hitting the cap means it was stopped
// mid-flight (timeout), while finishing within it is a normal run.
func remainingSteps(budget Budget, used int) int {
	if budget.MaxSteps <= 0 {
		return 0
	}
	return budget.MaxSteps - used + 1
}

// isCancellation identifies the telemetry error the child writes when
// it was stopped by our SIGINT rather than by a failure of its own.
func isCancellation(e string) bool {
	return strings.Contains(e, "context canceled") ||
		strings.Contains(e, "context deadline") ||
		strings.Contains(e, "request canceled by user")
}

// subprocessEnv builds the run's environment: the eval environment's
// credentials pass through; HOME and the XDG dirs are pinned so the
// global config layers merge nothing in. The parent's own CRUSH_* vars
// are stripped — a CRUSH_CLIENT_SERVER=1 left over in the operator's
// env would take the client/server path where the telemetry hook
// doesn't fire, silently breaking session continuation.
func (c CrushRunner) subprocessEnv(telemetryFile string, maxSteps int) []string {
	pinned := map[string]string{
		"HOME":              c.Home,
		"XDG_CONFIG_HOME":   filepath.Join(c.Home, ".config"),
		"XDG_DATA_HOME":     filepath.Join(c.Home, ".local", "share"),
		"XDG_STATE_HOME":    filepath.Join(c.Home, ".local", "state"),
		EvalTelemetryEnvVar: telemetryFile,
	}
	if maxSteps > 0 {
		pinned[EvalMaxStepsEnvVar] = strconv.Itoa(maxSteps)
	}
	if len(c.FlagKeys) > 0 {
		pinned[EvalFlagsEnvVar] = strings.Join(c.FlagKeys, ",")
	}
	// Replace rather than append: duplicated keys in environ are
	// resolved first-match by getenv, so a second HOME wouldn't pin.
	env := make([]string, 0, len(os.Environ()))
	for _, kv := range os.Environ() {
		k, _, _ := strings.Cut(kv, "=")
		// Harness-prefixed vars never inherit — an exported
		// EVAL_WORKDIR or CRUSH_MODE would shadow/derail the run.
		if strings.HasPrefix(k, "CRUSH_") || strings.HasPrefix(k, "EVAL_") {
			continue
		}
		if _, overridden := pinned[k]; !overridden {
			env = append(env, kv)
		}
	}
	// exec.Cmd.Env dedupes LAST-wins: the filtered parent env first,
	// then ExtraEnv (caller intent overrides inherited vars), then
	// pinned (harness invariants override everything).
	out := make([]string, 0, len(env)+len(pinned)+len(c.ExtraEnv))
	out = append(out, env...)
	out = append(out, c.ExtraEnv...)
	for k, v := range pinned {
		out = append(out, k+"="+v)
	}
	return out
}

func readTelemetry(path string) (runTelemetry, error) {
	var t runTelemetry
	data, err := os.ReadFile(path)
	if err != nil {
		return t, err
	}
	return t, json.Unmarshal(data, &t)
}

// tail keeps the last n bytes of output — the diagnostically useful
// end without the bulk.
func tail(b []byte, n int) []byte {
	if len(b) <= n {
		return b
	}
	return b[len(b)-n:]
}
