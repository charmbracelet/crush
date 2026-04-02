package tools

import (
	"context"
	_ "embed"
	"fmt"
	"strings"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/mailbox"
)

//go:embed task_stop.md
var taskStopDescription []byte

const TaskStopToolName = "task_stop"

type TaskStopParams struct {
	MailboxID string `json:"mailbox_id" description:"Mailbox identifier to request cancellation in"`
	TaskID    string `json:"task_id,omitempty" description:"Optional task ID to cancel; omit to cancel all tasks"`
	Reason    string `json:"reason,omitempty" description:"Optional cancellation reason"`
}

func NewTaskStopTool(service mailbox.Service) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		TaskStopToolName,
		string(taskStopDescription),
		func(_ context.Context, params TaskStopParams, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			mailboxID := strings.TrimSpace(params.MailboxID)
			if mailboxID == "" {
				return fantasy.NewTextErrorResponse("mailbox_id is required"), nil
			}
			if service == nil {
				return fantasy.ToolResponse{}, fmt.Errorf("mailbox service is not configured")
			}
			envelope, err := service.Stop(mailboxID, strings.TrimSpace(params.TaskID), strings.TrimSpace(params.Reason))
			if err != nil {
				return fantasy.NewTextErrorResponse(strings.TrimSpace(err.Error())), nil
			}
			if envelope.TargetTaskID == "" {
				return fantasy.NewTextResponse(fmt.Sprintf("Stop requested for mailbox %s.", envelope.MailboxID)), nil
			}
			return fantasy.NewTextResponse(fmt.Sprintf("Stop requested for task %s in mailbox %s.", envelope.TargetTaskID, envelope.MailboxID)), nil
		},
	)
}
