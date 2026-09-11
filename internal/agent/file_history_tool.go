package agent

import (
	"context"
	"encoding/json"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/filehistory"
	"github.com/charmbracelet/crush/internal/filepathext"
)

// fileHistoryTool runs inside hook interception, after input rewriting.
type fileHistoryTool struct{ fantasy.AgentTool }

func (t *fileHistoryTool) Run(ctx context.Context, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
	cwd, enabled := filehistory.WorkingDir(ctx)
	if enabled && (call.Name == "edit" || call.Name == "multiedit" || call.Name == "write") {
		var input struct {
			FilePath string `json:"file_path"`
		}
		if err := json.Unmarshal([]byte(call.Input), &input); err != nil {
			return fantasy.ToolResponse{}, err
		}
		if input.FilePath != "" {
			if err := filehistory.Declare(ctx, filepathext.SmartJoin(cwd, input.FilePath)); err != nil {
				response := fantasy.NewTextErrorResponse("File history could not save the preimage: " + err.Error())
				response.StopTurn = true
				return response, nil
			}
		}
	}
	return t.AgentTool.Run(ctx, call)
}
