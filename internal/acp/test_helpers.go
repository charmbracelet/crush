package acp

import (
	"context"

	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/session"
)

// stubSessionService is a minimal mock of session.Service for testing.
type stubSessionService struct{}

func (s *stubSessionService) Subscribe(_ context.Context) <-chan pubsub.Event[session.Session] {
	return make(chan pubsub.Event[session.Session])
}
func (s *stubSessionService) Create(_ context.Context, title string) (session.Session, error) {
	return session.Session{ID: "test-" + title, Title: title}, nil
}
func (s *stubSessionService) CreateTitleSession(_ context.Context, parentSessionID string) (session.Session, error) {
	return session.Session{ID: "test-child"}, nil
}
func (s *stubSessionService) CreateTaskSession(_ context.Context, toolCallID, parentSessionID, title string) (session.Session, error) {
	return session.Session{ID: "test-task"}, nil
}
func (s *stubSessionService) Get(_ context.Context, id string) (session.Session, error) {
	return session.Session{ID: id}, nil
}
func (s *stubSessionService) GetLast(_ context.Context) (session.Session, error) {
	return session.Session{ID: "last"}, nil
}
func (s *stubSessionService) List(_ context.Context) ([]session.Session, error) {
	return []session.Session{{ID: "s1", Title: "Test"}}, nil
}
func (s *stubSessionService) Save(_ context.Context, s2 session.Session) (session.Session, error) { return s2, nil }
func (s *stubSessionService) UpdateTitleAndUsage(_ context.Context, _, _ string, _, _ int64, _ float64) error { return nil }
func (s *stubSessionService) Rename(_ context.Context, _, _ string) error { return nil }
func (s *stubSessionService) Delete(_ context.Context, _ string) error { return nil }
func (s *stubSessionService) CreateAgentToolSessionID(_, _ string) string { return "tool-session" }
func (s *stubSessionService) ParseAgentToolSessionID(_ string) (string, string, bool) { return "", "", true }
func (s *stubSessionService) IsAgentToolSession(_ string) bool { return false }
