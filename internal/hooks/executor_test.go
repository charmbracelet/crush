package hooks

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecutor(t *testing.T) {
	// Create temp directory for test hooks.
	tempDir := t.TempDir()

	t.Run("executes simple hook with env vars", func(t *testing.T) {
		hookPath := filepath.Join(tempDir, "test-hook.sh")
		hookScript := `#!/bin/bash
export CRUSH_PERMISSION=approve
export CRUSH_MESSAGE="test message"
`
		err := os.WriteFile(hookPath, []byte(hookScript), 0o755)
		require.NoError(t, err)

		executor := NewExecutor(tempDir)
		ctx := context.Background()
		hookCtx := HookContext{
			HookType:   HookPreToolUse,
			SessionID:  "test-session",
			WorkingDir: tempDir,
			Data: map[string]any{
				"tool_input": map[string]any{
					"command": "ls",
				},
			},
		}

		result, err := executor.Execute(ctx, hookPath, hookCtx)

		require.NoError(t, err)
		assert.True(t, result.Continue)
		assert.Equal(t, "approve", result.Permission)
		assert.Equal(t, "test message", result.Message)
	})

	t.Run("helper functions are available", func(t *testing.T) {
		hookPath := filepath.Join(tempDir, "helper-test.sh")
		hookScript := `#!/bin/bash
crush_approve "auto approved"
`
		err := os.WriteFile(hookPath, []byte(hookScript), 0o755)
		require.NoError(t, err)

		executor := NewExecutor(tempDir)
		ctx := context.Background()
		hookCtx := HookContext{
			HookType:   HookPreToolUse,
			SessionID:  "test-session",
			WorkingDir: tempDir,
			Data:       map[string]any{},
		}

		result, err := executor.Execute(ctx, hookPath, hookCtx)

		require.NoError(t, err)
		assert.Equal(t, "approve", result.Permission)
		assert.Equal(t, "auto approved", result.Message)
	})

	t.Run("crush_deny sets continue=false and exits", func(t *testing.T) {
		hookPath := filepath.Join(tempDir, "deny-test.sh")
		hookScript := `#!/bin/bash
crush_deny "blocked"
`
		err := os.WriteFile(hookPath, []byte(hookScript), 0o755)
		require.NoError(t, err)

		executor := NewExecutor(tempDir)
		ctx := context.Background()
		hookCtx := HookContext{
			HookType:   HookPreToolUse,
			SessionID:  "test-session",
			WorkingDir: tempDir,
			Data:       map[string]any{},
		}

		result, err := executor.Execute(ctx, hookPath, hookCtx)

		require.NoError(t, err)
		assert.False(t, result.Continue)
		assert.Equal(t, "deny", result.Permission)
		assert.Equal(t, "blocked", result.Message)
	})

	t.Run("reads JSON from stdin", func(t *testing.T) {
		hookPath := filepath.Join(tempDir, "stdin-test.sh")
		hookScript := `#!/bin/bash
COMMAND=$(crush_get_tool_input command)
if [ "$COMMAND" = "dangerous" ]; then
  crush_deny "dangerous command"
fi
`
		err := os.WriteFile(hookPath, []byte(hookScript), 0o755)
		require.NoError(t, err)

		executor := NewExecutor(tempDir)
		ctx := context.Background()
		hookCtx := HookContext{
			HookType:   HookPreToolUse,
			SessionID:  "test-session",
			WorkingDir: tempDir,
			Data: map[string]any{
				"tool_input": map[string]any{
					"command": "dangerous",
				},
			},
		}

		result, err := executor.Execute(ctx, hookPath, hookCtx)

		require.NoError(t, err)
		assert.False(t, result.Continue)
		assert.Equal(t, "deny", result.Permission)
	})

	t.Run("env variables are set correctly", func(t *testing.T) {
		hookPath := filepath.Join(tempDir, "env-test.sh")
		hookScript := `#!/bin/bash
if [ "$CRUSH_HOOK_TYPE" = "pre-tool-use" ] && \
   [ "$CRUSH_SESSION_ID" = "test-123" ] && \
   [ "$CRUSH_TOOL_NAME" = "bash" ]; then
  export CRUSH_MESSAGE="env vars correct"
fi
`
		err := os.WriteFile(hookPath, []byte(hookScript), 0o755)
		require.NoError(t, err)

		executor := NewExecutor(tempDir)
		ctx := context.Background()
		hookCtx := HookContext{
			HookType:   HookPreToolUse,
			SessionID:  "test-123",
			WorkingDir: tempDir,
			ToolName:   "bash",
			ToolCallID: "call-123",
			Data:       map[string]any{},
		}

		result, err := executor.Execute(ctx, hookPath, hookCtx)

		require.NoError(t, err)
		assert.Equal(t, "env vars correct", result.Message)
	})

	t.Run("supports JSON output for complex mutations", func(t *testing.T) {
		hookPath := filepath.Join(tempDir, "json-test.sh")
		hookScript := `#!/bin/bash
cat <<EOF
{
  "permission": "approve",
  "modified_input": {
    "command": "ls -la",
    "safe": true
  }
}
EOF
`
		err := os.WriteFile(hookPath, []byte(hookScript), 0o755)
		require.NoError(t, err)

		executor := NewExecutor(tempDir)
		ctx := context.Background()
		hookCtx := HookContext{
			HookType:   HookPreToolUse,
			SessionID:  "test-session",
			WorkingDir: tempDir,
			Data:       map[string]any{},
		}

		result, err := executor.Execute(ctx, hookPath, hookCtx)

		require.NoError(t, err)
		assert.Equal(t, "approve", result.Permission)
		assert.Equal(t, "ls -la", result.ModifiedInput["command"])
		assert.Equal(t, true, result.ModifiedInput["safe"])
	})

	t.Run("handles exit code 1 as error", func(t *testing.T) {
		hookPath := filepath.Join(tempDir, "error-test.sh")
		hookScript := `#!/bin/bash
echo "error occurred" >&2
exit 1
`
		err := os.WriteFile(hookPath, []byte(hookScript), 0o755)
		require.NoError(t, err)

		executor := NewExecutor(tempDir)
		ctx := context.Background()
		hookCtx := HookContext{
			HookType:   HookPreToolUse,
			SessionID:  "test-session",
			WorkingDir: tempDir,
			Data:       map[string]any{},
		}

		_, err = executor.Execute(ctx, hookPath, hookCtx)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "hook failed with exit code 1")
	})

	t.Run("context files helper", func(t *testing.T) {
		hookPath := filepath.Join(tempDir, "files-test.sh")
		hookScript := `#!/bin/bash
crush_add_context_file "file1.md"
crush_add_context_file "file2.txt"
`
		err := os.WriteFile(hookPath, []byte(hookScript), 0o755)
		require.NoError(t, err)

		executor := NewExecutor(tempDir)
		ctx := context.Background()
		hookCtx := HookContext{
			HookType:   HookUserPromptSubmit,
			SessionID:  "test-session",
			WorkingDir: tempDir,
			Data:       map[string]any{},
		}

		result, err := executor.Execute(ctx, hookPath, hookCtx)

		require.NoError(t, err)
		assert.Equal(t, []string{"file1.md", "file2.txt"}, result.ContextFiles)
	})

	t.Run("context content helper", func(t *testing.T) {
		hookPath := filepath.Join(tempDir, "content-test.sh")
		hookScript := `#!/bin/bash
crush_add_context "This is additional context"
`
		err := os.WriteFile(hookPath, []byte(hookScript), 0o755)
		require.NoError(t, err)

		executor := NewExecutor(tempDir)
		ctx := context.Background()
		hookCtx := HookContext{
			HookType:   HookUserPromptSubmit,
			SessionID:  "test-session",
			WorkingDir: tempDir,
			Data:       map[string]any{},
		}

		result, err := executor.Execute(ctx, hookPath, hookCtx)

		require.NoError(t, err)
		assert.Equal(t, "This is additional context", result.ContextContent)
	})

	t.Run("returns error if hook file doesn't exist", func(t *testing.T) {
		executor := NewExecutor(tempDir)
		ctx := context.Background()
		hookCtx := HookContext{
			HookType:   HookPreToolUse,
			SessionID:  "test-session",
			WorkingDir: tempDir,
			Data:       map[string]any{},
		}

		_, err := executor.Execute(ctx, "/nonexistent/hook.sh", hookCtx)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to read hook")
	})

	t.Run("passes custom environment variables", func(t *testing.T) {
		hookPath := filepath.Join(tempDir, "custom-env-test.sh")
		hookScript := `#!/bin/bash
if [ "$CUSTOM_API_KEY" = "secret123" ] && [ "$CUSTOM_REGION" = "us-west-2" ]; then
  export CRUSH_MESSAGE="custom env vars set correctly"
fi
`
		err := os.WriteFile(hookPath, []byte(hookScript), 0o755)
		require.NoError(t, err)

		executor := NewExecutor(tempDir)
		ctx := context.Background()
		hookCtx := HookContext{
			HookType:   HookPreToolUse,
			SessionID:  "test-session",
			WorkingDir: tempDir,
			Data:       map[string]any{},
			Environment: map[string]string{
				"CUSTOM_API_KEY": "secret123",
				"CUSTOM_REGION":  "us-west-2",
			},
		}

		result, err := executor.Execute(ctx, hookPath, hookCtx)

		require.NoError(t, err)
		assert.Equal(t, "custom env vars set correctly", result.Message)
	})

	t.Run("modify input helper function", func(t *testing.T) {
		hookPath := filepath.Join(tempDir, "modify-input-test.sh")
		hookScript := `#!/bin/bash
crush_modify_input "command" "ls -la"
crush_modify_input "working_dir" "/tmp"
`
		err := os.WriteFile(hookPath, []byte(hookScript), 0o755)
		require.NoError(t, err)

		executor := NewExecutor(tempDir)
		ctx := context.Background()
		hookCtx := HookContext{
			HookType:   HookPreToolUse,
			SessionID:  "test-session",
			WorkingDir: tempDir,
			Data:       map[string]any{},
		}

		result, err := executor.Execute(ctx, hookPath, hookCtx)

		require.NoError(t, err)
		require.NotNil(t, result.ModifiedInput)
		assert.Equal(t, "ls -la", result.ModifiedInput["command"])
		assert.Equal(t, "/tmp", result.ModifiedInput["working_dir"])
	})

	t.Run("modify output helper function", func(t *testing.T) {
		hookPath := filepath.Join(tempDir, "modify-output-test.sh")
		hookScript := `#!/bin/bash
crush_modify_output "status" "redacted"
crush_modify_output "data" "[REDACTED]"
`
		err := os.WriteFile(hookPath, []byte(hookScript), 0o755)
		require.NoError(t, err)

		executor := NewExecutor(tempDir)
		ctx := context.Background()
		hookCtx := HookContext{
			HookType:   HookPostToolUse,
			SessionID:  "test-session",
			WorkingDir: tempDir,
			Data:       map[string]any{},
		}

		result, err := executor.Execute(ctx, hookPath, hookCtx)

		require.NoError(t, err)
		require.NotNil(t, result.ModifiedOutput)
		assert.Equal(t, "redacted", result.ModifiedOutput["status"])
		assert.Equal(t, "[REDACTED]", result.ModifiedOutput["data"])
	})

	t.Run("modify input with JSON types", func(t *testing.T) {
		hookPath := filepath.Join(tempDir, "modify-input-json-test.sh")
		hookScript := `#!/bin/bash
crush_modify_input "offset" "100"
crush_modify_input "limit" "50"
crush_modify_input "run_in_background" "true"
crush_modify_input "ignore" '["*.log","*.tmp"]'
`
		err := os.WriteFile(hookPath, []byte(hookScript), 0o755)
		require.NoError(t, err)

		executor := NewExecutor(tempDir)
		ctx := context.Background()
		hookCtx := HookContext{
			HookType:   HookPreToolUse,
			SessionID:  "test-session",
			WorkingDir: tempDir,
			Data:       map[string]any{},
		}

		result, err := executor.Execute(ctx, hookPath, hookCtx)

		require.NoError(t, err)
		require.NotNil(t, result.ModifiedInput)
		assert.Equal(t, float64(100), result.ModifiedInput["offset"])
		assert.Equal(t, float64(50), result.ModifiedInput["limit"])
		assert.Equal(t, true, result.ModifiedInput["run_in_background"])
		assert.Equal(t, []any{"*.log", "*.tmp"}, result.ModifiedInput["ignore"])
	})
}

func TestGetHelpersScript(t *testing.T) {
	script := GetHelpersScript()

	assert.NotEmpty(t, script)
	assert.Contains(t, script, "crush_approve")
	assert.Contains(t, script, "crush_deny")
	assert.Contains(t, script, "crush_add_context")
}
