package acp

import (
	"context"

	"github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/coder/acp-go-sdk"
)

// Sink receives events from Crush's pubsub system and translates them to ACP
// session updates.
type Sink struct {
	ctx       context.Context
	cancel    context.CancelFunc
	conn      *acp.AgentSideConnection
	sessionID string

	// Track text deltas per message to avoid re-sending content.
	textOffsets      map[string]int
	reasoningOffsets map[string]int
}

// NewSink creates a new event sink for the given session.
func NewSink(ctx context.Context, conn *acp.AgentSideConnection, sessionID string) *Sink {
	sinkCtx, cancel := context.WithCancel(ctx)
	return &Sink{
		ctx:              sinkCtx,
		cancel:           cancel,
		conn:             conn,
		sessionID:        sessionID,
		textOffsets:      make(map[string]int),
		reasoningOffsets: make(map[string]int),
	}
}

// Start subscribes to messages, permissions, and sessions, forwarding events to ACP.
func (s *Sink) Start(messages message.Service, permissions permission.Service, sessions session.Service) {
	// Subscribe to message events.
	go func() {
		msgCh := messages.Subscribe(s.ctx)
		for {
			select {
			case event, ok := <-msgCh:
				if !ok {
					return
				}
				s.HandleMessage(event)
			case <-s.ctx.Done():
				return
			}
		}
	}()

	// Subscribe to permission events.
	go func() {
		permCh := permissions.Subscribe(s.ctx)
		for {
			select {
			case event, ok := <-permCh:
				if !ok {
					return
				}
				s.HandlePermission(event.Payload, permissions)
			case <-s.ctx.Done():
				return
			}
		}
	}()

	// Subscribe to session events for todo/plan updates.
	go func() {
		sessCh := sessions.Subscribe(s.ctx)
		for {
			select {
			case event, ok := <-sessCh:
				if !ok {
					return
				}
				s.HandleSession(event)
			case <-s.ctx.Done():
				return
			}
		}
	}()

	// Subscribe to MCP events to refresh commands when prompts change.
	go func() {
		mcpCh := mcp.SubscribeEvents(s.ctx)
		for {
			select {
			case event, ok := <-mcpCh:
				if !ok {
					return
				}
				s.HandleMCPEvent(pubsub.Event[mcp.Event](event))
			case <-s.ctx.Done():
				return
			}
		}
	}()

	// Publish initial commands.
	s.PublishCommands()
}

// Stop cancels the sink's subscriptions.
func (s *Sink) Stop() {
	s.cancel()
}
