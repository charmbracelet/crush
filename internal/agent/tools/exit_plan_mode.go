package tools

import (
	"context"
	_ "embed"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/permission"
)

//go:embed exit_plan_mode.md
var exitPlanModeDescription string

const ExitPlanModeToolName = "exit_plan_mode"

type ExitPlanModeParams struct {
	Plan string `json:"plan" description:"The full implementation plan, in markdown, to present to the user for approval"`
}

type ExitPlanModePermissionsParams struct {
	Plan string `json:"plan"`
}

func NewExitPlanModeTool(permissions permission.Service) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		ExitPlanModeToolName,
		exitPlanModeDescription,
		func(ctx context.Context, params ExitPlanModeParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if !permissions.PlanMode() {
				return fantasy.NewTextErrorResponse("Not in plan mode; exit_plan_mode is only valid while plan mode is active."), nil
			}
			if params.Plan == "" {
				return fantasy.NewTextErrorResponse("plan is required"), nil
			}

			p, err := permissions.Request(
				ctx,
				permission.CreatePermissionRequest{
					SessionID:   GetSessionFromContext(ctx),
					ToolCallID:  call.ID,
					ToolName:    ExitPlanModeToolName,
					Action:      "plan",
					Description: "Approve the implementation plan and exit plan mode",
					Params: ExitPlanModePermissionsParams{
						Plan: params.Plan,
					},
				},
			)
			if err != nil {
				return fantasy.ToolResponse{}, err
			}
			if !p {
				return NewPermissionDeniedResponse(), nil
			}

			permissions.SetPlanMode(false)
			return fantasy.NewTextResponse("Plan approved. Plan mode is now off — proceed with the implementation."), nil
		},
	)
}
