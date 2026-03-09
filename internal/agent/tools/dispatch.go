package tools

import (
	"context"
	_ "embed"
	"fmt"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/dispatch"
)

//go:embed dispatch.md
var dispatchDescription []byte

const DispatchToolName = "dispatch_task"

// DispatchParams contains the constrained parameters for dispatching a task.
// Workers must be registered in the system. Task content is defined by the system.
type DispatchParams struct {
	Worker    string         `json:"worker" description:"Name of the registered worker to dispatch the task to"`
	Task      string         `json:"task" description:"The task description for the worker to execute"`
	Variables map[string]any `json:"variables,omitempty" description:"Optional structured data to pass to the worker"`
}

type DispatchResponse struct {
	DispatchID string `json:"dispatch_id"`
	Worker     string `json:"worker"`
	Status     string `json:"status"`
}

func NewDispatchTool(dispatchSvc dispatch.Service) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		DispatchToolName,
		string(dispatchDescription),
		func(ctx context.Context, params DispatchParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			sessionID := GetSessionFromContext(ctx)

			// Use the constrained Dispatch method
			msg, err := dispatchSvc.Dispatch(ctx, dispatch.DispatchParams{
				Worker:    params.Worker,
				Task:      params.Task,
				Variables: params.Variables,
				SessionID: sessionID,
			})
			if err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("failed to dispatch task: %w", err)
			}

			response := fmt.Sprintf("Task dispatched to %s.\nDispatch ID: %s\nThe worker has been spawned and will process this task.", params.Worker, msg.ID)

			metadata := DispatchResponse{
				DispatchID: msg.ID,
				Worker:     params.Worker,
				Status:     string(msg.Status),
			}

			return fantasy.WithResponseMetadata(fantasy.NewTextResponse(response), metadata), nil
		})
}

// Note: check_dispatch and reply_dispatch are no longer agent tools.
// Workers interact with the dispatch system via the API:
//   - GET /dispatch/{id} - Read task
//   - POST /dispatch/{id}/result - Submit result
//
// These tools are kept for internal/host use only.
