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
	JobOutputToolName = "job_output"
)

//go:embed job_output.md
var jobOutputDescription []byte

type JobOutputParams struct {
	ShellID     string `json:"shell_id" description:"The ID of the background shell to retrieve output from"`
	MaxWaitTime *int   `json:"max_wait_time,omitempty" description:"Maximum time in seconds to wait for the job to complete. If not set or set to 0, will return immediately without waiting"`
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
		string(jobOutputDescription),
		func(ctx context.Context, params JobOutputParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if params.ShellID == "" {
				return fantasy.NewTextErrorResponse("missing shell_id"), nil
			}

			bgManager := shell.GetBackgroundShellManager()
			bgShell, ok := bgManager.Get(params.ShellID)
			if !ok {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("background shell not found: %s", params.ShellID)), nil
			}

			// Get initial output
			stdout, stderr, done, err := bgShell.GetOutput()

			// If job is not done and max_wait_time is specified, wait for completion
			if !done && params.MaxWaitTime != nil && *params.MaxWaitTime > 0 {
				waitTime := time.Duration(*params.MaxWaitTime) * time.Second
				waitDone := make(chan bool, 1)

				// Start a goroutine to wait for the job to complete
				go func() {
					bgShell.Wait()
					waitDone <- true
				}()

				// Wait for either completion or timeout
				select {
				case <-waitDone:
					// Job completed, get final output
					stdout, stderr, done, err = bgShell.GetOutput()
				case <-time.After(waitTime):
					// Timeout occurred, job is still running
					// Get the most recent output before timeout
					stdout, stderr, done, err = bgShell.GetOutput()
				case <-ctx.Done():
					// Context was cancelled
					return fantasy.NewTextErrorResponse("operation cancelled"), nil
				}
			}

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
			} else if params.MaxWaitTime != nil && *params.MaxWaitTime > 0 {
				// Job is still running after waiting period
				outputParts = append(outputParts, fmt.Sprintf("Task is still running after waiting %d seconds. Try calling again with a longer wait time.", *params.MaxWaitTime))
			}

			output := strings.Join(outputParts, "\n")

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
		})
}
