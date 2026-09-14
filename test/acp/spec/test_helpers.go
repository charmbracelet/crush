package acp

import (
	"context"
	"fmt"
	"sync"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/session"
)

// stubSessionService is a minimal mock of session.Service for testing.
type stubSessionService struct {
	mu       sync.Mutex
	sessions []session.Session
	nextID   int
}

func (s *stubSessionService) Subscribe(_ context.Context) <-chan pubsub.Event[session.Session] {
	return make(chan pubsub.Event[session.Session])
}

func (s *stubSessionService) Create(_ context.Context, title string) (session.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextID++
	id := fmt.Sprintf("sess-%d", s.nextID)
	sess := session.Session{ID: id, Title: title}
	s.sessions = append(s.sessions, sess)
	return sess, nil
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
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]session.Session, len(s.sessions))
	copy(out, s.sessions)
	return out, nil
}

func (s *stubSessionService) Save(_ context.Context, s2 session.Session) (session.Session, error) {
	return s2, nil
}

func (s *stubSessionService) UpdateTitleAndUsage(_ context.Context, _, _ string, _, _ int64, _ float64) error {
	return nil
}
func (s *stubSessionService) Rename(_ context.Context, _, _ string) error { return nil }
func (s *stubSessionService) Delete(_ context.Context, _ string) error    { return nil }
func (s *stubSessionService) CreateAgentToolSessionID(_, _ string) string { return "tool-session" }
func (s *stubSessionService) ParseAgentToolSessionID(_ string) (string, string, bool) {
	return "", "", true
}
func (s *stubSessionService) IsAgentToolSession(_ string) bool { return false }

// stubMessageStore is a minimal in-memory mock of message.Service for testing.
type stubMessageStore struct {
	bySession map[string][]message.Message
}

func newStubMessageStore() *stubMessageStore {
	return &stubMessageStore{bySession: map[string][]message.Message{}}
}

func (s *stubMessageStore) Subscribe(_ context.Context) <-chan pubsub.Event[message.Message] {
	return make(chan pubsub.Event[message.Message])
}

func (s *stubMessageStore) Create(_ context.Context, sessionID string, _ message.CreateMessageParams) (message.Message, error) {
	return message.Message{ID: "m", SessionID: sessionID}, nil
}

func (s *stubMessageStore) Update(_ context.Context, m message.Message) error {
	s.bySession[m.SessionID] = append(s.bySession[m.SessionID], m)
	return nil
}

func (s *stubMessageStore) Get(_ context.Context, _ string) (message.Message, error) {
	return message.Message{}, nil
}

func (s *stubMessageStore) List(_ context.Context, sessionID string) ([]message.Message, error) {
	return s.bySession[sessionID], nil
}

func (s *stubMessageStore) ListUserMessages(_ context.Context, sessionID string) ([]message.Message, error) {
	return s.bySession[sessionID], nil
}

func (s *stubMessageStore) ListAllUserMessages(_ context.Context) ([]message.Message, error) {
	return nil, nil
}

func (s *stubMessageStore) GetLastAssistantMessage(_ context.Context, _ string) (message.Message, error) {
	return message.Message{}, nil
}

func (s *stubMessageStore) Delete(_ context.Context, _ string) error { return nil }

func (s *stubMessageStore) DeleteSessionMessages(_ context.Context, _ string) error { return nil }

func (s *stubMessageStore) Flush(_ context.Context, _ string) error { return nil }

func (s *stubMessageStore) FlushAll(_ context.Context) error { return nil }
