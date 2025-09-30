package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

func TestMain(m *testing.M) {
	_, err := config.Init(".", "", true)
	if err != nil {
		panic("Failed to initialize config: " + err.Error())
	}

	os.Exit(m.Run())
}

func TestOpenAIClientStreamChoices(t *testing.T) {
	// Create a mock server that returns Server-Sent Events with empty choices
	// This simulates the 🤡 behavior when a server returns 200 instead of 404
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(http.StatusOK)

		emptyChoicesChunk := map[string]any{
			"id":      "chat-completion-test",
			"object":  "chat.completion.chunk",
			"created": time.Now().Unix(),
			"model":   "test-model",
			"choices": []any{}, // Empty choices array that causes panic
		}

		jsonData, _ := json.Marshal(emptyChoicesChunk)
		w.Write([]byte("data: " + string(jsonData) + "\n\n"))
		w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	// Create OpenAI client pointing to our mock server
	client := &openaiClient{
		providerOptions: providerClientOptions{
			modelType:     config.SelectedModelTypeLarge,
			apiKey:        "test-key",
			systemMessage: "test",
			model: func(config.SelectedModelType) catwalk.Model {
				return catwalk.Model{
					ID:   "test-model",
					Name: "test-model",
				}
			},
		},
		client: openai.NewClient(
			option.WithAPIKey("test-key"),
			option.WithBaseURL(server.URL),
		),
	}

	// Create test messages
	messages := []message.Message{
		{
			Role:  message.User,
			Parts: []message.ContentPart{message.TextContent{Text: "Hello"}},
		},
	}

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	eventsChan := client.stream(ctx, messages, nil)

	// Collect events - this will panic without the bounds check
	for event := range eventsChan {
		t.Logf("Received event: %+v", event)
		if event.Type == EventError || event.Type == EventComplete {
			break
		}
	}
}

func TestOpenAIClient429InsufficientQuotaError(t *testing.T) {
	client := &openaiClient{
		providerOptions: providerClientOptions{
			modelType:     config.SelectedModelTypeLarge,
			apiKey:        "test-key",
			systemMessage: "test",
			config: config.ProviderConfig{
				ID:     "test-openai",
				APIKey: "test-key",
			},
			model: func(config.SelectedModelType) catwalk.Model {
				return catwalk.Model{
					ID:   "test-model",
					Name: "test-model",
				}
			},
		},
	}

	// Test insufficient_quota error should not retry
	apiErr := &openai.Error{
		StatusCode: 429,
		Message:    "You exceeded your current quota, please check your plan and billing details. For more information on this error, read the docs: https://platform.openai.com/docs/guides/error-codes/api-errors.",
		Type:       "insufficient_quota",
		Code:       "insufficient_quota",
	}

	retry, _, err := client.shouldRetry(1, apiErr)
	if retry {
		t.Error("Expected shouldRetry to return false for insufficient_quota error, but got true")
	}
	if err == nil {
		t.Error("Expected shouldRetry to return an error for insufficient_quota, but got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "quota") {
		t.Errorf("Expected error message to mention quota, got: %v", err)
	}
}

func TestOpenAIClient429RateLimitError(t *testing.T) {
	client := &openaiClient{
		providerOptions: providerClientOptions{
			modelType:     config.SelectedModelTypeLarge,
			apiKey:        "test-key",
			systemMessage: "test",
			config: config.ProviderConfig{
				ID:     "test-openai",
				APIKey: "test-key",
			},
			model: func(config.SelectedModelType) catwalk.Model {
				return catwalk.Model{
					ID:   "test-model",
					Name: "test-model",
				}
			},
		},
	}

	// Test regular rate limit error should retry
	apiErr := &openai.Error{
		StatusCode: 429,
		Message:    "Rate limit reached for requests",
		Type:       "rate_limit_exceeded",
		Code:       "rate_limit_exceeded",
	}

	retry, _, err := client.shouldRetry(1, apiErr)
	if !retry {
		t.Error("Expected shouldRetry to return true for rate_limit_exceeded error, but got false")
	}
	if err != nil {
		t.Errorf("Expected shouldRetry to return nil error for rate_limit_exceeded, but got: %v", err)
	}
}
