package tools

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/filepathext"
)

//go:embed save_plan.md
var savePlanDescription string

type SavePlanParams struct {
	FilePath string `json:"file_path" description:"The file path to save the plan to (e.g. plan.md)"`
	Content  string `json:"content" description:"The plan content in markdown format"`
}

const SavePlanToolName = "save_plan"

func NewSavePlanTool(workingDir string) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		SavePlanToolName,
		savePlanDescription,
		func(ctx context.Context, params SavePlanParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if params.FilePath == "" {
				return fantasy.NewTextErrorResponse("file_path is required"), nil
			}
			if params.Content == "" {
				return fantasy.NewTextErrorResponse("content is required"), nil
			}

			targetPath := filepathext.SmartJoin(workingDir, params.FilePath)
			dir := filepath.Dir(targetPath)
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("error creating directory: %w", err)
			}

			if err := os.WriteFile(targetPath, []byte(params.Content), 0o644); err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("error writing plan file: %w", err)
			}

			return fantasy.NewTextResponse(fmt.Sprintf("Plan successfully saved to %s", targetPath)), nil
		},
	)
}
