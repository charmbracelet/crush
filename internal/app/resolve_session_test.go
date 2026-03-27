package app

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/stretchr/testify/require"
)

// mockSessionService is a minimal mock of session.Service for testing resolveSession.
type mockSessionService struct {
	sessions []session.Session
	created  []session.Session
}

func (m *mockSessionService) Subscribe(context.Context) <-chan pubsub.Event[session.Session] {
	return make(chan pubsub.Event[session.Session])
}

func (m *mockSessionService) Create(_ context.Context, title string) (session.Session, error) {
	s := session.Session{ID: "new-session-id", Title: title}
	m.created = append(m.created, s)
	return s, nil
}

func (m *mockSessionService) CreateTitleSession(context.Context, string) (session.Session, error) {
	return session.Session{}, nil
}

func (m *mockSessionService) CreateTaskSession(context.Context, string, string, string) (session.Session, error) {
	return session.Session{}, nil
}

func (m *mockSessionService) CreateHandoffSession(_ context.Context, sourceSessionID, title, goal, draftPrompt string, files []string) (session.Session, error) {
	s := session.Session{
		ID:                     "handoff-session-id",
		Kind:                   session.KindHandoff,
		Title:                  title,
		HandoffSourceSessionID: sourceSessionID,
		HandoffGoal:            goal,
		HandoffDraftPrompt:     draftPrompt,
		HandoffRelevantFiles:   files,
	}
	m.created = append(m.created, s)
	return s, nil
}

func (m *mockSessionService) Get(_ context.Context, id string) (session.Session, error) {
	for _, s := range m.sessions {
		if s.ID == id {
			return s, nil
		}
	}
	return session.Session{}, sql.ErrNoRows
}

func (m *mockSessionService) GetLast(_ context.Context) (session.Session, error) {
	if len(m.sessions) > 0 {
		return m.sessions[0], nil
	}
	return session.Session{}, sql.ErrNoRows
}

func (m *mockSessionService) List(context.Context) ([]session.Session, error) {
	return m.sessions, nil
}

func (m *mockSessionService) Save(_ context.Context, s session.Session) (session.Session, error) {
	return s, nil
}

func (m *mockSessionService) UpdateCollaborationMode(_ context.Context, sessionID string, mode session.CollaborationMode) (session.Session, error) {
	return session.Session{ID: sessionID, CollaborationMode: mode}, nil
}

func (m *mockSessionService) UpdatePermissionMode(_ context.Context, sessionID string, mode session.PermissionMode) (session.Session, error) {
	return session.Session{ID: sessionID, PermissionMode: mode}, nil
}

func (m *mockSessionService) SetDefaultPermissionMode(session.PermissionMode) {}

func (m *mockSessionService) UpdateTitleAndUsage(context.Context, string, string, int64, int64, float64) error {
	return nil
}

func (m *mockSessionService) Rename(context.Context, string, string) error {
	return nil
}

func (m *mockSessionService) Delete(context.Context, string) error {
	return nil
}

func (m *mockSessionService) CreateAgentToolSessionID(messageID, toolCallID string) string {
	return fmt.Sprintf("%s$$%s", messageID, toolCallID)
}

func (m *mockSessionService) ParseAgentToolSessionID(sessionID string) (string, string, bool) {
	parts := strings.Split(sessionID, "$$")
	if len(parts) != 2 {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func (m *mockSessionService) IsAgentToolSession(sessionID string) bool {
	_, _, ok := m.ParseAgentToolSessionID(sessionID)
	return ok
}

func newTestApp(sessions session.Service) *App {
	return &App{Sessions: sessions}
}

func TestResolveSession_NewSession(t *testing.T) {
	mock := &mockSessionService{}
	app := newTestApp(mock)

	sess, err := app.resolveSession(t.Context(), "", false)
	require.NoError(t, err)
	require.Equal(t, "new-session-id", sess.ID)
	require.Len(t, mock.created, 1)
}

func TestResolveSession_ContinueByID(t *testing.T) {
	mock := &mockSessionService{
		sessions: []session.Session{
			{ID: "existing-id", Title: "Old session"},
		},
	}
	app := newTestApp(mock)

	sess, err := app.resolveSession(t.Context(), "existing-id", false)
	require.NoError(t, err)
	require.Equal(t, "existing-id", sess.ID)
	require.Equal(t, "Old session", sess.Title)
	require.Empty(t, mock.created)
}

func TestResolveSession_ContinueByID_NotFound(t *testing.T) {
	mock := &mockSessionService{}
	app := newTestApp(mock)

	_, err := app.resolveSession(t.Context(), "nonexistent", false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "session not found")
}

func TestResolveSession_ContinueByID_ChildSession(t *testing.T) {
	mock := &mockSessionService{
		sessions: []session.Session{
			{ID: "child-id", ParentSessionID: "parent-id", Title: "Child session"},
		},
	}
	app := newTestApp(mock)

	_, err := app.resolveSession(t.Context(), "child-id", false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot continue a child session")
}

func TestResolveSession_ContinueByID_AgentToolSession(t *testing.T) {
	mock := &mockSessionService{}
	app := newTestApp(mock)

	_, err := app.resolveSession(t.Context(), "msg123$$tool456", false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot continue an agent tool session")
}

func TestResolveSession_Last(t *testing.T) {
	mock := &mockSessionService{
		sessions: []session.Session{
			{ID: "most-recent", Title: "Latest session"},
			{ID: "older", Title: "Older session"},
		},
	}
	app := newTestApp(mock)

	sess, err := app.resolveSession(t.Context(), "", true)
	require.NoError(t, err)
	require.Equal(t, "most-recent", sess.ID)
	require.Empty(t, mock.created)
}

func TestResolveSession_Last_ChildSession(t *testing.T) {
	mock := &mockSessionService{
		sessions: []session.Session{
			{ID: "child-id", ParentSessionID: "parent-id", Title: "Child session"},
		},
	}
	app := newTestApp(mock)

	_, err := app.resolveSession(t.Context(), "", true)
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot continue a child session")
}

func TestResolveSession_Last_NoSessions(t *testing.T) {
	mock := &mockSessionService{}
	app := newTestApp(mock)

	_, err := app.resolveSession(t.Context(), "", true)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no sessions found")
}
