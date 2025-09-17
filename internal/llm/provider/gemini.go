package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/charmbracelet/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/llm/tools"
	"github.com/charmbracelet/crush/internal/log"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/google/uuid"
	"google.golang.org/genai"
)

type geminiClient struct {
	providerOptions providerClientOptions
	client          *genai.Client
}

type GeminiClient ProviderClient

func newGeminiClient(opts providerClientOptions) GeminiClient {
	client, err := createGeminiClient(opts)
	if err != nil {
		slog.Error("Failed to create Gemini client", "error", err)
		return nil
	}

	return &geminiClient{
		providerOptions: opts,
		client:          client,
	}
}

func createGeminiClient(opts providerClientOptions) (*genai.Client, error) {
	cc := &genai.ClientConfig{
		APIKey:  opts.apiKey,
		Backend: genai.BackendGeminiAPI,
	}
	if config.Get().Options.Debug {
		cc.HTTPClient = log.NewHTTPClient()
	}
	client, err := genai.NewClient(context.Background(), cc)
	if err != nil {
		return nil, err
	}
	return client, nil
}

func (g *geminiClient) convertMessages(messages []message.Message) []*genai.Content {
	var history []*genai.Content
	for _, msg := range messages {
		switch msg.Role {
		case message.User:
			var parts []*genai.Part
			parts = append(parts, &genai.Part{Text: msg.Content().String()})
			for _, binaryContent := range msg.BinaryContent() {
				imageFormat := strings.Split(binaryContent.MIMEType, "/")
				parts = append(parts, &genai.Part{InlineData: &genai.Blob{
					MIMEType: imageFormat[1],
					Data:     binaryContent.Data,
				}})
			}
			history = append(history, &genai.Content{
				Parts: parts,
				Role:  genai.RoleUser,
			})
		case message.Assistant:
			var assistantParts []*genai.Part

			if msg.Content().String() != "" {
				assistantParts = append(assistantParts, &genai.Part{Text: msg.Content().String()})
			}

			if len(msg.ToolCalls()) > 0 {
				for _, call := range msg.ToolCalls() {
					if !call.Finished {
						continue
					}
					args, _ := parseJSONToMap(call.Input)
					assistantParts = append(assistantParts, &genai.Part{
						FunctionCall: &genai.FunctionCall{
							Name: call.Name,
							Args: args,
						},
					})
				}
			}

			if len(assistantParts) > 0 {
				history = append(history, &genai.Content{
					Role:  genai.RoleModel,
					Parts: assistantParts,
				})
			}

		case message.Tool:
			var toolParts []*genai.Part
			for _, result := range msg.ToolResults() {
				response := map[string]any{"result": result.Content}
				parsed, err := parseJSONToMap(result.Content)
				if err == nil {
					response = parsed
				}

				var toolCall message.ToolCall
				for _, m := range messages {
					if m.Role == message.Assistant {
						for _, call := range m.ToolCalls() {
							if call.ID == result.ToolCallID {
								toolCall = call
								break
							}
						}
					}
				}

				toolParts = append(toolParts, &genai.Part{
					FunctionResponse: &genai.FunctionResponse{
						Name:     toolCall.Name,
						Response: response,
					},
				})
			}
			if len(toolParts) > 0 {
				history = append(history, &genai.Content{
					Parts: toolParts,
					Role:  genai.RoleUser,
				})
			}
		}
	}

	return history
}

func (g *geminiClient) convertTools(tools []tools.BaseTool) []*genai.Tool {
	geminiTool := &genai.Tool{}
	geminiTool.FunctionDeclarations = make([]*genai.FunctionDeclaration, 0, len(tools))

	for _, tool := range tools {
		info := tool.Info()
		declaration := &genai.FunctionDeclaration{
			Name:        info.Name,
			Description: info.Description,
			Parameters: &genai.Schema{
				Type:       genai.TypeObject,
				Properties: convertSchemaProperties(info.Parameters),
				Required:   info.Required,
			},
		}

		geminiTool.FunctionDeclarations = append(geminiTool.FunctionDeclarations, declaration)
	}

	return []*genai.Tool{geminiTool}
}

func (g *geminiClient) finishReason(reason genai.FinishReason) message.FinishReason {
	switch reason {
	case genai.FinishReasonStop:
		return message.FinishReasonEndTurn
	case genai.FinishReasonMaxTokens:
		return message.FinishReasonMaxTokens
	default:
		return message.FinishReasonUnknown
	}
}

func (g *geminiClient) send(ctx context.Context, messages []message.Message, tools []tools.BaseTool) (*ProviderResponse, error) {
	// Convert messages
	geminiMessages := g.convertMessages(messages)
	model := g.providerOptions.model(g.providerOptions.modelType)
	cfg := config.Get()

	modelConfig := cfg.Models[config.SelectedModelTypeLarge]
	if g.providerOptions.modelType == config.SelectedModelTypeSmall {
		modelConfig = cfg.Models[config.SelectedModelTypeSmall]
	}

	maxTokens := model.DefaultMaxTokens
	if modelConfig.MaxTokens > 0 {
		maxTokens = modelConfig.MaxTokens
	}
	systemMessage := g.providerOptions.systemMessage
	if g.providerOptions.systemPromptPrefix != "" {
		systemMessage = g.providerOptions.systemPromptPrefix + "\n" + systemMessage
	}
	history := geminiMessages[:len(geminiMessages)-1] // All but last message
	lastMsg := geminiMessages[len(geminiMessages)-1]
	config := &genai.GenerateContentConfig{
		MaxOutputTokens: int32(maxTokens),
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{{Text: systemMessage}},
		},
	}
	config.Tools = g.convertTools(tools)
	chat, _ := g.client.Chats.Create(ctx, model.ID, config, history)

	attempts := 0
	for {
		attempts++
		var toolCalls []message.ToolCall

		var lastMsgParts []genai.Part
		for _, part := range lastMsg.Parts {
			lastMsgParts = append(lastMsgParts, *part)
		}
		resp, err := chat.SendMessage(ctx, lastMsgParts...)
		// If there is an error we are going to see if we can retry the call
		if err != nil {
			retry, after, retryErr := g.shouldRetry(attempts, err)
			if retryErr != nil {
				return nil, retryErr
			}
			if retry {
				slog.Warn("Retrying due to rate limit", "attempt", attempts, "max_retries", maxRetries, "error", err)
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(time.Duration(after) * time.Millisecond):
					continue
				}
			}
			return nil, retryErr
		}

		content := ""

		if len(resp.Candidates) > 0 && resp.Candidates[0].Content != nil {
			for _, part := range resp.Candidates[0].Content.Parts {
				switch {
				case part.Text != "":
					content = string(part.Text)
				case part.FunctionCall != nil:
					id := "call_" + uuid.New().String()
					args, _ := json.Marshal(part.FunctionCall.Args)
					toolCalls = append(toolCalls, message.ToolCall{
						ID:       id,
						Name:     part.FunctionCall.Name,
						Input:    string(args),
						Type:     "function",
						Finished: true,
					})
				}
			}
		}
		finishReason := message.FinishReasonEndTurn
		if len(resp.Candidates) > 0 {
			finishReason = g.finishReason(resp.Candidates[0].FinishReason)
		}
		if len(toolCalls) > 0 {
			finishReason = message.FinishReasonToolUse
		}

		return &ProviderResponse{
			Content:      content,
			ToolCalls:    toolCalls,
			Usage:        g.usage(resp),
			FinishReason: finishReason,
		}, nil
	}
}

func (g *geminiClient) stream(ctx context.Context, messages []message.Message, tools []tools.BaseTool) <-chan ProviderEvent {
	// Convert messages
	geminiMessages := g.convertMessages(messages)

	model := g.providerOptions.model(g.providerOptions.modelType)
	cfg := config.Get()

	modelConfig := cfg.Models[config.SelectedModelTypeLarge]
	if g.providerOptions.modelType == config.SelectedModelTypeSmall {
		modelConfig = cfg.Models[config.SelectedModelTypeSmall]
	}
	maxTokens := model.DefaultMaxTokens
	if modelConfig.MaxTokens > 0 {
		maxTokens = modelConfig.MaxTokens
	}

	// Override max tokens if set in provider options
	if g.providerOptions.maxTokens > 0 {
		maxTokens = g.providerOptions.maxTokens
	}
	systemMessage := g.providerOptions.systemMessage
	if g.providerOptions.systemPromptPrefix != "" {
		systemMessage = g.providerOptions.systemPromptPrefix + "\n" + systemMessage
	}
	history := geminiMessages[:len(geminiMessages)-1] // All but last message
	lastMsg := geminiMessages[len(geminiMessages)-1]
	config := &genai.GenerateContentConfig{
		MaxOutputTokens: int32(maxTokens),
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{{Text: systemMessage}},
		},
	}
	config.Tools = g.convertTools(tools)
	chat, _ := g.client.Chats.Create(ctx, model.ID, config, history)

	attempts := 0
	eventChan := make(chan ProviderEvent)

	go func() {
		defer close(eventChan)

		for {
			attempts++

			currentContent := ""
			toolCalls := []message.ToolCall{}
			var finalResp *genai.GenerateContentResponse

			eventChan <- ProviderEvent{Type: EventContentStart}

			var lastMsgParts []genai.Part

			for _, part := range lastMsg.Parts {
				lastMsgParts = append(lastMsgParts, *part)
			}

			for resp, err := range chat.SendMessageStream(ctx, lastMsgParts...) {
				if err != nil {
					retry, after, retryErr := g.shouldRetry(attempts, err)
					if retryErr != nil {
						eventChan <- ProviderEvent{Type: EventError, Error: retryErr}
						return
					}
					if retry {
						slog.Warn("Retrying due to rate limit", "attempt", attempts, "max_retries", maxRetries, "error", err)
						select {
						case <-ctx.Done():
							if ctx.Err() != nil {
								eventChan <- ProviderEvent{Type: EventError, Error: ctx.Err()}
							}

							return
						case <-time.After(time.Duration(after) * time.Millisecond):
							continue
						}
					} else {
						eventChan <- ProviderEvent{Type: EventError, Error: err}
						return
					}
				}

				finalResp = resp

				if len(resp.Candidates) > 0 && resp.Candidates[0].Content != nil {
					for _, part := range resp.Candidates[0].Content.Parts {
						switch {
						case part.Text != "":
							delta := string(part.Text)
							if delta != "" {
								eventChan <- ProviderEvent{
									Type:    EventContentDelta,
									Content: delta,
								}
								currentContent += delta
							}
						case part.FunctionCall != nil:
							id := "call_" + uuid.New().String()
							args, _ := json.Marshal(part.FunctionCall.Args)
							newCall := message.ToolCall{
								ID:       id,
								Name:     part.FunctionCall.Name,
								Input:    string(args),
								Type:     "function",
								Finished: true,
							}

							toolCalls = append(toolCalls, newCall)
						}
					}
				} else {
					// no content received
					break
				}
			}

			eventChan <- ProviderEvent{Type: EventContentStop}

			if finalResp != nil {
				finishReason := message.FinishReasonEndTurn
				if len(finalResp.Candidates) > 0 {
					finishReason = g.finishReason(finalResp.Candidates[0].FinishReason)
				}
				if len(toolCalls) > 0 {
					finishReason = message.FinishReasonToolUse
				}
				eventChan <- ProviderEvent{
					Type: EventComplete,
					Response: &ProviderResponse{
						Content:      currentContent,
						ToolCalls:    toolCalls,
						Usage:        g.usage(finalResp),
						FinishReason: finishReason,
					},
				}
				return
			} else {
				eventChan <- ProviderEvent{
					Type:  EventError,
					Error: errors.New("no content received"),
				}
			}
		}
	}()

	return eventChan
}

func (g *geminiClient) shouldRetry(attempts int, err error) (bool, int64, error) {
	// Check if error is a rate limit error
	if attempts > maxRetries {
		return false, 0, fmt.Errorf("maximum retry attempts reached for rate limit: %d retries", maxRetries)
	}

	// Gemini doesn't have a standard error type we can check against
	// So we'll check the error message for rate limit indicators
	if errors.Is(err, io.EOF) {
		return false, 0, err
	}

	errMsg := err.Error()
	isRateLimit := contains(errMsg, "rate limit", "quota exceeded", "too many requests")

	// Check for token expiration (401 Unauthorized)
	if contains(errMsg, "unauthorized", "invalid api key", "api key expired") {
		g.providerOptions.apiKey, err = config.Get().Resolve(g.providerOptions.config.APIKey)
		if err != nil {
			return false, 0, fmt.Errorf("failed to resolve API key: %w", err)
		}
		g.client, err = createGeminiClient(g.providerOptions)
		if err != nil {
			return false, 0, fmt.Errorf("failed to create Gemini client after API key refresh: %w", err)
		}
		return true, 0, nil
	}

	// Check for common rate limit error messages

	if !isRateLimit {
		return false, 0, err
	}

	// Calculate backoff with jitter
	backoffMs := 2000 * (1 << (attempts - 1))
	jitterMs := int(float64(backoffMs) * 0.2)
	retryMs := backoffMs + jitterMs

	return true, int64(retryMs), nil
}

func (g *geminiClient) usage(resp *genai.GenerateContentResponse) TokenUsage {
	if resp == nil || resp.UsageMetadata == nil {
		return TokenUsage{}
	}

	return TokenUsage{
		InputTokens:         int64(resp.UsageMetadata.PromptTokenCount),
		OutputTokens:        int64(resp.UsageMetadata.CandidatesTokenCount),
		CacheCreationTokens: 0, // Not directly provided by Gemini
		CacheReadTokens:     int64(resp.UsageMetadata.CachedContentTokenCount),
	}
}

func (g *geminiClient) Model() catwalk.Model {
	return g.providerOptions.model(g.providerOptions.modelType)
}

// Helper functions
func parseJSONToMap(jsonStr string) (map[string]any, error) {
	var result map[string]any
	err := json.Unmarshal([]byte(jsonStr), &result)
	return result, err
}

func convertSchemaProperties(parameters map[string]any) map[string]*genai.Schema {
	properties := make(map[string]*genai.Schema)

	for name, param := range parameters {
		properties[name] = convertToSchema(param)
	}

	return properties
}

func convertToSchema(param any) *genai.Schema {
	schema := &genai.Schema{Type: genai.TypeString}

	paramMap, ok := param.(map[string]any)
	if !ok {
		return schema
	}

	if desc, ok := paramMap["description"].(string); ok {
		schema.Description = desc
	}

	typeVal, hasType := paramMap["type"]
	if !hasType {
		return schema
	}

	typeStr, ok := typeVal.(string)
	if !ok {
		return schema
	}

	schema.Type = mapJSONTypeToGenAI(typeStr)

	switch typeStr {
	case "array":
		schema.Items = processArrayItems(paramMap)
	case "object":
		if props, ok := paramMap["properties"].(map[string]any); ok {
			schema.Properties = convertSchemaProperties(props)
		}
	}

	return schema
}

func processArrayItems(paramMap map[string]any) *genai.Schema {
	items, ok := paramMap["items"].(map[string]any)
	if !ok {
		return nil
	}

	return convertToSchema(items)
}

func mapJSONTypeToGenAI(jsonType string) genai.Type {
	switch jsonType {
	case "string":
		return genai.TypeString
	case "number":
		return genai.TypeNumber
	case "integer":
		return genai.TypeInteger
	case "boolean":
		return genai.TypeBoolean
	case "array":
		return genai.TypeArray
	case "object":
		return genai.TypeObject
	default:
		return genai.TypeString // Default to string for unknown types
	}
}

func contains(s string, substrs ...string) bool {
	for _, substr := range substrs {
		if strings.Contains(strings.ToLower(s), strings.ToLower(substr)) {
			return true
		}
	}
	return false
}
