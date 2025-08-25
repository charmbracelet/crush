package agent

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))

	exitVal := m.Run()
	os.Exit(exitVal)
}

// mockMessageService implements a minimal message.Service for testing
type mockMessageService struct {
	messages map[string][]message.Message
}

func newMockMessageService() *mockMessageService {
	return &mockMessageService{
		messages: make(map[string][]message.Message),
	}
}

func (m *mockMessageService) Create(ctx context.Context, sessionID string, params message.CreateMessageParams) (message.Message, error) {
	msg := message.Message{
		ID:        fmt.Sprintf("msg-%d", len(m.messages[sessionID])+1),
		SessionID: sessionID,
		Role:      params.Role,
		Parts:     params.Parts,
		Model:     params.Model,
		Provider:  params.Provider,
	}
	m.messages[sessionID] = append(m.messages[sessionID], msg)
	return msg, nil
}

func (m *mockMessageService) List(ctx context.Context, sessionID string) ([]message.Message, error) {
	return m.messages[sessionID], nil
}

func (m *mockMessageService) Get(ctx context.Context, id string) (message.Message, error) {
	for _, msgs := range m.messages {
		for _, msg := range msgs {
			if msg.ID == id {
				return msg, nil
			}
		}
	}
	return message.Message{}, nil
}

func (m *mockMessageService) Update(ctx context.Context, msg message.Message) error {
	for sid, msgs := range m.messages {
		for i, existingMsg := range msgs {
			if existingMsg.ID == msg.ID {
				m.messages[sid][i] = msg
				return nil
			}
		}
	}
	return nil
}

func (m *mockMessageService) Delete(ctx context.Context, id string) error {
	for sid, msgs := range m.messages {
		for i, msg := range msgs {
			if msg.ID == id {
				m.messages[sid] = append(msgs[:i], msgs[i+1:]...)
				return nil
			}
		}
	}
	return nil
}

func (m *mockMessageService) DeleteSessionMessages(ctx context.Context, sessionID string) error {
	delete(m.messages, sessionID)
	return nil
}

func (m *mockMessageService) Subscribe(ctx context.Context) <-chan pubsub.Event[message.Message] {
	ch := make(chan pubsub.Event[message.Message])
	close(ch) // Close immediately for testing
	return ch
}

// mockSessionService implements a minimal session.Service for testing
type mockSessionService struct {
	sessions map[string]session.Session
}

func newMockSessionService() *mockSessionService {
	return &mockSessionService{
		sessions: make(map[string]session.Session),
	}
}

func (m *mockSessionService) Create(ctx context.Context, title string) (session.Session, error) {
	s := session.Session{
		ID:    fmt.Sprintf("session-%d", len(m.sessions)+1),
		Title: title,
	}
	m.sessions[s.ID] = s
	return s, nil
}

func (m *mockSessionService) Get(ctx context.Context, id string) (session.Session, error) {
	s, exists := m.sessions[id]
	if !exists {
		return session.Session{}, nil
	}
	return s, nil
}

func (m *mockSessionService) Update(ctx context.Context, s session.Session) error {
	m.sessions[s.ID] = s
	return nil
}

func (m *mockSessionService) List(ctx context.Context) ([]session.Session, error) {
	var result []session.Session
	for _, s := range m.sessions {
		result = append(result, s)
	}
	return result, nil
}

func (m *mockSessionService) Delete(ctx context.Context, id string) error {
	delete(m.sessions, id)
	return nil
}

func (m *mockSessionService) CreateTitleSession(ctx context.Context, parentSessionID string) (session.Session, error) {
	s := session.Session{
		ID:              fmt.Sprintf("title-session-%d", len(m.sessions)+1),
		Title:           "Title Session",
		ParentSessionID: parentSessionID,
	}
	m.sessions[s.ID] = s
	return s, nil
}

func (m *mockSessionService) CreateTaskSession(ctx context.Context, toolCallID, parentSessionID, title string) (session.Session, error) {
	s := session.Session{
		ID:              fmt.Sprintf("task-session-%d", len(m.sessions)+1),
		Title:           title,
		ParentSessionID: parentSessionID,
	}
	m.sessions[s.ID] = s
	return s, nil
}

func (m *mockSessionService) Save(ctx context.Context, s session.Session) (session.Session, error) {
	m.sessions[s.ID] = s
	return s, nil
}

func (m *mockSessionService) Subscribe(ctx context.Context) <-chan pubsub.Event[session.Session] {
	ch := make(chan pubsub.Event[session.Session])
	close(ch) // Close immediately for testing
	return ch
}

func TestInjectCrushMdIfNeeded_FirstMessage(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()
	
	// Create CRUSH.md file
	crushContent := "# Project Configuration\nThis is the CRUSH.md content."
	crushPath := filepath.Join(tmpDir, "CRUSH.md")
	err := os.WriteFile(crushPath, []byte(crushContent), 0644)
	require.NoError(t, err)
	
	// Initialize config with our temp directory
	_, err = config.Init(tmpDir, "", true)
	require.NoError(t, err)
	
	// Create mock services
	msgService := newMockMessageService()
	sessService := newMockSessionService()
	
	// Create a session
	sess, err := sessService.Create(context.Background(), "test session")
	require.NoError(t, err)
	
	// Create agent with mock services
	a := &agent{
		messages: msgService,
		sessions: sessService,
	}
	
	// Test with no existing messages (first message scenario)
	msgs := []message.Message{}
	result, err := a.injectCrushMdIfNeeded(context.Background(), sess.ID, msgs)
	require.NoError(t, err)
	
	// Should have one system message with CRUSH.md content
	require.Len(t, result, 1)
	require.Equal(t, message.System, result[0].Role)
	require.Len(t, result[0].Parts, 1)
	
	textPart, ok := result[0].Parts[0].(message.TextContent)
	require.True(t, ok)
	require.Equal(t, crushContent, textPart.Text)
}

func TestInjectCrushMdIfNeeded_NotFirstMessage(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()
	
	// Create CRUSH.md file
	crushContent := "# Project Configuration\nThis is the CRUSH.md content."
	crushPath := filepath.Join(tmpDir, "CRUSH.md")
	err := os.WriteFile(crushPath, []byte(crushContent), 0644)
	require.NoError(t, err)
	
	// Initialize config with our temp directory
	_, err = config.Init(tmpDir, "", true)
	require.NoError(t, err)
	
	// Create mock services
	msgService := newMockMessageService()
	sessService := newMockSessionService()
	
	// Create a session
	sess, err := sessService.Create(context.Background(), "test session")
	require.NoError(t, err)
	
	// Add an existing message
	existingMsg, err := msgService.Create(context.Background(), sess.ID, message.CreateMessageParams{
		Role:  message.User,
		Parts: []message.ContentPart{message.TextContent{Text: "Previous message"}},
	})
	require.NoError(t, err)
	
	// Create agent with mock services
	a := &agent{
		messages: msgService,
		sessions: sessService,
	}
	
	// Test with existing messages (not first message)
	msgs := []message.Message{existingMsg}
	result, err := a.injectCrushMdIfNeeded(context.Background(), sess.ID, msgs)
	require.NoError(t, err)
	
	// Should return the same messages without injection
	require.Len(t, result, 1)
	require.Equal(t, existingMsg.ID, result[0].ID)
}

func TestInjectCrushMdIfNeeded_AfterSummary(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()
	
	// Create CRUSH.md file
	crushContent := "# Project Configuration\nThis is the CRUSH.md content."
	crushPath := filepath.Join(tmpDir, "CRUSH.md")
	err := os.WriteFile(crushPath, []byte(crushContent), 0644)
	require.NoError(t, err)
	
	// Initialize config with our temp directory
	_, err = config.Init(tmpDir, "", true)
	require.NoError(t, err)
	
	// Create mock services
	msgService := newMockMessageService()
	sessService := newMockSessionService()
	
	// Create a session with a summary message ID
	sess, err := sessService.Create(context.Background(), "test session")
	require.NoError(t, err)
	
	// Create a summary message
	summaryMsg, err := msgService.Create(context.Background(), sess.ID, message.CreateMessageParams{
		Role:  message.User,
		Parts: []message.ContentPart{message.TextContent{Text: "Summary of previous conversation"}},
	})
	require.NoError(t, err)
	
	// Update session with summary message ID
	sess.SummaryMessageID = summaryMsg.ID
	err = sessService.Update(context.Background(), sess)
	require.NoError(t, err)
	
	// Create agent with mock services
	a := &agent{
		messages: msgService,
		sessions: sessService,
	}
	
	// Test with summary message (after compaction)
	msgs := []message.Message{summaryMsg}
	result, err := a.injectCrushMdIfNeeded(context.Background(), sess.ID, msgs)
	require.NoError(t, err)
	
	// Should have two messages: system message with CRUSH.md + summary message
	require.Len(t, result, 2)
	require.Equal(t, message.System, result[0].Role)
	
	textPart, ok := result[0].Parts[0].(message.TextContent)
	require.True(t, ok)
	require.Equal(t, crushContent, textPart.Text)
	
	require.Equal(t, summaryMsg.ID, result[1].ID)
}

func TestInjectCrushMdIfNeeded_NoCrushFile(t *testing.T) {
	// Create a temporary directory for testing without CRUSH.md
	tmpDir := t.TempDir()
	
	// Initialize config with our temp directory
	_, err := config.Init(tmpDir, "", true)
	require.NoError(t, err)
	
	// Create mock services
	msgService := newMockMessageService()
	sessService := newMockSessionService()
	
	// Create a session
	sess, err := sessService.Create(context.Background(), "test session")
	require.NoError(t, err)
	
	// Create agent with mock services
	a := &agent{
		messages: msgService,
		sessions: sessService,
	}
	
	// Test with no existing messages and no CRUSH.md file
	msgs := []message.Message{}
	result, err := a.injectCrushMdIfNeeded(context.Background(), sess.ID, msgs)
	require.NoError(t, err)
	
	// Should return empty messages since no CRUSH.md exists
	require.Len(t, result, 0)
}

func TestInjectCrushMdIfNeeded_ErrorHandling(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()
	
	// Create CRUSH.md file
	crushContent := "# Project Configuration\nThis is the CRUSH.md content."
	crushPath := filepath.Join(tmpDir, "CRUSH.md")
	err := os.WriteFile(crushPath, []byte(crushContent), 0644)
	require.NoError(t, err)
	
	// Initialize config with our temp directory
	_, err = config.Init(tmpDir, "", true)
	require.NoError(t, err)
	
	// Create mock services
	msgService := newMockMessageService()
	sessService := newMockSessionService()
	
	// Create agent with mock services
	a := &agent{
		messages: msgService,
		sessions: sessService,
	}
	
	// Test with invalid session ID (session doesn't exist)
	msgs := []message.Message{}
	result, err := a.injectCrushMdIfNeeded(context.Background(), "invalid-session", msgs)
	
	// Should still work even if session lookup fails
	require.NoError(t, err)
	require.Len(t, result, 1)
	require.Equal(t, message.System, result[0].Role)
}