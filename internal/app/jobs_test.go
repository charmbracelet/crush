package app

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/shell"
	"github.com/stretchr/testify/require"
)

// The background shell manager is a process-wide singleton, so its log
// directory is set once for the whole package. Setting it per test would let
// parallel tests redirect each other's transcripts.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "crush-job-logs")
	if err != nil {
		panic(err)
	}
	shell.GetBackgroundShellManager().SetLogDir(dir)
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// startTestJob runs a command to completion on the shared manager and returns
// the finished job, cleaning it up afterwards.
func startTestJob(t *testing.T, command, description, sessionID string) *shell.BackgroundShell {
	t.Helper()

	mgr := shell.GetBackgroundShellManager()
	bs, err := mgr.Start(t.Context(), shell.StartOptions{
		WorkingDir:  t.TempDir(),
		Command:     command,
		Description: description,
		SessionID:   sessionID,
	})
	require.NoError(t, err)
	t.Cleanup(func() { mgr.Remove(bs.ID) })

	require.Eventually(t, bs.IsDone, 10*time.Second, 10*time.Millisecond)
	return bs
}

func TestShouldReportJob(t *testing.T) {
	t.Parallel()

	t.Run("reports a backgrounded job that finished on its own", func(t *testing.T) {
		bs := startTestJob(t, "echo hi", "", "session-1")
		bs.MarkBackgrounded()
		require.True(t, shouldReportJob(bs))
	})

	t.Run("stays quiet about a job that never went to the background", func(t *testing.T) {
		// A fast command has its output returned inline, so reporting it
		// again would just repeat what the caller already saw.
		bs := startTestJob(t, "echo hi", "", "session-1")
		require.False(t, shouldReportJob(bs))
	})

	t.Run("stays quiet about a job with no session", func(t *testing.T) {
		bs := startTestJob(t, "echo hi", "", "")
		bs.MarkBackgrounded()
		require.False(t, shouldReportJob(bs))
	})
}

func TestFormatJobReportSuccess(t *testing.T) {
	t.Parallel()

	bs := startTestJob(t, "echo all good", "run the thing", "session-1")
	report := formatJobReport(bs)

	require.Contains(t, report, "<background-job-report id=")
	require.Contains(t, report, bs.ID)
	require.Contains(t, report, "Command: echo all good")
	require.Contains(t, report, "Description: run the thing")
	require.Contains(t, report, "Outcome: finished successfully")
	require.Contains(t, report, "all good")
	require.True(t, strings.HasSuffix(report, "</background-job-report>"))
}

func TestFormatJobReportFailureCarriesExitCode(t *testing.T) {
	t.Parallel()

	bs := startTestJob(t, "echo boom >&2; exit 7", "", "session-1")
	report := formatJobReport(bs)

	require.Contains(t, report, "Outcome: failed with exit code 7")
	require.Contains(t, report, "boom")
}

func TestFormatJobReportTruncatesAndLinksTranscript(t *testing.T) {
	t.Parallel()

	bs := startTestJob(t, "for i in $(seq 1 400); do echo line$i; done", "", "session-1")
	report := formatJobReport(bs)

	require.Contains(t, report, "Last 50 lines:")
	require.Contains(t, report, "line400", "the tail must include the end of the output")
	require.NotContains(t, report, "line1\n", "the tail must drop the start of the output")
	require.Contains(t, report, "Full transcript: "+bs.LogPath)
}

func TestFormatJobReportHandlesNoOutput(t *testing.T) {
	t.Parallel()

	bs := startTestJob(t, "true", "", "session-1")
	report := formatJobReport(bs)

	require.Contains(t, report, "No output.")
	require.NotContains(t, report, "Full transcript:")
}
