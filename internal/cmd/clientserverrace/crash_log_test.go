package clientserverrace_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestRespawnKeepsPreviousServerOutput guards against spawning a server
// truncating the previous server's stdout.log and stderr.log, which hold
// the only record of a crash.
func TestRespawnKeepsPreviousServerOutput(t *testing.T) {
	sb := newSandbox(t, crushBinary(t), "CRUSH_CLIENT_SERVER=1")
	t.Cleanup(func() { shutdownServer(t, sb.socket) })

	// With no provider configured, `crush run` stops once it has a
	// workspace on a server.
	run := func() {
		t.Helper()
		ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
		defer cancel()
		c := exec.CommandContext(ctx, sb.bin, "--host", sb.host, "--cwd", sb.project, "run", "hi")
		c.Env = sb.env
		out, _ := c.CombinedOutput()
		require.Contains(t, strings.Join(strings.Fields(strings.ToLower(string(out))), " "),
			"no providers configured", "client output:\n%s", out)
	}

	run()
	serverDirs, err := filepath.Glob(filepath.Join(sb.dir, "xdg_cache_home", "crush", "server-*"))
	require.NoError(t, err)
	require.Len(t, serverDirs, 1)
	shutdownServer(t, sb.socket)
	require.NoFileExists(t, sb.socket)

	logs := map[string]string{
		"stderr.log": "panic: crash of the previous server\n",
		"stdout.log": "output of the previous server\n",
	}
	for name, content := range logs {
		f, err := os.OpenFile(filepath.Join(serverDirs[0], name), os.O_WRONLY|os.O_APPEND, 0)
		require.NoError(t, err)
		_, err = f.WriteString(content)
		require.NoError(t, err)
		require.NoError(t, f.Close())
	}

	run()
	for name, content := range logs {
		got, err := os.ReadFile(filepath.Join(serverDirs[0], name))
		require.NoError(t, err)
		require.Contains(t, string(got), content, "%s was truncated", name)
	}
}
