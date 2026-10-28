package clientserverrace_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestServerSignalExitStatus checks that `crush server` reports the signal
// that stopped it the way a shell would, as 128 plus the signal number:
// service managers stop a unit with SIGTERM and expect 143, not the 130 of
// an interactive Ctrl+C.
func TestServerSignalExitStatus(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping server signal test in -short mode")
	}
	// Windows cannot send SIGINT or SIGTERM to another process.
	if runtime.GOOS == "windows" {
		t.Skip("skipping signal test on windows")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("skipping: 'go' not available on PATH")
	}

	bin := buildCrushBinary(t, repoRootFromTest(t))

	for _, tc := range []struct {
		sig  syscall.Signal
		want int
	}{
		{syscall.SIGINT, 130},
		{syscall.SIGTERM, 143},
	} {
		t.Run(tc.sig.String(), func(t *testing.T) {
			// Use /tmp so the socket path fits in sockaddr_un; see
			// TestClientServerSpawnRace.
			runDir, err := os.MkdirTemp("/tmp", "crush-sig-")
			require.NoError(t, err)
			t.Cleanup(func() { _ = os.RemoveAll(runDir) })

			for _, d := range []string{"cache", "config", "data", "home"} {
				require.NoError(t, os.MkdirAll(filepath.Join(runDir, d), 0o700))
			}
			socketPath := filepath.Join(runDir, "crush.sock")

			c := exec.CommandContext(t.Context(), bin, "server", "--host", "unix://"+socketPath)
			c.Dir = runDir
			c.Env = append(
				os.Environ(),
				"HOME="+filepath.Join(runDir, "home"),
				"XDG_CACHE_HOME="+filepath.Join(runDir, "cache"),
				"XDG_CONFIG_HOME="+filepath.Join(runDir, "config"),
				"XDG_DATA_HOME="+filepath.Join(runDir, "data"),
				// Clear overrides that would take precedence over the
				// XDG directories above.
				"CRUSH_CACHE_DIR=",
				"CRUSH_GLOBAL_CONFIG=",
				"CRUSH_GLOBAL_DATA=",
				"CRUSH_DISABLE_PROVIDER_AUTO_UPDATE=1",
			)
			var out strings.Builder
			c.Stdout = &out
			c.Stderr = &out
			require.NoError(t, c.Start())

			exited := make(chan struct{})
			go func() {
				_ = c.Wait()
				close(exited)
			}()
			t.Cleanup(func() {
				_ = c.Process.Kill()
				<-exited
			})

			deadline := time.Now().Add(20 * time.Second)
			for pingHealth(socketPath) != nil {
				select {
				case <-exited:
					t.Fatalf("server exited before it was ready: %v\n%s", c.ProcessState, out.String())
				case <-time.After(50 * time.Millisecond):
				}
				if time.Now().After(deadline) {
					_ = c.Process.Kill()
					<-exited
					t.Fatalf("server not ready on %s after 20s\n%s", socketPath, out.String())
				}
			}

			require.NoError(t, c.Process.Signal(tc.sig))
			select {
			case <-exited:
			case <-time.After(30 * time.Second):
				t.Fatalf("server still running 30s after %v", tc.sig)
			}
			require.Equal(t, tc.want, c.ProcessState.ExitCode(),
				"exit status after %v (%v)\n%s", tc.sig, c.ProcessState, out.String())
		})
	}
}
