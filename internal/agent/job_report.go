package agent

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/shell"
)

// reportJob tells sessionID that a background job it started has finished:
// folded into a turn in progress, or starting one, so the agent need not
// poll or block on the job. Jobs that finish together each deliver their
// own report; the session lock serializes them into the next step.
func (c *coordinator) reportJob(sessionID string, job *shell.BackgroundShell) {
	// The session may have been deleted while the job ran, and waking it
	// would only build a model to fail on the missing row.
	if _, err := c.sessions.Get(context.Background(), sessionID); err != nil {
		slog.Debug("Dropping a job report for a missing session", "job", job.ID, "session", sessionID, "error", err)
		return
	}
	c.deliverReport(sessionID, jobReport(job, c.cfg.Config().Options.DataDirectory))
}

// jobReport describes a finished job: its command, how it ended, how long
// it ran, and its output as job_output shows it.
func jobReport(job *shell.BackgroundShell, spillDir string) message.SubAgentReport {
	result := tools.ReadJobResult(job, spillDir)
	outcome := "Completed"
	if result.Failed {
		outcome = "Failed"
	}
	if done := job.CompletedAt(); !done.IsZero() {
		outcome += fmt.Sprintf(" after %s", done.Sub(job.StartedAt).Round(time.Second))
	}
	return message.SubAgentReport{
		Label:  tools.JobLabel(job),
		Output: fmt.Sprintf("Command: %s\n%s\n\n%s", job.Command, outcome, result.Text),
		Failed: result.Failed,
		Job:    job.ID,
	}
}
