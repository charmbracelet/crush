package model

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/session"
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

func (w *nestedWorkspace) ParseAgentToolSessionID(sessionID string) (string, string, bool) {
	messageID, toolCallID, ok := strings.Cut(sessionID, "$$")
	return messageID, toolCallID, ok
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

	// The load command probes only open dispatches.
	busy := u.probeOpenDispatches(top)
	require.ElementsMatch(t, []string{"m2$$d1", "m2$$d2"}, ws.busyCalls)
	require.Equal(t, map[string]bool{"m2$$d1": true, "m2$$d2": false}, busy)

	// Building the items reads nothing from the workspace; the nested
	// transcripts are read by a command issued once they are up.
	ws.reset()
	u.beginSessionLoad()
	u.setSessionMessagesWithBusy(top, busy)
	cmd := u.loadNestedTranscriptsCmd(top)
	require.NotNil(t, cmd)
	require.Empty(t, ws.listCalls, "building items must not read transcripts")
	require.Empty(t, ws.busyCalls, "building items must not probe the backend")

	// The command reads every level.
	msg, ok := cmd().(nestedTranscriptsMsg)
	require.True(t, ok)
	require.ElementsMatch(t, []string{"m1$$a1", "m1$$f1", "m1$$x1", "m2$$d1", "m2$$d2", "n1$$a2"}, ws.listCalls)
	require.Len(t, msg.transcripts, 3, "failed and empty reads are skipped")

	// Attaching them reads nothing either.
	ws.reset()
	_, _ = u.Update(msg)
	require.Empty(t, ws.listCalls, "attaching nested tools must not read transcripts")
	require.Empty(t, ws.busyCalls, "attaching nested tools must not probe the backend")

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

// childEvent is a message event from the nested agent session sessionID.
func childEvent(typ pubsub.EventType, sessionID string, msg message.Message) pubsub.Event[message.Message] {
	msg.SessionID = sessionID
	return pubsub.Event[message.Message]{Type: typ, Payload: msg}
}

func TestNestedTranscriptsKeepEventsThatArriveWhileTheyAreRead(t *testing.T) {
	t.Parallel()

	top := []message.Message{
		assistantWithCalls("m1", toolCall("a1", agent.AgentToolName, `{"prompt":"look"}`)),
	}
	// The snapshot holds the nested view call, but not its result yet.
	ws := &nestedWorkspace{
		testWorkspace: testWorkspace{cfg: &config.Config{Options: &config.Options{}}},
		transcripts: map[string][]message.Message{
			"m1$$a1": {assistantWithCalls("n1", toolCall("v1", tools.ViewToolName, `{"file_path":"a.go"}`))},
		},
	}
	u := newTestUI()
	u.com.Workspace = ws
	u.session = &session.Session{ID: "s1"}

	_, seq := u.beginSessionLoad()
	_, _ = u.Update(loadSessionMsg{seq: seq, session: u.session, messages: top})
	require.True(t, u.sessionLoad.nestedPending)
	// The read in the command happens before the events below.
	cmd := u.loadNestedTranscriptsCmd(top)
	require.NotNil(t, cmd)
	snapshot, ok := cmd().(nestedTranscriptsMsg)
	require.True(t, ok)

	// After the read and before the snapshot lands, the nested view call
	// gets its result and the subagent starts another call.
	_, _ = u.Update(childEvent(pubsub.CreatedEvent, "m1$$a1", message.Message{
		ID:   "n1-result",
		Role: message.Tool,
		Parts: []message.ContentPart{message.ToolResult{
			ToolCallID: "v1",
			Name:       tools.ViewToolName,
			Content:    "package a",
		}},
	}))
	_, _ = u.Update(childEvent(pubsub.CreatedEvent, "m1$$a1",
		assistantWithCalls("n2", toolCall("g1", tools.GlobToolName, `{"pattern":"*.go"}`))))

	_, _ = u.Update(snapshot)
	require.False(t, u.sessionLoad.nestedPending)
	require.Empty(t, u.sessionLoad.childEvents)

	agentItem, ok := u.chat.MessageItem("a1").(*chat.AgentToolMessageItem)
	require.True(t, ok)
	nestedTools := agentItem.NestedTools()
	require.Len(t, nestedTools, 2, "a nested call made while the snapshot was read is kept")
	require.Equal(t, "v1", nestedTools[0].ToolCall().ID)
	require.Equal(t, "g1", nestedTools[1].ToolCall().ID)
	view, ok := nestedTools[0].(chat.Animatable)
	require.True(t, ok)
	require.False(t, view.Spinning(), "a result that arrived after the read must not be lost")
	require.Equal(t, chat.MessageItem(agentItem), u.chat.MessageItem("g1"), "nested IDs map to their container")
}

func TestSessionLoadAppliesOnlyTheLatestLoad(t *testing.T) {
	t.Parallel()

	u := newTestUI()
	u.com.Workspace = &nestedWorkspace{
		testWorkspace: testWorkspace{cfg: &config.Config{Options: &config.Options{}}},
	}

	_ = u.loadSession("A")
	seqA, ctxA := u.sessionLoad.seq, u.sessionLoad.ctx
	_ = u.loadSession("B")
	seqB := u.sessionLoad.seq
	require.Error(t, ctxA.Err(), "a superseded load is cancelled")

	// B's load lands first, then the slower load of A.
	_, _ = u.Update(loadSessionMsg{seq: seqB, session: &session.Session{ID: "B"}})
	_, _ = u.Update(loadSessionMsg{seq: seqA, session: &session.Session{ID: "A"}})
	require.Equal(t, "B", u.session.ID, "a superseded load must not replace the chosen session")

	// Starting a new session abandons a load still in flight.
	_ = u.loadSession("C")
	seqC := u.sessionLoad.seq
	u.newSession()
	_, _ = u.Update(loadSessionMsg{seq: seqC, session: &session.Session{ID: "C"}})
	require.Nil(t, u.session, "an abandoned load must not bring a session back")
}

func TestNestedTranscriptsOfASupersededLoadAreDropped(t *testing.T) {
	t.Parallel()

	top := []message.Message{
		assistantWithCalls("m1", toolCall("a1", agent.AgentToolName, `{"prompt":"look"}`)),
	}
	ws := &nestedWorkspace{
		testWorkspace: testWorkspace{cfg: &config.Config{Options: &config.Options{}}},
		transcripts: map[string][]message.Message{
			"m1$$a1": {assistantWithCalls("n1", toolCall("v1", tools.ViewToolName, `{"file_path":"a.go"}`))},
		},
	}
	u := newTestUI()
	u.com.Workspace = ws
	u.session = &session.Session{ID: "s1"}

	ctx, seq := u.beginSessionLoad()
	u.setSessionMessages(top)
	cmd := u.loadNestedTranscriptsCmd(top)
	require.NotNil(t, cmd)

	// A reload starts before the nested read runs: the read is skipped and
	// a late answer is ignored.
	_, _ = u.beginSessionLoad()
	require.Error(t, ctx.Err())
	require.Nil(t, cmd())
	require.Empty(t, ws.listCalls, "a cancelled load reads no nested transcripts")

	_, _ = u.Update(nestedTranscriptsMsg{seq: seq, transcripts: ws.transcripts})
	agentItem, ok := u.chat.MessageItem("a1").(*chat.AgentToolMessageItem)
	require.True(t, ok)
	require.Empty(t, agentItem.NestedTools())
}
