package tools

import (
	"context"
	_ "embed"
	"fmt"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/shell"
)

const (
	JobWaitToolName = "job_wait"
)

//go:embed job_wait.md
var jobWaitDescription []byte

type JobWaitParams struct {
	ShellID string `json:"shell_id" description:"The ID of the background shell to wait for"`
}

type JobWaitResponseMetadata struct {
	ShellID          string `json:"shell_id"`
	Command          string `json:"command"`
	Description      string `json:"description"`
	Done             bool   `json:"done"`
	ExitCode         int    `json:"exit_code,omitempty"`
	WorkingDirectory string `json:"working_directory"`
}

func NewJobWaitTool() fantasy.AgentTool {
	return fantasy.NewAgentTool(
		JobWaitToolName,
		string(jobWaitDescription),
		func(ctx context.Context, params JobWaitParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if params.ShellID == "" {
				return fantasy.NewTextErrorResponse("missing shell_id"), nil
			}

			bgManager := shell.GetBackgroundShellManager()
			bgShell, ok := bgManager.Get(params.ShellID)
			if !ok {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("background shell not found: %s", params.ShellID)), nil
			}

			if !bgShell.WaitContext(ctx) {
				return fantasy.ToolResponse{}, ctx.Err()
			}

			result, metadata := formatJobToolResponse(bgShell, params.ShellID, false)
			jobWaitMetadata := JobWaitResponseMetadata{
				ShellID:          metadata.ShellID,
				Command:          metadata.Command,
				Description:      metadata.Description,
				Done:             metadata.Done,
				ExitCode:         metadata.ExitCode,
				WorkingDirectory: metadata.WorkingDirectory,
			}
			return fantasy.WithResponseMetadata(fantasy.NewTextResponse(result), jobWaitMetadata), nil
		},
	)
}
