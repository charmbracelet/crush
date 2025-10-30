package tools

import (
	"context"
	_ "embed"
	"fmt"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/shell"
)

const (
	BashKillToolName = "bash_kill"
)

//go:embed bash_kill.md
var bashKillDescription []byte

type BashKillParams struct {
	ShellID string `json:"shell_id" description:"The ID of the background shell to terminate"`
}

type BashKillResponseMetadata struct {
	ShellID string `json:"shell_id"`
}

func NewBashKillTool() fantasy.AgentTool {
	return fantasy.NewAgentTool(
		BashKillToolName,
		string(bashKillDescription),
		func(ctx context.Context, params BashKillParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if params.ShellID == "" {
				return fantasy.NewTextErrorResponse("missing shell_id"), nil
			}

			bgManager := shell.GetBackgroundShellManager()
			err := bgManager.Kill(params.ShellID)
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}

			metadata := BashKillResponseMetadata(params)

			result := fmt.Sprintf("Background shell %s terminated successfully", params.ShellID)
			return fantasy.WithResponseMetadata(fantasy.NewTextResponse(result), metadata), nil
		})
}
