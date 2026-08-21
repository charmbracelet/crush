package tools

import (
	"context"
	_ "embed"
	"fmt"
	"strings"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/shell"
)

const (
	JobOutputToolName    = "job_output"
	DefaultWaitTimeout   = 60 // seconds: default max wait time when wait=true
	MaxWaitTimeout       = 600 // seconds: absolute max wait time (10 minutes)
)

//go:embed job_output.md
var jobOutputDescription string

type JobOutputParams struct {
	ShellID string `json:"shell_id" description:"The ID of the background shell to retrieve output from"`
	Wait    bool   `json:"wait" description:"If true, block until the background shell completes before returning output"`
	// TimeoutSeconds sets the maximum time to wait in seconds when wait=true.
	// Default: 60. Setting to 0 uses the default. Maximum: 600 (10 minutes).
	// For long-running processes (e.g. servers), set to a reasonable value
	// or use wait=false (the default) to poll periodically instead.
	TimeoutSeconds int `json:"timeout_seconds,omitempty" description:"Max wait time in seconds when wait=true (default: 60, max: 600)"`
}

type JobOutputResponseMetadata struct {
	ShellID          string `json:"shell_id"`
	Command          string `json:"command"`
	Description      string `json:"description"`
	Done             bool   `json:"done"`
	WorkingDirectory string `json:"working_directory"`
}

func NewJobOutputTool() fantasy.AgentTool {
	return fantasy.NewAgentTool(
		JobOutputToolName,
		jobOutputDescription,
		func(ctx context.Context, params JobOutputParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if params.ShellID == "" {
				return fantasy.NewTextErrorResponse("missing shell_id"), nil
			}

			bgManager := shell.GetBackgroundShellManager()
			bgShell, ok := bgManager.Get(params.ShellID)
			if !ok {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("background shell not found: %s", params.ShellID)), nil
			}

			if params.Wait {
				timeout := DefaultWaitTimeout
				if params.TimeoutSeconds > 0 {
					timeout = params.TimeoutSeconds
				}
				if timeout > MaxWaitTimeout {
					timeout = MaxWaitTimeout
				}
				waitCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
				defer cancel()
				bgShell.WaitContext(waitCtx)
			}

			stdout, stderr, done, err := bgShell.GetOutput()

			var outputParts []string
			if stdout != "" {
				outputParts = append(outputParts, stdout)
			}
			if stderr != "" {
				outputParts = append(outputParts, stderr)
			}

			status := "running"
			if done {
				status = "completed"
				if err != nil {
					exitCode := shell.ExitCode(err)
					if exitCode != 0 {
						outputParts = append(outputParts, fmt.Sprintf("Exit code %d", exitCode))
					}
				}
			}

			output := strings.Join(outputParts, "\n")
			output = TruncateOutput(output)

			metadata := JobOutputResponseMetadata{
				ShellID:          params.ShellID,
				Command:          bgShell.Command,
				Description:      bgShell.Description,
				Done:             done,
				WorkingDirectory: bgShell.WorkingDir,
			}

			if output == "" {
				output = BashNoOutput
			}

			result := fmt.Sprintf("Status: %s\n\n%s", status, output)
			return fantasy.WithResponseMetadata(fantasy.NewTextResponse(result), metadata), nil
		},
	)
}
