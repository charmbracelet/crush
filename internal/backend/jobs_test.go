package backend

import (
	"context"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/shell"
	"github.com/stretchr/testify/require"
)

// Job của workspace khác (cùng tiến trình server) không xem/huỷ được qua
// workspace này; job của chính nó thì được.
func TestJobsScopedToWorkspace(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	ws, _ := insertTestWorkspace(t, b, t.TempDir())
	other := t.TempDir()

	m := shell.GetBackgroundShellManager()
	mine, err := m.StartFor(context.Background(), shell.JobOwner{SessionID: "s1"}, ws.Path, nil, "sleep 30", "mine")
	require.NoError(t, err)
	foreign, err := m.StartFor(context.Background(), shell.JobOwner{}, other, nil, "sleep 30", "foreign")
	require.NoError(t, err)
	t.Cleanup(func() { _ = m.Kill(mine.ID); _ = m.Kill(foreign.ID) })

	require.Eventually(t, func() bool {
		jobs, err := b.ListJobs(ws.ID)
		return err == nil && len(jobs) == 1 && jobs[0].ID == mine.ID
	}, 5*time.Second, 10*time.Millisecond)

	_, err = b.JobOutput(ws.ID, foreign.ID)
	require.ErrorIs(t, err, ErrJobNotFound)
	require.ErrorIs(t, b.KillJob(t.Context(), ws.ID, foreign.ID), ErrJobNotFound)
	require.False(t, foreign.IsDone(), "a foreign job must not be killed")

	require.NoError(t, b.KillJob(t.Context(), ws.ID, mine.ID))
	require.True(t, mine.IsDone())
	_, stillListed := m.Get(mine.ID)
	require.True(t, stillListed, "a user stop keeps the job for the agent's job_output")
	require.NoError(t, b.KillJob(t.Context(), ws.ID, mine.ID), "stopping a finished job is a no-op")

	jobs, err := b.ListJobs(ws.ID)
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	require.Equal(t, shell.JobKilled, jobs[0].Status)
}

// Server bị yêu cầu dừng: mọi workspace được dừng (run bị huỷ và ghi lại),
// rồi mới tới hook dừng server (port upstream b1b34091).
func TestShutdownTearsDownEveryWorkspace(t *testing.T) {
	t.Parallel()
	b, serverShutdowns := newTestBackend(t)
	_, first := insertTestWorkspace(t, b, t.TempDir())
	_, second := insertTestWorkspace(t, b, t.TempDir())

	b.Shutdown()
	require.EqualValues(t, 1, first.Load())
	require.EqualValues(t, 1, second.Load())
	require.EqualValues(t, 1, serverShutdowns.Load())
}
