package cl_kernel

import (
	"context"
	"fmt"

	"github.com/charmbracelet/crushcl/internal/agent"

	"charm.land/fantasy"
)

// SessionAgentAdapter wraps a SessionAgent to implement the AgentRunner interface
type SessionAgentAdapter struct {
	agent     agent.SessionAgent
	sessionID string
}

// NewSessionAgentAdapter creates a new adapter for SessionAgent
func NewSessionAgentAdapter(agent agent.SessionAgent) *SessionAgentAdapter {
	return &SessionAgentAdapter{
		agent:     agent,
		sessionID: generateSessionID(),
	}
}

// Run implements AgentRunner interface
func (a *SessionAgentAdapter) Run(ctx context.Context, call AgentCall) (*AgentResult, error) {
	// Convert AgentCall to SessionAgentCall
	agentCall := agent.SessionAgentCall{
		SessionID:       call.SessionID,
		Prompt:          call.Prompt,
		MaxOutputTokens: call.MaxOutputTokens,
	}

	// Execute via SessionAgent
	result, err := a.agent.Run(ctx, agentCall)
	if err != nil {
		return nil, fmt.Errorf("session agent execution failed: %w", err)
	}

	// Convert fantasy.AgentResult to our AgentResult
	return a.fantasyResultToAgentResult(result), nil
}

// fantasyResultToAgentResult converts fantasy.AgentResult to our AgentResult
func (a *SessionAgentAdapter) fantasyResultToAgentResult(result *fantasy.AgentResult) *AgentResult {
	if result == nil {
		return &AgentResult{}
	}

	// Extract text content - Text() is a method on Response.Content
	text := result.Response.Content.Text()

	// Extract token usage - TotalUsage is a value type with int64 fields
	inputTokens := int(result.TotalUsage.InputTokens)
	outputTokens := int(result.TotalUsage.OutputTokens)

	return &AgentResult{
		Response: AgentResponse{
			Content: AgentResponseContent{
				Text: text,
			},
		},
		TotalUsage: TokenUsage{
			InputTokens:  inputTokens,
			OutputTokens: outputTokens,
		},
	}
}
