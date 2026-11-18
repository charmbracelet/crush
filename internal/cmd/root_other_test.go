//go:build !windows

package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/server"
	"github.com/stretchr/testify/require"
)

// fakeServerPIDFileEnv makes the test binary, re-executed as the server,
// write its pid to the named file and exit.
const fakeServerPIDFileEnv = "CRUSH_TEST_FAKE_SERVER_PID_FILE"

func TestMain(m *testing.M) {
	if pidFile := os.Getenv(fakeServerPIDFileEnv); pidFile != "" {
		if os.WriteFile(pidFile+".tmp", []byte(strconv.Itoa(os.Getpid())), 0o600) != nil ||
			os.Rename(pidFile+".tmp", pidFile) != nil {
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// TestStartDetachedServerReapsExitedServer guards a server that exits before
// its client lingering as a zombie until the client exits.
func TestStartDetachedServerReapsExitedServer(t *testing.T) {
	// Not parallel: it sets env vars.
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "server.pid")
	t.Setenv(fakeServerPIDFileEnv, pidFile)
	t.Setenv("CRUSH_CACHE_DIR", filepath.Join(dir, "cache"))
	hostURL, err := server.ParseHostURL("unix://" + filepath.Join(dir, "crush.sock"))
	require.NoError(t, err)
	_, err = startDetachedServer(commandWithContext(t.Context()), hostURL)
	require.NoError(t, err)

	var pid int
	require.Eventually(t, func() bool {
		data, err := os.ReadFile(pidFile)
		pid, _ = strconv.Atoi(string(data))
		return err == nil && pid > 0
	}, 10*time.Second, 10*time.Millisecond, "the spawned server never started")
	// Signal 0 fails with ESRCH only once the process has been reaped.
	require.Eventually(t, func() bool {
		return errors.Is(syscall.Kill(pid, 0), syscall.ESRCH)
	}, 10*time.Second, 10*time.Millisecond, "server %d was not reaped", pid)
}
