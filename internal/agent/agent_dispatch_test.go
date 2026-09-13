package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

func dispatchCall(t *testing.T, id, label, access string) message.ToolCall {
	t.Helper()
	input, err := json.Marshal(AgentDispatchParams{Prompt: "do " + label, Label: label, Access: access})
	require.NoError(t, err)
	return message.ToolCall{ID: id, Name: AgentDispatchToolName, Input: string(input), Finished: true}
}

func TestDispatchHistory(t *testing.T) {
	t.Parallel()

	reopenMeta, err := json.Marshal(AgentSendResponseMetadata{Label: "a", Reopened: true})
	require.NoError(t, err)

	msgs := []message.Message{
		{ID: "m1", Role: message.Assistant, Parts: []message.ContentPart{
			dispatchCall(t, "c1", "a", ""),
			dispatchCall(t, "c2", "b", "write"),
		}},
		{ID: "t1", Role: message.Tool, Parts: []message.ContentPart{
			message.ToolResult{ToolCallID: "c1", Name: AgentDispatchToolName, Content: "started"},
			message.ToolResult{ToolCallID: "c2", Name: AgentDispatchToolName, Content: "started"},
		}},
		{ID: "u1", Role: message.User, Parts: []message.ContentPart{
			message.SubAgentReport{Label: "a", Output: "done"},
		}},
		// A refused dispatch under a running label changes nothing.
		{ID: "m2", Role: message.Assistant, Parts: []message.ContentPart{
			dispatchCall(t, "c3", "b", ""),
		}},
		{ID: "t2", Role: message.Tool, Parts: []message.ContentPart{
			message.ToolResult{ToolCallID: "c3", Name: AgentDispatchToolName, Content: "already running", IsError: true},
		}},
		// A follow-up reopens a finished sub-agent.
		{ID: "t3", Role: message.Tool, Parts: []message.ContentPart{
			message.ToolResult{ToolCallID: "c4", Name: AgentSendToolName, Metadata: string(reopenMeta)},
		}},
	}

	records := dispatchHistory(msgs)
	require.Equal(t, []string{"a", "b"}, records.order)

	a := records.byLabel["a"]
	require.Equal(t, "c1", a.CallID)
	require.Equal(t, accessRead, a.Access)
	require.True(t, a.Open, "a reopened sub-agent is open again")
	require.NotNil(t, a.LastReport)

	b := records.byLabel["b"]
	require.Equal(t, "c2", b.CallID, "a refused dispatch must not replace the running one")
	require.Equal(t, "write", b.Access)
	require.True(t, b.Open)

	latest := LatestDispatches([]*message.Message{&msgs[0], &msgs[1], &msgs[2]})
	require.False(t, latest["a"].Open)
	require.True(t, latest["b"].Open)
}

func TestDispatchedAgentStaleRunCannotSettleNewerRun(t *testing.T) {
	t.Parallel()

	entry := newDispatchedAgent("a", accessRead, "child", "parent", nil)
	_, first, ok := entry.claim()
	require.True(t, ok)
	_, _, ok = entry.claim()
	require.False(t, ok, "a running sub-agent cannot be claimed twice")

	entry.settle(first, false)
	_, second, ok := entry.claim()
	require.True(t, ok)

	// The first run's late cleanup must not end the second run.
	entry.settle(first, false)
	require.True(t, entry.running())

	entry.settle(second, true)
	require.Equal(t, dispatchFailed, entry.snapshot().State)
}

// newDispatchTestCoordinator returns a coordinator with just enough wiring to
// run detached sub-agents whose work is a stand-in function.
func newDispatchTestCoordinator(t *testing.T) (*coordinator, fakeEnv) {
	t.Helper()
	sa, env := newStreamTestAgent(t)
	return &coordinator{
		sessions:   env.sessions,
		messages:   env.messages,
		mainAgent:  sa,
		dispatched: csync.NewMap[string, *dispatchedAgent](),
	}, env
}

// startBlockingDispatch registers a running sub-agent whose run blocks until
// it is stopped.
func startBlockingDispatch(t *testing.T, c *coordinator, parentID, label string) *dispatchedAgent {
	t.Helper()
	entry := newDispatchedAgent(label, accessRead, "child-"+label, parentID, nil)
	ctx, gen, ok := entry.claim()
	require.True(t, ok)
	c.dispatched.Set(dispatchKey(parentID, label), entry)
	c.runDispatched(ctx, entry, gen, func(ctx context.Context) (fantasy.ToolResponse, error) {
		<-ctx.Done()
		return fantasy.NewTextErrorResponse("canceled"), nil
	})
	return entry
}

func TestCancelLeavesDispatchedSubAgentsRunning(t *testing.T) {
	t.Parallel()
	c, env := newDispatchTestCoordinator(t)
	parent, err := env.sessions.Create(t.Context(), "parent")
	require.NoError(t, err)

	entry := startBlockingDispatch(t, c, parent.ID, "a")
	require.True(t, c.IsSessionBusy(entry.SessionID), "a running sub-agent's session is busy")

	c.Cancel(parent.ID)
	time.Sleep(100 * time.Millisecond)
	require.True(t, entry.running(), "interrupting the parent's turn must not stop its sub-agents")

	// Stopping it explicitly ends the run and records why, without a turn.
	require.True(t, entry.stop("not needed"))
	require.True(t, entry.awaitSettled(2*time.Second))
	require.False(t, c.IsSessionBusy(entry.SessionID))
	require.Equal(t, dispatchStopped, entry.snapshot().State)

	require.Eventually(t, func() bool {
		msgs, err := env.messages.List(t.Context(), parent.ID)
		if err != nil || len(msgs) != 1 {
			return false
		}
		reports := msgs[0].SubAgentReports()
		return len(reports) == 1 && reports[0].Failed && strings.Contains(reports[0].Output, "not needed")
	}, 2*time.Second, 20*time.Millisecond)
}

func TestCancelOnSubAgentSessionStopsIt(t *testing.T) {
	t.Parallel()
	c, env := newDispatchTestCoordinator(t)
	parent, err := env.sessions.Create(t.Context(), "parent")
	require.NoError(t, err)

	entry := startBlockingDispatch(t, c, parent.ID, "a")
	c.Cancel(entry.SessionID)
	require.True(t, entry.awaitSettled(2*time.Second))
	require.Equal(t, dispatchStopped, entry.snapshot().State)
}

func TestCancelAllStopsSubAgentsWithoutReporting(t *testing.T) {
	t.Parallel()
	c, env := newDispatchTestCoordinator(t)
	parent, err := env.sessions.Create(t.Context(), "parent")
	require.NoError(t, err)

	a := startBlockingDispatch(t, c, parent.ID, "a")
	b := startBlockingDispatch(t, c, parent.ID, "b")

	c.CancelAll()
	require.False(t, a.running())
	require.False(t, b.running())

	time.Sleep(100 * time.Millisecond)
	msgs, err := env.messages.List(t.Context(), parent.ID)
	require.NoError(t, err)
	require.Empty(t, msgs, "shutdown must not write reports into a closing database")
}

func TestReconcileDispatchesReportsInterruptedOnce(t *testing.T) {
	t.Parallel()
	c, env := newDispatchTestCoordinator(t)
	ctx := t.Context()
	parent, err := env.sessions.Create(ctx, "parent")
	require.NoError(t, err)

	_, err = env.messages.Create(ctx, parent.ID, message.CreateMessageParams{
		Role: message.Assistant,
		Parts: []message.ContentPart{
			dispatchCall(t, "c1", "cut-off", ""),
			dispatchCall(t, "c2", "reported", ""),
		},
	})
	require.NoError(t, err)
	_, err = env.messages.Create(ctx, parent.ID, message.CreateMessageParams{
		Role: message.Tool,
		Parts: []message.ContentPart{
			message.ToolResult{ToolCallID: "c1", Name: AgentDispatchToolName, Content: "started"},
			message.ToolResult{ToolCallID: "c2", Name: AgentDispatchToolName, Content: "started"},
		},
	})
	require.NoError(t, err)
	_, err = env.messages.Create(ctx, parent.ID, message.CreateMessageParams{
		Role:  message.User,
		Parts: []message.ContentPart{message.SubAgentReport{Label: "reported", Output: "all good"}},
	})
	require.NoError(t, err)

	c.reconcileDispatches(ctx, c.mainAgent, parent.ID)
	c.reconcileDispatches(ctx, c.mainAgent, parent.ID)

	msgs, err := env.messages.List(ctx, parent.ID)
	require.NoError(t, err)
	require.Len(t, msgs, 4, "exactly one interrupted report is written")
	reports := msgs[3].SubAgentReports()
	require.Len(t, reports, 1)
	require.Equal(t, "cut-off", reports[0].Label)
	require.True(t, reports[0].Failed)
	require.Contains(t, reports[0].Output, "Interrupted")

	// The parent's history now closes the dispatch.
	require.False(t, dispatchHistory(msgs).byLabel["cut-off"].Open)
}

func TestAgentStatusToolReportsLiveAndInterruptedSubAgents(t *testing.T) {
	t.Parallel()
	c, env := newDispatchTestCoordinator(t)
	ctx := t.Context()
	parent, err := env.sessions.Create(ctx, "parent")
	require.NoError(t, err)

	assistant, err := env.messages.Create(ctx, parent.ID, message.CreateMessageParams{
		Role: message.Assistant,
		Parts: []message.ContentPart{
			dispatchCall(t, "c1", "live", ""),
			dispatchCall(t, "c2", "dead", "write"),
		},
	})
	require.NoError(t, err)

	// The live one is running in this process and has made a tool call.
	childID := env.sessions.CreateAgentToolSessionID(assistant.ID, "c1")
	_, err = env.sessions.CreateTaskSession(ctx, childID, parent.ID, "Sub-agent: live")
	require.NoError(t, err)
	_, err = env.messages.Create(ctx, childID, message.CreateMessageParams{
		Role: message.Assistant,
		Parts: []message.ContentPart{
			message.TextContent{Text: "looking around"},
			message.ToolCall{ID: "x1", Name: "grep", Input: `{"pattern": "foo"}`, Finished: true},
		},
	})
	require.NoError(t, err)
	entry := newDispatchedAgent("live", accessRead, childID, parent.ID, nil)
	_, _, ok := entry.claim()
	require.True(t, ok)
	c.dispatched.Set(dispatchKey(parent.ID, "live"), entry)

	tool := c.agentStatusTool()
	toolCtx := context.WithValue(ctx, tools.SessionIDContextKey, parent.ID)

	resp, err := tool.Run(toolCtx, fantasy.ToolCall{ID: "s1", Name: AgentStatusToolName, Input: `{}`})
	require.NoError(t, err)
	require.False(t, resp.IsError, resp.Content)
	require.Contains(t, resp.Content, "- live: running")
	require.Contains(t, resp.Content, `grep {"pattern":"foo"} [running]`)
	require.Contains(t, resp.Content, "- dead: interrupted")
	require.Contains(t, resp.Content, "write access")

	resp, err = tool.Run(toolCtx, fantasy.ToolCall{ID: "s2", Name: AgentStatusToolName, Input: `{"label":"live"}`})
	require.NoError(t, err)
	require.Contains(t, resp.Content, "Task:\ndo live")
	require.Contains(t, resp.Content, "Latest output:\nlooking around")

	resp, err = tool.Run(toolCtx, fantasy.ToolCall{ID: "s3", Name: AgentStatusToolName, Input: `{"label":"nope"}`})
	require.NoError(t, err)
	require.True(t, resp.IsError)
}
