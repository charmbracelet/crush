package acp

import (
	"context"
	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/coder/acp-go-sdk"
	"log/slog"
)

// implementing App's EventSink
type agentEventSink struct {
	ctx            context.Context
	agent          *Agent
	updatesIter    *updateIterator
	lastUserPrompt string
}

var (
	_ app.EventSink[any] = (*agentEventSink)(nil)
)

func newAgentSink(ctx context.Context, agent *Agent) *agentEventSink {
	return &agentEventSink{
		ctx:         ctx,
		agent:       agent,
		updatesIter: newUpdatesIterator(),
	}
}

func (sink *agentEventSink) Send(msg any) {
	switch ev := msg.(type) {
	case pubsub.Event[message.Message]:
		sink.handleMessage(ev.Payload)
	case pubsub.Event[permission.PermissionRequest]:
		sink.handlePermission(ev.Payload)
	}
}

func (sink *agentEventSink) handleToolCall(t message.ToolCall) {
	slog.Info("handleToolCall", "tool", t)
}

func (sink *agentEventSink) handleMessage(m message.Message) {
	if m.Role == message.User && sink.lastUserPrompt == m.Content().String() {
		return
	}

	for update := range sink.updatesIter.next(&m) {
		if err := sink.agent.conn.SessionUpdate(sink.ctx, acp.SessionNotification{
			SessionId: acp.SessionId(m.SessionID),
			Update:    update,
		}); err != nil {
			slog.Error("session update failed", "error", err)
		}
	}
}

func (sink *agentEventSink) handlePermission(req permission.PermissionRequest) {
	sink.agent.RequestPermission(sink.ctx, req)
}

func (sink *agentEventSink) LastUserPrompt(prompt string) {
	sink.lastUserPrompt = prompt
}

func (sink *agentEventSink) Quit() {
	//
}
