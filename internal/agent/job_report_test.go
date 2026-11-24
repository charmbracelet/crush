package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/shell"
	"github.com/stretchr/testify/require"
)

func finishedJob(t *testing.T, command, description string) *shell.BackgroundShell {
	t.Helper()
	m := shell.GetBackgroundShellManager()
	job, err := m.Start(t.Context(), shell.JobScope{}, t.TempDir(), nil, shell.CoreutilsTurtle, command, description, 50)
	require.NoError(t, err)
	t.Cleanup(func() { _ = m.Kill(context.Background(), job.ID) })
	job.Wait()
	return job
}

func TestJobReport(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		job := finishedJob(t, "echo built", "build it")
		report := jobReport(job, t.TempDir())
		require.Equal(t, job.ID, report.Job)
		require.Equal(t, "build it", report.Label)
		require.False(t, report.Failed)
		require.Contains(t, report.Output, "Command: echo built\nCompleted after ")
		require.Contains(t, report.Output, "built")
	})

	t.Run("failure carries the exit code and stderr", func(t *testing.T) {
		t.Parallel()
		job := finishedJob(t, "echo oops >&2; exit 3", "")
		report := jobReport(job, t.TempDir())
		require.True(t, report.Failed)
		require.Contains(t, report.Output, "\nFailed after ")
		require.Contains(t, report.Output, "Exit code 3")
		require.Contains(t, report.Output, "oops")
		require.Equal(t, "echo oops >&2; exit 3", report.Label, "no description falls back to the command")
	})

	t.Run("no output says so", func(t *testing.T) {
		t.Parallel()
		report := jobReport(finishedJob(t, "true", "quiet"), t.TempDir())
		require.True(t, strings.HasSuffix(report.Output, "no output"), report.Output)
	})
}
