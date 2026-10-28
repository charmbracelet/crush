//go:build !windows

package cmd

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/server"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

// fakeServerPIDFileEnv turns the test binary, when startDetachedServer
// re-executes it as the server, into a server that writes its pid to the
// named file and exits at once instead of running the tests.
const fakeServerPIDFileEnv = "CRUSH_TEST_FAKE_SERVER_PID_FILE"

func TestMain(m *testing.M) {
	if pidFile := os.Getenv(fakeServerPIDFileEnv); pidFile != "" {
		tmp := pidFile + ".tmp"
		if err := os.WriteFile(tmp, []byte(strconv.Itoa(os.Getpid())), 0o600); err != nil {
			os.Exit(1)
		}
		if err := os.Rename(tmp, pidFile); err != nil {
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// TestStartDetachedServerReapsExitedServer checks that a server which exits
// while the client that spawned it is still running is reaped, instead of
// lingering as a zombie until that client exits. It sets process-wide
// environment variables, so it must not run in parallel.
func TestStartDetachedServerReapsExitedServer(t *testing.T) {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "server.pid")
	t.Setenv(fakeServerPIDFileEnv, pidFile)
	t.Setenv("CRUSH_CACHE_DIR", filepath.Join(dir, "cache"))

	hostURL, err := server.ParseHostURL("unix://" + filepath.Join(dir, "crush.sock"))
	require.NoError(t, err)
	cmd := &cobra.Command{}
	cmd.SetContext(t.Context())
	_, err = startDetachedServer(cmd, hostURL)
	require.NoError(t, err)

	var pid int
	require.Eventually(t, func() bool {
		data, err := os.ReadFile(pidFile)
		if err != nil {
			return false
		}
		pid, err = strconv.Atoi(string(data))
		return err == nil
	}, 10*time.Second, 10*time.Millisecond, "the spawned server never started")

	// Signal 0 succeeds while the server is running or a zombie, and fails
	// with ESRCH once it has exited and been reaped.
	deadline := time.Now().Add(10 * time.Second)
	for !errors.Is(syscall.Kill(pid, 0), syscall.ESRCH) {
		if time.Now().After(deadline) {
			stat, _ := exec.CommandContext(t.Context(), "ps", "-o", "stat=",
				"-p", strconv.Itoa(pid)).Output()
			t.Fatalf("server process %d was not reaped after exiting (ps stat %q)",
				pid, strings.TrimSpace(string(stat)))
		}
		time.Sleep(10 * time.Millisecond)
	}
}
