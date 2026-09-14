//go:build darwin || linux || freebsd || netbsd || openbsd

package pinentry

import (
	"bytes"
	"errors"
	"io"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/creack/pty"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

// TestInjectCtrlL runs the injectctrl helper in a pty (as the session
// leader with the pty as its controlling terminal, mirroring how Crush
// runs in a real terminal) and verifies that the injected Ctrl-L byte
// arrives in the terminal's input queue, where a pinentry dialog would
// read it.
func TestInjectCtrlL(t *testing.T) {
	t.Parallel()

	helper := buildFakeProc(t, "injectctrl.go", "injectctrl")

	cmd := exec.CommandContext(t.Context(), helper)
	ptmx, err := pty.Start(cmd)
	if err != nil {
		t.Skipf("cannot start pty: %v", err)
	}
	defer func() { _ = ptmx.Close() }()

	type result struct {
		data []byte
		err  error
	}
	done := make(chan result, 1)
	go func() {
		var buf bytes.Buffer
		_, err := io.Copy(&buf, ptmx)
		done <- result{buf.Bytes(), err}
	}()

	select {
	case res := <-done:
		out := string(res.data)
		if strings.Contains(out, "INJECT-ERROR") && strings.Contains(out, "permission denied") {
			t.Skip("TIOCSTI is disabled on this system")
		}
		require.Contains(t, out, "GOT-0c", "helper output: %q", out)
	case <-time.After(15 * time.Second):
		_ = cmd.Process.Kill()
		<-done
		t.Fatal("timed out waiting for injected byte")
	}
}

func TestInjectCtrlLNoTTY(t *testing.T) {
	t.Parallel()

	// Without a controlling terminal (the typical test environment), the
	// error must be a clean failure, not a panic.
	err := InjectCtrlL()
	if err == nil {
		t.Skip("test environment unexpectedly has a controlling terminal")
	}
	require.Error(t, err)
	require.True(t, errors.Is(err, unix.ENOENT) || errors.Is(err, unix.ENXIO) || errors.Is(err, unix.EIO), "unexpected error: %v", err)
}
