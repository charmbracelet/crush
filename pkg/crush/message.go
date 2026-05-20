package crush

import "github.com/charmbracelet/crush/internal/message"

type (
	Message             = message.Message
	MessageRole         = message.MessageRole
	ContentPart         = message.ContentPart
	TextContent         = message.TextContent
	ReasoningContent    = message.ReasoningContent
	ImageURLContent     = message.ImageURLContent
	BinaryContent       = message.BinaryContent
	ToolCall            = message.ToolCall
	ToolResult          = message.ToolResult
	Finish              = message.Finish
	FinishReason        = message.FinishReason
	Attachment          = message.Attachment
	CreateMessageParams = message.CreateMessageParams
	MessageService      = message.Service
)

const (
	RoleAssistant = message.Assistant
	RoleUser      = message.User
	RoleSystem    = message.System
	RoleTool      = message.Tool

	FinishReasonEndTurn   = message.FinishReasonEndTurn
	FinishReasonMaxTokens = message.FinishReasonMaxTokens
	FinishReasonToolUse   = message.FinishReasonToolUse
	FinishReasonCanceled  = message.FinishReasonCanceled
	FinishReasonError     = message.FinishReasonError
	FinishReasonUnknown   = message.FinishReasonUnknown
)
