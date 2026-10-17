package tools

import (
	"context"
	_ "embed"
	"fmt"
	"strings"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/honcho"
)

//go:embed honcho_chat.md
var honchoChatDescription string

type HonchoChatParams struct {
	Query string `json:"query" description:"A natural-language question about the user, their preferences, or their past decisions."`
}

type HonchoChatResponseMetadata struct {
	Query    string `json:"query"`
	Observer string `json:"observer"`
	Target   string `json:"target,omitempty"`
	Answered bool   `json:"answered"`
}

// NewHonchoChatTool builds the dialectic tool: it asks Honcho's
// derived model of the user a question and returns prose.
//
// The observer and target come from the configured observation mode.
// In unified mode the user's own collection answers, so several agents
// sharing a workspace draw on the same picture; in directional mode
// Crush asks its private view of the user.
func NewHonchoChatTool(svc *honcho.Service) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		HonchoChatToolName,
		honchoChatDescription,
		func(ctx context.Context, params HonchoChatParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if svc == nil {
				return memoryNotConfigured(), nil
			}
			query := strings.TrimSpace(params.Query)
			if query == "" {
				return fantasy.NewTextErrorResponse("query is required"), nil
			}

			mode := svc.Config().ObservationMode
			id := svc.Identity()
			observer := id.ObserverPeer(mode)
			target := id.ChatTarget(mode)

			answer, err := svc.Client().Chat(ctx, observer, honcho.ChatQuery{
				Query:     query,
				Target:    target,
				SessionID: id.SessionKey,
				// Low reasoning keeps this call usable inside a turn.
				// The high setting exists for offline analysis, not
				// for a model waiting on a tool result.
				ReasoningLevel: honcho.ReasoningLow,
			})
			if err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("memory query failed: %v", err)), nil
			}

			answer = strings.TrimSpace(answer)
			metadata := HonchoChatResponseMetadata{
				Query:    query,
				Observer: observer,
				Target:   target,
				Answered: answer != "",
			}

			if answer == "" {
				result := fmt.Sprintf(
					"Memory has no answer for %q yet. Not enough has been observed about the user on this topic.",
					query,
				)
				return fantasy.WithResponseMetadata(fantasy.NewTextResponse(result), metadata), nil
			}

			// The answer is model-generated text derived from earlier
			// conversation, so it carries the same instruction-
			// injection risk as any recalled memory. Label it.
			result := honcho.MemoryInstruction + "\n\n" + answer
			return fantasy.WithResponseMetadata(fantasy.NewTextResponse(result), metadata), nil
		},
	)
}
