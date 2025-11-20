package hooks

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/crush/internal/shell"
)

//go:embed helpers.sh
var helpersScript string

// Executor executes individual hook scripts.
type Executor struct {
	workingDir string
}

// NewExecutor creates a new hook executor.
func NewExecutor(workingDir string) *Executor {
	return &Executor{workingDir: workingDir}
}

// Execute runs a single hook script and returns the result.
func (e *Executor) Execute(ctx context.Context, hookPath string, context HookContext) (*HookResult, error) {
	hookScript, err := os.ReadFile(hookPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read hook: %w", err)
	}

	contextJSON, err := json.Marshal(context.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal context: %w", err)
	}

	// Wrap user hook in a function and prepend helper functions  
	// Read stdin before calling the function, then export it
	fullScript := fmt.Sprintf(`%s

# Save stdin to variable before entering function
_CRUSH_STDIN=$(cat)
export _CRUSH_STDIN

_crush_hook_main() {
%s
}

_crush_hook_main
`, helpersScript, string(hookScript))

	env := append(os.Environ(),
		"CRUSH_HOOK_TYPE="+string(context.HookType),
		"CRUSH_SESSION_ID="+context.SessionID,
		"CRUSH_WORKING_DIR="+context.WorkingDir,
	)

	if context.ToolName != "" {
		env = append(env,
			"CRUSH_TOOL_NAME="+context.ToolName,
			"CRUSH_TOOL_CALL_ID="+context.ToolCallID,
		)
	}

	for k, v := range context.Environment {
		env = append(env, k+"="+v)
	}

	hookShell := shell.NewShell(&shell.Options{
		WorkingDir: context.WorkingDir,
		Env:        env,
	})

	// Pass JSON context via stdin instead of heredoc
	stdin := strings.NewReader(string(contextJSON))
	stdout, stderr, err := hookShell.ExecWithStdin(ctx, fullScript, stdin)

	result := parseShellEnv(hookShell.GetEnv())
	exitCode := shell.ExitCode(err)
	switch exitCode {
	case 2:
		result.Continue = false
	case 1:
		return nil, fmt.Errorf("hook failed with exit code 1: %w\nstderr: %s", err, stderr)
	}

	if trimmed := strings.TrimSpace(stdout); len(trimmed) > 0 && trimmed[0] == '{' {
		if jsonResult, parseErr := parseJSONResult([]byte(trimmed)); parseErr == nil {
			mergeJSONResult(result, jsonResult)
		}
	}

	return result, nil
}

// GetHelpersScript returns the embedded helper script for display.
func GetHelpersScript() string {
	return helpersScript
}
