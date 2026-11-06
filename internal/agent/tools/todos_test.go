package tools

import (
	"context"
	"encoding/json"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/stretchr/testify/require"
)

type mockSessionService struct {
	*pubsub.Broker[session.Session]
	sessions map[string]session.Session
}

func newMockSessionService() *mockSessionService {
	return &mockSessionService{
		Broker:   pubsub.NewBroker[session.Session](),
		sessions: map[string]session.Session{},
	}
}

func (m *mockSessionService) Create(ctx context.Context, title string) (session.Session, error) {
	return session.Session{}, nil
}

func (m *mockSessionService) CreateTitleSession(ctx context.Context, parentSessionID string) (session.Session, error) {
	return session.Session{}, nil
}

func (m *mockSessionService) CreateTaskSession(ctx context.Context, toolCallID, parentSessionID, title string) (session.Session, error) {
	return session.Session{}, nil
}

func (m *mockSessionService) Get(ctx context.Context, id string) (session.Session, error) {
	return m.sessions[id], nil
}

func (m *mockSessionService) GetLast(ctx context.Context) (session.Session, error) {
	return session.Session{}, nil
}

func (m *mockSessionService) List(ctx context.Context) ([]session.Session, error) {
	return nil, nil
}

func (m *mockSessionService) Save(ctx context.Context, s session.Session) (session.Session, error) {
	m.sessions[s.ID] = s
	return s, nil
}

func (m *mockSessionService) SetChannel(ctx context.Context, sessionID, channel string) (session.Session, error) {
	return session.Session{}, nil
}

func (m *mockSessionService) UpdateTitleAndUsage(ctx context.Context, sessionID, title string, promptTokens, completionTokens int64, cost float64) error {
	return nil
}

func (m *mockSessionService) Rename(ctx context.Context, id string, title string) error {
	return nil
}

func (m *mockSessionService) Delete(ctx context.Context, id string) error {
	return nil
}

func (m *mockSessionService) MCPDisabledServers(ctx context.Context) ([]string, error) {
	return nil, nil
}

func (m *mockSessionService) SetMCPServerDisabled(ctx context.Context, name string, disabled bool) error {
	return nil
}

func (m *mockSessionService) MCPServersEnabled(ctx context.Context) ([]string, error) {
	return nil, nil
}

func (m *mockSessionService) CreateAgentToolSessionID(messageID, toolCallID string) string {
	return ""
}

func (m *mockSessionService) ParseAgentToolSessionID(sessionID string) (string, string, bool) {
	return "", "", false
}

func (m *mockSessionService) IsAgentToolSession(sessionID string) bool {
	return false
}

// A todo with a bogus status is a model mistake the model can fix, so it must
// come back as a failed tool result. Returning a Go error instead ends the
// whole agent turn.
func TestTodosToolReportsInvalidStatusAsToolError(t *testing.T) {
	t.Parallel()

	sessions := newMockSessionService()
	_, err := sessions.Save(t.Context(), session.Session{ID: "test-session"})
	require.NoError(t, err)

	ctx := context.WithValue(t.Context(), SessionIDContextKey, "test-session")
	tool := NewTodosTool(sessions)

	input, err := json.Marshal(TodosParams{Todos: []TodoItem{
		{Content: "do the thing", Status: "in-progress"},
	}})
	require.NoError(t, err)

	resp, err := tool.Run(ctx, fantasy.ToolCall{
		ID:    "todos-call",
		Name:  TodosToolName,
		Input: string(input),
	})
	require.NoError(t, err)
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, `Invalid status "in-progress"`)
}
