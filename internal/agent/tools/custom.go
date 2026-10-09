package tools

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/shell"
	"github.com/charmbracelet/crush/internal/version"
)

// customToolAbandonGrace is how long a call waits, after its deadline, for
// the interpreter to yield before the agent stops waiting on it.
const customToolAbandonGrace = time.Second

// runCustomShell is the executor behind a custom tool; tests replace it.
var runCustomShell = shell.Run

// GetCustomTools returns an agent tool for every enabled custom tool in
// config, sorted by name.
func GetCustomTools(permissions permission.Service, cfg *config.ConfigStore, wd, spillDir string) []fantasy.AgentTool {
	declared := cfg.Config().CustomTools
	var result []fantasy.AgentTool
	for _, name := range slices.Sorted(maps.Keys(declared)) {
		t := declared[name]
		if t.Disabled {
			continue
		}
		result = append(result, NewCustomTool(name, t, permissions, wd, spillDir))
	}
	return result
}

// NewCustomTool returns the agent tool for one custom tool declaration.
func NewCustomTool(name string, cfg config.CustomToolConfig, permissions permission.Service, wd, spillDir string) *CustomTool {
	return &CustomTool{name: name, cfg: cfg, permissions: permissions, workingDir: wd, spillDir: spillDir}
}

// CustomTool is an agent tool implemented by a shell command, usually
// declared by a plugin with `tool add`.
type CustomTool struct {
	name            string
	cfg             config.CustomToolConfig
	permissions     permission.Service
	workingDir      string
	spillDir        string
	providerOptions fantasy.ProviderOptions
}

func (c *CustomTool) SetProviderOptions(opts fantasy.ProviderOptions) {
	c.providerOptions = opts
}

func (c *CustomTool) ProviderOptions() fantasy.ProviderOptions {
	return c.providerOptions
}

func (c *CustomTool) Info() fantasy.ToolInfo {
	parameters := map[string]any{}
	if props, ok := c.cfg.Schema["properties"].(map[string]any); ok {
		parameters = maps.Clone(props)
	}
	for name, desc := range c.cfg.Params {
		parameters[name] = map[string]any{"type": "string", "description": desc}
	}
	required := c.cfg.Required
	if required == nil {
		required = []string{}
	}
	return fantasy.ToolInfo{
		Name:        c.name,
		Description: c.cfg.Description,
		Parameters:  parameters,
		Required:    required,
	}
}

func (c *CustomTool) Run(ctx context.Context, params fantasy.ToolCall) (fantasy.ToolResponse, error) {
	sessionID := GetSessionFromContext(ctx)
	if sessionID == "" {
		return fantasy.ToolResponse{}, fmt.Errorf("session ID is required for running a custom tool")
	}

	ok, err := c.permissions.Request(ctx, permission.CreatePermissionRequest{
		SessionID:   sessionID,
		ToolCallID:  params.ID,
		Path:        c.workingDir,
		ToolName:    c.name,
		Action:      "execute",
		Description: fmt.Sprintf("execute %s with the following parameters:", c.name),
		Params:      params.Input,
	})
	if err != nil {
		return fantasy.ToolResponse{}, err
	}
	if !ok {
		return NewPermissionDeniedResponse(), nil
	}

	timeout := c.cfg.TimeoutDuration()
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var stdout, stderr bytes.Buffer
	run := runCustomShell
	done := make(chan error, 1)
	go func() {
		done <- run(runCtx, shell.RunOptions{
			Command: c.cfg.Command,
			Cwd:     c.workingDir,
			Env:     c.env(sessionID, params.Input),
			Stdin:   strings.NewReader(params.Input),
			Stdout:  &stdout,
			Stderr:  &stderr,
		})
	}()

	// Same ownership rule as hooks: the buffers are only read once the
	// goroutine has delivered on done.
	select {
	case err = <-done:
	case <-runCtx.Done():
		select {
		case err = <-done:
		case <-time.After(customToolAbandonGrace):
			slog.Warn("Custom tool did not yield after cancel; abandoning goroutine", "tool", c.name)
			return c.interrupted(ctx, timeout), nil
		}
	}

	if shell.IsInterrupt(err) || (err != nil && runCtx.Err() != nil) {
		return c.interrupted(ctx, timeout), nil
	}

	out := TruncateOutput(strings.TrimRight(stdout.String(), "\n"), c.spillDir)
	errOut := TruncateOutput(strings.TrimRight(stderr.String(), "\n"), c.spillDir)
	if err != nil {
		msg := fmt.Sprintf("%s failed with exit code %d", c.name, shell.ExitCode(err))
		if errOut != "" {
			msg += "\n\nstderr:\n" + errOut
		}
		if out != "" {
			msg += "\n\nstdout:\n" + out
		}
		return fantasy.NewTextErrorResponse(msg), nil
	}
	if out == "" {
		out = "(no output)"
	}
	return fantasy.NewTextResponse(out), nil
}

func (c *CustomTool) interrupted(ctx context.Context, timeout time.Duration) fantasy.ToolResponse {
	if ctx.Err() != nil {
		return fantasy.NewTextErrorResponse(fmt.Sprintf("%s was cancelled", c.name))
	}
	return fantasy.NewTextErrorResponse(fmt.Sprintf("%s timed out after %s", c.name, timeout))
}

// env is the command's environment: Crush's own, the declared extras, and
// the call's context, so a command can read its input without stdin.
func (c *CustomTool) env(sessionID, input string) []string {
	env := os.Environ()
	for _, k := range slices.Sorted(maps.Keys(c.cfg.Env)) {
		env = append(env, k+"="+c.cfg.Env[k])
	}
	env = append(env,
		"CRUSH_TOOL_NAME="+c.name,
		"CRUSH_TOOL_INPUT="+input,
		"CRUSH_SESSION_ID="+sessionID,
		"CRUSH_PROJECT_DIR="+c.workingDir,
		"CRUSH_VERSION="+version.Version,
	)
	if c.cfg.Source != "" {
		env = append(env,
			"CRUSH_PLUGIN_FILE="+c.cfg.Source,
			"CRUSH_PLUGIN_DIR="+filepath.Dir(c.cfg.Source),
		)
	}
	return env
}
