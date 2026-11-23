package model

import (
	"testing"

	"github.com/charmbracelet/crush/internal/agent"
	agenttools "github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/chat"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

// TestToolProgressScopesToSessionAndDirectItem verifies that live tool
// progress updates ignore events from other sessions and do not mutate a
// parent agent item when a nested subagent tool shares its index entry.
func TestToolProgressScopesToSessionAndDirectItem(t *testing.T) {
	t.Parallel()

	u := newTestUI()
	u.session = &session.Session{ID: "sess-1"}
	sty := u.com.Styles

	bashItem := chat.NewToolMessageItem(sty, "m1", message.ToolCall{
		ID:       "bash-1",
		Name:     agenttools.BashToolName,
		Input:    `{"command":"echo hi"}`,
		Finished: true,
	}, nil, false, "/tmp")

	agentItem := chat.NewToolMessageItem(sty, "m1", message.ToolCall{
		ID:       "agent-1",
		Name:     agent.AgentToolName,
		Input:    `{"prompt":"do work"}`,
		Finished: true,
	}, nil, false, "/tmp")
	nestedBash := chat.NewToolMessageItem(sty, "m2", message.ToolCall{
		ID:       "nested-bash-1",
		Name:     agenttools.BashToolName,
		Input:    `{"command":"echo sub"}`,
		Finished: true,
	}, nil, false, "/tmp")
	agentItem.(chat.NestedToolContainer).SetNestedTools([]chat.ToolMessageItem{nestedBash})

	u.chat.AppendMessages(bashItem, agentItem)
	u.chat.UpdateNestedToolIDs("agent-1")

	// Different session must be ignored.
	u.Update(pubsub.Event[agenttools.ToolProgress]{
		Type: pubsub.UpdatedEvent,
		Payload: agenttools.ToolProgress{
			SessionID:  "other-session",
			ToolCallID: "bash-1",
			Output:     "wrong-session",
		},
	})
	require.NotContains(t, ansi.Strip(bashItem.Render(100)), "wrong-session")

	// Matching session updates the direct bash item.
	u.Update(pubsub.Event[agenttools.ToolProgress]{
		Type: pubsub.UpdatedEvent,
		Payload: agenttools.ToolProgress{
			SessionID:  "sess-1",
			ToolCallID: "bash-1",
			Output:     "live-line",
		},
	})
	require.Contains(t, ansi.Strip(bashItem.Render(100)), "live-line")

	// Nested tool ID aliases to the parent agent item in idInxMap; progress
	// for the nested call must not mutate or bump the parent agent item.
	vBefore := agentItem.Version()
	u.Update(pubsub.Event[agenttools.ToolProgress]{
		Type: pubsub.UpdatedEvent,
		Payload: agenttools.ToolProgress{
			SessionID:  "sess-1",
			ToolCallID: "nested-bash-1",
			Output:     "subagent-output",
		},
	})
	require.Equal(t, vBefore, agentItem.Version())
}
