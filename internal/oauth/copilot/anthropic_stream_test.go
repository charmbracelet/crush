package copilot

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"charm.land/fantasy"
	"charm.land/fantasy/providers/anthropic"
	"github.com/stretchr/testify/require"
)

const copilotTestModel = "gpt-5-mini"

type capturedAnthropicRequest struct {
	body    map[string]any
	headers http.Header
}

func newAnthropicCopilotFixtureServer(t *testing.T, chunks []string) (*httptest.Server, <-chan capturedAnthropicRequest) {
	t.Helper()

	requests := make(chan capturedAnthropicRequest, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		requests <- capturedAnthropicRequest{
			body:    body,
			headers: r.Header.Clone(),
		}

		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "missing flusher", http.StatusInternalServerError)
			return
		}
		for _, chunk := range chunks {
			_, _ = io.WriteString(w, chunk)
			flusher.Flush()
		}
	}))

	return server, requests
}

func newAnthropicCopilotModel(t *testing.T, baseURL string) fantasy.LanguageModel {
	t.Helper()

	provider, err := anthropic.New(
		anthropic.WithAPIKey("test-api-key"),
		anthropic.WithBaseURL(baseURL),
		anthropic.WithHTTPClient(NewClient(false, false)),
	)
	require.NoError(t, err)

	model, err := provider.LanguageModel(context.Background(), copilotTestModel)
	require.NoError(t, err)
	return model
}

func collectStreamParts(stream fantasy.StreamResponse) []fantasy.StreamPart {
	var parts []fantasy.StreamPart
	for part := range stream {
		parts = append(parts, part)
		if part.Type == fantasy.StreamPartTypeError || part.Type == fantasy.StreamPartTypeFinish {
			break
		}
	}
	return parts
}

func streamDeltas(parts []fantasy.StreamPart, partType fantasy.StreamPartType) []string {
	var deltas []string
	for _, part := range parts {
		if part.Type == partType && part.Delta != "" {
			deltas = append(deltas, part.Delta)
		}
	}
	return deltas
}

func hasStreamPart(parts []fantasy.StreamPart, partType fantasy.StreamPartType) bool {
	for _, part := range parts {
		if part.Type == partType {
			return true
		}
	}
	return false
}

func testPrompt(text string) fantasy.Prompt {
	return fantasy.Prompt{
		{
			Role: fantasy.MessageRoleUser,
			Content: []fantasy.MessagePart{
				fantasy.TextPart{Text: text},
			},
		},
	}
}

func TestAnthropicCopilotStream_ParsesThinkingAndTextFixture(t *testing.T) {
	t.Parallel()

	// These are parser fixtures only; protocol selection lives in copilot-api.
	chunks := []string{
		"event: message_start\n",
		"data: {\"type\":\"message_start\",\"message\":{\"id\":\"resp_yh4kko7j2w\",\"type\":\"message\",\"role\":\"assistant\",\"content\":[],\"model\":\"gpt-5-mini\",\"stop_reason\":null,\"stop_sequence\":null,\"usage\":{\"input_tokens\":0,\"output_tokens\":0}}}\n\n",
		"event: content_block_start\n",
		"data: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"thinking\",\"thinking\":\"\"}}\n\n",
		"event: content_block_delta\n",
		"data: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"thinking_delta\",\"thinking\":\"**Providing concise prime answer**\"}}\n\n",
		"event: content_block_stop\n",
		"data: {\"type\":\"content_block_stop\",\"index\":0}\n\n",
		"event: content_block_start\n",
		"data: {\"type\":\"content_block_start\",\"index\":1,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n",
		"event: content_block_delta\n",
		"data: {\"type\":\"content_block_delta\",\"index\":1,\"delta\":{\"type\":\"text_delta\",\"text\":\"Yes\"}}\n\n",
		"event: content_block_delta\n",
		"data: {\"type\":\"content_block_delta\",\"index\":1,\"delta\":{\"type\":\"text_delta\",\"text\":\", 17 is prime.\"}}\n\n",
		"event: content_block_stop\n",
		"data: {\"type\":\"content_block_stop\",\"index\":1}\n\n",
		"event: message_delta\n",
		"data: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\",\"stop_sequence\":null},\"usage\":{\"input_tokens\":23,\"output_tokens\":130,\"cache_read_input_tokens\":0}}\n\n",
		"event: message_stop\n",
		"data: {\"type\":\"message_stop\"}\n\n",
	}

	server, requests := newAnthropicCopilotFixtureServer(t, chunks)
	defer server.Close()

	model := newAnthropicCopilotModel(t, server.URL)
	stream, err := model.Stream(context.Background(), fantasy.Call{
		Prompt: testPrompt("Think step by step about whether 17 is prime."),
		ProviderOptions: anthropic.NewProviderOptions(&anthropic.ProviderOptions{
			Thinking: &anthropic.ThinkingProviderOption{BudgetTokens: 28672},
		}),
	})
	require.NoError(t, err)

	parts := collectStreamParts(stream)
	req := <-requests

	require.Equal(t, "2023-06-01", req.headers.Get("anthropic-version"))
	require.Equal(t, map[string]any{
		"budget_tokens": float64(28672),
		"type":          "enabled",
	}, req.body["thinking"])
	require.Equal(t, []string{"**Providing concise prime answer**"}, streamDeltas(parts, fantasy.StreamPartTypeReasoningDelta))
	require.Equal(t, []string{"Yes", ", 17 is prime."}, streamDeltas(parts, fantasy.StreamPartTypeTextDelta))
	require.True(t, hasStreamPart(parts, fantasy.StreamPartTypeReasoningStart))
	require.True(t, hasStreamPart(parts, fantasy.StreamPartTypeReasoningEnd))
	require.True(t, hasStreamPart(parts, fantasy.StreamPartTypeFinish))
}

func TestAnthropicCopilotStream_ParsesToolOnlyFixture(t *testing.T) {
	t.Parallel()

	chunks := []string{
		"event: message_start\n",
		"data: {\"type\":\"message_start\",\"message\":{\"id\":\"resp_9f8a1g77oq6\",\"type\":\"message\",\"role\":\"assistant\",\"content\":[],\"model\":\"gpt-5-mini\",\"stop_reason\":null,\"stop_sequence\":null,\"usage\":{\"input_tokens\":0,\"output_tokens\":0}}}\n\n",
		"event: content_block_start\n",
		"data: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"tool_use\",\"id\":\"call_bQ5TpbOy1ePUqHx9MSHts8YJ\",\"name\":\"echo_value\",\"input\":{}}}\n\n",
		"event: content_block_delta\n",
		"data: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"input_json_delta\",\"partial_json\":\"{\\\"\"}}\n\n",
		"event: content_block_delta\n",
		"data: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"input_json_delta\",\"partial_json\":\"value\"}}\n\n",
		"event: content_block_delta\n",
		"data: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"input_json_delta\",\"partial_json\":\"\\\":\\\"\"}}\n\n",
		"event: content_block_delta\n",
		"data: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"input_json_delta\",\"partial_json\":\"hello-tool\"}}\n\n",
		"event: content_block_delta\n",
		"data: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"input_json_delta\",\"partial_json\":\"\\\"}\"}}\n\n",
		"event: content_block_stop\n",
		"data: {\"type\":\"content_block_stop\",\"index\":0}\n\n",
		"event: message_delta\n",
		"data: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"tool_use\",\"stop_sequence\":null},\"usage\":{\"input_tokens\":59,\"output_tokens\":69,\"cache_read_input_tokens\":0}}\n\n",
		"event: message_stop\n",
		"data: {\"type\":\"message_stop\"}\n\n",
	}

	server, _ := newAnthropicCopilotFixtureServer(t, chunks)
	defer server.Close()

	model := newAnthropicCopilotModel(t, server.URL)
	stream, err := model.Stream(context.Background(), fantasy.Call{
		Prompt: testPrompt("Use the echo_value tool."),
	})
	require.NoError(t, err)

	parts := collectStreamParts(stream)

	require.Empty(t, streamDeltas(parts, fantasy.StreamPartTypeReasoningDelta))
	require.Empty(t, streamDeltas(parts, fantasy.StreamPartTypeTextDelta))
	require.True(t, hasStreamPart(parts, fantasy.StreamPartTypeToolInputStart))
	require.True(t, hasStreamPart(parts, fantasy.StreamPartTypeToolInputEnd))
	require.True(t, hasStreamPart(parts, fantasy.StreamPartTypeToolCall))

	var toolInput string
	for _, part := range parts {
		if part.Type == fantasy.StreamPartTypeToolCall {
			toolInput = part.ToolCallInput
		}
	}
	require.JSONEq(t, `{"value":"hello-tool"}`, toolInput)
}

func TestAnthropicCopilotStream_ParsesPostToolTextFixture(t *testing.T) {
	t.Parallel()

	chunks := []string{
		"event: message_start\n",
		"data: {\"type\":\"message_start\",\"message\":{\"id\":\"resp_x8ujih4alo\",\"type\":\"message\",\"role\":\"assistant\",\"content\":[],\"model\":\"gpt-5-mini\",\"stop_reason\":null,\"stop_sequence\":null,\"usage\":{\"input_tokens\":0,\"output_tokens\":0}}}\n\n",
		"event: content_block_start\n",
		"data: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n",
		"event: content_block_delta\n",
		"data: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"Done\"}}\n\n",
		"event: content_block_delta\n",
		"data: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"- tool returned hello-tool.\"}}\n\n",
		"event: content_block_stop\n",
		"data: {\"type\":\"content_block_stop\",\"index\":0}\n\n",
		"event: message_delta\n",
		"data: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\",\"stop_sequence\":null},\"usage\":{\"input_tokens\":91,\"output_tokens\":16,\"cache_read_input_tokens\":0}}\n\n",
		"event: message_stop\n",
		"data: {\"type\":\"message_stop\"}\n\n",
	}

	server, _ := newAnthropicCopilotFixtureServer(t, chunks)
	defer server.Close()

	model := newAnthropicCopilotModel(t, server.URL)
	stream, err := model.Stream(context.Background(), fantasy.Call{
		Prompt: testPrompt("Summarize the tool result."),
	})
	require.NoError(t, err)

	parts := collectStreamParts(stream)

	require.Equal(t, []string{"Done", "- tool returned hello-tool."}, streamDeltas(parts, fantasy.StreamPartTypeTextDelta))
	require.True(t, hasStreamPart(parts, fantasy.StreamPartTypeFinish))
}
