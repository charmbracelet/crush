package shell

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// reportCounter counts completion reports so a test can tell one report
// from none, and from a duplicate.
type reportCounter chan struct{}

func (c reportCounter) report() { c <- struct{}{} }

// got reports whether a report arrives within a moment.
func (c reportCounter) got() bool {
	select {
	case <-c:
		return true
	case <-time.After(time.Second):
		return false
	}
}

func startReporting(t *testing.T, m *BackgroundShellManager, command string) (*BackgroundShell, reportCounter) {
	t.Helper()
	bgShell, err := m.Start(t.Context(), JobScope{}, t.TempDir(), nil, CoreutilsTurtle, command, "", testMaxJobs)
	require.NoError(t, err)
	t.Cleanup(func() { _ = m.Kill(context.Background(), bgShell.ID) })
	reports := make(reportCounter, 4)
	bgShell.ReportWhenDone(reports.report)
	return bgShell, reports
}

func TestBackgroundShell_ReportWhenDone(t *testing.T) {
	t.Parallel()
	m := newBackgroundShellManager()

	t.Run("reports once when it finishes", func(t *testing.T) {
		t.Parallel()
		bgShell, reports := startReporting(t, m, "echo hi")
		bgShell.Wait()
		require.True(t, reports.got())
		require.False(t, reports.got(), "a job reports once")
	})

	t.Run("a killed job stays quiet", func(t *testing.T) {
		t.Parallel()
		bgShell, reports := startReporting(t, m, "sleep 100")
		require.NoError(t, m.Kill(t.Context(), bgShell.ID))
		require.False(t, reports.got())
	})

	t.Run("a job killed at shutdown stays quiet", func(t *testing.T) {
		t.Parallel()
		owner := new(int)
		bgShell, err := m.Start(t.Context(), JobScope{Owner: owner}, t.TempDir(), nil, CoreutilsTurtle, "sleep 100", "", testMaxJobs)
		require.NoError(t, err)
		reports := make(reportCounter, 4)
		bgShell.ReportWhenDone(reports.report)
		m.KillOwned(t.Context(), owner)
		require.False(t, reports.got())
	})

	t.Run("a reader that saw it finish takes the report's place", func(t *testing.T) {
		t.Parallel()
		bgShell, reports := startReporting(t, m, "sleep 0.2")
		done := bgShell.Read()
		bgShell.Wait()
		done(true)
		require.False(t, reports.got())
	})

	t.Run("a job that finishes after the reader looked still reports", func(t *testing.T) {
		t.Parallel()
		bgShell, reports := startReporting(t, m, "sleep 0.2")
		done := bgShell.Read()
		bgShell.Wait()
		// The reader read before the exit, so it did not see the finish.
		done(false)
		require.True(t, reports.got())
	})

	t.Run("a reader that never saw it finish leaves the report alone", func(t *testing.T) {
		t.Parallel()
		bgShell, reports := startReporting(t, m, "sleep 0.3")
		bgShell.Read()(false)
		bgShell.Wait()
		require.True(t, reports.got())
	})
}
