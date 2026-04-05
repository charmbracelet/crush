package tools

import (
	"context"
	_ "embed"
	"fmt"
	"strings"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/toolruntime"
)

//go:embed subtask_result.md
var subtaskResultDescription []byte

const SubtaskResultToolName = "subtask_result"

type SubtaskResultParams struct {
	SessionID string `json:"session_id,omitempty" description:"The child session ID from a previous Agent tool call"`
	AgentID   string `json:"agent_id,omitempty" description:"The agent ID from a background agent (alternative to session_id)"`
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
			limit := params.Limit
			if limit <= 0 {
				limit = defaultLimit
			}
			if limit > maxLimit {
				limit = maxLimit
			}

			// Check for background agent lookup first.
			agentID := strings.TrimSpace(params.AgentID)
			if agentID != "" {
				lookup := toolruntime.BackgroundAgentLookupFromContext(ctx)
				if lookup == nil {
					return fantasy.NewTextErrorResponse("Background agent lookup is not available"), nil
				}
				status, content, childSessionID, found := lookup(agentID)
				if !found {
					return fantasy.NewTextErrorResponse(fmt.Sprintf("Background agent %q not found", agentID)), nil
				}
				if status == "running" {
					return fantasy.NewTextResponse(fmt.Sprintf("Background agent %q is still running. Try again later.", agentID)), nil
				}

				// If we have a child session, delegate to the full session result.
				if childSessionID != "" && messages != nil {
					msgs, err := messages.List(ctx, childSessionID)
					if err == nil {
						for i := len(msgs) - 1; i >= 0; i-- {
							if msgs[i].Role == message.Assistant && !msgs[i].IsSummaryMessage {
								text := strings.TrimSpace(msgs[i].Content().Text)
								if text != "" {
									content = text
									break
								}
							}
						}
					}
				}

				runes := []rune(content)
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
				result := string(runes[offset:end])
				if truncated {
					omitted := len(runes) - end
					result = fmt.Sprintf("%s\n\n[Output truncated: showing characters %d-%d of %d. %d characters omitted. Use offset/limit to paginate.]", result, offset, end, len(runes), omitted)
				}
				return fantasy.NewTextResponse(fmt.Sprintf("Agent %q (%s):\n\n%s", agentID, status, result)), nil
			}

			// Fall back to session-based lookup.
			sessionID := strings.TrimSpace(params.SessionID)
			if sessionID == "" {
				return fantasy.NewTextErrorResponse("session_id or agent_id is required"), nil
			}
			if messages == nil {
				return fantasy.ToolResponse{}, fmt.Errorf("message service is not configured")
			}

			msgs, err := messages.List(ctx, sessionID)
			if err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("Failed to load session %s: %s", sessionID, err)), nil
			}

			var b strings.Builder
			fmt.Fprintf(&b, "Session: %s\n\n", sessionID)

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
