package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
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

func TestIsAzureOpenAI(t *testing.T) {
	tests := []struct {
		name     string
		baseURL  string
		expected bool
	}{
		{
			name:     "Azure OpenAI URL",
			baseURL:  "https://test.openai.azure.com/openai/deployments/gpt-4/chat/completions?api-version=2024-02-15-preview",
			expected: true,
		},
		{
			name:     "Azure OpenAI URL subdomain",
			baseURL:  "https://myresource.openai.azure.com",
			expected: true,
		},
		{
			name:     "Standard OpenAI URL",
			baseURL:  "https://api.openai.com/v1",
			expected: false,
		},
		{
			name:     "Empty URL",
			baseURL:  "",
			expected: false,
		},
		{
			name:     "Non-OpenAI URL",
			baseURL:  "https://api.anthropic.com/v1",
			expected: false,
		},
		{
			name:     "Case insensitive Azure URL",
			baseURL:  "https://TEST.OPENAI.AZURE.COM/api",
			expected: true,
		},
		{
			name:     "Azure Cognitive Services URL",
			baseURL:  "https://my-custom-name.cognitiveservices.azure.com/",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &openaiClient{
				providerOptions: providerClientOptions{
					baseURL: tt.baseURL,
				},
			}
			result := client.isAzureOpenAI()
			if result != tt.expected {
				t.Errorf("isAzureOpenAI() = %v, expected %v for URL: %s", result, tt.expected, tt.baseURL)
			}
		})
	}
}

func TestPreparedParamsProviderAware(t *testing.T) {
	tests := []struct {
		name                      string
		baseURL                   string
		modelCanReason            bool
		expectMaxTokens           bool
		expectMaxCompletionTokens bool
	}{
		{
			name:                      "Azure OpenAI with non-reasoning model",
			baseURL:                   "https://test.openai.azure.com",
			modelCanReason:            false,
			expectMaxTokens:           false,
			expectMaxCompletionTokens: true,
		},
		{
			name:                      "Azure OpenAI with reasoning model",
			baseURL:                   "https://test.openai.azure.com",
			modelCanReason:            true,
			expectMaxTokens:           false,
			expectMaxCompletionTokens: true,
		},
		{
			name:                      "Azure Cognitive Services with non-reasoning model",
			baseURL:                   "https://my-custom-name.cognitiveservices.azure.com/",
			modelCanReason:            false,
			expectMaxTokens:           false,
			expectMaxCompletionTokens: true,
		},
		{
			name:                      "Standard OpenAI with non-reasoning model",
			baseURL:                   "https://api.openai.com/v1",
			modelCanReason:            false,
			expectMaxTokens:           true,
			expectMaxCompletionTokens: false,
		},
		{
			name:                      "Standard OpenAI with reasoning model",
			baseURL:                   "https://api.openai.com/v1",
			modelCanReason:            true,
			expectMaxTokens:           false,
			expectMaxCompletionTokens: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &openaiClient{
				providerOptions: providerClientOptions{
					baseURL:   tt.baseURL,
					modelType: config.SelectedModelTypeLarge,
					model: func(config.SelectedModelType) catwalk.Model {
						return catwalk.Model{
							ID:               "test-model",
							Name:             "test-model",
							DefaultMaxTokens: 1000,
							CanReason:        tt.modelCanReason,
						}
					},
				},
			}

			messages := []openai.ChatCompletionMessageParamUnion{
				openai.UserMessage("test message"),
			}
			tools := []openai.ChatCompletionToolParam{}

			params := client.preparedParams(messages, tools)

			// Check MaxTokens
			if tt.expectMaxTokens {
				if !params.MaxTokens.Valid() {
					t.Error("Expected MaxTokens to be set, but it was not")
				} else if params.MaxTokens.Value <= 0 {
					t.Errorf("Expected MaxTokens to be positive, got %d", params.MaxTokens.Value)
				}
			} else {
				if params.MaxTokens.Valid() {
					t.Errorf("Expected MaxTokens to be unset, but it was set to %d", params.MaxTokens.Value)
				}
			}

			// Check MaxCompletionTokens
			if tt.expectMaxCompletionTokens {
				if !params.MaxCompletionTokens.Valid() {
					t.Error("Expected MaxCompletionTokens to be set, but it was not")
				} else if params.MaxCompletionTokens.Value <= 0 {
					t.Errorf("Expected MaxCompletionTokens to be positive, got %d", params.MaxCompletionTokens.Value)
				}
			} else {
				if params.MaxCompletionTokens.Valid() {
					t.Errorf("Expected MaxCompletionTokens to be unset, but it was set to %d", params.MaxCompletionTokens.Value)
				}
			}
		})
	}
}
