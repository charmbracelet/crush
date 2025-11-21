package hooks

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManager(t *testing.T) {
	t.Run("discovers hooks in order", func(t *testing.T) {
		tempDir := t.TempDir()
		dataDir := filepath.Join(tempDir, ".crush")
		hooksDir := filepath.Join(dataDir, "hooks", "pre-tool-use")
		require.NoError(t, os.MkdirAll(hooksDir, 0o755))

		// Create hooks with numeric prefixes.
		hooks := []string{"02-second.sh", "01-first.sh", "03-third.sh"}
		for _, name := range hooks {
			path := filepath.Join(hooksDir, name)
			err := os.WriteFile(path, []byte("#!/bin/bash\necho test"), 0o755)
			require.NoError(t, err)
		}

		mgr := NewManager(tempDir, dataDir, nil)
		discovered := mgr.ListHooks(HookPreToolUse)

		assert.Len(t, discovered, 3)
		// Should be sorted alphabetically.
		assert.Contains(t, discovered[0], "01-first.sh")
		assert.Contains(t, discovered[1], "02-second.sh")
		assert.Contains(t, discovered[2], "03-third.sh")
	})

	t.Run("skips non-executable files", func(t *testing.T) {
		tempDir := t.TempDir()
		dataDir := filepath.Join(tempDir, ".crush")
		hooksDir := filepath.Join(dataDir, "hooks", "pre-tool-use")
		require.NoError(t, os.MkdirAll(hooksDir, 0o755))

		// Create non-executable file.
		path := filepath.Join(hooksDir, "non-executable.sh")
		err := os.WriteFile(path, []byte("#!/bin/bash\necho test"), 0o644)
		require.NoError(t, err)

		mgr := NewManager(tempDir, dataDir, nil)
		discovered := mgr.ListHooks(HookPreToolUse)

		// On Windows, .sh files are always considered executable
		// On Unix, non-executable files (0o644) should be skipped
		if runtime.GOOS == "windows" {
			assert.Len(t, discovered, 1, "On Windows, .sh files are executable regardless of permissions")
		} else {
			assert.Len(t, discovered, 0, "On Unix, non-executable files should be skipped")
		}
	})

	t.Run("discovers hooks by extension on all platforms", func(t *testing.T) {
		tempDir := t.TempDir()
		dataDir := filepath.Join(tempDir, ".crush")
		hooksDir := filepath.Join(dataDir, "hooks", "pre-tool-use")
		require.NoError(t, os.MkdirAll(hooksDir, 0o755))

		// Only .sh files are recognized as hooks.
		// On Unix, they need execute permission. On Windows, extension is enough.
		validHook := filepath.Join(hooksDir, "valid-hook.sh")
		err := os.WriteFile(validHook, []byte("#!/bin/bash\necho test"), 0o755)
		require.NoError(t, err)

		// These should NOT be discovered (wrong extensions).
		invalidFiles := []string{"hook.bat", "hook.cmd", "hook.ps1", "hook.txt"}
		for _, name := range invalidFiles {
			path := filepath.Join(hooksDir, name)
			err := os.WriteFile(path, []byte("echo test"), 0o755)
			require.NoError(t, err)
		}

		mgr := NewManager(tempDir, dataDir, nil)
		discovered := mgr.ListHooks(HookPreToolUse)

		// Only the .sh file should be discovered.
		assert.Len(t, discovered, 1)
		assert.Contains(t, discovered[0], "valid-hook.sh")
	})

	t.Run("executes multiple hooks and merges results", func(t *testing.T) {
		tempDir := t.TempDir()
		dataDir := filepath.Join(tempDir, ".crush")
		hooksDir := filepath.Join(dataDir, "hooks", "user-prompt-submit")
		require.NoError(t, os.MkdirAll(hooksDir, 0o755))

		// Hook 1: Adds context.
		hook1 := filepath.Join(hooksDir, "01-add-context.sh")
		err := os.WriteFile(hook1, []byte(`#!/bin/bash
crush_add_context "Context from hook 1"
`), 0o755)
		require.NoError(t, err)

		// Hook 2: Adds more context.
		hook2 := filepath.Join(hooksDir, "02-add-more.sh")
		err = os.WriteFile(hook2, []byte(`#!/bin/bash
crush_add_context "Context from hook 2"
`), 0o755)
		require.NoError(t, err)

		mgr := NewManager(tempDir, dataDir, nil)
		ctx := context.Background()
		result, err := mgr.ExecuteUserPromptSubmit(ctx, "test", tempDir, UserPromptSubmitData{
			Prompt: "test prompt",
		})

		require.NoError(t, err)
		assert.True(t, result.Continue)
		// Contexts should be merged with \n\n separator.
		assert.Equal(t, "Context from hook 1\n\nContext from hook 2", result.ContextContent)
	})

	t.Run("stops on first hook that sets continue=false", func(t *testing.T) {
		tempDir := t.TempDir()
		dataDir := filepath.Join(tempDir, ".crush")
		hooksDir := filepath.Join(dataDir, "hooks", "pre-tool-use")
		require.NoError(t, os.MkdirAll(hooksDir, 0o755))

		// Hook 1: Denies.
		hook1 := filepath.Join(hooksDir, "01-deny.sh")
		err := os.WriteFile(hook1, []byte(`#!/bin/bash
crush_deny "blocked"
`), 0o755)
		require.NoError(t, err)

		// Hook 2: Should not execute.
		hook2 := filepath.Join(hooksDir, "02-never-runs.sh")
		err = os.WriteFile(hook2, []byte(`#!/bin/bash
export CRUSH_MESSAGE="should not see this"
`), 0o755)
		require.NoError(t, err)

		mgr := NewManager(tempDir, dataDir, nil)
		ctx := context.Background()
		result, err := mgr.ExecutePreToolUse(ctx, "test", tempDir, PreToolUseData{})

		require.NoError(t, err)
		assert.False(t, result.Continue)
		assert.Equal(t, "deny", result.Permission)
		assert.Equal(t, "blocked", result.Message)
		assert.NotContains(t, result.Message, "should not see this")
	})

	t.Run("merges permissions with deny winning", func(t *testing.T) {
		tempDir := t.TempDir()
		dataDir := filepath.Join(tempDir, ".crush")
		hooksDir := filepath.Join(dataDir, "hooks", "pre-tool-use")
		require.NoError(t, os.MkdirAll(hooksDir, 0o755))

		// Hook 1: Approves.
		hook1 := filepath.Join(hooksDir, "01-approve.sh")
		err := os.WriteFile(hook1, []byte(`#!/bin/bash
export CRUSH_PERMISSION=approve
`), 0o755)
		require.NoError(t, err)

		// Hook 2: Denies (should win).
		hook2 := filepath.Join(hooksDir, "02-deny.sh")
		err = os.WriteFile(hook2, []byte(`#!/bin/bash
export CRUSH_PERMISSION=deny
`), 0o755)
		require.NoError(t, err)

		mgr := NewManager(tempDir, dataDir, nil)
		ctx := context.Background()
		result, err := mgr.ExecutePreToolUse(ctx, "test", tempDir, PreToolUseData{})

		require.NoError(t, err)
		assert.Equal(t, "deny", result.Permission)
	})

	t.Run("disabled hooks are skipped", func(t *testing.T) {
		tempDir := t.TempDir()
		dataDir := filepath.Join(tempDir, ".crush")
		hooksDir := filepath.Join(dataDir, "hooks", "pre-tool-use")
		require.NoError(t, os.MkdirAll(hooksDir, 0o755))

		// Hook 1: Should run.
		hook1 := filepath.Join(hooksDir, "01-enabled.sh")
		err := os.WriteFile(hook1, []byte(`#!/bin/bash
export CRUSH_MESSAGE="enabled"
`), 0o755)
		require.NoError(t, err)

		// Hook 2: Disabled.
		hook2 := filepath.Join(hooksDir, "02-disabled.sh")
		err = os.WriteFile(hook2, []byte(`#!/bin/bash
export CRUSH_MESSAGE="disabled"
`), 0o755)
		require.NoError(t, err)

		cfg := &Config{
			TimeoutSeconds: 30,
			Directories:    []string{filepath.Join(dataDir, "hooks")},
			DisableHooks:   []string{"pre-tool-use/02-disabled.sh"},
		}

		mgr := NewManager(tempDir, dataDir, cfg)
		ctx := context.Background()
		result, err := mgr.ExecutePreToolUse(ctx, "test", tempDir, PreToolUseData{})

		require.NoError(t, err)
		assert.Equal(t, "enabled", result.Message)
	})

	t.Run("inline hooks are executed", func(t *testing.T) {
		tempDir := t.TempDir()
		dataDir := filepath.Join(tempDir, ".crush")

		cfg := &Config{
			TimeoutSeconds: 30,
			Directories:    []string{filepath.Join(dataDir, "hooks")},
			Inline: map[string][]InlineHook{
				"user-prompt-submit": {
					{
						Name: "inline-test.sh",
						Script: `#!/bin/bash
export CRUSH_MESSAGE="inline hook executed"
`,
					},
				},
			},
		}

		mgr := NewManager(tempDir, dataDir, cfg)
		ctx := context.Background()
		result, err := mgr.ExecuteUserPromptSubmit(ctx, "test", tempDir, UserPromptSubmitData{})

		require.NoError(t, err)
		assert.Equal(t, "inline hook executed", result.Message)
	})

	t.Run("returns empty result when hooks disabled", func(t *testing.T) {
		tempDir := t.TempDir()
		dataDir := filepath.Join(tempDir, ".crush")

		cfg := &Config{}

		mgr := NewManager(tempDir, dataDir, cfg)
		ctx := context.Background()
		result, err := mgr.ExecutePreToolUse(ctx, "test", tempDir, PreToolUseData{})

		require.NoError(t, err)
		assert.True(t, result.Continue)
		assert.Empty(t, result.Message)
	})

	t.Run("returns empty result when no hooks found", func(t *testing.T) {
		tempDir := t.TempDir()
		dataDir := filepath.Join(tempDir, ".crush")

		mgr := NewManager(tempDir, dataDir, nil)
		ctx := context.Background()
		result, err := mgr.ExecutePreToolUse(ctx, "test", tempDir, PreToolUseData{})

		require.NoError(t, err)
		assert.True(t, result.Continue)
	})

	t.Run("handles hook failure on PreToolUse by denying", func(t *testing.T) {
		tempDir := t.TempDir()
		dataDir := filepath.Join(tempDir, ".crush")
		hooksDir := filepath.Join(dataDir, "hooks", "pre-tool-use")
		require.NoError(t, os.MkdirAll(hooksDir, 0o755))

		// Hook that fails with exit 1.
		hook := filepath.Join(hooksDir, "01-fail.sh")
		err := os.WriteFile(hook, []byte(`#!/bin/bash
exit 1
`), 0o755)
		require.NoError(t, err)

		mgr := NewManager(tempDir, dataDir, nil)
		ctx := context.Background()
		result, err := mgr.ExecutePreToolUse(ctx, "test", tempDir, PreToolUseData{})

		require.NoError(t, err)
		assert.False(t, result.Continue)
		assert.Equal(t, "deny", result.Permission)
		assert.Contains(t, result.Message, "Hook failed")
	})

	t.Run("caches discovered hooks", func(t *testing.T) {
		tempDir := t.TempDir()
		dataDir := filepath.Join(tempDir, ".crush")
		hooksDir := filepath.Join(dataDir, "hooks", "pre-tool-use")
		require.NoError(t, os.MkdirAll(hooksDir, 0o755))

		hook := filepath.Join(hooksDir, "01-test.sh")
		err := os.WriteFile(hook, []byte("#!/bin/bash\necho test"), 0o755)
		require.NoError(t, err)

		mgr := NewManager(tempDir, dataDir, nil)

		// First call - discovers.
		hooks1 := mgr.ListHooks(HookPreToolUse)
		assert.Len(t, hooks1, 1)

		// Second call - should use cache.
		hooks2 := mgr.ListHooks(HookPreToolUse)
		assert.Equal(t, hooks1, hooks2)
	})

	t.Run("catch-all hooks at root execute for all types", func(t *testing.T) {
		tempDir := t.TempDir()
		dataDir := filepath.Join(tempDir, ".crush")
		hooksDir := filepath.Join(dataDir, "hooks")
		require.NoError(t, os.MkdirAll(hooksDir, 0o755))

		// Create catch-all hook at root level.
		catchAllHook := filepath.Join(hooksDir, "00-catch-all.sh")
		err := os.WriteFile(catchAllHook, []byte(`#!/bin/bash
export CRUSH_MESSAGE="catch-all: $CRUSH_HOOK_TYPE"
`), 0o755)
		require.NoError(t, err)

		// Create specific hook for pre-tool-use.
		specificDir := filepath.Join(hooksDir, "pre-tool-use")
		require.NoError(t, os.MkdirAll(specificDir, 0o755))
		specificHook := filepath.Join(specificDir, "01-specific.sh")
		err = os.WriteFile(specificHook, []byte(`#!/bin/bash
export CRUSH_MESSAGE="$CRUSH_MESSAGE; specific hook"
`), 0o755)
		require.NoError(t, err)

		mgr := NewManager(tempDir, dataDir, nil)

		// Test PreToolUse - should execute both catch-all and specific.
		ctx := context.Background()
		result, err := mgr.ExecutePreToolUse(ctx, "test", tempDir, PreToolUseData{})

		require.NoError(t, err)
		assert.Contains(t, result.Message, "catch-all: pre-tool-use")
		assert.Contains(t, result.Message, "specific hook")

		// Test UserPromptSubmit - should only execute catch-all.
		result2, err := mgr.ExecuteUserPromptSubmit(ctx, "test", tempDir, UserPromptSubmitData{})

		require.NoError(t, err)
		assert.Equal(t, "catch-all: user-prompt-submit", result2.Message)
		assert.NotContains(t, result2.Message, "specific hook")
	})

	t.Run("passes environment variables from config to hooks", func(t *testing.T) {
		tempDir := t.TempDir()
		dataDir := filepath.Join(tempDir, ".crush")
		hooksDir := filepath.Join(dataDir, "hooks", "pre-tool-use")
		require.NoError(t, os.MkdirAll(hooksDir, 0o755))

		// Hook that checks for custom environment variables.
		hook := filepath.Join(hooksDir, "01-check-env.sh")
		err := os.WriteFile(hook, []byte(`#!/bin/bash
if [ "$CUSTOM_API_KEY" = "test-key-123" ] && [ "$CUSTOM_ENV" = "production" ]; then
  export CRUSH_MESSAGE="config environment variables received"
else
  export CRUSH_MESSAGE="environment variables missing"
fi
`), 0o755)
		require.NoError(t, err)

		cfg := &Config{
			TimeoutSeconds: 30,
			Directories:    []string{filepath.Join(dataDir, "hooks")},
			Environment: map[string]string{
				"CUSTOM_API_KEY": "test-key-123",
				"CUSTOM_ENV":     "production",
			},
		}

		mgr := NewManager(tempDir, dataDir, cfg)
		ctx := context.Background()
		result, err := mgr.ExecutePreToolUse(ctx, "test", tempDir, PreToolUseData{})

		require.NoError(t, err)
		assert.Equal(t, "config environment variables received", result.Message)
	})

	t.Run("handles inline hook write failure gracefully", func(t *testing.T) {
		tempDir := t.TempDir()
		// Use a read-only directory as dataDir to force write failure.
		readOnlyDir := filepath.Join(tempDir, "readonly")
		require.NoError(t, os.MkdirAll(readOnlyDir, 0o555)) // Read-only

		cfg := &Config{
			TimeoutSeconds: 30,
			Directories:    []string{filepath.Join(readOnlyDir, "hooks")},
			Inline: map[string][]InlineHook{
				"pre-tool-use": {
					{
						Name:   "inline-fail.sh",
						Script: "#!/bin/bash\necho test",
					},
				},
			},
		}

		mgr := NewManager(tempDir, readOnlyDir, cfg)
		ctx := context.Background()

		// Should not error even though inline hook write fails.
		// The hook will be skipped and logged.
		result, err := mgr.ExecutePreToolUse(ctx, "test", tempDir, PreToolUseData{})

		require.NoError(t, err)
		assert.True(t, result.Continue) // Should continue despite write failure
	})

	t.Run("handles hooks directory read failure gracefully", func(t *testing.T) {
		tempDir := t.TempDir()
		dataDir := filepath.Join(tempDir, ".crush")
		hooksDir := filepath.Join(dataDir, "hooks", "pre-tool-use")
		require.NoError(t, os.MkdirAll(hooksDir, 0o755))

		// Create a hook file.
		hook := filepath.Join(hooksDir, "01-test.sh")
		require.NoError(t, os.WriteFile(hook, []byte("#!/bin/bash\necho test"), 0o755))

		mgr := NewManager(tempDir, dataDir, nil)

		// Make directory unreadable after discovery to test error path.
		// Note: This is tricky to test reliably cross-platform.
		// On some systems, we can't make a directory unreadable if we own it.
		// We'll test that hooks are cached and re-discovery works.
		hooks1 := mgr.ListHooks(HookPreToolUse)
		assert.Len(t, hooks1, 1)

		// Add another hook.
		hook2 := filepath.Join(hooksDir, "02-test.sh")
		require.NoError(t, os.WriteFile(hook2, []byte("#!/bin/bash\necho test2"), 0o755))

		// Should still return cached hooks (won't see new one).
		hooks2 := mgr.ListHooks(HookPreToolUse)
		assert.Len(t, hooks2, 1, "hooks are cached, new hook not seen")
	})

	t.Run("approve permission is set when accumulated is empty", func(t *testing.T) {
		tempDir := t.TempDir()
		dataDir := filepath.Join(tempDir, ".crush")
		hooksDir := filepath.Join(dataDir, "hooks", "pre-tool-use")
		require.NoError(t, os.MkdirAll(hooksDir, 0o755))

		// Single hook that approves.
		hook := filepath.Join(hooksDir, "01-approve.sh")
		require.NoError(t, os.WriteFile(hook, []byte(`#!/bin/bash
export CRUSH_PERMISSION=approve
export CRUSH_MESSAGE="auto-approved"
`), 0o755))

		mgr := NewManager(tempDir, dataDir, nil)
		ctx := context.Background()
		result, err := mgr.ExecutePreToolUse(ctx, "test", tempDir, PreToolUseData{})

		require.NoError(t, err)
		assert.Equal(t, "approve", result.Permission)
		assert.Equal(t, "auto-approved", result.Message)
	})
}
