package clientserverrace_test

import (
	"context"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestRespawnKeepsPreviousServerOutput is a regression test for the next
// client erasing what the previous server wrote. A detached server's stdout
// and stderr go to server-<host>/stdout.log and stderr.log, and stderr.log is
// the only record of a panic or other runtime crash, since crush.log never
// sees those. Spawning the replacement server used to truncate both files,
// so the crash trace was gone before anyone could read it.
func TestRespawnKeepsPreviousServerOutput(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping server respawn test in -short mode")
	}
	if runtime.GOOS == "windows" {
		t.Skip("skipping unix-socket specific test on windows")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("skipping: 'go' not available on PATH")
	}

	bin := buildCrushBinary(t, repoRootFromTest(t))

	// Use /tmp directly so the unix socket path stays under the 104-char
	// sockaddr_un limit on darwin.
	runDir, err := os.MkdirTemp("/tmp", "crush-respawn-")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(runDir) })

	socketPath := filepath.Join(runDir, "crush.sock")
	t.Cleanup(func() { shutdownServer(t, socketPath) })

	// Build the environment from scratch so that an API key in the
	// developer's environment cannot configure a real provider.
	env := []string{
		"PATH=" + os.Getenv("PATH"),
		"CRUSH_CLIENT_SERVER=1",
		"CRUSH_DISABLE_METRICS=1",
		"CRUSH_DISABLE_PROVIDER_AUTO_UPDATE=1",
	}
	for key, name := range map[string]string{
		"HOME":            "home",
		"XDG_CACHE_HOME":  "cache",
		"XDG_DATA_HOME":   "data",
		"XDG_CONFIG_HOME": "config",
	} {
		dir := filepath.Join(runDir, name)
		require.NoError(t, os.MkdirAll(dir, 0o700))
		env = append(env, key+"="+dir)
	}
	ws := filepath.Join(runDir, "ws")
	require.NoError(t, os.MkdirAll(ws, 0o700))

	// run starts a client, which spawns a server when none is running.
	// With no provider configured, `crush run` stops right after creating
	// its workspace, which proves that it reached a server.
	run := func() {
		t.Helper()
		ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
		defer cancel()
		c := exec.CommandContext(ctx, bin,
			"--host", "unix://"+socketPath,
			"--cwd", ws,
			"run", "hi",
		)
		c.Env = env
		out, _ := c.CombinedOutput()
		// The error handler wraps the message to the terminal width.
		require.Contains(t, strings.Join(strings.Fields(strings.ToLower(string(out))), " "),
			"no providers configured", "client output:\n%s", out)
	}

	run()
	serverDirs, err := filepath.Glob(filepath.Join(runDir, "cache", "crush", "server-*"))
	require.NoError(t, err)
	require.Len(t, serverDirs, 1)

	// Stop the server, then leave in its logs what a crash would have
	// written there.
	shutdownServer(t, socketPath)
	_, err = os.Stat(socketPath)
	require.ErrorIs(t, err, fs.ErrNotExist,
		"the first server must be gone, so that the next client spawns another")
	previous := []struct{ name, content string }{
		{"stderr.log", "panic: crash of the previous server\n"},
		{"stdout.log", "output of the previous server\n"},
	}
	for _, prev := range previous {
		f, err := os.OpenFile(filepath.Join(serverDirs[0], prev.name), os.O_WRONLY|os.O_APPEND, 0)
		require.NoError(t, err)
		_, err = f.WriteString(prev.content)
		require.NoError(t, err)
		require.NoError(t, f.Close())
	}

	run()
	for _, prev := range previous {
		got, err := os.ReadFile(filepath.Join(serverDirs[0], prev.name))
		require.NoError(t, err)
		require.Contains(t, string(got), prev.content,
			"spawning the next server must keep the previous server's %s", prev.name)
	}
}
