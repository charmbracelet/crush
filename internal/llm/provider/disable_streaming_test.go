package provider

import (
	"context"
	"testing"
	"time"

	"github.com/charmbracelet/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/llm/tools"
	"github.com/charmbracelet/crush/internal/message"
)

func TestDisableStreaming(t *testing.T) {
	// Create a mock provider configuration
	providerCfg := config.ProviderConfig{
		ID:   "test-provider",
		Name: "Test Provider",
		Type: catwalk.TypeOpenAI,
		Models: []catwalk.Model{
			{
				ID:               "test-model",
				Name:             "Test Model",
				DefaultMaxTokens: 1000,
			},
		},
	}

	// Create provider options with streaming disabled
	opts := providerClientOptions{
		config:           providerCfg,
		apiKey:           "test-key",
		disableStreaming: true,
		model: func(tp config.SelectedModelType) catwalk.Model {
			return catwalk.Model{
				ID:               "test-model",
				Name:             "Test Model",
				DefaultMaxTokens: 1000,
			}
		},
	}

	// Create a base provider with a mock client
	provider := &baseProvider[*mockProviderClient]{
		options: opts,
		client: &mockProviderClient{
			response: &ProviderResponse{
				Content:      "Test response",
				FinishReason: message.FinishReasonEndTurn,
			},
		},
	}

	// Test that StreamResponse returns non-streaming events when disableStreaming is true
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	messages := []message.Message{
		{
			Role: message.User,
			Parts: []message.ContentPart{
				message.TextContent{Text: "Hello"},
			},
		},
	}

	eventChan := provider.StreamResponse(ctx, messages, []tools.BaseTool{})

	// Collect events
	var events []ProviderEvent
	for event := range eventChan {
		events = append(events, event)
	}

	// Verify we got the expected events (content delta + complete)
	if len(events) != 2 {
		t.Errorf("Expected 2 events, got %d", len(events))
	}

	// Check first event is content delta
	if len(events) > 0 && events[0].Type != EventContentDelta {
		t.Errorf("Expected first event to be EventContentDelta, got %v", events[0].Type)
	}

	// Check last event is complete with the response
	if len(events) > 1 {
		if events[1].Type != EventComplete {
			t.Errorf("Expected last event to be EventComplete, got %v", events[1].Type)
		}
		if events[1].Response == nil {
			t.Error("Expected EventComplete to have a Response")
		} else if events[1].Response.Content != "Test response" {
			t.Errorf("Expected response content to be 'Test response', got %v", events[1].Response.Content)
		}
	}
}

func TestDisableStreamingWithToolCalls(t *testing.T) {
	// Create a mock provider configuration
	providerCfg := config.ProviderConfig{
		ID:   "test-provider",
		Name: "Test Provider",
		Type: catwalk.TypeOpenAI,
	}

	// Create provider options with streaming disabled
	opts := providerClientOptions{
		config:           providerCfg,
		apiKey:           "test-key",
		disableStreaming: true,
		model: func(tp config.SelectedModelType) catwalk.Model {
			return catwalk.Model{
				ID:               "test-model",
				Name:             "Test Model",
				DefaultMaxTokens: 1000,
			}
		},
	}

	// Create a base provider with a mock client that returns tool calls
	provider := &baseProvider[*mockProviderClient]{
		options: opts,
		client: &mockProviderClient{
			response: &ProviderResponse{
				Content: "",
				ToolCalls: []message.ToolCall{
					{
						ID:       "tool-1",
						Name:     "test_tool",
						Input:    `{"param": "value"}`,
						Finished: true,
					},
				},
				FinishReason: message.FinishReasonToolUse,
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	messages := []message.Message{
		{
			Role: message.User,
			Parts: []message.ContentPart{
				message.TextContent{Text: "Use a tool"},
			},
		},
	}

	eventChan := provider.StreamResponse(ctx, messages, []tools.BaseTool{})

	// Collect events
	var events []ProviderEvent
	for event := range eventChan {
		events = append(events, event)
	}

	// With tool calls and no content, we should only get the complete event
	if len(events) != 1 {
		t.Errorf("Expected 1 event for tool-only response, got %d", len(events))
	}

	// Check the event is complete with tool calls
	if len(events) > 0 {
		if events[0].Type != EventComplete {
			t.Errorf("Expected EventComplete, got %v", events[0].Type)
		}
		if events[0].Response == nil {
			t.Error("Expected EventComplete to have a Response")
		} else {
			if len(events[0].Response.ToolCalls) != 1 {
				t.Errorf("Expected 1 tool call, got %d", len(events[0].Response.ToolCalls))
			}
			if events[0].Response.FinishReason != message.FinishReasonToolUse {
				t.Errorf("Expected FinishReasonToolUse, got %v", events[0].Response.FinishReason)
			}
		}
	}
}

// Mock provider client for testing
type mockProviderClient struct {
	response *ProviderResponse
	err      error
}

func (m *mockProviderClient) send(ctx context.Context, messages []message.Message, tools []tools.BaseTool) (*ProviderResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.response, nil
}

func (m *mockProviderClient) stream(ctx context.Context, messages []message.Message, tools []tools.BaseTool) <-chan ProviderEvent {
	// This should not be called when streaming is disabled
	panic("stream should not be called when streaming is disabled")
}

func (m *mockProviderClient) Model() catwalk.Model {
	return catwalk.Model{
		ID:               "test-model",
		Name:             "Test Model",
		DefaultMaxTokens: 1000,
	}
}