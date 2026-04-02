package tools

import (
	"context"
	_ "embed"
	"fmt"
	"strings"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/message"
)

//go:embed subtask_result.md
var subtaskResultDescription []byte

const SubtaskResultToolName = "subtask_result"

type SubtaskResultParams struct {
	SessionID string `json:"session_id" description:"The child session ID from a previous Agent tool call"`
	Offset    int    `json:"offset,omitempty" description:"Line offset to start from (0-based, for paginating long outputs)"`
	Limit     int    `json:"limit,omitempty" description:"Maximum number of characters to return (default 16000)"`
}

func NewSubtaskResultTool(messages message.Service) fantasy.AgentTool {
	const defaultLimit = 16_000
	const maxLimit = 64_000

	return fantasy.NewAgentTool(
		SubtaskResultToolName,
		string(subtaskResultDescription),
		func(ctx context.Context, params SubtaskResultParams, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			sessionID := strings.TrimSpace(params.SessionID)
			if sessionID == "" {
				return fantasy.NewTextErrorResponse("session_id is required"), nil
			}
			if messages == nil {
				return fantasy.ToolResponse{}, fmt.Errorf("message service is not configured")
			}

			limit := params.Limit
			if limit <= 0 {
				limit = defaultLimit
			}
			if limit > maxLimit {
				limit = maxLimit
			}

			msgs, err := messages.List(ctx, sessionID)
			if err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("Failed to load session %s: %s", sessionID, err)), nil
			}

			var b strings.Builder
			b.WriteString(fmt.Sprintf("Session: %s\n\n", sessionID))

			for i := len(msgs) - 1; i >= 0; i-- {
				msg := msgs[i]
				if msg.Role != message.Assistant || msg.IsSummaryMessage {
					continue
				}
				text := strings.TrimSpace(msg.Content().Text)
				if text == "" {
					continue
				}
				b.WriteString(text)
				break
			}

			if b.Len() == 0 {
				return fantasy.NewTextResponse(fmt.Sprintf("No assistant response found in session %s", sessionID)), nil
			}

			result := b.String()
			runes := []rune(result)
			offset := params.Offset
			if offset < 0 {
				offset = 0
			}
			if offset > len(runes) {
				offset = len(runes)
			}

			end := offset + limit
			if end > len(runes) {
				end = len(runes)
			}

			truncated := offset > 0 || end < len(runes)
			result = string(runes[offset:end])

			if truncated {
				omitted := len(runes) - end
				result = fmt.Sprintf("%s\n\n[Output truncated: showing characters %d-%d of %d. %d characters omitted. Use offset/limit to paginate.]", result, offset, end, len(runes), omitted)
			}

			return fantasy.NewTextResponse(result), nil
		},
	)
}