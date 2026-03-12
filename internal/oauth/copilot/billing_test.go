package copilot

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBillingTransport_ChatCompletionsInitiatorDetection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		messages     []map[string]any
		expectedInit string
	}{
		{
			name: "new user request is billable",
			messages: []map[string]any{
				{"role": "user", "content": "Write a function to sort an array"},
			},
			expectedInit: InitiatorUser,
		},
		{
			name: "user follow-up after assistant history is billable",
			messages: []map[string]any{
				{"role": "system", "content": "You are helpful."},
				{"role": "user", "content": "Write a function"},
				{"role": "assistant", "content": "Here is the function."},
				{"role": "user", "content": "Can you optimize it?"},
			},
			expectedInit: InitiatorUser,
		},
		{
			name: "tool continuation after system prompt is free",
			messages: []map[string]any{
				{"role": "system", "content": "You are helpful."},
				{"role": "user", "content": "Inspect the repository."},
				{"role": "assistant", "content": "Let me inspect it.", "tool_calls": []any{map[string]any{"id": "call_1"}}},
				{"role": "tool", "content": "Repository contents..."},
			},
			expectedInit: InitiatorAgent,
		},
		{
			name: "resume prompt without explicit initiator remains billable",
			messages: []map[string]any{
				{"role": "user", "content": "The previous session was interrupted because it got too long, the initial user request was: `original prompt`"},
			},
			expectedInit: InitiatorUser,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			initiator := detectInitiator(t, map[string]any{"messages": tt.messages})
			if initiator != tt.expectedInit {
				require.Equal(t, tt.expectedInit, initiator, "getInitiatorType() mismatch")
			}
		})
	}
}

func TestBillingTransport_ResponsesAPIInitiatorDetection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		input        []map[string]any
		expectedInit string
	}{
		{
			name: "direct user request is billable",
			input: []map[string]any{
				{"type": "message", "role": "developer", "content": "You are helpful."},
				{"type": "message", "role": "user", "content": "Write a function."},
			},
			expectedInit: InitiatorUser,
		},
		{
			name: "user follow-up with assistant history is billable",
			input: []map[string]any{
				{"type": "message", "role": "developer", "content": "You are helpful."},
				{"type": "message", "role": "user", "content": "Write a function."},
				{"type": "message", "role": "assistant", "content": "Here is the function."},
				{"type": "message", "role": "user", "content": "Can you optimize it?"},
			},
			expectedInit: InitiatorUser,
		},
		{
			name: "function-call continuation is free",
			input: []map[string]any{
				{"type": "message", "role": "developer", "content": "You are helpful."},
				{"type": "message", "role": "user", "content": "Inspect the repository."},
				{"type": "function_call", "call_id": "call_1", "name": "read_file", "arguments": "{}"},
				{"type": "function_call_output", "call_id": "call_1", "output": "Repository contents..."},
			},
			expectedInit: InitiatorAgent,
		},
		{
			name: "assistant continuation after developer prompt is free",
			input: []map[string]any{
				{"type": "message", "role": "developer", "content": "You are helpful."},
				{"type": "message", "role": "assistant", "content": "Continuing from the previous tool result."},
			},
			expectedInit: InitiatorAgent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			initiator := detectInitiator(t, map[string]any{"input": tt.input})
			if initiator != tt.expectedInit {
				require.Equal(t, tt.expectedInit, initiator, "getInitiatorType() mismatch")
			}
		})
	}
}

func TestBillingTransport_EdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("empty messages", func(t *testing.T) {
		t.Parallel()

		initiator := detectInitiator(t, map[string]any{"messages": []any{}})
		require.Equal(t, InitiatorUser, initiator, "Empty messages should default to user")
	})

	t.Run("invalid JSON", func(t *testing.T) {
		t.Parallel()

		req, _ := http.NewRequestWithContext(context.Background(), "POST", "https://api.example.com/v1/chat/completions", strings.NewReader("invalid json"))
		transport := &billingTransport{}
		initiator := transport.getInitiatorType(req)
		require.Equal(t, InitiatorUser, initiator, "Invalid JSON should default to user")
	})

	t.Run("nil body", func(t *testing.T) {
		t.Parallel()

		req, _ := http.NewRequestWithContext(context.Background(), "POST", "https://api.example.com/v1/chat/completions", nil)
		transport := &billingTransport{}
		initiator := transport.getInitiatorType(req)
		require.Equal(t, InitiatorUser, initiator, "Nil body should default to user")
	})
}

func TestBillingTransport_ContextInitiator(t *testing.T) {
	t.Parallel()

	t.Run("context_with_user_initiator", func(t *testing.T) {
		t.Parallel()

		payload := map[string]any{
			"messages": []map[string]any{{"role": "assistant", "content": "Tool call result"}},
		}
		bodyBytes, _ := json.Marshal(payload)
		req, _ := http.NewRequestWithContext(context.Background(), "POST", "https://api.example.com/v1/chat/completions", bytes.NewReader(bodyBytes))
		req = req.WithContext(ContextWithInitiatorType(req.Context(), InitiatorUser))

		transport := &billingTransport{}
		initiator := transport.getInitiatorType(req)
		require.Equal(t, InitiatorUser, initiator, "Context initiator should override body detection")
	})

	t.Run("context_with_agent_initiator", func(t *testing.T) {
		t.Parallel()

		payload := map[string]any{
			"messages": []map[string]any{{"role": "user", "content": "New request"}},
		}
		bodyBytes, _ := json.Marshal(payload)
		req, _ := http.NewRequestWithContext(context.Background(), "POST", "https://api.example.com/v1/chat/completions", bytes.NewReader(bodyBytes))
		req = req.WithContext(ContextWithInitiatorType(req.Context(), InitiatorAgent))

		transport := &billingTransport{}
		initiator := transport.getInitiatorType(req)
		require.Equal(t, InitiatorAgent, initiator, "Context initiator should override body detection")
	})

	t.Run("resume_prompt_requires_explicit_agent_initiator", func(t *testing.T) {
		t.Parallel()

		payload := map[string]any{
			"messages": []map[string]any{{"role": "user", "content": "The previous session was interrupted because it got too long, the initial user request was: `test`"}},
		}
		bodyBytes, _ := json.Marshal(payload)
		req, _ := http.NewRequestWithContext(context.Background(), "POST", "https://api.example.com/v1/chat/completions", bytes.NewReader(bodyBytes))
		req = req.WithContext(ContextWithInitiatorType(req.Context(), InitiatorAgent))

		transport := &billingTransport{}
		initiator := transport.getInitiatorType(req)
		require.Equal(t, InitiatorAgent, initiator, "Explicit agent initiator should control resume billing")
	})

	t.Run("context_with_invalid_initiator_falls_back_to_body", func(t *testing.T) {
		t.Parallel()

		payload := map[string]any{
			"messages": []map[string]any{{"role": "assistant", "content": "Tool call"}},
		}
		bodyBytes, _ := json.Marshal(payload)
		req, _ := http.NewRequestWithContext(context.Background(), "POST", "https://api.example.com/v1/chat/completions", bytes.NewReader(bodyBytes))
		req = req.WithContext(context.WithValue(req.Context(), InitiatorTypeKey, "invalid_value"))

		transport := &billingTransport{}
		initiator := transport.getInitiatorType(req)
		require.Equal(t, InitiatorAgent, initiator, "Invalid context initiator should fall back to body detection")
	})
}

func detectInitiator(t *testing.T, payload map[string]any) string {
	t.Helper()

	bodyBytes, err := json.Marshal(payload)
	require.NoError(t, err, "Failed to marshal test data")

	req, err := http.NewRequestWithContext(context.Background(), "POST", "https://api.example.com/v1/chat/completions", bytes.NewReader(bodyBytes))
	require.NoError(t, err, "Failed to create request")

	transport := &billingTransport{
		debug:    false,
		wrapped:  http.DefaultTransport,
		fallback: false,
	}

	return transport.getInitiatorType(req)
}
