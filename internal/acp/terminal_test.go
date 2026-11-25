package acp

import (
	"context"
	"runtime"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	acp "github.com/coder/acp-go-sdk"
	"github.com/stretchr/testify/require"
)

func longRunningCommand() (string, []string) {
	if runtime.GOOS == "windows" {
		return "ping", []string{"-n", "30", "127.0.0.1"}
	}
	return "sleep", []string{"30"}
}

func envPrintCommand(name string) (string, []string) {
	if runtime.GOOS == "windows" {
		return "cmd", []string{"/c", "echo", "%" + name + "%"}
	}
	return "printenv", []string{name}
}

func waitTerminalExit(t *testing.T, registry *terminalRegistry, id acp.TerminalId) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _, ok := registry.wait(ctx, id)
	require.True(t, ok)
}

func TestTerminalLifecycle(t *testing.T) {
	registry := newTerminalRegistry()
	command, args := envPrintCommand("PATH")
	id := acp.TerminalId("life")

	_, err := registry.create(context.Background(), id, command, args, nil, t.TempDir(), nil)
	require.NoError(t, err)
	waitTerminalExit(t, registry, id)

	out, truncated, status, ok := registry.output(id)
	require.True(t, ok)
	require.NotEmpty(t, out)
	require.False(t, truncated)
	require.NotNil(t, status)

	require.True(t, registry.release(id))
	require.False(t, registry.release(id))
	_, _, _, ok = registry.output(id)
	require.False(t, ok)
}

func TestTerminalUnknownIDs(t *testing.T) {
	registry := newTerminalRegistry()
	id := acp.TerminalId("missing")

	out, truncated, status, ok := registry.output(id)
	require.False(t, ok)
	require.Empty(t, out)
	require.False(t, truncated)
	require.Nil(t, status)

	code, signal, ok := registry.wait(context.Background(), id)
	require.False(t, ok)
	require.Nil(t, code)
	require.Nil(t, signal)
	require.False(t, registry.kill(id))
	require.False(t, registry.release(id))
}

func TestTerminalKillAndRelease(t *testing.T) {
	registry := newTerminalRegistry()
	command, args := longRunningCommand()
	id := acp.TerminalId("killed")

	_, err := registry.create(context.Background(), id, command, args, nil, t.TempDir(), nil)
	require.NoError(t, err)
	require.True(t, registry.kill(id))
	waitTerminalExit(t, registry, id)

	_, _, status, ok := registry.output(id)
	require.True(t, ok)
	require.NotNil(t, status)
	require.True(t, registry.release(id))
	require.False(t, registry.kill(id))
}

func TestTerminalRejectsRelativeCwd(t *testing.T) {
	registry := newTerminalRegistry()
	command, args := envPrintCommand("PATH")

	_, err := registry.create(context.Background(), "relative", command, args, nil, "relative/dir", nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "absolute")
	_, _, _, ok := registry.output("relative")
	require.False(t, ok)
}

func TestTerminalEnvOverrides(t *testing.T) {
	registry := newTerminalRegistry()
	command, args := envPrintCommand("CRUSH_TERMINAL_TEST_ENV")
	env := []acp.EnvVariable{{Name: "CRUSH_TERMINAL_TEST_ENV", Value: "value"}}

	_, err := registry.create(context.Background(), "env", command, args, env, t.TempDir(), nil)
	require.NoError(t, err)
	waitTerminalExit(t, registry, "env")

	out, _, _, ok := registry.output("env")
	require.True(t, ok)
	require.True(t, strings.Contains(out, "value"), "got %q", out)
}

func TestTerminalOutputLimitRuneBoundary(t *testing.T) {
	process := &terminalProcess{limit: 5}
	process.appendOutput([]byte("αβγδ"))
	require.Equal(t, "γδ", string(process.output))
	require.True(t, process.truncated)
	require.True(t, utf8.ValidString(string(process.output)))
}

func TestTerminalWaitRespectsContext(t *testing.T) {
	registry := newTerminalRegistry()
	command, args := longRunningCommand()
	id := acp.TerminalId("wait-ctx")

	_, err := registry.create(context.Background(), id, command, args, nil, t.TempDir(), nil)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	code, signal, ok := registry.wait(ctx, id)
	require.False(t, ok)
	require.Nil(t, code)
	require.Nil(t, signal)
	require.True(t, registry.release(id))
}
