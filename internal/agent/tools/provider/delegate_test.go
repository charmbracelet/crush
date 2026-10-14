package provider

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/stretchr/testify/require"
)

// TestMain gỡ biến cấu hình thật của người chạy test (vd AGENT_CLI_CODEX_BIN
// trong ~/.bashrc): biến env thắng CLI giả mà test tiêm vào, nên nếu còn thì
// test sẽ chạy CLI THẬT với toàn quyền.
func TestMain(m *testing.M) {
	for _, k := range []string{claudeBinEnv, claudeModeEnv, codexBinEnv, codexSandboxEnv} {
		os.Unsetenv(k)
	}
	os.Exit(m.Run())
}

// fakePermissions ghi lại mọi lời xin quyền và trả lời theo allow.
type fakePermissions struct {
	allow    bool
	requests []permission.CreatePermissionRequest
}

func (f *fakePermissions) Request(_ context.Context, req permission.CreatePermissionRequest) (bool, error) {
	f.requests = append(f.requests, req)
	return f.allow, nil
}

// sessionCtx gắn session ID như agent làm trước khi gọi tool.
func sessionCtx(t *testing.T) context.Context {
	return context.WithValue(t.Context(), tools.SessionIDContextKey, "sess-test")
}

// writeMarkerCLI tạo CLI giả ghi tệp đánh dấu khi được chạy, rồi in stdout.
func writeMarkerCLI(t *testing.T, dir, name, stdout string) (script, marker string) {
	t.Helper()
	script = filepath.Join(dir, name)
	marker = filepath.Join(dir, name+".ran")
	require.NoError(t, os.WriteFile(script, []byte("#!/usr/bin/env bash\ntouch '"+marker+"'\ncat <<'OUT'\n"+stdout+"\nOUT\n"), 0o755))
	return script, marker
}

func TestDelegationDeniedDoesNotRunCLI(t *testing.T) {
	// Không parallel: t.Setenv.
	cases := []struct {
		toolName string
		binEnv   string
		stdout   string
		access   string
		build    func(dir string, perms permissionRequester) fantasy.AgentTool
	}{
		{ClaudeCodeToolName, claudeBinEnv, `{"result":"done","session_id":"s1","is_error":false}`, "--dangerously-skip-permissions", NewClaudeCodeTool},
		{CodexToolName, codexBinEnv, `{"type":"item.completed","item":{"type":"agent_message","text":"done"}}`, "danger-full-access", NewCodexTool},
	}
	for _, tc := range cases {
		t.Run(tc.toolName, func(t *testing.T) {
			dir := t.TempDir()
			script, marker := writeMarkerCLI(t, dir, "fake-"+tc.toolName, tc.stdout)
			t.Setenv(tc.binEnv, script)
			perms := &fakePermissions{allow: false}

			resp, err := tc.build(dir, perms).Run(sessionCtx(t), fantasy.ToolCall{
				ID:    "call-deny",
				Name:  tc.toolName,
				Input: `{"prompt":"delete everything"}`,
			})
			require.NoError(t, err)
			require.True(t, resp.IsError)
			require.Contains(t, resp.Content, "User denied permission")
			require.NoFileExists(t, marker, "denied delegation must not start the CLI")

			require.Len(t, perms.requests, 1, "every delegation must ask for permission")
			req := perms.requests[0]
			require.Equal(t, "sess-test", req.SessionID)
			require.Equal(t, "call-deny", req.ToolCallID)
			require.Equal(t, tc.toolName, req.ToolName)
			require.Equal(t, "execute", req.Action)
			require.Equal(t, dir, req.Path)
			require.Contains(t, req.Description, "delete everything")
			require.Contains(t, req.Description, tc.access, "the prompt must say how much power the CLI gets")
		})
	}
}

func TestDelegationGrantedRunsCLI(t *testing.T) {
	// Không parallel: t.Setenv.
	dir := t.TempDir()
	script, marker := writeMarkerCLI(t, dir, "fake-claude", `{"result":"done","session_id":"s1","is_error":false}`)
	t.Setenv(claudeBinEnv, script)
	perms := &fakePermissions{allow: true}

	resp, err := NewClaudeCodeTool(dir, perms).Run(sessionCtx(t), fantasy.ToolCall{
		ID:    "call-ok",
		Name:  ClaudeCodeToolName,
		Input: `{"prompt":"write hello.txt","resume":"s0"}`,
	})
	require.NoError(t, err)
	require.False(t, resp.IsError)
	require.Contains(t, resp.Content, "done")
	require.FileExists(t, marker)
	require.Len(t, perms.requests, 1)
	require.Contains(t, perms.requests[0].Description, "s0", "resuming must be visible in the prompt")
}

func TestDelegationRequiresSession(t *testing.T) {
	// Không parallel: t.Setenv.
	dir := t.TempDir()
	script, marker := writeMarkerCLI(t, dir, "fake-claude", `{"result":"done","is_error":false}`)
	t.Setenv(claudeBinEnv, script)
	perms := &fakePermissions{allow: true}

	_, err := NewClaudeCodeTool(dir, perms).Run(context.Background(), fantasy.ToolCall{
		ID:    "call-nosess",
		Name:  ClaudeCodeToolName,
		Input: `{"prompt":"write hello.txt"}`,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "session ID is required")
	require.Empty(t, perms.requests)
	require.NoFileExists(t, marker)
}

func TestDelegationPromptStartingWithDashStaysPositional(t *testing.T) {
	t.Parallel()

	// Cả hai CLI đều coi tham số bắt đầu bằng "-" là cờ (đã thử: claude báo
	// "unknown option", codex báo "unexpected argument"); "--" chặn điều đó.
	prompt := "- fix the login bug\n- add tests"
	for _, s := range []strategy{claudeStrategy{}, codexStrategy{}} {
		for _, resume := range []string{"", "id-1"} {
			args := s.args(prompt, resume)
			require.Equal(t, prompt, args[len(args)-1], "%s: prompt must be the last argument", s.name())
			sep := len(args) - 2
			if resume != "" && s.name() == CodexToolName {
				// codex exec resume [OPTIONS] -- <SESSION_ID> <PROMPT>
				require.Equal(t, resume, args[sep])
				sep--
			}
			require.Equal(t, "--", args[sep], "%s: options must end before the positionals", s.name())
		}
	}
}

func TestDelegationUsesConfiguredAccessMode(t *testing.T) {
	// Không parallel: t.Setenv.
	t.Setenv(claudeModeEnv, "acceptEdits")
	t.Setenv(codexSandboxEnv, "workspace-write")

	claudeArgs := newClaudeStrategy().args("task", "")
	require.Contains(t, claudeArgs, "acceptEdits")
	require.NotContains(t, claudeArgs, "--dangerously-skip-permissions")

	codexArgs := newCodexStrategy().args("task", "thread-1")
	require.Contains(t, codexArgs, `sandbox_mode="workspace-write"`)
	require.NotContains(t, codexArgs, "-s", "exec resume rejects -s")
}

func TestNewToolsRegistersOnlyInstalledCLIs(t *testing.T) {
	// Không parallel: t.Setenv.
	bin := t.TempDir()
	t.Setenv("PATH", bin)
	t.Setenv(claudeBinEnv, "")
	t.Setenv(codexBinEnv, "")

	all := []string{ClaudeCodeToolName, CodexToolName}
	got, err := NewTools(t.TempDir(), &fakePermissions{}, all)
	require.NoError(t, err)
	require.Empty(t, got, "no CLI installed: no delegation tool, no token cost")

	require.NoError(t, os.WriteFile(filepath.Join(bin, "claude"), []byte("#!/bin/sh\n"), 0o755))
	got, err = NewTools(t.TempDir(), &fakePermissions{}, all)
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, ClaudeCodeToolName, got[0].Info().Name)

	// Tool bị tắt (permissions deny) thì không đăng ký dù có CLI.
	got, err = NewTools(t.TempDir(), &fakePermissions{}, []string{CodexToolName})
	require.NoError(t, err)
	require.Empty(t, got)
}

func TestNewToolsRejectsBrokenExplicitBinary(t *testing.T) {
	// Không parallel: t.Setenv.
	t.Setenv(codexBinEnv, filepath.Join(t.TempDir(), "no-such-codex"))

	_, err := NewTools(t.TempDir(), &fakePermissions{}, []string{ClaudeCodeToolName, CodexToolName})
	require.Error(t, err)
	require.Contains(t, err.Error(), codexBinEnv)

	// Tool đã tắt thì biến BIN sai không được làm hỏng cả agent.
	_, err = NewTools(t.TempDir(), &fakePermissions{}, []string{ClaudeCodeToolName})
	require.NoError(t, err)
}

// Đường dẫn BIN tương đối được chốt thành tuyệt đối lúc dựng tool: CLI con
// chạy với thư mục làm việc là dự án, không phải cwd của server lúc dựng.
func TestNewToolsResolvesRelativeBinary(t *testing.T) {
	// Không parallel: t.Setenv, t.Chdir.
	binDir, project := t.TempDir(), t.TempDir()
	_, marker := writeMarkerCLI(t, binDir, "my-codex", `{"type":"item.completed","item":{"type":"agent_message","text":"done"}}`)
	t.Chdir(binDir)
	t.Setenv(codexBinEnv, "./my-codex")

	got, err := NewTools(project, &fakePermissions{allow: true}, []string{CodexToolName})
	require.NoError(t, err)
	require.Len(t, got, 1)

	t.Chdir(project) // ./my-codex không còn đúng ở đây
	resp, err := got[0].Run(sessionCtx(t), fantasy.ToolCall{ID: "call-rel", Name: CodexToolName, Input: `{"prompt":"hi"}`})
	require.NoError(t, err)
	require.Contains(t, resp.Content, "done")
	require.FileExists(t, marker)
}

// resume do model đặt: giá trị giống cờ không được thành cờ của CLI con (vd
// "--dangerously-skip-permissions" đè mức quyền người dùng cấu hình).
func TestDelegationRejectsFlagLikeResume(t *testing.T) {
	// Không parallel: t.Setenv.
	dir := t.TempDir()
	script, marker := writeMarkerCLI(t, dir, "fake-claude", `{"result":"done","is_error":false}`)
	t.Setenv(claudeBinEnv, script)
	perms := &fakePermissions{allow: true}

	for _, resume := range []string{"--dangerously-skip-permissions", "-x", "id with space", "id\x1b[2J"} {
		input, err := json.Marshal(DelegationParams{Prompt: "hi", Resume: resume})
		require.NoError(t, err)
		_, err = NewClaudeCodeTool(dir, perms).Run(sessionCtx(t), fantasy.ToolCall{
			ID:    "call-resume",
			Name:  ClaudeCodeToolName,
			Input: string(input),
		})
		require.Error(t, err, "resume %q", resume)
		require.Contains(t, err.Error(), "resume must be a session ID", "resume %q", resume)
	}
	require.Empty(t, perms.requests, "an invalid resume must be rejected before asking")
	require.NoFileExists(t, marker)

	// Dạng một token: id hợp lệ cũng không thể tách thành cờ riêng.
	args := (claudeStrategy{mode: "plan"}).args("p", "sess-1")
	require.Contains(t, args, "--resume=sess-1")
	require.NotContains(t, args, "--resume")
}
