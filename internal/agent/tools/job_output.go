package tools

import (
	"context"
	_ "embed"
	"fmt"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/shell"
)

const (
	JobOutputToolName = "job_output"
)

//go:embed job_output.md
var jobOutputDescription []byte

type JobOutputParams struct {
	ShellID string `json:"shell_id" description:"The ID of the background shell to retrieve output from"`
	Wait    bool   `json:"wait" description:"If true, block until the background shell completes before returning output"`
}

type JobOutputResponseMetadata struct {
	ShellID          string   `json:"shell_id"`
	Command          string   `json:"command"`
	Description      string   `json:"description"`
	Done             bool     `json:"done"`
	ExitCode         int      `json:"exit_code,omitempty"`
	WorkingDirectory string   `json:"working_directory"`
	DeprecationNotes []string `json:"deprecation_notes,omitempty"`
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

			if params.Wait {
				if !bgShell.WaitContext(ctx) {
					// Context was cancelled while waiting; propagate the
					// cancellation so the agent's error handler creates
					// the required tool-result message.
					return fantasy.ToolResponse{}, ctx.Err()
				}
			}

			result, meta := formatJobToolResponse(bgShell, params.ShellID, params.Wait)
			metadata := JobOutputResponseMetadata(meta)
			return fantasy.WithResponseMetadata(fantasy.NewTextResponse(result), metadata), nil
		})
}
