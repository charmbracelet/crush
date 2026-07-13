package tools

import (
	"context"
	_ "embed"
	"fmt"
	"strings"

	"charm.land/fantasy"
)

//go:embed goal_status.md
var goalStatusDescription string

const GoalStatusToolName = "goal_status"

type GoalStatusParams struct {
	Status  string `json:"status" description:"Terminal goal status: complete or blocked"`
	Summary string `json:"summary" description:"Concise verified outcome or exact external blocker"`
}

type GoalStatusMetadata struct {
	Status  string `json:"status"`
	Summary string `json:"summary"`
}

func NewGoalStatusTool() fantasy.AgentTool {
	return fantasy.NewAgentTool(
		GoalStatusToolName,
		goalStatusDescription,
		func(_ context.Context, params GoalStatusParams, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			status := strings.ToLower(strings.TrimSpace(params.Status))
			if status != "complete" && status != "blocked" {
				return fantasy.ToolResponse{}, fmt.Errorf("invalid goal status %q: use complete or blocked", params.Status)
			}
			summary := strings.TrimSpace(params.Summary)
			if summary == "" {
				return fantasy.ToolResponse{}, fmt.Errorf("goal status summary is required")
			}

			response := fantasy.NewTextResponse(fmt.Sprintf("Goal %s: %s", status, summary))
			return fantasy.WithResponseMetadata(response, GoalStatusMetadata{
				Status:  status,
				Summary: summary,
			}), nil
		},
	)
}
