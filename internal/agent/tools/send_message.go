package tools

import (
	"context"
	_ "embed"
	"fmt"
	"strings"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/mailbox"
	"github.com/charmbracelet/crush/internal/toolruntime"
)

//go:embed send_message.md
var sendMessageDescription []byte

const SendMessageToolName = "send_message"

type SendMessageParams struct {
	MailboxID string `json:"mailbox_id" description:"Mailbox identifier to deliver messages to"`
	TaskID    string `json:"task_id,omitempty" description:"Optional task ID for targeted delivery; omit to broadcast"`
	Message   string `json:"message" description:"Message content to deliver"`
}

func NewSendMessageTool(service mailbox.Service) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		SendMessageToolName,
		string(sendMessageDescription),
		func(ctx context.Context, params SendMessageParams, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			mailboxID := strings.TrimSpace(params.MailboxID)
			if mailboxID == "" {
				mailboxID = toolruntime.DelegationMailboxFromContext(ctx)
			}
			if mailboxID == "" {
				return fantasy.NewTextErrorResponse("mailbox_id is required (or configure it via delegation context)"), nil
			}
			message := strings.TrimSpace(params.Message)
			if message == "" {
				return fantasy.NewTextErrorResponse("message is required"), nil
			}
			if service == nil {
				return fantasy.ToolResponse{}, fmt.Errorf("mailbox service is not configured")
			}

			envelope, err := service.Send(mailboxID, strings.TrimSpace(params.TaskID), message)
			if err != nil {
				return fantasy.NewTextErrorResponse(strings.TrimSpace(err.Error())), nil
			}
			if envelope.TargetTaskID == "" {
				return fantasy.NewTextResponse(fmt.Sprintf("Message sent to mailbox %s.", envelope.MailboxID)), nil
			}
			return fantasy.NewTextResponse(fmt.Sprintf("Message sent to task %s in mailbox %s.", envelope.TargetTaskID, envelope.MailboxID)), nil
		},
	)
}
