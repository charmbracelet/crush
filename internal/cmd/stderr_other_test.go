//go:build !windows

package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

// TestDetachStderr swaps fd 2 process-wide, so it must not run in parallel.
func TestDetachStderr(t *testing.T) {
	realStderr, err := unix.Dup(2)
	require.NoError(t, err)
	realStderrFile := os.Stderr
	t.Cleanup(func() {
		_ = unix.Dup2(realStderr, 2)
		_ = unix.Close(realStderr)
		os.Stderr = realStderrFile
	})

	capturePath := filepath.Join(t.TempDir(), "stderr")
	capture, err := os.Create(capturePath)
	require.NoError(t, err)
	defer capture.Close()
	require.NoError(t, unix.Dup2(int(capture.Fd()), 2))

	restore, err := detachStderr()
	require.NoError(t, err)

	_, err = unix.Write(2, []byte("raw fd write\n"))
	require.NoError(t, err)
	_, err = fmt.Fprint(os.Stderr, "go write\n")
	require.NoError(t, err)

	child := exec.CommandContext(t.Context(), "sh", "-c", "echo child write >&2")
	child.Stderr = os.Stderr
	require.NoError(t, child.Run())

	restore()

	_, err = unix.Write(2, []byte("restored write\n"))
	require.NoError(t, err)

	got, err := os.ReadFile(capturePath)
	require.NoError(t, err)
	require.Equal(t, "go write\nchild write\nrestored write\n", string(got))
	require.Equal(t, uintptr(2), os.Stderr.Fd())
}
