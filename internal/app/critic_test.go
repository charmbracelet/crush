package app

import (
	"context"
	"testing"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/skills/critic"
	"github.com/stretchr/testify/require"
)

// mockSessionAgent is a minimal mock for agent.SessionAgent.
type mockSessionAgent struct {
	runCalled bool
}

func (m *mockSessionAgent) Run(ctx context.Context, call agent.SessionAgentCall) (*fantasy.AgentResult, error) {
	m.runCalled = true
	return &fantasy.AgentResult{}, nil
}
func (m *mockSessionAgent) SetModels(large, small agent.Model)          {}
func (m *mockSessionAgent) SetTools(tools []fantasy.AgentTool)          {}
func (m *mockSessionAgent) SetSystemPrompt(systemPrompt string)         {}
func (m *mockSessionAgent) Cancel(sessionID string)                     {}
func (m *mockSessionAgent) CancelAll()                                  {}
func (m *mockSessionAgent) IsSessionBusy(sessionID string) bool         { return false }
func (m *mockSessionAgent) IsBusy() bool                                { return false }
func (m *mockSessionAgent) QueuedPrompts(sessionID string) int          { return 0 }
func (m *mockSessionAgent) QueuedPromptsList(sessionID string) []string { return nil }
func (m *mockSessionAgent) ClearQueue(sessionID string)                 {}
func (m *mockSessionAgent) Summarize(context.Context, string, fantasy.ProviderOptions) error {
	return nil
}
func (m *mockSessionAgent) Model() agent.Model { return agent.Model{} }

func TestBuildCriticWrapper_Disabled(t *testing.T) {
	t.Parallel()
	app := &App{}
	wrapper := app.buildCriticWrapper(critic.CriticSkillConfig{Enabled: false})
	require.Nil(t, wrapper)
}

func TestBuildCriticWrapper_Enabled(t *testing.T) {
	t.Parallel()
	app := &App{
		FileTracker: &mockFileTracker{},
		LSPManager:  nil,
		Messages:    &mockMessageService{},
		CriticStore: nil,
	}
	cfg := critic.CriticSkillConfig{Enabled: true, MaxIterations: 1}
	wrapper := app.buildCriticWrapper(cfg)
	require.NotNil(t, wrapper)

	// The wrapper should return a *critic.Middleware when given a SessionAgent.
	mock := &mockSessionAgent{}
	wrapped := wrapper(mock)
	_, ok := wrapped.(*critic.Middleware)
	require.True(t, ok, "expected wrapper to return *critic.Middleware")
}

// mockFileTracker implements filetracker.Service minimally.
type mockFileTracker struct{}

func (m *mockFileTracker) RecordRead(ctx context.Context, sessionID, path string) {}
func (m *mockFileTracker) LastReadTime(ctx context.Context, sessionID, path string) time.Time {
	return time.Time{}
}
func (m *mockFileTracker) ListReadFiles(ctx context.Context, sessionID string) ([]string, error) {
	return nil, nil
}

// mockMessageService implements message.Service minimally.
type mockMessageService struct{}

func (m *mockMessageService) Create(ctx context.Context, sessionID string, params message.CreateMessageParams) (message.Message, error) {
	return message.Message{}, nil
}
func (m *mockMessageService) Update(ctx context.Context, msg message.Message) error { return nil }
func (m *mockMessageService) Get(ctx context.Context, id string) (message.Message, error) {
	return message.Message{}, nil
}
func (m *mockMessageService) List(ctx context.Context, sessionID string) ([]message.Message, error) {
	return nil, nil
}
func (m *mockMessageService) ListUserMessages(ctx context.Context, sessionID string) ([]message.Message, error) {
	return nil, nil
}
func (m *mockMessageService) ListAllUserMessages(ctx context.Context) ([]message.Message, error) {
	return nil, nil
}
func (m *mockMessageService) Delete(ctx context.Context, id string) error { return nil }
func (m *mockMessageService) DeleteSessionMessages(ctx context.Context, sessionID string) error {
	return nil
}
func (m *mockMessageService) Subscribe(ctx context.Context) <-chan pubsub.Event[message.Message] {
	return nil
}
