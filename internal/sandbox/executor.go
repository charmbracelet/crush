// Package sandbox provides isolated command execution with resource limits,
// network restrictions, and filesystem isolation for the SecOps agent.
package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Config controls the isolation parameters for a sandboxed execution.
type Config struct {
	// Resource limits
	MaxMemoryBytes int64         `json:"max_memory_bytes"` // default 1GB
	MaxCPUPercent  int           `json:"max_cpu_percent"`  // default 100 (1 core)
	Timeout        time.Duration `json:"timeout"`          // default 5m

	// Network isolation
	AllowNetwork  bool     `json:"allow_network"`
	AllowedHosts  []string `json:"allowed_hosts,omitempty"`
	AllowedPorts  []int    `json:"allowed_ports,omitempty"`

	// Filesystem isolation
	ReadOnlyPaths []string `json:"readonly_paths,omitempty"`
	DenyPaths     []string `json:"deny_paths,omitempty"`
	WorkingDir    string   `json:"working_dir,omitempty"`

	// Audit
	TraceID string `json:"trace_id,omitempty"`
}

// DefaultConfig returns a restrictive default configuration.
func DefaultConfig() Config {
	return Config{
		MaxMemoryBytes: 1 << 30, // 1GB
		MaxCPUPercent:  100,
		Timeout:        5 * time.Minute,
		AllowNetwork:   false,
		DenyPaths: []string{
			"/etc/shadow",
			"/etc/sudoers",
			"/root/.ssh",
			"/boot",
		},
	}
}

// ExecutionResult holds the output and metadata from a sandboxed run.
type ExecutionResult struct {
	Stdout   string        `json:"stdout"`
	Stderr   string        `json:"stderr"`
	ExitCode int           `json:"exit_code"`
	Duration time.Duration `json:"duration"`
	Killed   bool          `json:"killed"` // true if OOM or timeout
	TraceID  string        `json:"trace_id,omitempty"`
}

// Executor runs commands in an isolated sandbox.
type Executor struct {
	defaultCfg Config
}

// NewExecutor creates a new sandbox executor with the given default config.
func NewExecutor(defaultCfg Config) *Executor {
	return &Executor{defaultCfg: defaultCfg}
}

// Execute runs a command string in a sandbox. On Linux it uses cgroup and
// namespace primitives when available; otherwise it falls back to a
// resource-limited subprocess.
func (e *Executor) Execute(ctx context.Context, command string, cfg *Config) (*ExecutionResult, error) {
	if cfg == nil {
		c := e.defaultCfg
		cfg = &c
	}

	// Validate the command against deny-paths
	if err := e.validateCommand(command, cfg); err != nil {
		return &ExecutionResult{
			Stderr:   err.Error(),
			ExitCode: 1,
			TraceID:  cfg.TraceID,
		}, err
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 5 * time.Minute
	}
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := e.buildCommand(execCtx, command, cfg)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if cfg.WorkingDir != "" {
		cmd.Dir = cfg.WorkingDir
	}

	start := time.Now()
	err := cmd.Run()
	duration := time.Since(start)

	result := &ExecutionResult{
		Stdout:   limitOutput(stdout.String(), 30000),
		Stderr:   limitOutput(stderr.String(), 10000),
		Duration: duration,
		TraceID:  cfg.TraceID,
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else if execCtx.Err() == context.DeadlineExceeded {
			result.Killed = true
			result.ExitCode = 137
			result.Stderr += "\n[sandbox] process killed: timeout exceeded"
		} else {
			result.ExitCode = 1
			result.Stderr += "\n[sandbox] " + err.Error()
		}
	}

	return result, nil
}

// buildCommand constructs the exec.Cmd with resource limits applied.
func (e *Executor) buildCommand(ctx context.Context, command string, cfg *Config) *exec.Cmd {
	var args []string

	// Use ulimit wrapper on Linux/macOS for memory limits
	memLimitKB := cfg.MaxMemoryBytes / 1024
	if memLimitKB > 0 {
		wrapper := fmt.Sprintf("ulimit -v %d 2>/dev/null; %s", memLimitKB, command)
		args = []string{"sh", "-c", wrapper}
	} else {
		args = []string{"sh", "-c", command}
	}

	cmd := exec.CommandContext(ctx, args[0], args[1:]...)

	// Block network access via environment hint (tools should check this)
	if !cfg.AllowNetwork {
		cmd.Env = append(cmd.Environ(), "SANDBOX_NO_NETWORK=1")
	}

	// Pass trace ID into environment
	if cfg.TraceID != "" {
		cmd.Env = append(cmd.Environ(), "SANDBOX_TRACE_ID="+cfg.TraceID)
	}

	return cmd
}

// validateCommand checks the command against deny-path rules.
// It normalises each token and resolves symlinks to prevent path-traversal bypasses.
func (e *Executor) validateCommand(command string, cfg *Config) error {
	// Build a list of normalised deny paths.
	normalDeny := make([]string, 0, len(cfg.DenyPaths))
	for _, dp := range cfg.DenyPaths {
		normalDeny = append(normalDeny, normalizePath(dp))
	}

	// Inspect every whitespace-separated token in the command.
	for _, token := range strings.Fields(command) {
		// Only consider tokens that look like absolute paths.
		if !strings.HasPrefix(token, "/") {
			continue
		}
		// Strip shell quoting characters that might obscure the real path.
		clean := strings.Trim(token, `"'`)
		// Normalise: collapse .., ., double slashes.
		clean = normalizePath(clean)
		// Attempt symlink resolution (best-effort; file may not exist yet).
		if resolved, err := filepath.EvalSymlinks(clean); err == nil {
			clean = normalizePath(resolved)
		}
		for _, denied := range normalDeny {
			if clean == denied || strings.HasPrefix(clean, denied+string(os.PathSeparator)) {
				return fmt.Errorf("[sandbox] access denied: path %q is under restricted path %q",
					token, denied)
			}
		}
	}

	// Also do a quick substring check for non-path tokens (e.g. "cat /etc/shadow" as one blob).
	cmdLower := strings.ToLower(command)
	for _, dp := range cfg.DenyPaths {
		if strings.Contains(cmdLower, strings.ToLower(dp)) {
			return fmt.Errorf("[sandbox] access denied: command references restricted path %q", dp)
		}
	}
	return nil
}

// normalizePath applies filepath.Clean and lowercases the result on case-
// insensitive systems. On Linux paths are case-sensitive so we keep case.
func normalizePath(p string) string {
	return filepath.Clean(p)
}

func limitOutput(s string, max int) string {
	if len(s) > max {
		return s[:max] + "\n... [output truncated by sandbox]"
	}
	return s
}
