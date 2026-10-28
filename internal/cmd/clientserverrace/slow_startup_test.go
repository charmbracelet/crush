package clientserverrace_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// slowCatalogDelay is how long the fake provider catalog takes to answer.
// It has to outlast the client's default readiness budget for a server it
// did not start (10s) by a clear margin, and stay below the catalog
// clients' own 30s timeouts so that the server starts slowly rather than
// with a failed fetch.
const slowCatalogDelay = 15 * time.Second

// TestFirstClientWaitsForSlowServerStartup is a regression test for a first
// launch on a slow network. `crush server` binds its socket only after
// loading its config, which includes refreshing the provider catalog, and
// the client that spawned it used to give up after a fixed 10s: it failed
// with "failed to initialize crush server" while the server it had just
// started was still loading, and that server then came up orphaned.
func TestFirstClientWaitsForSlowServerStartup(t *testing.T) {
	bin, dir := startupTestSetup(t)

	// The catalog answers only after slowCatalogDelay, and then with an
	// error, so the server falls back to its embedded catalog. A request
	// that goes away first ends the handler early so that closing the
	// listener never waits out the delay.
	catalog := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(slowCatalogDelay):
			http.Error(w, "catalog unavailable", http.StatusServiceUnavailable)
		case <-r.Context().Done():
		}
	}))
	t.Cleanup(catalog.Close)

	// Whatever happens to the client, the server it spawned comes up once
	// the catalog has answered. Wait for it and stop it, so that a failing
	// run does not leave a server behind.
	socketPath := filepath.Join(dir, "crush.sock")
	t.Cleanup(func() {
		deadline := time.Now().Add(slowCatalogDelay + 30*time.Second)
		for time.Now().Before(deadline) && pingHealth(socketPath) != nil {
			time.Sleep(100 * time.Millisecond)
		}
		shutdownServer(t, socketPath)
	})

	output := runIsolatedClient(t, bin, dir, slowCatalogDelay+time.Minute,
		"CATWALK_URL="+catalog.URL,
		"HYPER_URL="+catalog.URL,
	)
	require.NotContains(t, output, readinessErrSubstr,
		"the client must keep waiting for the server it spawned while that server is still starting")
	// With no provider configured, `crush run` stops right after creating
	// its workspace, which proves it reached the server.
	require.Contains(t, output, "no providers configured")
}

// TestClientReportsSpawnedServerFailure covers the other half of waiting
// longer for a server the client spawned: when that server dies while
// starting, the client must stop waiting at once and pass on the server's
// own explanation instead of an unreachable socket.
func TestClientReportsSpawnedServerFailure(t *testing.T) {
	bin, dir := startupTestSetup(t)

	configDir := filepath.Join(dir, "config", "crush")
	require.NoError(t, os.MkdirAll(configDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "crush.json"), []byte("{ not json"), 0o600))

	output := runIsolatedClient(t, bin, dir, time.Minute, "CRUSH_DISABLE_PROVIDER_AUTO_UPDATE=1")
	require.Contains(t, output, "crush server exited before it was ready")
	require.Contains(t, output, "invalid json in config file")
}

// startupTestSetup builds the crush binary and returns it along with a fresh
// directory for one client and the server it spawns.
func startupTestSetup(t *testing.T) (bin, dir string) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping server startup test in -short mode")
	}
	if runtime.GOOS == "windows" {
		t.Skip("skipping unix-socket specific test on windows")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("skipping: 'go' not available on PATH")
	}

	bin = buildCrushBinary(t, repoRootFromTest(t))

	// Use /tmp directly so the unix socket path stays under the 104-char
	// sockaddr_un limit on darwin.
	dir, err := os.MkdirTemp("/tmp", "crush-startup-")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return bin, dir
}

// runIsolatedClient runs `crush run hi` against a server on dir/crush.sock,
// with HOME, the XDG directories and the workspace under dir, and returns
// its combined output in lower case with runs of whitespace collapsed.
func runIsolatedClient(t *testing.T, bin, dir string, timeout time.Duration, extraEnv ...string) string {
	t.Helper()

	// Build the environment from scratch rather than inheriting it: an API
	// key in the developer's environment would configure a real provider,
	// and a CRUSH_* override could change what the test is about.
	env := []string{"PATH=" + os.Getenv("PATH"), "CRUSH_CLIENT_SERVER=1", "CRUSH_DISABLE_METRICS=1"}
	for key, name := range map[string]string{
		"HOME":            "home",
		"XDG_CACHE_HOME":  "cache",
		"XDG_DATA_HOME":   "data",
		"XDG_CONFIG_HOME": "config",
	} {
		path := filepath.Join(dir, name)
		require.NoError(t, os.MkdirAll(path, 0o700))
		env = append(env, key+"="+path)
	}
	ws := filepath.Join(dir, "ws")
	require.NoError(t, os.MkdirAll(ws, 0o700))

	ctx, cancel := context.WithTimeout(t.Context(), timeout)
	defer cancel()
	c := exec.CommandContext(ctx, bin,
		"--host", "unix://"+filepath.Join(dir, "crush.sock"),
		"--cwd", ws,
		"run", "hi",
	)
	c.Env = append(env, extraEnv...)
	start := time.Now()
	out, _ := c.CombinedOutput()
	t.Logf("client exited after %s:\n%s", time.Since(start).Round(time.Second), out)
	// The error handler capitalizes the first letter of the message and
	// wraps it to the terminal width.
	return strings.Join(strings.Fields(strings.ToLower(string(out))), " ")
}
