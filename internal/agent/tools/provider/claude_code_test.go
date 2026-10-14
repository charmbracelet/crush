package provider

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"charm.land/fantasy"
	"github.com/stretchr/testify/require"
)

func TestBuildClaudeCodeArgs(t *testing.T) {
	t.Parallel()

	args := (claudeStrategy{}).args("fix the login bug", "")
	require.Equal(t, []string{
		"-p", "fix the login bug",
		"--output-format", "json",
		"--dangerously-skip-permissions",
	}, args)

	args = (claudeStrategy{}).args("now add tests", "sess-7")
	require.Contains(t, args, "--resume")
	require.Equal(t, "sess-7", args[len(args)-1])
}

func TestParseClaudeCodeResult(t *testing.T) {
	t.Parallel()

	out, err := (claudeStrategy{}).parse([]byte(`{"result":"done, 2 files changed","session_id":"sess-7","is_error":false}`))
	require.NoError(t, err)
	require.Equal(t, "done, 2 files changed", out.text)
	require.Equal(t, "sess-7", out.sessionID)

	_, err = (claudeStrategy{}).parse([]byte(`{"result":"could not compile","is_error":true}`))
	require.Error(t, err)
	require.True(t, isCLIReported(err))
	require.Contains(t, err.Error(), "could not compile")

	_, err = (claudeStrategy{}).parse([]byte("not json at all"))
	require.Error(t, err)
	require.False(t, isCLIReported(err))
}

func TestClaudeCodeToolRunsCLI(t *testing.T) {
	// Not parallel: swapping the package-level claudeCommand would race
	// with the other execution tests.
	dir := t.TempDir()
	script := filepath.Join(dir, "fake-claude")
	// The fake CLI records its working directory and args, then prints a
	// canned Claude Code result payload.
	require.NoError(t, os.WriteFile(script, []byte(`#!/usr/bin/env bash
pwd > "`+dir+`/observed-cwd.txt"
printf '%s\n' "$*" > "`+dir+`/observed-args.txt"
cat <<'JSON'
{"result":"wrote the file","session_id":"sess-7","is_error":false}
JSON
`), 0o755))

	t.Cleanup(func() { claudeCommand = "claude" })
	claudeCommand = script

	tool := NewClaudeCodeTool(dir)
	resp, err := tool.Run(context.Background(), fantasy.ToolCall{
		ID:    "call-1",
		Name:  ClaudeCodeToolName,
		Input: `{"prompt":"write hello.txt"}`,
	})
	require.NoError(t, err)
	require.Contains(t, resp.Content, "wrote the file")
	require.Contains(t, resp.Content, "sess-7")

	observedCWD, err := os.ReadFile(filepath.Join(dir, "observed-cwd.txt"))
	require.NoError(t, err)
	require.Equal(t, dir, strings.TrimSpace(string(observedCWD)))

	observedArgs, err := os.ReadFile(filepath.Join(dir, "observed-args.txt"))
	require.NoError(t, err)
	require.Contains(t, string(observedArgs), "--dangerously-skip-permissions")
	require.Contains(t, string(observedArgs), "write hello.txt")
}

func TestClaudeCodeToolMissingBinary(t *testing.T) {
	// Not parallel: swapping the package-level claudeCommand would race
	// with the other execution tests.

	t.Cleanup(func() { claudeCommand = "claude" })
	claudeCommand = "claude-does-not-exist-anywhere"

	tool := NewClaudeCodeTool(t.TempDir())
	_, err := tool.Run(context.Background(), fantasy.ToolCall{
		ID:    "call-2",
		Name:  ClaudeCodeToolName,
		Input: `{"prompt":"noop"}`,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found in PATH")
}

func TestClaudeCodeToolReportsIsError(t *testing.T) {
	// Not parallel: swapping the package-level claudeCommand would race
	// with the other execution tests.

	dir := t.TempDir()
	script := filepath.Join(dir, "fake-claude")
	require.NoError(t, os.WriteFile(script, []byte(`#!/usr/bin/env bash
cat <<'JSON'
{"result":"build failed and I could not fix it","is_error":true}
JSON
`), 0o755))

	t.Cleanup(func() { claudeCommand = "claude" })
	claudeCommand = script

	tool := NewClaudeCodeTool(dir)
	_, err := tool.Run(context.Background(), fantasy.ToolCall{
		ID:    "call-3",
		Name:  ClaudeCodeToolName,
		Input: `{"prompt":"fix build"}`,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "build failed")
}

func TestClaudeCodeToolStructuredResultWinsOverExitCode(t *testing.T) {
	// Not parallel: swapping the package-level claudeCommand would race
	// with the other execution tests. Claude exits nonzero on some
	// failures while still printing the result JSON, so the structured
	// payload must win over the generic exec error — both for success
	// and for reported failures.
	dir := t.TempDir()

	successScript := filepath.Join(dir, "claude-success")
	require.NoError(t, os.WriteFile(successScript, []byte(`#!/usr/bin/env bash
cat <<'JSON'
{"result":"wrote the file","session_id":"sess-7","is_error":false}
JSON
exit 1
`), 0o755))
	t.Cleanup(func() { claudeCommand = "claude" })
	claudeCommand = successScript

	tool := NewClaudeCodeTool(dir)
	resp, err := tool.Run(context.Background(), fantasy.ToolCall{
		ID:    "call-5",
		Name:  ClaudeCodeToolName,
		Input: `{"prompt":"write hello.txt"}`,
	})
	require.NoError(t, err)
	require.Contains(t, resp.Content, "wrote the file")

	failureScript := filepath.Join(dir, "claude-failure")
	require.NoError(t, os.WriteFile(failureScript, []byte(`#!/usr/bin/env bash
cat <<'JSON'
{"result":"You have hit your usage limit. Try again at 1:55 PM.","is_error":true}
JSON
exit 1
`), 0o755))
	claudeCommand = failureScript

	_, err = tool.Run(context.Background(), fantasy.ToolCall{
		ID:    "call-6",
		Name:  ClaudeCodeToolName,
		Input: `{"prompt":"noop"}`,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "usage limit")
}

func TestDelegationRequiresPrompt(t *testing.T) {
	t.Parallel()

	// The shared runner rejects an empty prompt identically for every
	// provider strategy.
	for _, tool := range []fantasy.AgentTool{
		NewClaudeCodeTool(t.TempDir()),
		NewCodexTool(t.TempDir()),
	} {
		_, err := tool.Run(context.Background(), fantasy.ToolCall{
			ID:    "call-empty",
			Name:  "any",
			Input: `{"prompt":"   "}`,
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "prompt is required")
	}
}
