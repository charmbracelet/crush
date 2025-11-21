package hooks

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestReadmeExamples tests that all examples from the README work as documented.
func TestReadmeExamples(t *testing.T) {
	t.Parallel()

	t.Run("block dangerous commands", func(t *testing.T) {
		t.Parallel()
		tempDir := t.TempDir()
		hooksDir := filepath.Join(tempDir, ".crush", "hooks", "pre-tool-use")
		require.NoError(t, os.MkdirAll(hooksDir, 0o755))

		hookScript := `#!/bin/bash
if [ "$CRUSH_TOOL_NAME" = "bash" ]; then
  COMMAND=$(crush_get_tool_input command)
  if [[ "$COMMAND" =~ "rm -rf /" ]]; then
    crush_deny "Blocked dangerous command"
  fi
fi
`
		hookPath := filepath.Join(hooksDir, "01-block-dangerous.sh")
		require.NoError(t, os.WriteFile(hookPath, []byte(hookScript), 0o755))

		manager := NewManager(tempDir, filepath.Join(tempDir, ".crush"), nil)

		// Test: Should block "rm -rf /"
		result, err := manager.ExecutePreToolUse(context.Background(), "test", tempDir, PreToolUseData{
			ToolName:   "bash",
			ToolCallID: "call-1",
			ToolInput: map[string]any{
				"command": "rm -rf /",
			},
		})

		require.NoError(t, err)
		assert.False(t, result.Continue, "Should stop execution for dangerous command")
		assert.Equal(t, "deny", result.Permission)
		assert.Contains(t, result.Message, "Blocked dangerous command")

		// Test: Should allow safe commands
		result2, err := manager.ExecutePreToolUse(context.Background(), "test", tempDir, PreToolUseData{
			ToolName:   "bash",
			ToolCallID: "call-2",
			ToolInput: map[string]any{
				"command": "ls -la",
			},
		})

		require.NoError(t, err)
		assert.True(t, result2.Continue, "Should allow safe commands")
	})

	t.Run("auto-approve read-only tools", func(t *testing.T) {
		t.Parallel()
		tempDir := t.TempDir()
		hooksDir := filepath.Join(tempDir, ".crush", "hooks", "pre-tool-use")
		require.NoError(t, os.MkdirAll(hooksDir, 0o755))

		hookScript := `#!/bin/bash
case "$CRUSH_TOOL_NAME" in
  view|ls|grep|glob)
    crush_approve "Auto-approved read-only tool"
    ;;
  bash)
    COMMAND=$(crush_get_tool_input command)
    if [[ "$COMMAND" =~ ^(ls|cat|grep) ]]; then
      crush_approve "Auto-approved safe bash command"
    fi
    ;;
esac
`
		hookPath := filepath.Join(hooksDir, "01-auto-approve.sh")
		require.NoError(t, os.WriteFile(hookPath, []byte(hookScript), 0o755))

		manager := NewManager(tempDir, filepath.Join(tempDir, ".crush"), nil)

		// Test: Should auto-approve view tool
		result, err := manager.ExecutePreToolUse(context.Background(), "test", tempDir, PreToolUseData{
			ToolName:   "view",
			ToolCallID: "call-1",
		})

		require.NoError(t, err)
		assert.True(t, result.Continue)
		assert.Equal(t, "approve", result.Permission)
		assert.Contains(t, result.Message, "Auto-approved read-only tool")

		// Test: Should auto-approve safe bash commands
		result2, err := manager.ExecutePreToolUse(context.Background(), "test", tempDir, PreToolUseData{
			ToolName:   "bash",
			ToolCallID: "call-2",
			ToolInput: map[string]any{
				"command": "ls -la",
			},
		})

		require.NoError(t, err)
		assert.True(t, result2.Continue)
		assert.Equal(t, "approve", result2.Permission)
		assert.Contains(t, result2.Message, "Auto-approved safe bash command")
	})

	t.Run("add git context", func(t *testing.T) {
		t.Parallel()
		tempDir := t.TempDir()
		hooksDir := filepath.Join(tempDir, ".crush", "hooks", "user-prompt-submit")
		require.NoError(t, os.MkdirAll(hooksDir, 0o755))

		// Initialize git repo with a branch
		gitDir := filepath.Join(tempDir, ".git")
		require.NoError(t, os.MkdirAll(gitDir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/main\n"), 0o644))

		hookScript := `#!/bin/bash
BRANCH=$(git branch --show-current 2>/dev/null)
if [ -n "$BRANCH" ]; then
  crush_add_context "Current branch: $BRANCH"
fi

if [ -f "README.md" ]; then
  crush_add_context_file "README.md"
fi
`
		hookPath := filepath.Join(hooksDir, "01-add-context.sh")
		require.NoError(t, os.WriteFile(hookPath, []byte(hookScript), 0o755))

		// Create README.md
		readmePath := filepath.Join(tempDir, "README.md")
		require.NoError(t, os.WriteFile(readmePath, []byte("# Test Project\n"), 0o644))

		manager := NewManager(tempDir, filepath.Join(tempDir, ".crush"), nil)

		result, err := manager.ExecuteUserPromptSubmit(context.Background(), "test", tempDir, UserPromptSubmitData{
			Prompt: "help me",
		})

		require.NoError(t, err)
		assert.True(t, result.Continue)
		// Should add context file (using relative path)
		require.Len(t, result.ContextFiles, 1)
		assert.Equal(t, "README.md", result.ContextFiles[0])
	})

	t.Run("audit logging", func(t *testing.T) {
		t.Parallel()
		tempDir := t.TempDir()
		hooksDir := filepath.Join(tempDir, ".crush", "hooks", "post-tool-use")
		require.NoError(t, os.MkdirAll(hooksDir, 0o755))

		auditFile := filepath.Join(tempDir, "audit.log")
		hookScript := `#!/bin/bash
AUDIT_FILE="` + auditFile + `"
TIMESTAMP=$(date -Iseconds)
echo "$TIMESTAMP|$CRUSH_TOOL_NAME|$CRUSH_TOOL_CALL_ID" >> "$AUDIT_FILE"
`
		hookPath := filepath.Join(hooksDir, "01-audit.sh")
		require.NoError(t, os.WriteFile(hookPath, []byte(hookScript), 0o755))

		manager := NewManager(tempDir, filepath.Join(tempDir, ".crush"), nil)

		result, err := manager.ExecutePostToolUse(context.Background(), "test", tempDir, PostToolUseData{
			ToolName:   "bash",
			ToolCallID: "call-123",
		})

		require.NoError(t, err)
		assert.True(t, result.Continue)

		// Verify audit log was written
		content, err := os.ReadFile(auditFile)
		require.NoError(t, err)
		assert.Contains(t, string(content), "bash|call-123")
	})

	t.Run("catch-all hook", func(t *testing.T) {
		t.Parallel()
		tempDir := t.TempDir()
		hooksDir := filepath.Join(tempDir, ".crush", "hooks")
		require.NoError(t, os.MkdirAll(hooksDir, 0o755))

		logFile := filepath.Join(tempDir, "global.log")
		hookScript := `#!/bin/bash
echo "Hook: $CRUSH_HOOK_TYPE" >> "` + logFile + `"
`
		hookPath := filepath.Join(hooksDir, "00-global-log.sh")
		require.NoError(t, os.WriteFile(hookPath, []byte(hookScript), 0o755))

		manager := NewManager(tempDir, filepath.Join(tempDir, ".crush"), nil)

		// Test with different hook types
		_, err := manager.ExecutePreToolUse(context.Background(), "test", tempDir, PreToolUseData{})
		require.NoError(t, err)

		_, err = manager.ExecuteUserPromptSubmit(context.Background(), "test", tempDir, UserPromptSubmitData{})
		require.NoError(t, err)

		// Verify both hook types were logged
		content, err := os.ReadFile(logFile)
		require.NoError(t, err)
		assert.Contains(t, string(content), "Hook: pre-tool-use")
		assert.Contains(t, string(content), "Hook: user-prompt-submit")
	})

	t.Run("rate limiting", func(t *testing.T) {
		t.Parallel()
		tempDir := t.TempDir()
		hooksDir := filepath.Join(tempDir, ".crush", "hooks", "pre-tool-use")
		require.NoError(t, os.MkdirAll(hooksDir, 0o755))

		usageLog := filepath.Join(tempDir, "usage.log")
		// Pre-populate with entries
		today := "2024-01-15" // Fixed date for testing
		for i := 0; i < 5; i++ {
			f, err := os.OpenFile(usageLog, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
			require.NoError(t, err)
			_, err = f.WriteString(today + "\n")
			require.NoError(t, err)
			f.Close()
		}

		hookScript := `#!/bin/bash
COUNT=$(grep -c "2024-01-15" "` + usageLog + `" 2>/dev/null || echo "0")
if [ "$COUNT" -ge 3 ]; then
  export CRUSH_CONTINUE=false
  export CRUSH_MESSAGE="Rate limit exceeded"
fi
`
		hookPath := filepath.Join(hooksDir, "01-rate-limit.sh")
		require.NoError(t, os.WriteFile(hookPath, []byte(hookScript), 0o755))

		manager := NewManager(tempDir, filepath.Join(tempDir, ".crush"), nil)

		result, err := manager.ExecutePreToolUse(context.Background(), "test", tempDir, PreToolUseData{})

		require.NoError(t, err)
		assert.False(t, result.Continue, "Should stop execution when rate limit exceeded")
		assert.Contains(t, result.Message, "Rate limit exceeded")
	})

	t.Run("conditional context", func(t *testing.T) {
		t.Parallel()
		tempDir := t.TempDir()
		hooksDir := filepath.Join(tempDir, ".crush", "hooks", "user-prompt-submit")
		require.NoError(t, os.MkdirAll(hooksDir, 0o755))

		// Create package.json
		packageJSON := filepath.Join(tempDir, "package.json")
		require.NoError(t, os.WriteFile(packageJSON, []byte(`{"name": "test"}`), 0o644))

		hookScript := `#!/bin/bash
if [ -f "package.json" ]; then
  crush_add_context_file "package.json"
fi
`
		hookPath := filepath.Join(hooksDir, "01-conditional.sh")
		require.NoError(t, os.WriteFile(hookPath, []byte(hookScript), 0o755))

		manager := NewManager(tempDir, filepath.Join(tempDir, ".crush"), nil)

		result, err := manager.ExecuteUserPromptSubmit(context.Background(), "test", tempDir, UserPromptSubmitData{})

		require.NoError(t, err)
		assert.True(t, result.Continue)
		require.Len(t, result.ContextFiles, 1)
		assert.Equal(t, "package.json", result.ContextFiles[0])
	})

	t.Run("JSON output example", func(t *testing.T) {
		t.Parallel()
		tempDir := t.TempDir()
		hooksDir := filepath.Join(tempDir, ".crush", "hooks", "pre-tool-use")
		require.NoError(t, os.MkdirAll(hooksDir, 0o755))

		hookScript := `#!/bin/bash
COMMAND=$(crush_get_tool_input command)
SAFE_CMD=$(echo "$COMMAND" | sed 's/--force//')
echo "{\"modified_input\": {\"command\": \"$SAFE_CMD\"}}"
`
		hookPath := filepath.Join(hooksDir, "01-modify.sh")
		require.NoError(t, os.WriteFile(hookPath, []byte(hookScript), 0o755))

		manager := NewManager(tempDir, filepath.Join(tempDir, ".crush"), nil)

		result, err := manager.ExecutePreToolUse(context.Background(), "test", tempDir, PreToolUseData{
			ToolName:   "bash",
			ToolCallID: "call-1",
			ToolInput: map[string]any{
				"command": "rm --force file.txt",
			},
		})

		require.NoError(t, err)
		assert.True(t, result.Continue)
		require.NotNil(t, result.ModifiedInput)
		assert.Equal(t, "rm  file.txt", result.ModifiedInput["command"])
	})

	t.Run("environment variables example", func(t *testing.T) {
		t.Parallel()
		tempDir := t.TempDir()
		hooksDir := filepath.Join(tempDir, ".crush", "hooks", "pre-tool-use")
		require.NoError(t, os.MkdirAll(hooksDir, 0o755))

		hookScript := `#!/bin/bash
export CRUSH_PERMISSION=approve
export CRUSH_MESSAGE="Auto-approved"
`
		hookPath := filepath.Join(hooksDir, "01-env-vars.sh")
		require.NoError(t, os.WriteFile(hookPath, []byte(hookScript), 0o755))

		manager := NewManager(tempDir, filepath.Join(tempDir, ".crush"), nil)

		result, err := manager.ExecutePreToolUse(context.Background(), "test", tempDir, PreToolUseData{})

		require.NoError(t, err)
		assert.True(t, result.Continue)
		assert.Equal(t, "approve", result.Permission)
		assert.Equal(t, "Auto-approved", result.Message)
	})

	t.Run("exit codes example", func(t *testing.T) {
		t.Parallel()
		tempDir := t.TempDir()
		hooksDir := filepath.Join(tempDir, ".crush", "hooks", "pre-tool-use")
		require.NoError(t, os.MkdirAll(hooksDir, 0o755))

		usageLog := filepath.Join(tempDir, "usage.log")
		// Create usage log with entries
		for i := 0; i < 150; i++ {
			f, err := os.OpenFile(usageLog, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
			require.NoError(t, err)
			_, err = f.WriteString("2024-01-15\n")
			require.NoError(t, err)
			f.Close()
		}

		hookScript := `#!/bin/bash
COUNT=$(grep -c "2024-01-15" "` + usageLog + `")
if [ "$COUNT" -gt 100 ]; then
  echo "Rate limit exceeded" >&2
  exit 2  # Stops execution
fi
`
		hookPath := filepath.Join(hooksDir, "01-exit-code.sh")
		require.NoError(t, os.WriteFile(hookPath, []byte(hookScript), 0o755))

		manager := NewManager(tempDir, filepath.Join(tempDir, ".crush"), nil)

		result, err := manager.ExecutePreToolUse(context.Background(), "test", tempDir, PreToolUseData{})

		require.NoError(t, err)
		assert.False(t, result.Continue, "Exit code 2 should stop execution")
	})

	t.Run("helper functions comprehensive test", func(t *testing.T) {
		t.Parallel()
		tempDir := t.TempDir()
		hooksDir := filepath.Join(tempDir, ".crush", "hooks", "user-prompt-submit")
		require.NoError(t, os.MkdirAll(hooksDir, 0o755))

		// Test all helper functions in one hook
		hookScript := `#!/bin/bash
# Read stdin once into variable
CONTEXT=$(cat)

# Test input parsing
PROMPT=$(echo "$CONTEXT" | crush_get_prompt)
MODEL=$(echo "$CONTEXT" | crush_get_input model)

# Test context helpers
crush_add_context "Using model: $MODEL"

# Test logging
crush_log "Processing prompt"

# Test modification
export CRUSH_MODIFIED_PROMPT="Enhanced: $PROMPT"
`
		hookPath := filepath.Join(hooksDir, "01-helpers.sh")
		require.NoError(t, os.WriteFile(hookPath, []byte(hookScript), 0o755))

		manager := NewManager(tempDir, filepath.Join(tempDir, ".crush"), nil)

		result, err := manager.ExecuteUserPromptSubmit(context.Background(), "test", tempDir, UserPromptSubmitData{
			Prompt: "original prompt",
			Model:  "gpt-4",
		})

		require.NoError(t, err)
		assert.True(t, result.Continue)
		assert.Contains(t, result.ContextContent, "Using model: gpt-4")
		require.NotNil(t, result.ModifiedPrompt)
		// Trim any trailing whitespace/CRLF for cross-platform compatibility
		assert.Equal(t, "Enhanced: original prompt", strings.TrimSpace(*result.ModifiedPrompt))
	})

	t.Run("is_first_message flag", func(t *testing.T) {
		t.Parallel()
		tempDir := t.TempDir()
		hooksDir := filepath.Join(tempDir, ".crush", "hooks", "user-prompt-submit")
		require.NoError(t, os.MkdirAll(hooksDir, 0o755))

		// Hook that adds README only on first message
		hookScript := `#!/bin/bash
IS_FIRST=$(crush_get_input is_first_message)
if [ "$IS_FIRST" = "true" ]; then
  crush_add_context "This is the first message"
else
  crush_add_context "This is a follow-up message"
fi
`
		hookPath := filepath.Join(hooksDir, "01-first-msg.sh")
		require.NoError(t, os.WriteFile(hookPath, []byte(hookScript), 0o755))

		manager := NewManager(tempDir, filepath.Join(tempDir, ".crush"), nil)

		// Test: First message
		result1, err := manager.ExecuteUserPromptSubmit(context.Background(), "test", tempDir, UserPromptSubmitData{
			Prompt:         "first prompt",
			IsFirstMessage: true,
		})
		require.NoError(t, err)
		assert.Contains(t, result1.ContextContent, "This is the first message")

		// Test: Follow-up message
		result2, err := manager.ExecuteUserPromptSubmit(context.Background(), "test", tempDir, UserPromptSubmitData{
			Prompt:         "follow-up prompt",
			IsFirstMessage: false,
		})
		require.NoError(t, err)
		assert.Contains(t, result2.ContextContent, "This is a follow-up message")
	})
}

// TestReadmeQuickExamples tests the quick examples from the quick reference.
func TestReadmeQuickExamples(t *testing.T) {
	t.Parallel()

	t.Run("hook ordering", func(t *testing.T) {
		t.Parallel()
		tempDir := t.TempDir()
		hooksDir := filepath.Join(tempDir, ".crush", "hooks", "pre-tool-use")
		require.NoError(t, os.MkdirAll(hooksDir, 0o755))

		// Create hooks with specific order
		hook1 := `#!/bin/bash
export CRUSH_MESSAGE="first"
`
		hook2 := `#!/bin/bash
export CRUSH_MESSAGE="${CRUSH_MESSAGE:-}; second"
`
		hook3 := `#!/bin/bash
export CRUSH_MESSAGE="${CRUSH_MESSAGE:-}; third"
`

		require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "01-first.sh"), []byte(hook1), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "02-second.sh"), []byte(hook2), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "99-third.sh"), []byte(hook3), 0o755))

		manager := NewManager(tempDir, filepath.Join(tempDir, ".crush"), nil)

		result, err := manager.ExecutePreToolUse(context.Background(), "test", tempDir, PreToolUseData{})

		require.NoError(t, err)
		// Messages should be merged in order
		assert.Contains(t, result.Message, "first")
		assert.Contains(t, result.Message, "second")
		assert.Contains(t, result.Message, "third")
	})

	t.Run("mixed env vars and JSON", func(t *testing.T) {
		t.Parallel()
		tempDir := t.TempDir()
		hooksDir := filepath.Join(tempDir, ".crush", "hooks", "pre-tool-use")
		require.NoError(t, os.MkdirAll(hooksDir, 0o755))

		hookScript := `#!/bin/bash
# Set via environment variable
export CRUSH_PERMISSION=approve

# Output via JSON
echo '{"message": "Combined output", "modified_input": {"key": "value"}}'
`
		hookPath := filepath.Join(hooksDir, "01-mixed.sh")
		require.NoError(t, os.WriteFile(hookPath, []byte(hookScript), 0o755))

		manager := NewManager(tempDir, filepath.Join(tempDir, ".crush"), nil)

		result, err := manager.ExecutePreToolUse(context.Background(), "test", tempDir, PreToolUseData{})

		require.NoError(t, err)
		assert.True(t, result.Continue)
		assert.Equal(t, "approve", result.Permission)
		assert.Equal(t, "Combined output", result.Message)
		assert.Equal(t, "value", result.ModifiedInput["key"])
	})
}
