package agent

import (
	"context"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/fantasy/ai"
)

type Coordinator interface {
	// INFO: (kujtim) this is not used yet we will use this when we have multiple agents
	SetMainAgent(string)
	Run(ctx context.Context, sessionID, prompt string, attachments ...message.Attachment) (*ai.AgentResult, error)
}

type coordinator struct {
	cfg          *config.Config
	currentAgent SessionAgent
}
