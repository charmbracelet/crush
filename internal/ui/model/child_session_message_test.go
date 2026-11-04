package model

import (
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/ui/chat"
	"github.com/stretchr/testify/require"
)

// childSessionWorkspace adds the agent-tool session ID parsing that
// handleChildSessionMessage needs to the minimal test workspace.
type childSessionWorkspace struct {
	*testWorkspace
}

func (childSessionWorkspace) ParseAgentToolSessionID(sessionID string) (string, string, bool) {
	messageID, toolCallID, ok := strings.Cut(sessionID, "$$")
	return messageID, toolCallID, ok
}

func childSessionEvent(sessionID string, parts ...message.ContentPart) pubsub.Event[message.Message] {
	return pubsub.Event[message.Message]{
		Type: pubsub.UpdatedEvent,
		Payload: message.Message{
			ID:        "child-msg",
			SessionID: sessionID,
			Role:      message.Assistant,
			Parts:     parts,
		},
	}
}

func newChildSessionUI(t *testing.T) (*UI, *chat.AgentToolMessageItem) {
	t.Helper()

	u := newTestUI()
	u.com.Workspace = &childSessionWorkspace{testWorkspace: &testWorkspace{}}

	parent := chat.NewAgentToolMessageItem(
		u.com.Styles,
		message.ToolCall{ID: "agent-call", Name: "agent", Input: `{"prompt":"look"}`},
		nil,
		false,
	)
	u.chat.AppendMessages(parent)
	return u, parent
}

func TestHandleChildSessionMessage_NestsToolCallsUnderTheirAgent(t *testing.T) {
	t.Parallel()

	u, parent := newChildSessionUI(t)
	const session = "parent-msg$$agent-call"

	u.handleChildSessionMessage(childSessionEvent(session,
		message.ToolCall{ID: "glob-1", Name: "glob", Input: `{"pattern":"*.go"}`},
	))
	require.Len(t, parent.NestedTools(), 1)
	require.Equal(t, "glob-1", parent.NestedTools()[0].ToolCall().ID)

	// A later event for the same call updates it instead of adding a copy.
	u.handleChildSessionMessage(childSessionEvent(session,
		message.ToolCall{ID: "glob-1", Name: "glob", Input: `{"pattern":"*.go"}`, Finished: true},
		message.ToolCall{ID: "view-1", Name: "view", Input: `{"file_path":"a.go"}`},
		message.ToolResult{ToolCallID: "glob-1", Content: "a.go"},
	))
	nested := parent.NestedTools()
	require.Len(t, nested, 2)
	require.True(t, nested[0].ToolCall().Finished)
	require.Equal(t, "view-1", nested[1].ToolCall().ID)
}

func TestHandleChildSessionMessage_IgnoresEventsThatDoNotNameAnAgentItem(t *testing.T) {
	t.Parallel()

	u, parent := newChildSessionUI(t)
	call := message.ToolCall{ID: "glob-1", Name: "glob", Input: `{"pattern":"*.go"}`}

	// A session ID that is not an agent tool session.
	require.Nil(t, u.handleChildSessionMessage(childSessionEvent("plain-session", call)))
	// An agent tool session whose call is not in the chat.
	require.Nil(t, u.handleChildSessionMessage(childSessionEvent("m$$missing-call", call)))
	// No tool calls or results at all.
	require.Nil(t, u.handleChildSessionMessage(childSessionEvent("m$$agent-call")))
	require.Empty(t, parent.NestedTools())

	// A nested tool's ID also resolves to its parent in the chat, but only
	// the item that owns the call may take the event.
	u.handleChildSessionMessage(childSessionEvent("m$$agent-call", call))
	require.Len(t, parent.NestedTools(), 1)
	u.handleChildSessionMessage(childSessionEvent("m$$glob-1",
		message.ToolCall{ID: "view-1", Name: "view", Input: `{"file_path":"a.go"}`},
	))
	require.Len(t, parent.NestedTools(), 1)
}
