package app

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/crush/internal/shell"
)

// jobReportTailLines is how much of a finished job's output is quoted back
// into the conversation. Enough to see what happened, bounded so a chatty
// job cannot flood the context; the full transcript is linked instead.
const jobReportTailLines = 50

// setupJobReporting points background jobs at the project data directory and
// arranges for a finished job to announce itself.
//
// Background commands used to be write-only: the agent started one and then
// had to remember to come back and ask. Reporting on completion means a long
// build or test run behaves like any other piece of delegated work, and the
// conversation picks it up when it is actually ready.
func (app *App) setupJobReporting() {
	mgr := shell.GetBackgroundShellManager()
	mgr.SetLogDir(filepath.Join(app.config.Config().Options.DataDirectory, "jobs"))
	mgr.SetCompletionHandler(app.reportFinishedJob)
}

// reportFinishedJob delivers a completion report for one job. It runs on the
// job's own goroutine, so the delivery itself is handed to a new one.
func (app *App) reportFinishedJob(bs *shell.BackgroundShell) {
	if !shouldReportJob(bs) {
		return
	}

	coordinator := app.AgentCoordinator
	if coordinator == nil {
		return
	}

	report := formatJobReport(bs)
	go func() {
		// Detached from the caller's context: the job outlived whatever
		// started it, and the report should survive the same way. The
		// coordinator decides what happens next — folded into the running
		// turn if the session is busy, or a fresh turn if it is idle.
		ctx := context.WithoutCancel(app.globalCtx)
		if _, err := coordinator.Run(ctx, bs.SessionID, report); err != nil {
			slog.Error("Failed to report background job completion",
				"job", bs.ID, "session", bs.SessionID, "error", err)
		}
	}()
}

// shouldReportJob decides whether a finished job is worth interrupting the
// conversation for.
func shouldReportJob(bs *shell.BackgroundShell) bool {
	switch {
	case bs.Killed():
		// The caller stopped this one deliberately and already knows.
		return false
	case !bs.Backgrounded():
		// The command finished inside its caller's waiting window, so its
		// output was returned inline and a report would just repeat it.
		return false
	case bs.SessionID == "":
		// Nothing to report back to.
		return false
	default:
		return true
	}
}

// formatJobReport renders the message the agent receives when a job ends.
func formatJobReport(bs *shell.BackgroundShell) string {
	_, _, _, execErr := bs.GetOutput()
	exitCode := shell.ExitCode(execErr)

	outcome := "finished successfully"
	if exitCode != 0 {
		outcome = fmt.Sprintf("failed with exit code %d", exitCode)
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "<background-job-report id=%q>\n", bs.ID)
	fmt.Fprintf(&sb, "Command: %s\n", bs.Command)
	if bs.Description != "" {
		fmt.Fprintf(&sb, "Description: %s\n", bs.Description)
	}
	fmt.Fprintf(&sb, "Outcome: %s\n", outcome)
	if elapsed := jobElapsed(bs); elapsed > 0 {
		fmt.Fprintf(&sb, "Duration: %s\n", elapsed.Round(time.Second))
	}

	tail, truncated := bs.Tail(jobReportTailLines)
	switch {
	case tail == "":
		sb.WriteString("\nNo output.\n")
	case truncated:
		fmt.Fprintf(&sb, "\nLast %d lines:\n%s\n", jobReportTailLines, tail)
	default:
		fmt.Fprintf(&sb, "\nOutput:\n%s\n", tail)
	}

	if bs.LogPath != "" && truncated {
		fmt.Fprintf(&sb, "\nFull transcript: %s\n", bs.LogPath)
	}

	sb.WriteString("\nThis job was started earlier and has now completed. " +
		"Continue what you were doing, taking the result into account. " +
		"Only mention it to the user if it changes something.\n")
	sb.WriteString("</background-job-report>")
	return sb.String()
}

func jobElapsed(bs *shell.BackgroundShell) time.Duration {
	completed := bs.CompletedAt()
	if bs.StartedAt.IsZero() || completed.IsZero() {
		return 0
	}
	return completed.Sub(bs.StartedAt)
}
