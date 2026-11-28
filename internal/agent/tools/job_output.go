package tools

import (
	"context"
	_ "embed"
	"fmt"
	"html/template"
	"strings"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/shell"
)

const (
	JobOutputToolName = "job_output"

	// jobOutputWaitLimit caps how long a wait=true call blocks, so a long
	// job hands control back to the agent instead of stalling the turn.
	// Keep job_output.md.tpl, bash.md.tpl, and the wait param in sync.
	jobOutputWaitLimit = 2 * time.Minute
)

//go:embed job_output.md.tpl
var jobOutputDescriptionTmpl string

var jobOutputDescriptionTpl = template.Must(template.New("jobOutputDescription").Parse(jobOutputDescriptionTmpl))

type JobOutputParams struct {
	ShellID string `json:"shell_id" description:"The ID of the background shell to retrieve output from"`
	Wait    bool   `json:"wait" description:"If true, block until the background shell completes, for at most 2 minutes, before returning output"`
}

type JobOutputResponseMetadata struct {
	ShellID          string `json:"shell_id"`
	Command          string `json:"command"`
	Description      string `json:"description"`
	Done             bool   `json:"done"`
	WorkingDirectory string `json:"working_directory"`
}

func NewJobOutputTool(spillDir string, jobs BackgroundJobs, progress pubsub.Publisher[ToolProgress]) fantasy.AgentTool {
	return newJobOutputTool(spillDir, jobs, progress, jobOutputWaitLimit)
}

// newJobOutputTool is NewJobOutputTool with an injectable wait limit, so
// tests need not wait the full two minutes.
func newJobOutputTool(spillDir string, jobs BackgroundJobs, progress pubsub.Publisher[ToolProgress], waitLimit time.Duration) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		JobOutputToolName,
		jobOutputToolDescription(jobs.Reports()),
		func(ctx context.Context, params JobOutputParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if params.ShellID == "" {
				return fantasy.NewTextErrorResponse("missing shell_id"), nil
			}

			bgManager := shell.GetBackgroundShellManager()
			scope, err := jobs.Scope(ctx)
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			bgShell, ok := bgManager.Lookup(scope, params.ShellID)
			if !ok {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("background shell not found: %s", params.ShellID)), nil
			}

			// Reading the result here stands in for the completion report,
			// so hold it back and drop it if this call sees the finish.
			endRead := bgShell.Read()
			if params.Wait {
				waitCtx, cancel := context.WithTimeout(ctx, waitLimit)
				waitWithProgress(waitCtx, bgShell, newProgressReporter(progress, GetSessionFromContext(ctx), call.ID))
				cancel()
			}

			result := ReadJobResult(bgShell, spillDir)
			// A cancelled call's result may never reach the model, so it
			// does not stand in for the report.
			endRead(result.Done && ctx.Err() == nil)

			status := "running"
			if result.Done {
				status = "completed"
			}

			metadata := JobOutputResponseMetadata{
				ShellID:          params.ShellID,
				Command:          bgShell.Command,
				Description:      bgShell.Description,
				Done:             result.Done,
				WorkingDirectory: bgShell.WorkingDir,
			}

			text := fmt.Sprintf("Status: %s\n\n%s", status, result.Text)
			if params.Wait && !result.Done && ctx.Err() == nil {
				text += stoppedWaitingNote(jobs.Reports())
			}
			return fantasy.WithResponseMetadata(fantasy.NewTextResponse(text), metadata), nil
		},
	)
}

// waitWithProgress blocks until the shell finishes or ctx is cancelled,
// publishing live output while it waits.
func waitWithProgress(ctx context.Context, bgShell *shell.BackgroundShell, progress *progressReporter) {
	select {
	case <-ctx.Done():
		return
	case <-bgShell.Done():
		return
	default:
	}
	progress.report(bgShell)

	ticker := time.NewTicker(progressPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-bgShell.Done():
			return
		case <-ticker.C:
			progress.report(bgShell)
		}
	}
}

// jobOutputToolDescription is the tool's description, which points the
// agent at job reports when they are on.
func jobOutputToolDescription(jobReports bool) string {
	var out strings.Builder
	if err := jobOutputDescriptionTpl.Execute(&out, struct{ JobReports bool }{jobReports}); err != nil {
		// The template is embedded, so this only fails on a broken build.
		panic("failed to execute job_output description template: " + err.Error())
	}
	return out.String()
}

// stoppedWaitingNote tells the agent a wait ran out with the job still
// going, and what to do instead of waiting again.
func stoppedWaitingNote(reports bool) string {
	if reports {
		return "\n\nStopped waiting: the job is still running and the output above is what it has printed so far. It will report back when it finishes, so keep working on other things or end your turn."
	}
	return "\n\nStopped waiting: the job is still running and the output above is what it has printed so far. Keep working on other things and check back; wait again only if nothing else is left to do."
}
