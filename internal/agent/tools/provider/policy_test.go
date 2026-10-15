package provider

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/execpolicy"
	"github.com/stretchr/testify/require"
)

// argv luôn mang model cụ thể: claude không tự chọn model mặc định (vd Fable),
// codex không theo model trong config của nó.
func TestDelegationArgsPinTheModel(t *testing.T) {
	t.Parallel()
	require.Contains(t, (claudeStrategy{}).args("x", "", "claude-sonnet-5-5"), "--model=claude-sonnet-5-5")
	require.NotContains(t, (claudeStrategy{}).args("x", "", "claude-sonnet-5-5"), "--fallback-model")
	codex := (codexStrategy{}).args("x", "thread-1", "gpt-6.1-sol")
	require.Contains(t, codex, `model="gpt-6.1-sol"`)
	require.Equal(t, "x", codex[len(codex)-1])
}

// Mặc định (không bật giao việc): CLI có trên máy cũng không thành tool.
func TestNewToolsDefaultPolicyRegistersNothing(t *testing.T) {
	// Không parallel: t.Setenv.
	bin := t.TempDir()
	for _, name := range []string{"claude", "codex"} {
		require.NoError(t, os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\n"), 0o755))
	}
	t.Setenv("PATH", bin)
	t.Setenv(claudeBinEnv, "")
	t.Setenv(codexBinEnv, "")
	all := []string{ClaudeCodeToolName, CodexToolName}

	got, err := NewTools(t.TempDir(), &fakePermissions{}, all, execpolicy.Policy{})
	require.NoError(t, err)
	require.Empty(t, got, "installed is not authorized")

	// Có danh sách model nhưng chưa bật giao việc: vẫn không có tool.
	listed, err := execpolicy.FromEnv(func(k string) string {
		if k == execpolicy.AllowedModelsEnv {
			return "claude_code:claude-opus-5-5 codex:gpt-6.1-sol"
		}
		return ""
	})
	require.NoError(t, err)
	got, err = NewTools(t.TempDir(), &fakePermissions{}, all, listed)
	require.NoError(t, err)
	require.Empty(t, got)

	got, err = NewTools(t.TempDir(), &fakePermissions{}, all, testPolicy)
	require.NoError(t, err)
	require.Len(t, got, 2)
	require.Contains(t, got[0].Info().Description, testClaudeModel, "the model learns which models it may pick")
}

// Model lạ (Fable, alias, bản cũ) bị từ chối trước khi hỏi quyền và trước khi
// CLI chạy; model được phép thì chạy với đúng model đó.
func TestDelegationRefusesUnlistedModel(t *testing.T) {
	// Không parallel: t.Setenv.
	dir := t.TempDir()
	script, marker := writeMarkerCLI(t, dir, "fake-claude", `{"result":"done","session_id":"s1","is_error":false}`)
	t.Setenv(claudeBinEnv, script)

	for _, model := range []string{"claude-fable-5-1", "opus", "claude-opus-4-1", "claude-opus-6-0"} {
		perms := &fakePermissions{allow: true}
		input, err := json.Marshal(DelegationParams{Prompt: "hi", Model: model})
		require.NoError(t, err)
		resp, err := NewClaudeCodeTool(dir, perms, "claude-opus-5-5", "claude-sonnet-5-5", "claude-haiku-5-5").Run(sessionCtx(t), fantasy.ToolCall{ID: "c", Name: ClaudeCodeToolName, Input: string(input)})
		require.NoError(t, err)
		require.Truef(t, resp.IsError, "model %q", model)
		require.Contains(t, resp.Content, execpolicy.CodeModelNotAllowed)
		require.Emptyf(t, perms.requests, "model %q must be refused before asking", model)
		require.NoFileExists(t, marker)
	}

	// Không có model nào được phép: mọi lần gọi bị từ chối (đóng an toàn).
	perms := &fakePermissions{allow: true}
	resp, err := NewClaudeCodeTool(dir, perms).Run(sessionCtx(t), fantasy.ToolCall{ID: "c", Name: ClaudeCodeToolName, Input: `{"prompt":"hi"}`})
	require.NoError(t, err)
	require.True(t, resp.IsError)
	require.Empty(t, perms.requests)

	// Được phép: chạy với đúng model, hộp thoại nói model nào.
	argsFile := filepath.Join(dir, "args.txt")
	recorder := filepath.Join(dir, "fake-claude-args")
	require.NoError(t, os.WriteFile(recorder, []byte("#!/usr/bin/env bash\nprintf '%s\\n' \"$@\" > '"+argsFile+"'\necho '{\"result\":\"done\",\"session_id\":\"s1\",\"is_error\":false}'\n"), 0o755))
	t.Setenv(claudeBinEnv, recorder)
	perms = &fakePermissions{allow: true}
	resp, err = NewClaudeCodeTool(dir, perms, "claude-opus-5-5", "claude-haiku-5-5").Run(sessionCtx(t), fantasy.ToolCall{ID: "c", Name: ClaudeCodeToolName, Input: `{"prompt":"hi","model":"claude-haiku-5-5"}`})
	require.NoError(t, err)
	require.False(t, resp.IsError, resp.Content)
	require.Len(t, perms.requests, 1)
	require.Contains(t, perms.requests[0].Description, "model claude-haiku-5-5")
	raw, err := os.ReadFile(argsFile)
	require.NoError(t, err)
	require.Contains(t, string(raw), "--model=claude-haiku-5-5\n")
}
