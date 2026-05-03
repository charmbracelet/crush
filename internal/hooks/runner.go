package hooks

import (
	"bytes"
	"context"
	"log/slog"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/crush/internal/config"
)

// compiledHook pairs a HookConfig with its compiled matcher regex. A nil
// matcher means "match every tool".
type compiledHook struct {
	cfg     config.HookConfig
	matcher *regexp.Regexp
}

// Runner executes hook commands and aggregates their results.
type Runner struct {
	hooks      []compiledHook
	cwd        string
	projectDir string
}

// NewRunner creates a Runner from the given hook configs. Each hook's
// Matcher is compiled here so the Runner is self-sufficient; callers do
// not have to pre-compile matchers on the config, and reloads or merges
// that rebuild HookConfig values can't silently strip compiled state.
//
// Hooks whose matcher fails to compile are skipped with a warning rather
// than treated as match-everything. ValidateHooks is expected to have
// caught syntax errors earlier, so this is defense in depth.
func NewRunner(hooks []config.HookConfig, cwd, projectDir string) *Runner {
	compiled := make([]compiledHook, 0, len(hooks))
	for _, h := range hooks {
		ch := compiledHook{cfg: h}
		if h.Matcher != "" {
			re, err := regexp.Compile(h.Matcher)
			if err != nil {
				slog.Warn("Hook matcher failed to compile; skipping hook",
					"matcher", h.Matcher,
					"command", h.Command,
					"error", err,
				)
				continue
			}
			ch.matcher = re
		}
		compiled = append(compiled, ch)
	}
	return &Runner{
		hooks:      compiled,
		cwd:        cwd,
		projectDir: projectDir,
	}
}

// Hooks returns the hook configs the runner was created with, in config
// order. Hooks whose matcher failed to compile at construction are
// omitted. Intended for diagnostics; callers should not rely on ordering
// or identity beyond that.
func (r *Runner) Hooks() []config.HookConfig {
	out := make([]config.HookConfig, len(r.hooks))
	for i, h := range r.hooks {
		out[i] = h.cfg
	}
	return out
}

// Run executes all matching hooks for the given event and tool, returning
// an aggregated result.
func (r *Runner) Run(ctx context.Context, eventName, sessionID, toolName, toolInputJSON string) (AggregateResult, error) {
	matching := r.matchingHooks(toolName)
	if len(matching) == 0 {
		return AggregateResult{Decision: DecisionNone}, nil
	}

	// Deduplicate by command string.
	seen := make(map[string]bool, len(matching))
	var deduped []config.HookConfig
	for _, h := range matching {
		if seen[h.Command] {
			continue
		}
		seen[h.Command] = true
		deduped = append(deduped, h)
	}

	envVars := BuildEnv(eventName, toolName, sessionID, r.cwd, r.projectDir, toolInputJSON)
	payload := BuildPayload(eventName, sessionID, r.cwd, toolName, toolInputJSON)

	results := make([]HookResult, len(deduped))
	var wg sync.WaitGroup
	wg.Add(len(deduped))

	for i, h := range deduped {
		go func(idx int, hook config.HookConfig) {
			defer wg.Done()
			results[idx] = r.runOne(ctx, hook, envVars, payload)
		}(i, h)
	}
	wg.Wait()

	agg := aggregate(results, toolInputJSON)
	agg.Hooks = make([]HookInfo, len(deduped))
	for i, h := range deduped {
		agg.Hooks[i] = HookInfo{
			Name:         h.Command,
			Matcher:      h.Matcher,
			Decision:     results[i].Decision.String(),
			Halt:         results[i].Halt,
			Reason:       results[i].Reason,
			InputRewrite: results[i].UpdatedInput != "",
		}
	}
	slog.Info("Hook completed",
		"event", eventName,
		"tool", toolName,
		"hooks", len(deduped),
		"decision", agg.Decision.String(),
	)
	return agg, nil
}

// matchingHooks returns hooks whose matcher matches the tool name (or has
// no matcher, which matches everything).
func (r *Runner) matchingHooks(toolName string) []config.HookConfig {
	var matched []config.HookConfig
	for _, h := range r.hooks {
		if h.matcher == nil || h.matcher.MatchString(toolName) {
			matched = append(matched, h.cfg)
		}
	}
	return matched
}

// runOne executes a single hook command and returns its result.
func (r *Runner) runOne(parentCtx context.Context, hook config.HookConfig, envVars []string, payload []byte) HookResult {
	timeout := hook.TimeoutDuration()
	ctx, cancel := context.WithTimeout(parentCtx, timeout)
	defer cancel()

	shell, shellArg := pickShell(hook.Command)
	cmd := exec.CommandContext(ctx, shell, shellArg, hook.Command)
	cmd.WaitDelay = time.Second
	cmd.Env = envVars
	cmd.Dir = r.cwd
	cmd.Stdin = bytes.NewReader(payload)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	if ctx.Err() != nil {
		// Distinguish timeout from parent cancellation.
		if parentCtx.Err() != nil {
			slog.Debug("Hook cancelled by parent context", "command", hook.Command)
		} else {
			slog.Warn("Hook timed out", "command", hook.Command, "timeout", timeout)
		}
		return HookResult{Decision: DecisionNone}
	}

	if err != nil {
		exitCode := -1
		if cmd.ProcessState != nil {
			exitCode = cmd.ProcessState.ExitCode()
		}
		switch exitCode {
		case 2:
			// Exit code 2 = block this tool call. Stderr is the reason.
			reason := strings.TrimSpace(stderr.String())
			if reason == "" {
				reason = "blocked by hook"
			}
			return HookResult{
				Decision: DecisionDeny,
				Reason:   reason,
			}
		case HaltExitCode:
			// Exit code 49 = halt the whole turn. Stderr is the reason.
			reason := strings.TrimSpace(stderr.String())
			if reason == "" {
				reason = "turn halted by hook"
			}
			return HookResult{
				Decision: DecisionDeny,
				Halt:     true,
				Reason:   reason,
			}
		default:
			// Other non-zero exits are non-blocking errors.
			slog.Warn("Hook failed with non-blocking error",
				"command", hook.Command,
				"exit_code", exitCode,
				"stderr", strings.TrimSpace(stderr.String()),
			)
			return HookResult{Decision: DecisionNone}
		}
	}

	// Exit code 0 — parse stdout JSON.
	result := parseStdout(stdout.String())
	slog.Debug("Hook executed",
		"command", hook.Command,
		"decision", result.Decision.String(),
	)
	return result
}

// pickShell returns the shell and flag to use for the given hook command.
// On Windows, .bat/.cmd files run under cmd /c; .sh files run under sh -c;
// everything else prefers sh if available and falls back to cmd /c.
func pickShell(command string) (string, string) {
	if runtime.GOOS != "windows" {
		return "sh", "-c"
	}
	command = strings.TrimSpace(command)
	first := command
	if strings.HasPrefix(command, `"`) {
		if end := strings.Index(command[1:], `"`); end >= 0 {
			first = command[1 : end+1]
		}
	} else if i := strings.IndexAny(command, " \t"); i > 0 {
		first = command[:i]
	}
	switch strings.ToLower(filepath.Ext(first)) {
	case ".bat", ".cmd":
		return "cmd", "/c"
	case ".sh":
		return "sh", "-c"
	default:
		if _, err := exec.LookPath("sh"); err != nil {
			return "cmd", "/c"
		}
		return "sh", "-c"
	}
}
