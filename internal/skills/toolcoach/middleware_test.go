package toolcoach

import (
	"context"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent"
	"github.com/stretchr/testify/require"
)

// mockSessionAgent is a minimal implementation of agent.SessionAgent for testing.
type mockSessionAgent struct {
	tools []fantasy.AgentTool
	calls int
}

func (m *mockSessionAgent) Run(_ context.Context, _ agent.SessionAgentCall) (*fantasy.AgentResult, error) {
	m.calls++
	return &fantasy.AgentResult{}, nil
}

func (m *mockSessionAgent) SetModels(_ agent.Model, _ agent.Model) {}
func (m *mockSessionAgent) SetTools(tools []fantasy.AgentTool)     { m.tools = tools }
func (m *mockSessionAgent) SetSystemPrompt(_ string)               {}
func (m *mockSessionAgent) Cancel(_ string)                        {}
func (m *mockSessionAgent) CancelAll()                             {}
func (m *mockSessionAgent) IsSessionBusy(_ string) bool            { return false }
func (m *mockSessionAgent) IsBusy() bool                           { return false }
func (m *mockSessionAgent) QueuedPrompts(_ string) int             { return 0 }
func (m *mockSessionAgent) QueuedPromptsList(_ string) []string    { return nil }
func (m *mockSessionAgent) ClearQueue(_ string)                    {}
func (m *mockSessionAgent) Summarize(_ context.Context, _ string, _ fantasy.ProviderOptions) error {
	return nil
}
func (m *mockSessionAgent) Model() agent.Model { return agent.Model{} }

func TestMiddleware_Disabled(t *testing.T) {
	t.Parallel()

	primary := &mockSessionAgent{}
	mw := NewMiddleware(primary, ToolcoachConfig{Enabled: false})
	require.NotNil(t, mw)

	_, err := mw.Run(context.Background(), agent.SessionAgentCall{SessionID: "s1"})
	require.NoError(t, err)
	require.Equal(t, 1, primary.calls)
}

func TestMiddleware_PerCallDisable(t *testing.T) {
	t.Parallel()

	primary := &mockSessionAgent{}
	mw := NewMiddleware(primary, ToolcoachConfig{Enabled: true, MaxPatternsPerTurn: 3})
	require.NotNil(t, mw)

	f := false
	_, err := mw.Run(context.Background(), agent.SessionAgentCall{
		SessionID:        "s1",
		ToolcoachEnabled: &f,
	})
	require.NoError(t, err)
	require.Equal(t, 1, primary.calls)
}

func TestMiddleware_SetToolsWraps(t *testing.T) {
	t.Parallel()

	primary := &mockSessionAgent{}
	mw := NewMiddleware(primary, ToolcoachConfig{Enabled: true, MaxPatternsPerTurn: 3})
	require.NotNil(t, mw)

	// SetTools stores the tools but also forwards to primary.
	mw.SetTools([]fantasy.AgentTool{})
	require.NotNil(t, mw.tools)
}

func TestMiddleware_Delegates(t *testing.T) {
	t.Parallel()

	primary := &mockSessionAgent{}
	mw := NewMiddleware(primary, ToolcoachConfig{Enabled: true, MaxPatternsPerTurn: 3})

	// All delegated methods should not panic.
	mw.SetModels(agent.Model{}, agent.Model{})
	mw.SetSystemPrompt("test")
	mw.Cancel("s1")
	mw.CancelAll()
	_ = mw.IsSessionBusy("s1")
	_ = mw.IsBusy()
	_ = mw.QueuedPrompts("s1")
	_ = mw.QueuedPromptsList("s1")
	mw.ClearQueue("s1")
	_ = mw.Summarize(context.Background(), "s1", fantasy.ProviderOptions{})
	_ = mw.Model()
}
