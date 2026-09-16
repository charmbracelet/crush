//go:build !windows

package shell

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestBackgroundShellManager_KillWithEscapedProcess drives the real case
// behind the bounded wait: a job whose child moves itself into a new
// session survives the process-group kill and holds the shell's output
// pipe open, so the shell goroutine never finishes.
func TestBackgroundShellManager_KillWithEscapedProcess(t *testing.T) {
	t.Parallel()

	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 is needed to detach a process into its own session")
	}

	pidFile := filepath.Join(t.TempDir(), "escaped.pid")
	command := fmt.Sprintf(
		`%s -c 'import subprocess,sys,time; p=subprocess.Popen(["sleep","120"], start_new_session=True); open(sys.argv[1],"w").write(str(p.pid)); time.sleep(120)' %s`,
		python, pidFile,
	)

	manager := newBackgroundShellManager()
	bgShell, err := manager.Start(t.Context(), t.TempDir(), nil, command, "escapes its process group")
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		_, statErr := os.Stat(pidFile)
		return statErr == nil
	}, 20*time.Second, 100*time.Millisecond, "detached child never started")

	t.Cleanup(func() {
		raw, readErr := os.ReadFile(pidFile)
		if readErr != nil {
			return
		}
		if pid, convErr := strconv.Atoi(strings.TrimSpace(string(raw))); convErr == nil {
			_ = syscall.Kill(pid, syscall.SIGKILL)
		}
	})

	errc := make(chan error, 1)
	go func() { errc <- manager.Kill(t.Context(), bgShell.ID) }()

	select {
	case err := <-errc:
		require.ErrorIs(t, err, ErrKillEscaped)
	case <-time.After(killGrace + 30*time.Second):
		t.Fatal("Kill never returned while a detached process held the output pipe")
	}
}
