package cmd

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/server"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

// childTestEnv marks a test binary re-run by runChildTest. Logging is set up
// once per process, so tests that set it up do so in a child process.
const childTestEnv = "CRUSH_TEST_CHILD"

// runChildTest re-runs the calling test in a child process with env added,
// and returns the child's output. The child sees childTestEnv set. Its
// files stay open until it exits, which is before the parent's temporary
// directories are removed, as Windows requires.
func runChildTest(t *testing.T, env ...string) string {
	t.Helper()
	exe, err := os.Executable()
	require.NoError(t, err)
	c := exec.CommandContext(t.Context(), exe, "-test.run=^"+t.Name()+"$")
	c.Env = append(append(os.Environ(), childTestEnv+"=1"), env...)
	out, err := c.CombinedOutput()
	require.NoError(t, err, "child test failed:\n%s", out)
	return string(out)
}

// TestConnectToServer_ClientStaysOutOfServerLog covers clients logging into
// the server's rotating log file. Every client then holds the file open, so
// on Windows the server cannot rotate it while any client is attached, and
// the server's logging stops. A client must log to a file of its own until
// it knows its workspace, and then to the workspace's log.
func TestConnectToServer_ClientStaysOutOfServerLog(t *testing.T) {
	if os.Getenv(childTestEnv) == "1" {
		connectAndLog(t, os.Getenv("CRUSH_TEST_DATA_DIR"))
		return
	}
	t.Parallel()

	runChildTest(t,
		"CRUSH_CACHE_DIR="+t.TempDir(),
		"CRUSH_TEST_DATA_DIR="+t.TempDir(),
		"CRUSH_DISABLE_METRICS=1",
	)
}

// connectAndLog connects to a fake server the way the CLI does, logs before
// and after, and checks where the records landed.
func connectAndLog(t *testing.T, dataDir string) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/workspaces", r.URL.Path)
		require.NoError(t, json.NewEncoder(w).Encode(proto.Workspace{
			ID:     "ws",
			Config: &config.Config{Options: &config.Options{DataDirectory: dataDir}},
		}))
	}))
	t.Cleanup(srv.Close)
	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	clientHost = "tcp://" + u.Host
	hostURL, err := server.ParseHostURL(clientHost)
	require.NoError(t, err)

	// The server has already started its log.
	perHostDir, err := perHostServerDir(hostURL)
	require.NoError(t, err)
	serverLog := filepath.Join(perHostDir, "crush.log")
	require.NoError(t, os.WriteFile(serverLog, []byte(`{"msg":"Server started"}`+"\n"), 0o600))

	cmd := &cobra.Command{}
	cmd.Flags().Bool("debug", false, "")
	cmd.Flags().Bool("yolo", false, "")
	cmd.Flags().StringSlice("channels", nil, "")
	cmd.Flags().String("data-dir", "", "")
	cmd.Flags().String("cwd", "", "")
	cmd.SetContext(t.Context())

	require.NoError(t, ensureServer(cmd, hostURL))
	slog.Warn("Client record before connecting")
	_, _, _, err = connectToServer(cmd)
	require.NoError(t, err)
	slog.Warn("Client record after connecting")

	// Windows refuses to rename a file while another handle on it is open,
	// so this is the server rotating its log with a client attached.
	require.NoError(t, os.Rename(serverLog, serverLog+".1"),
		"the server must be able to rotate its log while a client runs")
	got, err := os.ReadFile(serverLog + ".1")
	require.NoError(t, err)
	require.NotContains(t, string(got), "Client record", "a client must not write the server's log")

	clientLog := filepath.Join(perHostDir, "client.log")
	got, err = os.ReadFile(clientLog)
	require.NoError(t, err, "the client must log its startup to a file of its own")
	require.Contains(t, string(got), "Client record before connecting")
	require.NotContains(t, string(got), "Client record after connecting")
	require.NoError(t, os.Rename(clientLog, clientLog+".1"),
		"the client must release its startup log once it knows its workspace")

	got, err = os.ReadFile(filepath.Join(dataDir, "logs", "crush.log"))
	require.NoError(t, err, "the client must log to its workspace once it knows it")
	require.Contains(t, string(got), "Client record after connecting")
}

// TestLogs_ClientServerModeShowsServerLog covers `crush logs` in
// client/server mode. The agent runs in the server, which logs to its own
// file, so showing only the workspace's log finds nothing about it.
func TestLogs_ClientServerModeShowsServerLog(t *testing.T) {
	const host = "tcp://127.0.0.1:1"
	if os.Getenv(childTestEnv) == "1" {
		rootCmd.SetArgs([]string{"logs", "--cwd", os.Getenv("CRUSH_TEST_PROJECT"), "--host", host})
		require.NoError(t, rootCmd.ExecuteContext(t.Context()))
		return
	}
	t.Parallel()

	hostURL, err := server.ParseHostURL(host)
	require.NoError(t, err)
	cacheDir := t.TempDir()
	project := t.TempDir()
	writeLogLines(t, filepath.Join(cacheDir, "server-"+safeHostName(hostURL), "client.log"),
		`{"time":"2026-01-02T03:04:04Z","level":"WARN","msg":"Client startup record"}`,
	)
	writeLogLines(t, filepath.Join(project, ".crush", "logs", "crush.log"),
		`{"time":"2026-01-02T03:04:05Z","level":"INFO","msg":"Client record first"}`,
		`{"time":"2026-01-02T03:04:07Z","level":"INFO","msg":"Client record last"}`,
	)
	// The offset changes with daylight saving time, so records are ordered
	// by the instant they were logged, not by their text.
	writeLogLines(t, filepath.Join(cacheDir, "server-"+safeHostName(hostURL), "crush.log"),
		`{"time":"2026-01-02T03:04:06Z","level":"INFO","msg":"Server record in UTC"}`,
		`{"time":"2026-01-02T04:04:06.5+01:00","level":"INFO","msg":"Server record with offset"}`,
	)

	out := runChildTest(t,
		"CRUSH_CLIENT_SERVER=1",
		"CRUSH_CACHE_DIR="+cacheDir,
		"CRUSH_GLOBAL_CONFIG="+t.TempDir(),
		"CRUSH_GLOBAL_DATA="+t.TempDir(),
		"CRUSH_DISABLE_PROVIDER_AUTO_UPDATE=1",
		"CRUSH_TEST_PROJECT="+project,
	)

	var order []int
	for _, msg := range []string{
		"Client startup record",
		"Client record first",
		"Server record in UTC",
		"Server record with offset",
		"Client record last",
	} {
		i := strings.Index(out, msg)
		require.NotEqual(t, -1, i, "crush logs must show %q:\n%s", msg, out)
		order = append(order, i)
	}
	require.IsIncreasing(t, order, "records must be shown in the order they were logged:\n%s", out)
}

func writeLogLines(t *testing.T, path string, lines ...string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600))
}
