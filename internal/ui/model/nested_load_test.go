package model

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/chat"
	"github.com/stretchr/testify/require"
)

// nestedWorkspace serves nested agent transcripts and counts every read, so
// tests can pin that building the chat items reads nothing.
type nestedWorkspace struct {
	testWorkspace

	mu           sync.Mutex
	transcripts  map[string][]message.Message
	failing      map[string]bool
	busySessions map[string]bool
	listCalls    []string
	busyCalls    []string
}

func (w *nestedWorkspace) CreateAgentToolSessionID(messageID, toolCallID string) string {
	return fmt.Sprintf("%s$$%s", messageID, toolCallID)
}

func (w *nestedWorkspace) ListMessages(_ context.Context, sessionID string) ([]message.Message, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.listCalls = append(w.listCalls, sessionID)
	if w.failing[sessionID] {
		return nil, errors.New("boom")
	}
	return w.transcripts[sessionID], nil
}

func (w *nestedWorkspace) AgentIsSessionBusy(sessionID string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.busyCalls = append(w.busyCalls, sessionID)
	return w.busySessions[sessionID]
}

func (w *nestedWorkspace) reset() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.listCalls = nil
	w.busyCalls = nil
}

func assistantWithCalls(id string, calls ...message.ToolCall) message.Message {
	parts := make([]message.ContentPart, 0, len(calls))
	for _, c := range calls {
		parts = append(parts, c)
	}
	return message.Message{ID: id, Role: message.Assistant, Parts: parts}
}

func toolCall(id, name, input string) message.ToolCall {
	return message.ToolCall{ID: id, Name: name, Input: input, Finished: true}
}

func dispatchCall(t *testing.T, id, label string) message.ToolCall {
	t.Helper()
	input, err := json.Marshal(agent.AgentDispatchParams{Prompt: "go", Label: label})
	require.NoError(t, err)
	return toolCall(id, agent.AgentDispatchToolName, string(input))
}

func TestSessionLoadReadsNestedTranscriptsOffTheEventLoop(t *testing.T) {
	t.Parallel()

	top := []message.Message{
		{ID: "u1", Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "hi"}}},
		assistantWithCalls("m1",
			toolCall("a1", agent.AgentToolName, `{"prompt":"look"}`),
			toolCall("f1", tools.AgenticFetchToolName, `{"prompt":"fetch"}`),
			toolCall("b1", tools.BashToolName, `{"command":"ls"}`),
			toolCall("x1", agent.AgentToolName, `{"prompt":"broken"}`),
		),
		assistantWithCalls("m2",
			dispatchCall(t, "d1", "live"),
			dispatchCall(t, "d2", "dead"),
		),
	}
	ws := &nestedWorkspace{
		testWorkspace: testWorkspace{cfg: &config.Config{Options: &config.Options{}}},
		transcripts: map[string][]message.Message{
			"m1$$a1": {assistantWithCalls("n1",
				toolCall("v1", tools.ViewToolName, `{"file_path":"a.go"}`),
				toolCall("a2", agent.AgentToolName, `{"prompt":"deeper"}`),
			)},
			"n1$$a2": {assistantWithCalls("n2",
				toolCall("g1", tools.GlobToolName, `{"pattern":"*.go"}`),
			)},
			"m1$$f1": {assistantWithCalls("n3",
				toolCall("w1", tools.WebFetchToolName, `{"url":"https://example.com"}`),
			)},
		},
		failing:      map[string]bool{"m1$$x1": true},
		busySessions: map[string]bool{"m2$$d1": true},
	}
	u := newTestUI()
	u.com.Workspace = ws

	// The command side reads every level and probes only open dispatches.
	nested := nestedTranscripts{
		messages: u.loadNestedTranscripts(top),
		busy:     u.probeOpenDispatches(top),
	}
	require.ElementsMatch(t, []string{"m1$$a1", "m1$$f1", "m1$$x1", "m2$$d1", "m2$$d2", "n1$$a2"}, ws.listCalls)
	require.Len(t, nested.messages, 3, "failed and empty reads are skipped")
	require.ElementsMatch(t, []string{"m2$$d1", "m2$$d2"}, ws.busyCalls)
	require.Equal(t, map[string]bool{"m2$$d1": true, "m2$$d2": false}, nested.busy)

	// Building the items reads nothing from the workspace.
	ws.reset()
	u.setSessionMessagesWithNested(top, nested)
	require.Empty(t, ws.listCalls, "building items must not read transcripts")
	require.Empty(t, ws.busyCalls, "building items must not probe the backend")

	agentItem, ok := u.chat.MessageItem("a1").(*chat.AgentToolMessageItem)
	require.True(t, ok)
	nestedTools := agentItem.NestedTools()
	require.Len(t, nestedTools, 2)
	require.Equal(t, "v1", nestedTools[0].ToolCall().ID)
	deeper, ok := nestedTools[1].(*chat.AgentToolMessageItem)
	require.True(t, ok)
	require.Len(t, deeper.NestedTools(), 1, "agents inside agents are attached too")
	require.Equal(t, "g1", deeper.NestedTools()[0].ToolCall().ID)

	fetchItem, ok := u.chat.MessageItem("f1").(chat.NestedToolContainer)
	require.True(t, ok)
	require.Len(t, fetchItem.NestedTools(), 1)

	broken, ok := u.chat.MessageItem("x1").(chat.NestedToolContainer)
	require.True(t, ok)
	require.Empty(t, broken.NestedTools(), "a failed read shows no nested tools")

	running := func(id string) bool {
		item, ok := u.chat.MessageItem(id).(*chat.AgentToolMessageItem)
		require.True(t, ok)
		status, ok := item.DispatchStatus()
		require.True(t, ok)
		return status.Running
	}
	require.True(t, running("d1"), "a live subagent keeps spinning")
	require.False(t, running("d2"), "a subagent that is not busy settles")
}

func TestSetSessionMessagesWithoutNestedDataSettlesDispatches(t *testing.T) {
	t.Parallel()

	ws := &nestedWorkspace{
		testWorkspace: testWorkspace{cfg: &config.Config{Options: &config.Options{}}},
		busySessions:  map[string]bool{"m1$$d1": true},
	}
	u := newTestUI()
	u.com.Workspace = ws
	u.setSessionMessages([]message.Message{assistantWithCalls("m1", dispatchCall(t, "d1", "live"))})
	require.Empty(t, ws.busyCalls)

	item, ok := u.chat.MessageItem("d1").(*chat.AgentToolMessageItem)
	require.True(t, ok)
	status, ok := item.DispatchStatus()
	require.True(t, ok)
	require.False(t, status.Running, "a dispatch with no busy answer is treated as finished")
}
