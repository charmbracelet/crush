package tools

import (
	"context"
	_ "embed"
	"fmt"
	"strings"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/shell"
)

const (
	JobOutputToolName = "job_output"
)

//go:embed job_output.md
var jobOutputDescription string

type JobOutputParams struct {
	ShellID string `json:"shell_id" description:"The ID of the background shell to retrieve output from"`
	Wait    bool   `json:"wait" description:"If true, block until the background shell completes before returning output"`
	Offset  int64  `json:"offset" description:"Byte offset to read from. Pass the next_offset from a previous call to see only new output. Defaults to 0 (the whole transcript)."`
}

type JobOutputResponseMetadata struct {
	ShellID          string `json:"shell_id"`
	Command          string `json:"command"`
	Description      string `json:"description"`
	Done             bool   `json:"done"`
	WorkingDirectory string `json:"working_directory"`
	// NextOffset is the byte offset to pass on the next call to read only
	// what has been written since this one.
	NextOffset int64 `json:"next_offset"`
	// LogPath is the full transcript on disk, readable with the normal file
	// tools when the output is too big to return inline.
	LogPath string `json:"log_path,omitempty"`
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
				bgShell.WaitContext(ctx)
			}

			output, nextOffset, err := bgShell.ReadFrom(params.Offset)
			if err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("failed to read output for job %s: %v", params.ShellID, err)), nil
			}

			done := bgShell.IsDone()
			status := "running"
			if done {
				status = "completed"
				if _, _, _, execErr := bgShell.GetOutput(); execErr != nil {
					if exitCode := shell.ExitCode(execErr); exitCode != 0 {
						output = strings.TrimRight(output, "\n") + fmt.Sprintf("\nExit code %d", exitCode)
					}
				}
			}

			truncated := false
			if full := TruncateOutput(output); full != output {
				output = full
				truncated = true
			}

			metadata := JobOutputResponseMetadata{
				ShellID:          params.ShellID,
				Command:          bgShell.Command,
				Description:      bgShell.Description,
				Done:             done,
				WorkingDirectory: bgShell.WorkingDir,
				NextOffset:       nextOffset,
				LogPath:          bgShell.LogPath,
			}

			if strings.TrimSpace(output) == "" {
				if params.Offset > 0 {
					output = "no new output"
				} else {
					output = BashNoOutput
				}
			}

			var footer strings.Builder
			if !done {
				fmt.Fprintf(&footer, "\n\nPass offset=%d next time to read only new output.", nextOffset)
			}
			if truncated && bgShell.LogPath != "" {
				fmt.Fprintf(&footer, "\n\nFull transcript: %s", bgShell.LogPath)
			}

			result := fmt.Sprintf("Status: %s\n\n%s%s", status, output, footer.String())
			return fantasy.WithResponseMetadata(fantasy.NewTextResponse(result), metadata), nil
		},
	)
}
