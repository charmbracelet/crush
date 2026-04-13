package hooks

import (
	"bytes"
	"context"
	"log/slog"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/charmbracelet/crush/internal/config"
)

// Runner executes hook commands and aggregates their results.
type Runner struct {
	hooks      []config.HookConfig
	cwd        string
	projectDir string
}

// NewRunner creates a Runner from the given hook configs.
func NewRunner(hooks []config.HookConfig, cwd, projectDir string) *Runner {
	return &Runner{
		hooks:      hooks,
		cwd:        cwd,
		projectDir: projectDir,
	}
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

	agg := aggregate(results)
	agg.Hooks = make([]HookInfo, len(deduped))
	for i, h := range deduped {
		agg.Hooks[i] = HookInfo{
			Name:         filepath.Base(h.Command),
			Matcher:      h.Matcher,
			Decision:     results[i].Decision.String(),
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
		re := h.MatcherRegex()
		if re == nil || re.MatchString(toolName) {
			matched = append(matched, h)
		}
	}
	return matched
}

// runOne executes a single hook command and returns its result.
func (r *Runner) runOne(parentCtx context.Context, hook config.HookConfig, envVars []string, payload []byte) HookResult {
	timeout := hook.TimeoutDuration()
	ctx, cancel := context.WithTimeout(parentCtx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", hook.Command)
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
			// Exit code 2 = block. Stderr is the reason.
			reason := strings.TrimSpace(stderr.String())
			if reason == "" {
				reason = "blocked by hook"
			}
			return HookResult{
				Decision: DecisionDeny,
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
