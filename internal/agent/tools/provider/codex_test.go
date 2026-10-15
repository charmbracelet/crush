package provider

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"charm.land/fantasy"
	"github.com/stretchr/testify/require"
)

func TestBuildCodexArgs(t *testing.T) {
	t.Parallel()

	args := (codexStrategy{}).args("fix the login bug", "", "")
	require.Equal(t, []string{
		"exec",
		"--json",
		"-c", `sandbox_mode="danger-full-access"`,
		"--skip-git-repo-check",
		"--",
		"fix the login bug",
	}, args)

	// "exec resume" không có -s: sandbox phải đi qua -c (đã thử với
	// codex-cli 0.159.2), id và prompt là hai tham số vị trí sau "--".
	args = (codexStrategy{}).args("now add tests", "thread-9", "")
	require.Equal(t, []string{
		"exec",
		"resume",
		"--json",
		"-c", `sandbox_mode="danger-full-access"`,
		"--skip-git-repo-check",
		"--",
		"thread-9",
		"now add tests",
	}, args)
}

func TestParseCodexEvents(t *testing.T) {
	t.Parallel()

	stdout := strings.Join([]string{
		`{"type":"thread.started","thread_id":"thread-9"}`,
		`{"type":"item.started","item":{"type":"command_execution","command":"ls"}}`,
		`{"type":"item.completed","item":{"type":"command_execution","aggregated_output":"main.go"}}`,
		`{"type":"item.completed","item":{"type":"reasoning","text":"thinking"}}`,
		`{"type":"item.completed","item":{"type":"agent_message","text":"done, 2 files changed"}}`,
		`{"type":"turn.completed"}`,
	}, "\n")

	out, err := (codexStrategy{}).parse([]byte(stdout))
	require.NoError(t, err)
	require.Equal(t, "thread-9", out.sessionID)
	require.Equal(t, "done, 2 files changed", out.text)
}

func TestParseCodexEventsKeepsLastAgentMessage(t *testing.T) {
	t.Parallel()

	stdout := strings.Join([]string{
		`{"type":"thread.started","thread_id":"thread-9"}`,
		`{"type":"item.completed","item":{"type":"agent_message","text":"intermediate"}}`,
		`{"type":"item.completed","item":{"type":"agent_message","text":"final answer"}}`,
	}, "\n")

	out, err := (codexStrategy{}).parse([]byte(stdout))
	require.NoError(t, err)
	require.Equal(t, "final answer", out.text)
}

func TestParseCodexEventsWithoutMessage(t *testing.T) {
	t.Parallel()

	stdout := strings.Join([]string{
		`{"type":"thread.started","thread_id":"thread-9"}`,
		`{"type":"item.completed","item":{"type":"reasoning","text":"thinking"}}`,
	}, "\n")

	_, err := (codexStrategy{}).parse([]byte(stdout))
	require.Error(t, err)
	require.Contains(t, err.Error(), "no agent message")

	_, err = (codexStrategy{}).parse([]byte("not json"))
	require.Error(t, err)
}

func TestParseCodexEventsReportsCLIError(t *testing.T) {
	t.Parallel()

	// A structured error event must come back as a CLI-reported failure,
	// and it wins over whatever partial output preceded it.
	stdout := strings.Join([]string{
		`{"type":"thread.started","thread_id":"thread-9"}`,
		`{"type":"error","message":"stream disconnected"}`,
	}, "\n")

	_, err := (codexStrategy{}).parse([]byte(stdout))
	require.Error(t, err)
	require.True(t, isCLIReported(err))
	require.Equal(t, "stream disconnected", err.Error())

	stdout = `{"type":"turn.failed","error":{"message":"You have hit your usage limit."}}`
	_, err = (codexStrategy{}).parse([]byte(stdout))
	require.Error(t, err)
	require.True(t, isCLIReported(err))
	require.Equal(t, "You have hit your usage limit.", err.Error())

	// No error events: not CLI-reported.
	stdout = `{"type":"turn.completed"}`
	_, err = (codexStrategy{}).parse([]byte(stdout))
	require.Error(t, err)
	require.False(t, isCLIReported(err))
}

func TestCodexToolRunsCLI(t *testing.T) {
	// Not parallel: swapping the package-level codexCommand would race
	// with the other execution tests.
	dir := t.TempDir()
	script := filepath.Join(dir, "fake-codex")
	// The fake CLI records its working directory and args, then prints a
	// canned Codex JSONL event stream.
	require.NoError(t, os.WriteFile(script, []byte(`#!/usr/bin/env bash
pwd > "`+dir+`/observed-cwd.txt"
printf '%s\n' "$*" > "`+dir+`/observed-args.txt"
cat <<'JSONL'
{"type":"thread.started","thread_id":"thread-9"}
{"type":"item.completed","item":{"type":"agent_message","text":"wrote the file"}}
JSONL
`), 0o755))

	t.Cleanup(func() { codexCommand = "codex" })
	codexCommand = script

	tool := NewCodexTool(dir, &fakePermissions{allow: true}, testCodexModel)
	resp, err := tool.Run(sessionCtx(t), fantasy.ToolCall{
		ID:    "call-1",
		Name:  CodexToolName,
		Input: `{"prompt":"write hello.txt"}`,
	})
	require.NoError(t, err)
	require.Contains(t, resp.Content, "wrote the file")
	require.Contains(t, resp.Content, "thread-9")

	observedCWD, err := os.ReadFile(filepath.Join(dir, "observed-cwd.txt"))
	require.NoError(t, err)
	require.Equal(t, dir, strings.TrimSpace(string(observedCWD)))

	observedArgs, err := os.ReadFile(filepath.Join(dir, "observed-args.txt"))
	require.NoError(t, err)
	require.Contains(t, string(observedArgs), "exec --json")
	require.Contains(t, string(observedArgs), "danger-full-access")
	require.Contains(t, string(observedArgs), "write hello.txt")
}

func TestCodexToolMissingBinary(t *testing.T) {
	// Not parallel: swapping the package-level codexCommand would race
	// with the other execution tests.

	t.Cleanup(func() { codexCommand = "codex" })
	codexCommand = "codex-does-not-exist-anywhere"

	tool := NewCodexTool(t.TempDir(), &fakePermissions{allow: true}, testCodexModel)
	_, err := tool.Run(sessionCtx(t), fantasy.ToolCall{
		ID:    "call-2",
		Name:  CodexToolName,
		Input: `{"prompt":"noop"}`,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found in PATH")
}

func TestCodexToolSurfacesErrorEvent(t *testing.T) {
	// Not parallel: swapping the package-level codexCommand would race
	// with the other execution tests. Codex reports failures in the event
	// stream even when it exits nonzero, so the structured event must win
	// over the generic exec failure.
	dir := t.TempDir()
	script := filepath.Join(dir, "fake-codex")
	require.NoError(t, os.WriteFile(script, []byte(`#!/usr/bin/env bash
cat <<'JSONL'
{"type":"thread.started","thread_id":"thread-9"}
{"type":"error","message":"You have hit your usage limit. Try again at 1:55 PM."}
{"type":"turn.failed","error":{"message":"You have hit your usage limit. Try again at 1:55 PM."}}
JSONL
exit 1
`), 0o755))

	t.Cleanup(func() { codexCommand = "codex" })
	codexCommand = script

	tool := NewCodexTool(dir, &fakePermissions{allow: true}, testCodexModel)
	_, err := tool.Run(sessionCtx(t), fantasy.ToolCall{
		ID:    "call-4",
		Name:  CodexToolName,
		Input: `{"prompt":"noop"}`,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "usage limit")
}
