package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/charmbracelet/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

// mockAnthropicError creates a test error that behaves like anthropic.Error
type mockAnthropicError struct {
	statusCode int
	message    string
	response   *http.Response
	request    *http.Request
}

func (e *mockAnthropicError) Error() string {
	// Mimic the actual anthropic.Error format
	return fmt.Sprintf("POST %s: %d %s", e.request.URL, e.statusCode, http.StatusText(e.statusCode))
}

func (e *mockAnthropicError) StatusCode() int {
	return e.statusCode
}

// Helper to create an anthropic.Error for testing
func createMockError(statusCode int, message string) *anthropic.Error {
	// Create a mock response
	resp := &http.Response{
		StatusCode: statusCode,
		Header:     http.Header{},
		Body:       io.NopCloser(strings.NewReader(message)),
	}

	// Create the actual error type used by the SDK
	apiErr := &anthropic.Error{
		StatusCode: statusCode,
		Response:   resp,
		Request:    &http.Request{Method: "POST"},
	}

	// The Error() method concatenates statusInfo + JSON.raw
	// We'll use UnmarshalJSON with a JSON object that will set JSON.raw to our message
	jsonData := fmt.Sprintf(`"%s"`, message) // Wrap in quotes to make it valid JSON
	_ = apiErr.UnmarshalJSON([]byte(jsonData))

	return apiErr
}

func TestAnthropicClient_ConvertMessages_HandlesOrphanedToolCalls(t *testing.T) {
	t.Parallel()
	client := &anthropicClient{
		providerOptions: providerClientOptions{
			modelType:     config.SelectedModelTypeLarge,
			apiKey:        "test-key",
			systemMessage: "test",
			model: func(config.SelectedModelType) catwalk.Model {
				return catwalk.Model{
					ID:   "claude-3-5-sonnet-latest",
					Name: "Claude 3.5 Sonnet",
				}
			},
		},
	}

	t.Run("properly paired tool calls and results", func(t *testing.T) {
		t.Parallel()
		messages := []message.Message{
			{
				Role: message.User,
				Parts: []message.ContentPart{
					message.TextContent{Text: "Hello"},
				},
			},
			{
				Role: message.Assistant,
				Parts: []message.ContentPart{
					message.TextContent{Text: "I'll help you with that."},
					message.ToolCall{
						ID:       "tool_1",
						Name:     "test_tool",
						Input:    `{"param": "value"}`,
						Finished: true,
					},
				},
			},
			{
				Role: message.Tool,
				Parts: []message.ContentPart{
					message.ToolResult{
						ToolCallID: "tool_1",
						Content:    "Tool executed successfully",
						IsError:    false,
					},
				},
			},
		}

		anthropicMessages := client.convertMessages(messages)
		require.Len(t, anthropicMessages, 3)

		// Check that no placeholder tool results were added
		toolMessage := anthropicMessages[2]
		require.Len(t, toolMessage.Content, 1)
	})

	t.Run("orphaned tool call without result", func(t *testing.T) {
		t.Parallel()
		messages := []message.Message{
			{
				Role: message.User,
				Parts: []message.ContentPart{
					message.TextContent{Text: "Hello"},
				},
			},
			{
				Role: message.Assistant,
				Parts: []message.ContentPart{
					message.ToolCall{
						ID:       "orphaned_tool",
						Name:     "test_tool",
						Input:    `{"param": "value"}`,
						Finished: true,
					},
				},
			},
			{
				Role: message.User,
				Parts: []message.ContentPart{
					message.TextContent{Text: "Continue"},
				},
			},
		}

		anthropicMessages := client.convertMessages(messages)

		// Should have 4 messages: user, assistant, placeholder tool result, user
		require.Len(t, anthropicMessages, 4)

		// Check that a placeholder tool result was injected
		placeholderMessage := anthropicMessages[2]
		require.Len(t, placeholderMessage.Content, 1)

		// Verify the placeholder contains error result
		toolResult := placeholderMessage.Content[0]
		require.NotNil(t, toolResult.OfToolResult)
		require.Equal(t, "orphaned_tool", toolResult.OfToolResult.ToolUseID)
		// IsError is an Opt[bool] type, check if it's set and true
		require.True(t, toolResult.OfToolResult.IsError.Value)
		// Content is a union type, check if it contains our error message
		contentJSON, _ := json.Marshal(toolResult.OfToolResult.Content)
		require.Contains(t, string(contentJSON), "interrupted")
	})

	t.Run("multiple orphaned tool calls", func(t *testing.T) {
		t.Parallel()
		messages := []message.Message{
			{
				Role: message.Assistant,
				Parts: []message.ContentPart{
					message.ToolCall{
						ID:       "tool_1",
						Name:     "test_tool_1",
						Input:    `{"param": "value1"}`,
						Finished: true,
					},
					message.ToolCall{
						ID:       "tool_2",
						Name:     "test_tool_2",
						Input:    `{"param": "value2"}`,
						Finished: true,
					},
				},
			},
			{
				Role: message.User,
				Parts: []message.ContentPart{
					message.TextContent{Text: "Continue"},
				},
			},
		}

		anthropicMessages := client.convertMessages(messages)

		// Should have 3 messages: assistant, placeholder tool results, user
		require.Len(t, anthropicMessages, 3)

		// Check that placeholder tool results were injected for both tools
		placeholderMessage := anthropicMessages[1]
		require.Len(t, placeholderMessage.Content, 2)

		// Collect tool IDs from placeholder results
		toolIDs := make(map[string]bool)
		for _, content := range placeholderMessage.Content {
			require.NotNil(t, content.OfToolResult)
			require.True(t, content.OfToolResult.IsError.Value)
			toolIDs[content.OfToolResult.ToolUseID] = true
		}

		require.True(t, toolIDs["tool_1"], "Missing placeholder for tool_1")
		require.True(t, toolIDs["tool_2"], "Missing placeholder for tool_2")
	})

	// When a Tool message follows but only covers some (not all) tool_use IDs,
	// we must merge the real tool_result(s) with placeholder error results for
	// the missing tool_use IDs into a single user message immediately after the
	// assistant response, and skip processing the original Tool message to avoid
	// duplicate tool_result blocks.
	t.Run("partial tool_result coverage is merged and next Tool is skipped", func(t *testing.T) {
		t.Parallel()
		messages := []message.Message{
			{
				Role: message.Assistant,
				Parts: []message.ContentPart{
					message.ToolCall{ID: "tool_a", Name: "t1", Input: `{"x":1}`, Finished: true},
					message.ToolCall{ID: "tool_b", Name: "t2", Input: `{"y":2}`, Finished: true},
				},
			},
			{
				Role: message.Tool,
				Parts: []message.ContentPart{
					// Only one real tool_result is supplied by the next Tool message
					message.ToolResult{ToolCallID: "tool_a", Content: "ok", IsError: false},
				},
			},
			{
				Role:  message.User,
				Parts: []message.ContentPart{message.TextContent{Text: "Continue"}},
			},
		}

		anthropicMessages := client.convertMessages(messages)

		// Expect: assistant, merged user tool_results (real + placeholder), user
		require.Len(t, anthropicMessages, 3)
		merged := anthropicMessages[1]
		// Should contain 2 entries: one real, one placeholder for missing tool_b
		require.Len(t, merged.Content, 2)

		var hasReal, hasPlaceholder bool
		for _, cb := range merged.Content {
			require.NotNil(t, cb.OfToolResult)
			if cb.OfToolResult.ToolUseID == "tool_a" {
				require.False(t, cb.OfToolResult.IsError.Value)
				hasReal = true
			}
			if cb.OfToolResult.ToolUseID == "tool_b" {
				require.True(t, cb.OfToolResult.IsError.Value)
				hasPlaceholder = true
			}
		}
		require.True(t, hasReal, "expected real tool_result for tool_a")
		require.True(t, hasPlaceholder, "expected placeholder tool_result for tool_b")
	})

	t.Run("canceled message with empty content", func(t *testing.T) {
		t.Parallel()
		messages := []message.Message{
			{
				Role: message.Assistant,
				Parts: []message.ContentPart{
					message.Finish{
						Reason:  message.FinishReasonCanceled,
						Message: "Request cancelled",
					},
				},
			},
		}

		anthropicMessages := client.convertMessages(messages)

		// Empty assistant messages should be skipped
		require.Len(t, anthropicMessages, 0)
	})
}

func TestAnthropicClient_ShouldRetry_ToolUseError(t *testing.T) {
	t.Parallel()
	client := &anthropicClient{
		providerOptions: providerClientOptions{
			apiKey: "test-key",
		},
	}

	t.Run("tool_use/tool_result mismatch error", func(t *testing.T) {
		t.Parallel()
		// Create an error that contains the tool_use/tool_result text
		apiErr := createMockError(400,
			"messages.92: `tool_use` ids were found without `tool_result` blocks immediately after: toolu_01AWFE1DffRECo8rXkvRUkxx")

		retry, _, err := client.shouldRetry(1, apiErr)

		require.False(t, retry, "Should not retry tool_use/tool_result mismatch")
		require.Error(t, err)
		// The error wraps the original, so check for our custom message
		require.Contains(t, err.Error(), "conversation history error")
	})

	t.Run("context limit error", func(t *testing.T) {
		t.Parallel()
		apiErr := createMockError(400,
			"input length and `max_tokens` exceed context limit: 150000 + 50000 > 200000")

		retry, _, err := client.shouldRetry(1, apiErr)

		require.True(t, retry, "Should retry context limit error with adjusted max_tokens")
		require.NoError(t, err)
	})

	t.Run("rate limit error", func(t *testing.T) {
		t.Parallel()
		apiErr := createMockError(429, "Rate limit exceeded")

		retry, after, err := client.shouldRetry(1, apiErr)

		require.True(t, retry, "Should retry rate limit error")
		require.NoError(t, err)
		require.Greater(t, after, int64(0), "Should have backoff delay")
	})

	t.Run("max retries exceeded", func(t *testing.T) {
		t.Parallel()
		apiErr := createMockError(429, "Rate limit exceeded")

		retry, _, err := client.shouldRetry(maxRetries+1, apiErr)

		require.False(t, retry, "Should not retry after max retries")
		require.Error(t, err)
		require.Contains(t, err.Error(), "maximum retry attempts reached")
	})
}

func TestAnthropicClient_StreamErrorHandling(t *testing.T) {
	t.Parallel()
	t.Run("non-retryable error sends EventError", func(t *testing.T) {
		t.Parallel()
		// Create a mock server that returns a 400 error
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			response := map[string]any{
				"type": "error",
				"error": map[string]any{
					"type":    "invalid_request_error",
					"message": "Invalid request format",
				},
			}
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		client := &anthropicClient{
			providerOptions: providerClientOptions{
				modelType:     config.SelectedModelTypeLarge,
				apiKey:        "test-key",
				systemMessage: "test",
				model: func(config.SelectedModelType) catwalk.Model {
					return catwalk.Model{
						ID:               "claude-3-5-sonnet-latest",
						Name:             "Claude 3.5 Sonnet",
						DefaultMaxTokens: 4096,
					}
				},
			},
			client: anthropic.NewClient(
				option.WithAPIKey("test-key"),
				option.WithBaseURL(server.URL),
			),
		}

		messages := []message.Message{
			{
				Role: message.User,
				Parts: []message.ContentPart{
					message.TextContent{Text: "Hello"},
				},
			},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		eventsChan := client.stream(ctx, messages, nil)

		// Collect events
		var errorReceived bool
		for event := range eventsChan {
			if event.Type == EventError {
				errorReceived = true
				require.Error(t, event.Error)
				// The error message might be wrapped, just check it exists
				require.NotEmpty(t, event.Error.Error())
			}
		}

		require.True(t, errorReceived, "Should have received an EventError")
	})

	t.Run("context cancellation sends EventError", func(t *testing.T) {
		t.Parallel()
		// Create a mock server that delays response
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(2 * time.Second)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client := &anthropicClient{
			providerOptions: providerClientOptions{
				modelType:     config.SelectedModelTypeLarge,
				apiKey:        "test-key",
				systemMessage: "test",
				model: func(config.SelectedModelType) catwalk.Model {
					return catwalk.Model{
						ID:               "claude-3-5-sonnet-latest",
						Name:             "Claude 3.5 Sonnet",
						DefaultMaxTokens: 4096,
					}
				},
			},
			client: anthropic.NewClient(
				option.WithAPIKey("test-key"),
				option.WithBaseURL(server.URL),
			),
		}

		messages := []message.Message{
			{
				Role: message.User,
				Parts: []message.ContentPart{
					message.TextContent{Text: "Hello"},
				},
			},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		eventsChan := client.stream(ctx, messages, nil)

		// Collect events
		var errorReceived bool
		for event := range eventsChan {
			if event.Type == EventError {
				errorReceived = true
				require.Error(t, event.Error)
				require.Contains(t, event.Error.Error(), "context")
			}
		}

		require.True(t, errorReceived, "Should have received an EventError for context cancellation")
	})
}

func TestAnthropicClient_HandleContextLimitError(t *testing.T) {
	t.Parallel()
	client := &anthropicClient{}

	tests := []struct {
		name           string
		errorMessage   string
		expectedTokens int
		expectedOk     bool
	}{
		{
			name:           "valid context limit error",
			errorMessage:   "input length and `max_tokens` exceed context limit: 150000 + 50000 > 200000",
			expectedTokens: 49000, // 200000 - 150000 - 1000 (buffer)
			expectedOk:     true,
		},
		{
			name:           "context limit with smaller values",
			errorMessage:   "input length and `max_tokens` exceed context limit: 5000 + 3000 > 8000",
			expectedTokens: 2000, // 8000 - 5000 - 1000 (buffer)
			expectedOk:     true,
		},
		{
			name:           "context limit that would result in too small max_tokens",
			errorMessage:   "input length and `max_tokens` exceed context limit: 199500 + 1000 > 200000",
			expectedTokens: 1000, // Minimum threshold
			expectedOk:     true,
		},
		{
			name:           "non-context-limit error",
			errorMessage:   "Invalid request format",
			expectedTokens: 0,
			expectedOk:     false,
		},
		{
			name:           "malformed context limit error",
			errorMessage:   "input length and max_tokens exceed context limit: abc + def > ghi",
			expectedTokens: 0,
			expectedOk:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			apiErr := createMockError(400, tt.errorMessage)
			// Set the body so Error() returns our message
			apiErr.Response.Body = io.NopCloser(strings.NewReader(tt.errorMessage))

			adjustedTokens, ok := client.handleContextLimitError(apiErr)

			require.Equal(t, tt.expectedOk, ok, "Expected ok=%v for error: %s", tt.expectedOk, tt.errorMessage)
			if tt.expectedOk {
				require.Equal(t, tt.expectedTokens, adjustedTokens, "Unexpected adjusted tokens for error: %s", tt.errorMessage)
			}
		})
	}
}

//nolint:tparallel // subtests intentionally not parallel; they mutate global config.Get().Models and would race.
func TestAnthropicClient_PreparedMessages_WithThinking(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name             string
		modelType        config.SelectedModelType
		canReason        bool
		think            bool
		expectedThinking bool
	}{
		{
			name:             "thinking enabled for reasoning model",
			modelType:        config.SelectedModelTypeLarge,
			canReason:        true,
			think:            true,
			expectedThinking: true,
		},
		{
			name:             "thinking disabled when model can't reason",
			modelType:        config.SelectedModelTypeLarge,
			canReason:        false,
			think:            true,
			expectedThinking: false,
		},
		{
			name:             "thinking disabled when config says not to think",
			modelType:        config.SelectedModelTypeLarge,
			canReason:        true,
			think:            false,
			expectedThinking: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// not parallel; mutates global config
			// Set up config for test
			cfg := config.Get()
			if cfg.Models == nil {
				cfg.Models = make(map[config.SelectedModelType]config.SelectedModel)
			}
			cfg.Models[tt.modelType] = config.SelectedModel{
				Think: tt.think,
			}

			client := &anthropicClient{
				providerOptions: providerClientOptions{
					modelType:     tt.modelType,
					systemMessage: "test",
					model: func(config.SelectedModelType) catwalk.Model {
						return catwalk.Model{
							ID:               "claude-3-5-sonnet-latest",
							Name:             "Claude 3.5 Sonnet",
							DefaultMaxTokens: 4096,
							CanReason:        tt.canReason,
						}
					},
				},
			}

			params := client.preparedMessages([]anthropic.MessageParam{}, []anthropic.ToolUnionParam{})

			if tt.expectedThinking {
				// Check if thinking is set (not nil and has a value)
				hasThinking := params.Thinking.OfEnabled != nil || params.Thinking.OfDisabled != nil
				require.True(t, hasThinking, "Expected thinking to be enabled")
				// Temperature is an Opt[float64] type
				require.Equal(t, float64(1), params.Temperature.Value, "Expected temperature=1 when thinking")
			} else {
				// Check if thinking is not set (both fields are nil)
				hasThinking := params.Thinking.OfEnabled != nil || params.Thinking.OfDisabled != nil
				require.False(t, hasThinking, "Expected thinking to be disabled")
				// Temperature is an Opt[float64] type
				require.Equal(t, float64(0), params.Temperature.Value, "Expected temperature=0 when not thinking")
			}
		})
	}
}
