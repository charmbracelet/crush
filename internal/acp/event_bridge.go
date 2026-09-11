// Package acp provides the ACP event bridge that translates Crush pubsub
// events into ACP session/update notifications.
package acp

import (
	"context"
	"log/slog"

	acp "github.com/coder/acp-go-sdk"
	"github.com/charmbracelet/crush/internal/app"
	notify "github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/pubsub"
)

// eventBridge translates Crush agent events into ACP session updates.
type eventBridge struct {
	conn   *acp.AgentSideConnection
	app    *app.App
	log    *slog.Logger
	cancel context.CancelFunc
}

// newEventBridge creates a bridge that listens for agent events and
// forwards them as ACP session/update notifications.
func newEventBridge(conn *acp.AgentSideConnection, app *app.App, log *slog.Logger) *eventBridge {
	return &eventBridge{
		conn: conn,
		app:  app,
		log:  log,
	}
}

// Start begins listening for agent events. It returns when ctx is cancelled.
func (b *eventBridge) Start(ctx context.Context) error {
	ctx, b.cancel = context.WithCancel(ctx)

	// Subscribe to agent notifications (errors, auth events, etc.)
	if b.app.AgentNotifications() != nil { go b.subscribeNotifications(ctx, b.app.AgentNotifications()) }
	// Subscribe to run completions (turn-end signals)
	if b.app.RunCompletions() != nil { go b.subscribeRunCompletions(ctx, b.app.RunCompletions()) }

	<-ctx.Done()
	return ctx.Err()
}

// Shutdown stops the event bridge.
func (b *eventBridge) Shutdown() {
	if b.cancel != nil {
		b.cancel()
	}
}

func (b *eventBridge) subscribeNotifications(ctx context.Context, broker *pubsub.Broker[notify.Notification]) {
	ch := broker.Subscribe(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case ev := <-ch:
			b.handleNotification(ctx, ev.Payload)
		}
	}
}

func (b *eventBridge) subscribeRunCompletions(ctx context.Context, broker *pubsub.Broker[notify.RunComplete]) {
	ch := broker.Subscribe(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case ev := <-ch:
			b.handleRunComplete(ctx, ev.Payload)
		}
	}
}

// handleNotification converts agent notifications to ACP session updates.
func (b *eventBridge) handleNotification(ctx context.Context, n notify.Notification) {
	sessionID := acp.SessionId(n.SessionID)
	switch n.Type {
	case notify.TypeAgentError:
		b.log.Info("ACP: agent error", "sessionId", n.SessionID, "message", n.Message)
		text := "[Error] " + n.Message
		update := acp.SessionNotification{
			SessionId: sessionID,
			Update:    acp.UpdateAgentMessageText(text),
		}
		if err := b.conn.SessionUpdate(ctx, update); err != nil {
			b.log.Warn("Failed to send session update", "error", err)
		}
	case notify.TypeAgentFinished:
		b.log.Info("ACP: agent finished", "sessionId", n.SessionID)
		// No explicit update needed - prompt response handles completion
	case notify.TypeReAuthenticate:
		b.log.Info("ACP: re-auth required", "sessionId", n.SessionID, "provider", n.ProviderID)
		text := "[Auth Required] Please re-authenticate with " + n.ProviderID
		update := acp.SessionNotification{
			SessionId: sessionID,
			Update:    acp.UpdateAgentMessageText(text),
		}
		if err := b.conn.SessionUpdate(ctx, update); err != nil {
			b.log.Warn("Failed to send session update", "error", err)
		}
	default:
		b.log.Debug("ACP: unknown notification type", "type", n.Type)
	}
}

// handleRunComplete sends a final update when a turn completes.
func (b *eventBridge) handleRunComplete(ctx context.Context, rc notify.RunComplete) {
	sessionID := acp.SessionId(rc.SessionID)
	b.log.Info("ACP: run complete", "sessionId", rc.SessionID, "cancelled", rc.Cancelled)

	var text string
	switch {
	case rc.Cancelled:
		text = "[Cancelled] The operation was cancelled."
	case rc.Error != "":
		text = "[Error] " + rc.Error
	default:
		// Success case: no update needed, Prompt response handles it
		return
	}

	update := acp.SessionNotification{
		SessionId: sessionID,
		Update:    acp.UpdateAgentMessageText(text),
	}
	if err := b.conn.SessionUpdate(ctx, update); err != nil {
		b.log.Warn("Failed to send completion update", "error", err)
	}
}
