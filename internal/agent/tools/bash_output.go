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
	BashOutputToolName = "bash_output"
)

//go:embed bash_output.md
var bashOutputDescription []byte

type BashOutputParams struct {
	ShellID string `json:"shell_id" description:"The ID of the background shell to retrieve output from"`
}

type BashOutputResponseMetadata struct {
	ShellID          string `json:"shell_id"`
	Done             bool   `json:"done"`
	WorkingDirectory string `json:"working_directory"`
}

func NewBashOutputTool() fantasy.AgentTool {
	return fantasy.NewAgentTool(
		BashOutputToolName,
		string(bashOutputDescription),
		func(ctx context.Context, params BashOutputParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if params.ShellID == "" {
				return fantasy.NewTextErrorResponse("missing shell_id"), nil
			}

			bgManager := shell.GetBackgroundShellManager()
			bgShell, ok := bgManager.Get(params.ShellID)
			if !ok {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("background shell not found: %s", params.ShellID)), nil
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

			metadata := BashOutputResponseMetadata{
				ShellID:          params.ShellID,
				Done:             done,
				WorkingDirectory: bgShell.GetWorkingDir(),
			}

			if output == "" {
				output = BashNoOutput
			}

			result := fmt.Sprintf("Shell ID: %s\nStatus: %s\n\nOutput:\n%s", params.ShellID, status, output)
			return fantasy.WithResponseMetadata(fantasy.NewTextResponse(result), metadata), nil
		})
}
