package tools

import (
	"context"
	_ "embed"
	"fmt"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/session"
)

//go:embed plan_exit.md
var planExitDescription []byte

const PlanExitToolName = "plan_exit"

func NewPlanExitTool(sessions session.Service) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		PlanExitToolName,
		string(planExitDescription),
		func(ctx context.Context, _ struct{}, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			sessionID := GetSessionFromContext(ctx)
			if sessionID == "" {
				return fantasy.ToolResponse{}, fmt.Errorf("session ID is required for plan_exit")
			}

			sess, err := sessions.Get(ctx, sessionID)
			if err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("failed to get session: %w", err)
			}
			if sess.CollaborationMode != session.CollaborationModePlan {
				return fantasy.NewTextErrorResponse("plan_exit can only be used in Plan Mode"), nil
			}

			return fantasy.NewTextResponse("Plan marked ready for review."), nil
		},
	)
}
