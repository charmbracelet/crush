package toolchain

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/pubsub"
)

// mockMessageService is a mock implementation of message.Service for testing.
type mockMessageService struct {
	*pubsub.Broker[message.Message]
}

func newMockMessageService() *mockMessageService {
	return &mockMessageService{
		Broker: pubsub.NewBroker[message.Message](),
	}
}

func (m *mockMessageService) Create(ctx context.Context, sessionID string, params message.CreateMessageParams) (message.Message, error) {
	return message.Message{}, nil
}

func (m *mockMessageService) Update(ctx context.Context, msg message.Message) error {
	return nil
}

func (m *mockMessageService) Get(ctx context.Context, id string) (message.Message, error) {
	return message.Message{}, nil
}

func (m *mockMessageService) List(ctx context.Context, sessionID string) ([]message.Message, error) {
	return nil, nil
}

func (m *mockMessageService) Delete(ctx context.Context, id string) error {
	return nil
}

func (m *mockMessageService) DeleteSessionMessages(ctx context.Context, sessionID string) error {
	return nil
}

func TestDetector_NewDetector(t *testing.T) {
	cfg := DefaultConfig()
	d := NewDetector(cfg)

	if d == nil {
		t.Fatal("expected non-nil detector")
	}

	if d.config.Enabled != cfg.Enabled {
		t.Errorf("expected Enabled=%v, got %v", cfg.Enabled, d.config.Enabled)
	}

	if d.config.MinCalls != cfg.MinCalls {
		t.Errorf("expected MinCalls=%d, got %d", cfg.MinCalls, d.config.MinCalls)
	}
}

func TestDetector_NewDetectorWithDefaults(t *testing.T) {
	d := NewDetectorWithDefaults()

	if d == nil {
		t.Fatal("expected non-nil detector")
	}

	defaultCfg := DefaultConfig()
	if d.config.MinCalls != defaultCfg.MinCalls {
		t.Errorf("expected MinCalls=%d, got %d", defaultCfg.MinCalls, d.config.MinCalls)
	}
}

func TestDetector_ChainTracking(t *testing.T) {
	d := NewDetectorWithDefaults()

	// Initially no chains
	if d.ActiveChainCount() != 0 {
		t.Errorf("expected 0 active chains, got %d", d.ActiveChainCount())
	}

	// Create an assistant message with tool calls
	msg := message.Message{
		ID:        "msg-1",
		SessionID: "session-1",
		Role:      message.Assistant,
		Parts: []message.ContentPart{
			message.ToolCall{
				ID:       "tc-1",
				Name:     "bash",
				Input:    `{"command": "ls -la"}`,
				Finished: false,
			},
		},
	}

	d.handleMessageCreated(msg)

	// Should have one chain now
	if d.ActiveChainCount() != 1 {
		t.Errorf("expected 1 active chain, got %d", d.ActiveChainCount())
	}

	chain := d.GetChain("msg-1")
	if chain == nil {
		t.Fatal("expected to find chain for msg-1")
	}

	if chain.Len() != 1 {
		t.Errorf("expected chain length 1, got %d", chain.Len())
	}

	if chain.Calls[0].Name != "bash" {
		t.Errorf("expected tool name 'bash', got '%s'", chain.Calls[0].Name)
	}
}

func TestDetector_ChainUpdate(t *testing.T) {
	d := NewDetectorWithDefaults()

	// Create initial message with one tool call
	msg := message.Message{
		ID:        "msg-1",
		SessionID: "session-1",
		Role:      message.Assistant,
		Parts: []message.ContentPart{
			message.ToolCall{
				ID:       "tc-1",
				Name:     "bash",
				Input:    `{"command": "ls -la"}`,
				Finished: false,
			},
		},
	}

	d.handleMessageCreated(msg)

	// Update with a second tool call
	msg.Parts = append(msg.Parts, message.ToolCall{
		ID:       "tc-2",
		Name:     "view",
		Input:    `{"file_path": "/etc/passwd"}`,
		Finished: false,
	})

	d.handleMessageUpdated(msg)

	chain := d.GetChain("msg-1")
	if chain == nil {
		t.Fatal("expected to find chain")
	}

	if chain.Len() != 2 {
		t.Errorf("expected chain length 2, got %d", chain.Len())
	}
}

func TestDetector_ToolResultProcessing(t *testing.T) {
	d := NewDetectorWithDefaults()

	// Create message with tool call
	assistantMsg := message.Message{
		ID:        "msg-1",
		SessionID: "session-1",
		Role:      message.Assistant,
		Parts: []message.ContentPart{
			message.ToolCall{
				ID:       "tc-1",
				Name:     "bash",
				Input:    `{"command": "echo hello"}`,
				Finished: true,
			},
		},
	}

	d.handleMessageCreated(assistantMsg)

	// Process tool result
	toolMsg := message.Message{
		ID:        "msg-2",
		SessionID: "session-1",
		Role:      message.Tool,
		Parts: []message.ContentPart{
			message.ToolResult{
				ToolCallID: "tc-1",
				Content:    "hello",
				IsError:    false,
			},
		},
	}

	d.handleMessageUpdated(toolMsg)

	chain := d.GetChain("msg-1")
	if chain == nil {
		t.Fatal("expected to find chain")
	}

	if chain.Calls[0].Output != "hello" {
		t.Errorf("expected output 'hello', got '%s'", chain.Calls[0].Output)
	}

	if chain.Calls[0].IsError {
		t.Error("expected IsError=false")
	}
}

func TestDetector_ChainCompletion(t *testing.T) {
	d := NewDetectorWithDefaults()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Subscribe to events
	eventCh := d.Subscribe(ctx)

	// Create message with tool call
	msg := message.Message{
		ID:        "msg-1",
		SessionID: "session-1",
		Role:      message.Assistant,
		Parts: []message.ContentPart{
			message.ToolCall{
				ID:       "tc-1",
				Name:     "bash",
				Input:    `{"command": "ls"}`,
				Finished: true,
			},
		},
	}

	d.handleMessageCreated(msg)

	// Receive the chain started event
	select {
	case event := <-eventCh:
		if event.Type != ChainStartedEvent {
			t.Errorf("expected ChainStartedEvent, got %s", event.Type)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for ChainStartedEvent")
	}

	// Mark message as finished
	msg.Parts = append(msg.Parts, message.Finish{
		Reason: message.FinishReasonEndTurn,
		Time:   time.Now().Unix(),
	})

	d.handleMessageUpdated(msg)

	// Receive the chain completed event
	select {
	case event := <-eventCh:
		if event.Type != ChainCompletedEvent {
			t.Errorf("expected ChainCompletedEvent, got %s", event.Type)
		}
		if event.Payload.Chain.MessageID != "msg-1" {
			t.Errorf("expected message ID 'msg-1', got '%s'", event.Payload.Chain.MessageID)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for ChainCompletedEvent")
	}
}

func TestDetector_GetChainForSession(t *testing.T) {
	d := NewDetectorWithDefaults()

	// Create chains for two sessions
	msg1 := message.Message{
		ID:        "msg-1",
		SessionID: "session-1",
		Role:      message.Assistant,
		Parts: []message.ContentPart{
			message.ToolCall{ID: "tc-1", Name: "bash"},
		},
	}

	msg2 := message.Message{
		ID:        "msg-2",
		SessionID: "session-2",
		Role:      message.Assistant,
		Parts: []message.ContentPart{
			message.ToolCall{ID: "tc-2", Name: "view"},
		},
	}

	d.handleMessageCreated(msg1)
	d.handleMessageCreated(msg2)

	chain1 := d.GetChainForSession("session-1")
	if chain1 == nil {
		t.Fatal("expected to find chain for session-1")
	}
	if chain1.MessageID != "msg-1" {
		t.Errorf("expected message ID 'msg-1', got '%s'", chain1.MessageID)
	}

	chain2 := d.GetChainForSession("session-2")
	if chain2 == nil {
		t.Fatal("expected to find chain for session-2")
	}
	if chain2.MessageID != "msg-2" {
		t.Errorf("expected message ID 'msg-2', got '%s'", chain2.MessageID)
	}

	// Non-existent session
	chain3 := d.GetChainForSession("session-3")
	if chain3 != nil {
		t.Error("expected nil for non-existent session")
	}
}

func TestDetector_ClearChain(t *testing.T) {
	d := NewDetectorWithDefaults()

	msg := message.Message{
		ID:        "msg-1",
		SessionID: "session-1",
		Role:      message.Assistant,
		Parts: []message.ContentPart{
			message.ToolCall{ID: "tc-1", Name: "bash"},
		},
	}

	d.handleMessageCreated(msg)

	if d.ActiveChainCount() != 1 {
		t.Errorf("expected 1 active chain, got %d", d.ActiveChainCount())
	}

	d.ClearChain("msg-1")

	if d.ActiveChainCount() != 0 {
		t.Errorf("expected 0 active chains after clear, got %d", d.ActiveChainCount())
	}

	if d.GetChain("msg-1") != nil {
		t.Error("expected nil chain after clear")
	}
}

func TestDetector_ClearSessionChains(t *testing.T) {
	d := NewDetectorWithDefaults()

	// Create multiple chains for same session
	msg1 := message.Message{
		ID:        "msg-1",
		SessionID: "session-1",
		Role:      message.Assistant,
		Parts: []message.ContentPart{
			message.ToolCall{ID: "tc-1", Name: "bash"},
		},
	}

	msg2 := message.Message{
		ID:        "msg-2",
		SessionID: "session-1",
		Role:      message.Assistant,
		Parts: []message.ContentPart{
			message.ToolCall{ID: "tc-2", Name: "view"},
		},
	}

	msg3 := message.Message{
		ID:        "msg-3",
		SessionID: "session-2",
		Role:      message.Assistant,
		Parts: []message.ContentPart{
			message.ToolCall{ID: "tc-3", Name: "grep"},
		},
	}

	d.handleMessageCreated(msg1)
	d.handleMessageCreated(msg2)
	d.handleMessageCreated(msg3)

	if d.ActiveChainCount() != 3 {
		t.Errorf("expected 3 active chains, got %d", d.ActiveChainCount())
	}

	d.ClearSessionChains("session-1")

	if d.ActiveChainCount() != 1 {
		t.Errorf("expected 1 active chain after clearing session-1, got %d", d.ActiveChainCount())
	}

	if d.GetChain("msg-3") == nil {
		t.Error("expected chain for msg-3 to remain")
	}
}

func TestDetector_GetAllChains(t *testing.T) {
	d := NewDetectorWithDefaults()

	msg1 := message.Message{
		ID:        "msg-1",
		SessionID: "session-1",
		Role:      message.Assistant,
		Parts: []message.ContentPart{
			message.ToolCall{ID: "tc-1", Name: "bash"},
		},
	}

	msg2 := message.Message{
		ID:        "msg-2",
		SessionID: "session-2",
		Role:      message.Assistant,
		Parts: []message.ContentPart{
			message.ToolCall{ID: "tc-2", Name: "view"},
		},
	}

	d.handleMessageCreated(msg1)
	d.handleMessageCreated(msg2)

	chains := d.GetAllChains()
	if len(chains) != 2 {
		t.Errorf("expected 2 chains, got %d", len(chains))
	}
}

func TestDetector_IgnoresNonAssistantMessages(t *testing.T) {
	d := NewDetectorWithDefaults()

	// User message should be ignored
	userMsg := message.Message{
		ID:        "msg-1",
		SessionID: "session-1",
		Role:      message.User,
		Parts: []message.ContentPart{
			message.TextContent{Text: "Hello"},
		},
	}

	d.handleMessageCreated(userMsg)

	if d.ActiveChainCount() != 0 {
		t.Errorf("expected 0 chains for user message, got %d", d.ActiveChainCount())
	}
}

func TestDetector_IgnoresAssistantWithoutToolCalls(t *testing.T) {
	d := NewDetectorWithDefaults()

	// Assistant message without tool calls should be ignored
	assistantMsg := message.Message{
		ID:        "msg-1",
		SessionID: "session-1",
		Role:      message.Assistant,
		Parts: []message.ContentPart{
			message.TextContent{Text: "Here's my response"},
		},
	}

	d.handleMessageCreated(assistantMsg)

	if d.ActiveChainCount() != 0 {
		t.Errorf("expected 0 chains for assistant without tool calls, got %d", d.ActiveChainCount())
	}
}

func TestDetector_ConfigGetSet(t *testing.T) {
	d := NewDetectorWithDefaults()

	originalCfg := d.Config()
	if !originalCfg.Enabled {
		t.Error("expected default config to have Enabled=true")
	}

	newCfg := Config{
		Enabled:  false,
		MinCalls: 5,
	}
	d.SetConfig(newCfg)

	updatedCfg := d.Config()
	if updatedCfg.Enabled {
		t.Error("expected updated config to have Enabled=false")
	}
	if updatedCfg.MinCalls != 5 {
		t.Errorf("expected MinCalls=5, got %d", updatedCfg.MinCalls)
	}
}

func TestDetector_ErrorToolResult(t *testing.T) {
	d := NewDetectorWithDefaults()

	// Create message with tool call
	assistantMsg := message.Message{
		ID:        "msg-1",
		SessionID: "session-1",
		Role:      message.Assistant,
		Parts: []message.ContentPart{
			message.ToolCall{
				ID:       "tc-1",
				Name:     "bash",
				Input:    `{"command": "invalid-command"}`,
				Finished: true,
			},
		},
	}

	d.handleMessageCreated(assistantMsg)

	// Process error tool result
	toolMsg := message.Message{
		ID:        "msg-2",
		SessionID: "session-1",
		Role:      message.Tool,
		Parts: []message.ContentPart{
			message.ToolResult{
				ToolCallID: "tc-1",
				Content:    "command not found: invalid-command",
				IsError:    true,
			},
		},
	}

	d.handleMessageUpdated(toolMsg)

	chain := d.GetChain("msg-1")
	if chain == nil {
		t.Fatal("expected to find chain")
	}

	if !chain.Calls[0].IsError {
		t.Error("expected IsError=true")
	}

	if !chain.HasErrors() {
		t.Error("expected chain.HasErrors()=true")
	}

	if chain.ErrorCount() != 1 {
		t.Errorf("expected error count 1, got %d", chain.ErrorCount())
	}
}

func TestDetector_MultipleSessions(t *testing.T) {
	d := NewDetectorWithDefaults()

	// Create chains in multiple sessions
	sessions := []string{"session-1", "session-2", "session-3"}
	for i, sessionID := range sessions {
		msg := message.Message{
			ID:        fmt.Sprintf("msg-%d", i+1),
			SessionID: sessionID,
			Role:      message.Assistant,
			Parts: []message.ContentPart{
				message.ToolCall{
					ID:   fmt.Sprintf("tc-%d", i+1),
					Name: "bash",
				},
			},
		}
		d.handleMessageCreated(msg)
	}

	if d.ActiveChainCount() != 3 {
		t.Errorf("expected 3 active chains, got %d", d.ActiveChainCount())
	}

	// Verify each session has its own chain
	for i, sessionID := range sessions {
		chain := d.GetChainForSession(sessionID)
		if chain == nil {
			t.Errorf("expected chain for %s", sessionID)
			continue
		}
		expectedMsgID := fmt.Sprintf("msg-%d", i+1)
		if chain.MessageID != expectedMsgID {
			t.Errorf("expected message ID '%s' for %s, got '%s'", expectedMsgID, sessionID, chain.MessageID)
		}
	}
}
