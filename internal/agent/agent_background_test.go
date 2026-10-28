package agent

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"charm.land/fantasy"

	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBackgroundLabel(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "agent-123456", backgroundLabel(AgentToolName, "toolcall123456"))
	assert.Equal(t, "search-abc123", backgroundLabel(tools.AgenticFetchToolName, "toolu_abc123"))
}

// A non-interactive coordinator keeps blocking, so the agent's output comes
// straight back and nothing is left in the registry.
func TestRunSubAgentMaybeBackground_Headless(t *testing.T) {
	const providerID = "test-provider"
	env := testEnv(t)
	coord := newTestCoordinator(t, env, providerID, config.ProviderConfig{ID: providerID})
	coord.interactive = false
	coord.dispatched = csync.NewMap[string, *dispatchedAgent]()
	coord.backgroundAfter = time.Millisecond

	parent, err := env.sessions.Create(t.Context(), "Parent")
	require.NoError(t, err)

	agent := newMockAgent(providerID, 4096, func(context.Context, SessionAgentCall) (*fantasy.AgentResult, error) {
		return agentResultWithText("blocking answer"), nil
	})

	resp, err := coord.runSubAgentMaybeBackground(t.Context(), AgentToolName, accessRead, subAgentParams{
		Agent:          agent,
		SessionID:      parent.ID,
		AgentMessageID: "msg-1",
		ToolCallID:     "call-1",
		Prompt:         "do something",
		SessionTitle:   "Test",
	})
	require.NoError(t, err)
	assert.Equal(t, "blocking answer", resp.Content)
	assert.Equal(t, 0, coord.dispatched.Len())
}

// A fast interactive call finishes before the threshold, so it still returns
// its output directly and leaves the registry empty.
func TestRunSubAgentMaybeBackground_Foreground(t *testing.T) {
	const providerID = "test-provider"
	env := testEnv(t)
	coord := newTestCoordinator(t, env, providerID, config.ProviderConfig{ID: providerID})
	coord.interactive = true
	coord.dispatched = csync.NewMap[string, *dispatchedAgent]()
	coord.backgroundAfter = 5 * time.Second

	parent, err := env.sessions.Create(t.Context(), "Parent")
	require.NoError(t, err)

	agent := newMockAgent(providerID, 4096, func(context.Context, SessionAgentCall) (*fantasy.AgentResult, error) {
		return agentResultWithText("fast answer"), nil
	})

	resp, err := coord.runSubAgentMaybeBackground(t.Context(), AgentToolName, accessRead, subAgentParams{
		Agent:          agent,
		SessionID:      parent.ID,
		AgentMessageID: "msg-1",
		ToolCallID:     "call-1",
		Prompt:         "do something",
		SessionTitle:   "Test",
	})
	require.NoError(t, err)
	assert.Equal(t, "fast answer", resp.Content)
	assert.False(t, resp.IsError)
	assert.Equal(t, 0, coord.dispatched.Len(), "a fast call must not linger in the registry")
}

// A slow interactive call crosses the threshold, so it hands off: the tool
// returns a background notice, the subagent is registered under a label, and
// once it settles its outcome is reported into the parent session.
func TestRunSubAgentMaybeBackground_Background(t *testing.T) {
	const providerID = "test-provider"
	env := testEnv(t)
	coord := newTestCoordinator(t, env, providerID, config.ProviderConfig{ID: providerID})
	coord.interactive = true
	coord.dispatched = csync.NewMap[string, *dispatchedAgent]()
	coord.backgroundAfter = 20 * time.Millisecond

	parent, err := env.sessions.Create(t.Context(), "Parent")
	require.NoError(t, err)

	// The subagent runs until its detached context is cancelled, which is
	// how it outruns the threshold and, later, how stop unwinds it.
	agent := newMockAgent(providerID, 4096, func(ctx context.Context, _ SessionAgentCall) (*fantasy.AgentResult, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	})

	resp, err := coord.runSubAgentMaybeBackground(t.Context(), AgentToolName, accessRead, subAgentParams{
		Agent:          agent,
		SessionID:      parent.ID,
		AgentMessageID: "msg-1",
		ToolCallID:     "toolcall999999",
		Prompt:         "long task",
		SessionTitle:   "Test",
	})
	require.NoError(t, err)
	assert.Contains(t, resp.Content, "background")

	var meta AgentBackgroundMetadata
	require.NoError(t, json.Unmarshal([]byte(resp.Metadata), &meta))
	assert.True(t, meta.Backgrounded)
	label := backgroundLabel(AgentToolName, "toolcall999999")
	assert.Equal(t, label, meta.Label)
	assert.Equal(t, "long task", meta.Task)

	entry, ok := coord.dispatched.Get(dispatchKey(parent.ID, label))
	require.True(t, ok, "backgrounded subagent must be in the registry")
	assert.True(t, entry.running())

	// Stopping it settles the run, which records a report in the parent.
	require.True(t, entry.stop("test stop"))
	require.True(t, entry.awaitSettled(2*time.Second))

	require.Eventually(t, func() bool {
		msgs, err := env.messages.List(t.Context(), parent.ID)
		if err != nil {
			return false
		}
		for _, m := range msgs {
			for _, r := range m.SubAgentReports() {
				if r.Label == label && r.Failed {
					return true
				}
			}
		}
		return false
	}, 2*time.Second, 10*time.Millisecond, "a stopped background subagent must report back")
}

// dispatchHistory promotes a backgrounded blocking call to a first-class
// subagent, and a later report closes it, just as for a dispatch.
func TestDispatchHistoryBackgroundedBlockingCall(t *testing.T) {
	t.Parallel()

	bgMeta, err := json.Marshal(AgentBackgroundMetadata{
		Label:        "search-1",
		Backgrounded: true,
		Access:       accessRead,
		Task:         "search x",
	})
	require.NoError(t, err)

	fetchInput, err := json.Marshal(tools.AgenticFetchParams{Prompt: "search x"})
	require.NoError(t, err)

	msgs := []message.Message{
		{ID: "m1", Role: message.Assistant, Parts: []message.ContentPart{
			message.ToolCall{ID: "c1", Name: tools.AgenticFetchToolName, Input: string(fetchInput), Finished: true},
			// A second blocking call that finished in the foreground has
			// no background metadata, so it must not become a subagent.
			message.ToolCall{ID: "c2", Name: AgentToolName, Input: `{"prompt":"quick"}`, Finished: true},
		}},
		{ID: "t1", Role: message.Tool, Parts: []message.ContentPart{
			message.ToolResult{ToolCallID: "c1", Name: tools.AgenticFetchToolName, Content: "moved to background", Metadata: string(bgMeta)},
			message.ToolResult{ToolCallID: "c2", Name: AgentToolName, Content: "quick answer"},
		}},
	}

	records := dispatchHistory(msgs)
	require.Equal(t, []string{"search-1"}, records.order)

	rec := records.byLabel["search-1"]
	require.NotNil(t, rec)
	assert.Equal(t, tools.AgenticFetchToolName, rec.Tool)
	assert.Equal(t, "m1", rec.MessageID)
	assert.Equal(t, "c1", rec.CallID)
	assert.Equal(t, "search x", rec.Task)
	assert.True(t, rec.Open)

	// A report under that label closes it.
	msgs = append(msgs, message.Message{ID: "u1", Role: message.User, Parts: []message.ContentPart{
		message.SubAgentReport{Label: "search-1", Output: "found"},
	}})
	rec = dispatchHistory(msgs).byLabel["search-1"]
	require.NotNil(t, rec)
	assert.False(t, rec.Open)
	require.NotNil(t, rec.LastReport)
	assert.Equal(t, "found", rec.LastReport.Output)
}
