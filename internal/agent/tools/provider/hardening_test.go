package provider

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Mức quyền gõ sai bị từ chối lúc dựng tool, lỗi nêu tên biến; tool đã bị
// tắt (không nằm trong allowed) thì không bị kiểm.
func TestNewToolsRejectsInvalidAccessModes(t *testing.T) {
	// Không parallel: t.Setenv.
	dir := t.TempDir()
	claude, _ := writeMarkerCLI(t, dir, "claude", `{}`)
	codex, _ := writeMarkerCLI(t, dir, "codex", `{}`)
	t.Setenv(claudeBinEnv, claude)
	t.Setenv(codexBinEnv, codex)

	t.Setenv(claudeModeEnv, "acceptEdit")
	_, err := NewTools(dir, &fakePermissions{}, []string{ClaudeCodeToolName}, testPolicy)
	require.ErrorContains(t, err, claudeModeEnv)
	_, err = NewTools(dir, &fakePermissions{}, []string{CodexToolName}, testPolicy)
	require.NoError(t, err, "a disabled tool's settings are not checked")

	t.Setenv(claudeModeEnv, "acceptEdits")
	t.Setenv(codexSandboxEnv, "workspace")
	_, err = NewTools(dir, &fakePermissions{}, []string{ClaudeCodeToolName, CodexToolName}, testPolicy)
	require.ErrorContains(t, err, codexSandboxEnv)

	t.Setenv(codexSandboxEnv, "read-only")
	tools, err := NewTools(dir, &fakePermissions{}, []string{ClaudeCodeToolName, CodexToolName}, testPolicy)
	require.NoError(t, err)
	require.Len(t, tools, 2)
}

// Hộp thoại nói mức quyền bằng lời, cảnh báo về "cho phép cả session", và
// không để escape terminal hay ký tự đảo chiều trong prompt đi thẳng ra màn hình.
func TestPermissionDescriptionIsReadableAndSafe(t *testing.T) {
	t.Parallel()
	prompt := "fix it\n\x1b[2J\x1b[1;1Hlooks harmless\u202eexe.gnp\r"
	desc := permissionDescription(claudeStrategy{}, "/work", DelegationParams{Prompt: prompt})
	require.Contains(t, desc, "FULL ACCESS")
	require.Contains(t, desc, "--dangerously-skip-permissions")
	require.Contains(t, desc, "whole session")
	require.NotContains(t, desc, "\x1b")
	require.NotContains(t, desc, "\u202e")
	require.NotContains(t, desc, "\r")
	require.Contains(t, desc, "fix it\n", "newlines stay readable")

	desc = permissionDescription(codexStrategy{sandbox: "read-only"}, "/work", DelegationParams{Prompt: "x"})
	require.Contains(t, desc, "sandboxed: read-only")
}

// CLI con không thừa kế các biến gắn nó vào một phiên Claude Code khác;
// cấu hình của người dùng thì giữ.
func TestChildEnvDropsClaudeSessionLink(t *testing.T) {
	t.Parallel()
	env := childEnv([]string{
		"PATH=/usr/bin",
		"CLAUDECODE=1",
		"CLAUDE_CODE_ENTRYPOINT=cli",
		"CLAUDE_CODE_MESSAGING_SOCKET=/tmp/sock",
		"CLAUDE_CODE_OAUTH_TOKEN=keep-me",
		"CLAUDE_CODE_USE_BEDROCK=1",
		"ANTHROPIC_API_KEY=keep-me-too",
		"HERDR_ENV=1",
		"HERDR_PANE_ID=p1",
	})
	require.Equal(t, []string{
		"PATH=/usr/bin",
		"CLAUDE_CODE_OAUTH_TOKEN=keep-me",
		"CLAUDE_CODE_USE_BEDROCK=1",
		"ANTHROPIC_API_KEY=keep-me-too",
	}, env)
}

// Huỷ lượt giao việc dừng cả tiến trình cháu đang giữ pipe stdout: Run trả
// về ngay thay vì treo tới khi tiến trình cháu tự thoát.
func TestRunCommandStopsWholeProcessGroupOnCancel(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("process groups are Unix-only")
	}
	t.Parallel()
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "grandchild.pid")
	script := filepath.Join(dir, "cli")
	require.NoError(t, os.WriteFile(script, []byte("#!/bin/sh\nsleep 60 &\necho $! > '"+pidFile+"'\nsleep 60\n"), 0o755))

	ctx, cancel := context.WithCancel(t.Context())
	go func() {
		require.Eventually(t, func() bool { _, err := os.Stat(pidFile); return err == nil }, 5*time.Second, 10*time.Millisecond)
		cancel()
	}()

	start := time.Now()
	_, err := runCommand(ctx, dir, script)
	require.Error(t, err)
	require.Less(t, time.Since(start), 3*time.Second, "Run must not wait for the grandchild holding stdout")

	raw, readErr := os.ReadFile(pidFile)
	require.NoError(t, readErr)
	pid, convErr := strconv.Atoi(strings.TrimSpace(string(raw)))
	require.NoError(t, convErr)
	require.Eventually(t, func() bool { return !processAlive(pid) }, 3*time.Second, 20*time.Millisecond,
		"the grandchild must be stopped with its group")
}

// processAlive coi tiến trình zombie (đã chết, chờ được dọn) là không còn sống.
func processAlive(pid int) bool {
	stat, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
	if err != nil {
		return false
	}
	fields := strings.Fields(string(stat[strings.LastIndexByte(string(stat), ')')+1:]))
	return len(fields) > 0 && fields[0] != "Z"
}

// Ký tự vô hình (zero-width, BOM, soft hyphen, tag, Hangul filler, ngăn
// dòng Unicode, vùng riêng) không lọt vào hộp thoại duyệt.
func TestDisplaySafeHidesInvisibleCharacters(t *testing.T) {
	t.Parallel()
	for _, r := range []rune{'\u200B', '\u200C', '\u200D', '\u2060', '\uFEFF', '\u00AD', '\u2028', '\u2029', '\U000E0041', '\u3164', '\uE000'} {
		got := displaySafe("a" + string(r) + "b")
		require.Equalf(t, "a\uFFFDb", got, "U+%04X", r)
	}
	visible := "xin ch\u00e0o\n\tok \u2014 \u2713"
	require.Equal(t, visible, displaySafe(visible), "visible text stays")
}

// Dấu kết hợp vô hình (variation selector, CGJ, Khmer, Mongolian FVS) bị thay;
// một VS16 ngay sau ký tự biểu tượng (cách vẽ emoji) thì giữ (người soát độc
// lập, P2-4).
func TestDisplaySafeHidesCombiningPayloads(t *testing.T) {
	t.Parallel()
	for _, r := range []rune{'\uFE00', '\uFE0D', '\U000E0100', '\U000E01EF', '\u034F', '\u17B4', '\u180B', '\u180F'} {
		require.Equalf(t, "a\uFFFD", displaySafe("a"+string(r)), "U+%04X", r)
	}
	hidden := "ok \U0001F642\uFE0F\uFE0F\uFE0F"
	require.Equal(t, "ok \U0001F642\uFE0F\uFFFD\uFFFD", displaySafe(hidden), "only one selector after a symbol")
	require.Equal(t, "\u2764\uFE0F", displaySafe("\u2764\uFE0F"), "emoji presentation stays")
	require.Equal(t, "x\uFFFD", displaySafe("x\uFE0F"), "a selector after a letter is hidden data")
}
