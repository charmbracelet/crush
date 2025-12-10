package agent

import (
	"context"
	"strings"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/stretchr/testify/require"
)

// Mock session service for testing
type mockSessionService struct {
	session.Service
}

func (m *mockSessionService) CreateAgentToolSessionID(messageID string, toolCallID message.ToolCallID) string {
	return messageID + "$$" + string(toolCallID)
}

func (m *mockSessionService) ParseAgentToolSessionID(sessionID string) (string, message.ToolCallID, bool) {
	parts := strings.Split(sessionID, "$$")
	if len(parts) != 2 {
		return "", message.EmptyToolCallId, false
	}
	return parts[0], message.ToolCallID(parts[1]), true
}

func (m *mockSessionService) IsAgentToolSession(sessionID string) bool {
	_, _, ok := m.ParseAgentToolSessionID(sessionID)
	return ok
}

// Mock message service for testing
type mockMessageService struct{}

func (m *mockMessageService) Create(ctx context.Context, sessionID string, param message.CreateMessageParams) (message.Message, error) {
	return message.Message{}, nil
}

func (m *mockMessageService) Update(ctx context.Context, msg message.Message) error {
	return nil
}

func (m *mockMessageService) Get(ctx context.Context, sessionID string) (message.Message, error) {
	return message.Message{}, nil
}

func (m *mockMessageService) List(ctx context.Context, sessionID string) ([]message.Message, error) {
	return []message.Message{}, nil
}

func (m *mockMessageService) Delete(ctx context.Context, id string) error {
	return nil
}

func (m *mockMessageService) DeleteSessionMessages(ctx context.Context, sessionID string) error {
	return nil
}

func (m *mockMessageService) Subscribe(ctx context.Context) <-chan pubsub.Event[message.Message] {
	// Return a closed channel - tests don't need real pubsub
	ch := make(chan pubsub.Event[message.Message])
	close(ch)
	return ch
}

func TestAgentToolDeadlockPrevention(t *testing.T) {
	t.Parallel()

	// Create mock session service
	sessions := &mockSessionService{}

	// Create a test session agent with minimal setup
	agent := NewSessionAgent(SessionAgentOptions{
		LargeModel:           Model{},
		SmallModel:           Model{},
		SystemPromptPrefix:   "test",
		SystemPrompt:         "test prompt",
		DisableAutoSummarize: true,
		IsYolo:               false,
		Sessions:             sessions,
		Messages:             &mockMessageService{},
		Tools:                []fantasy.AgentTool{},
	})

	// Test 1: Verify normal session cleanup works
	t.Run("Normal Session Cleanup", func(t *testing.T) {
		normalSessionID := "normal-session-123"

		// Simulate session being marked as busy
		agent.(*sessionAgent).activeRequests.Set(normalSessionID, func() {})
		require.True(t, agent.IsSessionBusy(normalSessionID))

		// Cleanup should remove session from activeRequests
		agent.(*sessionAgent).cleanupActiveRequests(normalSessionID)
		require.False(t, agent.IsSessionBusy(normalSessionID))
	})

	// Test 2: Verify agent tool session cleanup includes parent
	t.Run("Agent Tool Session Cleanup", func(t *testing.T) {
		parentSessionID := "parent-session-456"
		messageID := "msg-789"
		toolCallID := message.ToolCallID("tool-012")
		agentToolSessionID := sessions.CreateAgentToolSessionID(messageID, toolCallID)

		// Simulate parent session being marked as busy (as it would be when agent tool starts)
		agent.(*sessionAgent).activeRequests.Set(parentSessionID, func() {})
		agent.(*sessionAgent).activeRequests.Set(agentToolSessionID, func() {})

		require.True(t, agent.IsSessionBusy(parentSessionID), "Parent session should be busy")
		require.True(t, agent.IsSessionBusy(agentToolSessionID), "Agent tool session should be busy")
		require.True(t, sessions.IsAgentToolSession(agentToolSessionID), "Should detect agent tool session")

		// The key insight: parentSessionID == messageID (this is how agent tool is structured)
		// So when we cleanup agent tool session, it should also cleanup the parent
		// Key insight: our cleanup logic extracts messageID from agent tool session
		// and cleans up session using that messageID, not parentSessionID
		// So we need to mark messageID as busy to test the cleanup
		agent.(*sessionAgent).activeRequests.Set(messageID, func() {})

		// Cleanup agent tool session should also clean up parent session
		agent.(*sessionAgent).cleanupActiveRequests(agentToolSessionID)

		require.False(t, agent.IsSessionBusy(agentToolSessionID), "Agent tool session should be cleaned up")
		require.False(t, agent.IsSessionBusy(messageID), "Parent session (messageID) should be cleaned up to prevent deadlock")
		require.True(t, agent.IsSessionBusy(parentSessionID), "Original parent session should still be busy (different from messageID)")
	})

	// Test 3: Verify non-agent tool session doesn't affect other sessions
	t.Run("Non-Agent Tool Session Cleanup", func(t *testing.T) {
		session1 := "session-1"
		session2 := "session-2"

		// Mark both sessions as busy
		agent.(*sessionAgent).activeRequests.Set(session1, func() {})
		agent.(*sessionAgent).activeRequests.Set(session2, func() {})

		require.True(t, agent.IsSessionBusy(session1))
		require.True(t, agent.IsSessionBusy(session2))

		// Cleanup session1 should only affect session1
		agent.(*sessionAgent).cleanupActiveRequests(session1)

		require.False(t, agent.IsSessionBusy(session1))
		require.True(t, agent.IsSessionBusy(session2))
	})

	// Test 4: Verify concurrent access doesn't cause panic
	t.Run("Concurrent Cleanup", func(t *testing.T) {
		sessionIDs := []string{"session-1", "session-2", "session-3"}

		// Mark all sessions as busy
		for _, id := range sessionIDs {
			agent.(*sessionAgent).activeRequests.Set(id, func() {})
		}

		// Run cleanup concurrently
		done := make(chan bool, len(sessionIDs))
		for _, id := range sessionIDs {
			go func(sessionID string) {
				defer func() {
					done <- true
				}()
				agent.(*sessionAgent).cleanupActiveRequests(sessionID)
			}(id)
		}

		// Wait for all cleanup to complete
		for range len(sessionIDs) {
			<-done
		}

		// All sessions should be cleaned up
		for _, id := range sessionIDs {
			require.False(t, agent.IsSessionBusy(id))
		}
	})
}

func TestParseAgentToolSessionID(t *testing.T) {
	t.Parallel()

	sessions := &mockSessionService{}

	// Test valid agent tool session ID
	messageID := "msg-123"
	toolCallID := message.ToolCallID("tool-456")
	agentToolSessionID := sessions.CreateAgentToolSessionID(messageID, toolCallID)

	parsedMessageID, parsedToolCallID, ok := sessions.ParseAgentToolSessionID(agentToolSessionID)
	require.True(t, ok)
	require.Equal(t, messageID, parsedMessageID)
	require.Equal(t, toolCallID, parsedToolCallID) // Compare ToolCallID types directly

	// Test invalid session ID
	_, _, ok = sessions.ParseAgentToolSessionID("invalid-session-id")
	require.False(t, ok)

	// Test empty session ID
	_, _, ok = sessions.ParseAgentToolSessionID("")
	require.False(t, ok)
}
