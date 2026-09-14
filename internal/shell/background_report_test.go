package shell

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// waitForShell blocks until the job finishes or the test times out.
func waitForShell(t *testing.T, bs *BackgroundShell) {
	t.Helper()
	select {
	case <-bs.done:
	case <-time.After(10 * time.Second):
		t.Fatal("background shell did not finish in time")
	}
}

func TestBackgroundShellReadFromReturnsOnlyNewOutput(t *testing.T) {
	t.Parallel()

	manager := newBackgroundShellManager()
	manager.SetLogDir(t.TempDir())

	bgShell, err := manager.Start(t.Context(), StartOptions{
		WorkingDir: t.TempDir(),
		Command:    "echo one; sleep 0.4; echo two",
	})
	require.NoError(t, err)
	t.Cleanup(func() { manager.Kill(bgShell.ID) })

	// Read whatever exists once the first line has landed.
	var (
		first  string
		offset int64
	)
	require.Eventually(t, func() bool {
		first, offset, err = bgShell.ReadFrom(0)
		return err == nil && strings.Contains(first, "one")
	}, 5*time.Second, 20*time.Millisecond)
	require.NotContains(t, first, "two")
	require.Positive(t, offset)

	waitForShell(t, bgShell)

	// Reading from the previous offset must yield only what came after it.
	second, next, err := bgShell.ReadFrom(offset)
	require.NoError(t, err)
	require.Contains(t, second, "two")
	require.NotContains(t, second, "one", "reading from an offset must not repeat earlier output")
	require.Greater(t, next, offset)

	// Reading from the end yields nothing and holds the offset steady.
	empty, stable, err := bgShell.ReadFrom(next)
	require.NoError(t, err)
	require.Empty(t, empty)
	require.Equal(t, next, stable)
}

func TestBackgroundShellReadFromInterleavesStdoutAndStderr(t *testing.T) {
	t.Parallel()

	manager := newBackgroundShellManager()
	manager.SetLogDir(t.TempDir())

	bgShell, err := manager.Start(t.Context(), StartOptions{
		WorkingDir: t.TempDir(),
		Command:    "echo to-stdout; echo to-stderr >&2",
	})
	require.NoError(t, err)
	t.Cleanup(func() { manager.Kill(bgShell.ID) })

	waitForShell(t, bgShell)

	combined, _, err := bgShell.ReadFrom(0)
	require.NoError(t, err)
	require.Contains(t, combined, "to-stdout")
	require.Contains(t, combined, "to-stderr")
}

func TestBackgroundShellTail(t *testing.T) {
	t.Parallel()

	manager := newBackgroundShellManager()
	manager.SetLogDir(t.TempDir())

	bgShell, err := manager.Start(t.Context(), StartOptions{
		WorkingDir: t.TempDir(),
		Command:    "for i in 1 2 3 4 5; do echo line$i; done",
	})
	require.NoError(t, err)
	t.Cleanup(func() { manager.Kill(bgShell.ID) })

	waitForShell(t, bgShell)

	tail, truncated := bgShell.Tail(2)
	require.True(t, truncated)
	require.Equal(t, "line4\nline5", tail)

	all, truncated := bgShell.Tail(50)
	require.False(t, truncated)
	require.Equal(t, "line1\nline2\nline3\nline4\nline5", all)
}

func TestBackgroundShellCompletionHandlerFires(t *testing.T) {
	t.Parallel()

	manager := newBackgroundShellManager()
	manager.SetLogDir(t.TempDir())

	reported := make(chan *BackgroundShell, 1)
	manager.SetCompletionHandler(func(bs *BackgroundShell) {
		reported <- bs
	})

	bgShell, err := manager.Start(t.Context(), StartOptions{
		WorkingDir: t.TempDir(),
		Command:    "echo done",
		SessionID:  "session-1",
	})
	require.NoError(t, err)

	select {
	case got := <-reported:
		require.Equal(t, bgShell.ID, got.ID)
		require.Equal(t, "session-1", got.SessionID)
		// The job must already read as finished by the time it is
		// reported, or a handler cannot describe its outcome.
		require.True(t, got.IsDone(), "job must be done before it is reported")
		require.False(t, got.Killed())
		output, _, err := got.ReadFrom(0)
		require.NoError(t, err)
		require.Contains(t, output, "done", "output must be flushed before reporting")
	case <-time.After(10 * time.Second):
		t.Fatal("completion handler never fired")
	}
}

func TestBackgroundShellCompletionHandlerMarksKilled(t *testing.T) {
	t.Parallel()

	manager := newBackgroundShellManager()
	manager.SetLogDir(t.TempDir())

	reported := make(chan *BackgroundShell, 1)
	manager.SetCompletionHandler(func(bs *BackgroundShell) {
		reported <- bs
	})

	bgShell, err := manager.Start(t.Context(), StartOptions{
		WorkingDir: t.TempDir(),
		Command:    "sleep 30",
	})
	require.NoError(t, err)

	require.NoError(t, manager.Kill(bgShell.ID))

	select {
	case got := <-reported:
		require.True(t, got.Killed(), "a job stopped on request must be marked killed so it is not reported")
	case <-time.After(10 * time.Second):
		t.Fatal("completion handler never fired")
	}
}

func TestBackgroundShellBackgroundedDefaultsFalse(t *testing.T) {
	t.Parallel()

	manager := newBackgroundShellManager()
	manager.SetLogDir(t.TempDir())

	bgShell, err := manager.Start(t.Context(), StartOptions{
		WorkingDir: t.TempDir(),
		Command:    "echo quick",
	})
	require.NoError(t, err)
	t.Cleanup(func() { manager.Kill(bgShell.ID) })

	// A job only counts as backgrounded once its caller gives up waiting,
	// which is what keeps fast commands from reporting twice.
	require.False(t, bgShell.Backgrounded())
	bgShell.MarkBackgrounded()
	require.True(t, bgShell.Backgrounded())
}

func TestBackgroundShellRemoveDeletesLog(t *testing.T) {
	t.Parallel()

	manager := newBackgroundShellManager()
	manager.SetLogDir(t.TempDir())

	bgShell, err := manager.Start(t.Context(), StartOptions{
		WorkingDir: t.TempDir(),
		Command:    "echo bye",
	})
	require.NoError(t, err)
	waitForShell(t, bgShell)

	require.NotEmpty(t, bgShell.LogPath)
	require.FileExists(t, bgShell.LogPath)

	require.NoError(t, manager.Remove(bgShell.ID))
	_, err = os.Stat(bgShell.LogPath)
	require.True(t, os.IsNotExist(err), "removing a job must clean up its transcript")
}

func TestBackgroundShellJobsSnapshot(t *testing.T) {
	t.Parallel()

	manager := newBackgroundShellManager()
	manager.SetLogDir(t.TempDir())

	finished, err := manager.Start(t.Context(), StartOptions{
		WorkingDir:  t.TempDir(),
		Command:     "exit 3",
		Description: "failing job",
		SessionID:   "session-a",
	})
	require.NoError(t, err)
	waitForShell(t, finished)

	running, err := manager.Start(context.Background(), StartOptions{
		WorkingDir: t.TempDir(),
		Command:    "sleep 30",
		SessionID:  "session-b",
	})
	require.NoError(t, err)
	t.Cleanup(func() { manager.Kill(running.ID) })

	jobs := manager.Jobs()
	require.Len(t, jobs, 2)

	byID := map[string]BackgroundShellInfo{}
	for _, j := range jobs {
		byID[j.ID] = j
	}

	done := byID[finished.ID]
	require.True(t, done.Done)
	require.Equal(t, 3, done.ExitCode)
	require.Equal(t, "failing job", done.Description)
	require.Equal(t, "session-a", done.SessionID)

	live := byID[running.ID]
	require.False(t, live.Done)
	require.Equal(t, "session-b", live.SessionID)
}
