package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/bedrock"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/charmbracelet/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/llm/tools"
	"github.com/charmbracelet/crush/internal/message"
)

type anthropicProvider struct {
	*baseProvider
	useBedrock        bool
	client            anthropic.Client
	adjustedMaxTokens int // Used when context limit is hit
}

func NewAnthropicProvider(base *baseProvider, useBedrock bool) Provider {
	return &anthropicProvider{
		baseProvider: base,
		client:       createAnthropicClient(base, useBedrock),
	}
}

func createAnthropicClient(opts *baseProvider, useBedrock bool) anthropic.Client {
	anthropicClientOptions := []option.RequestOption{}

	// Check if Authorization header is provided in extra headers
	hasBearerAuth := false
	if opts.extraHeaders != nil {
		for key := range opts.extraHeaders {
			if strings.ToLower(key) == "authorization" {
				hasBearerAuth = true
				break
			}
		}
	}

	isBearerToken := strings.HasPrefix(opts.apiKey, "Bearer ")

	if opts.apiKey != "" && !hasBearerAuth {
		if isBearerToken {
			slog.Debug("API key starts with 'Bearer ', using as Authorization header")
			anthropicClientOptions = append(anthropicClientOptions, option.WithHeader("Authorization", opts.apiKey))
		} else {
			// Use standard X-Api-Key header
			anthropicClientOptions = append(anthropicClientOptions, option.WithAPIKey(opts.apiKey))
		}
	} else if hasBearerAuth {
		slog.Debug("Skipping X-Api-Key header because Authorization header is provided")
	}
	if useBedrock {
		anthropicClientOptions = append(anthropicClientOptions, bedrock.WithLoadDefaultConfig(context.Background()))
	}
	for _, header := range opts.extraHeaders {
		anthropicClientOptions = append(anthropicClientOptions, option.WithHeaderAdd(header, opts.extraHeaders[header]))
	}
	for key, value := range opts.extraBody {
		anthropicClientOptions = append(anthropicClientOptions, option.WithJSONSet(key, value))
	}
	return anthropic.NewClient(anthropicClientOptions...)
}

func (a *anthropicProvider) convertMessages(messages []message.Message) (anthropicMessages []anthropic.MessageParam) {
	for i, msg := range messages {
		cache := false
		if i > len(messages)-3 {
			cache = true
		}
		switch msg.Role {
		case message.User:
			content := anthropic.NewTextBlock(msg.Content().String())
			if cache && !a.disableCache {
				content.OfText.CacheControl = anthropic.CacheControlEphemeralParam{
					Type: "ephemeral",
				}
			}
			var contentBlocks []anthropic.ContentBlockParamUnion
			contentBlocks = append(contentBlocks, content)
			for _, binaryContent := range msg.BinaryContent() {
				base64Image := binaryContent.String(catwalk.InferenceProviderAnthropic)
				imageBlock := anthropic.NewImageBlockBase64(binaryContent.MIMEType, base64Image)
				contentBlocks = append(contentBlocks, imageBlock)
			}
			anthropicMessages = append(anthropicMessages, anthropic.NewUserMessage(contentBlocks...))

		case message.Assistant:
			blocks := []anthropic.ContentBlockParamUnion{}

			// Add thinking blocks first if present (required when thinking is enabled with tool use)
			if reasoningContent := msg.ReasoningContent(); reasoningContent.Thinking != "" {
				thinkingBlock := anthropic.NewThinkingBlock(reasoningContent.Signature, reasoningContent.Thinking)
				blocks = append(blocks, thinkingBlock)
			}

			if msg.Content().String() != "" {
				content := anthropic.NewTextBlock(msg.Content().String())
				if cache && !a.disableCache {
					content.OfText.CacheControl = anthropic.CacheControlEphemeralParam{
						Type: "ephemeral",
					}
				}
				blocks = append(blocks, content)
			}

			for _, toolCall := range msg.ToolCalls() {
				var inputMap map[string]any
				err := json.Unmarshal([]byte(toolCall.Input), &inputMap)
				if err != nil {
					continue
				}
				blocks = append(blocks, anthropic.NewToolUseBlock(toolCall.ID, inputMap, toolCall.Name))
			}

			if len(blocks) == 0 {
				slog.Warn("There is a message without content, investigate, this should not happen")
				continue
			}
			anthropicMessages = append(anthropicMessages, anthropic.NewAssistantMessage(blocks...))

		case message.Tool:
			results := make([]anthropic.ContentBlockParamUnion, len(msg.ToolResults()))
			for i, toolResult := range msg.ToolResults() {
				results[i] = anthropic.NewToolResultBlock(toolResult.ToolCallID, toolResult.Content, toolResult.IsError)
			}
			anthropicMessages = append(anthropicMessages, anthropic.NewUserMessage(results...))
		}
	}
	return
}

func (a *anthropicProvider) convertTools(tools []tools.BaseTool) []anthropic.ToolUnionParam {
	anthropicTools := make([]anthropic.ToolUnionParam, len(tools))

	for i, tool := range tools {
		info := tool.Info()
		toolParam := anthropic.ToolParam{
			Name:        info.Name,
			Description: anthropic.String(info.Description),
			InputSchema: anthropic.ToolInputSchemaParam{
				Properties: info.Parameters,
				Required:   info.Required,
			},
		}

		if i == len(tools)-1 && !a.disableCache {
			toolParam.CacheControl = anthropic.CacheControlEphemeralParam{
				Type: "ephemeral",
			}
		}

		anthropicTools[i] = anthropic.ToolUnionParam{OfTool: &toolParam}
	}

	return anthropicTools
}

func (a *anthropicProvider) finishReason(reason string) message.FinishReason {
	switch reason {
	case "end_turn":
		return message.FinishReasonEndTurn
	case "max_tokens":
		return message.FinishReasonMaxTokens
	case "tool_use":
		return message.FinishReasonToolUse
	case "stop_sequence":
		return message.FinishReasonEndTurn
	default:
		return message.FinishReasonUnknown
	}
}

func (a *anthropicProvider) isThinkingEnabled(model string) bool {
	return a.Model(model).CanReason && a.think
}

func (a *anthropicProvider) preparedMessages(modelID string, messages []anthropic.MessageParam, tools []anthropic.ToolUnionParam) anthropic.MessageNewParams {
	model := a.Model(modelID)
	var thinkingParam anthropic.ThinkingConfigParamUnion
	temperature := anthropic.Float(0)

	maxTokens := model.DefaultMaxTokens
	if a.maxTokens > 0 {
		maxTokens = a.maxTokens
	}
	if a.isThinkingEnabled(modelID) {
		thinkingParam = anthropic.ThinkingConfigParamOfEnabled(int64(float64(maxTokens) * 0.8))
		temperature = anthropic.Float(1)
	}

	// Use adjusted max tokens if context limit was hit
	if a.adjustedMaxTokens > 0 {
		maxTokens = int64(a.adjustedMaxTokens)
	}

	systemBlocks := []anthropic.TextBlockParam{}

	// Add custom system prompt prefix if configured
	if a.systemPromptPrefix != "" {
		systemBlocks = append(systemBlocks, anthropic.TextBlockParam{
			Text: a.systemPromptPrefix,
			CacheControl: anthropic.CacheControlEphemeralParam{
				Type: "ephemeral",
			},
		})
	}

	systemBlocks = append(systemBlocks, anthropic.TextBlockParam{
		Text: a.systemMessage,
		CacheControl: anthropic.CacheControlEphemeralParam{
			Type: "ephemeral",
		},
	})

	return anthropic.MessageNewParams{
		Model:       anthropic.Model(model.ID),
		MaxTokens:   maxTokens,
		Temperature: temperature,
		Messages:    messages,
		Tools:       tools,
		Thinking:    thinkingParam,
		System:      systemBlocks,
	}
}

func (a *anthropicProvider) Send(ctx context.Context, model string, messages []message.Message, tools []tools.BaseTool) (*ProviderResponse, error) {
	messages = a.cleanMessages(messages)
	return a.send(ctx, model, messages, tools)
}

func (a *anthropicProvider) send(ctx context.Context, model string, messages []message.Message, tools []tools.BaseTool) (response *ProviderResponse, err error) {
	attempts := 0
	for {
		attempts++
		// Prepare messages on each attempt in case max_tokens was adjusted
		preparedMessages := a.preparedMessages(model, a.convertMessages(messages), a.convertTools(tools))
		if a.debug {
			jsonData, _ := json.Marshal(preparedMessages)
			slog.Debug("Prepared messages", "messages", string(jsonData))
		}

		var opts []option.RequestOption
		if a.isThinkingEnabled(model) {
			opts = append(opts, option.WithHeaderAdd("anthropic-beta", "interleaved-thinking-2025-05-14"))
		}
		anthropicResponse, err := a.client.Messages.New(
			ctx,
			preparedMessages,
			opts...,
		)
		// If there is an error we are going to see if we can retry the call
		if err != nil {
			slog.Error("Error in Anthropic API call", "error", err)
			retry, after, retryErr := a.shouldRetry(attempts, err)
			if retryErr != nil {
				return nil, retryErr
			}
			if retry {
				slog.Warn("Retrying due to rate limit", "attempt", attempts, "max_retries", maxRetries)
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
		for _, block := range anthropicResponse.Content {
			if text, ok := block.AsAny().(anthropic.TextBlock); ok {
				content += text.Text
			}
		}

		return &ProviderResponse{
			Content:   content,
			ToolCalls: a.toolCalls(*anthropicResponse),
			Usage:     a.usage(*anthropicResponse),
		}, nil
	}
}

func (a *anthropicProvider) Stream(ctx context.Context, model string, messages []message.Message, tools []tools.BaseTool) <-chan ProviderEvent {
	messages = a.cleanMessages(messages)
	return a.stream(ctx, model, messages, tools)
}

func (a *anthropicProvider) stream(ctx context.Context, model string, messages []message.Message, tools []tools.BaseTool) <-chan ProviderEvent {
	attempts := 0
	eventChan := make(chan ProviderEvent)
	go func() {
		for {
			attempts++
			// Prepare messages on each attempt in case max_tokens was adjusted
			preparedMessages := a.preparedMessages(model, a.convertMessages(messages), a.convertTools(tools))
			if a.debug {
				jsonData, _ := json.Marshal(preparedMessages)
				slog.Debug("Prepared messages", "messages", string(jsonData))
			}

			var opts []option.RequestOption
			if a.isThinkingEnabled(model) {
				opts = append(opts, option.WithHeaderAdd("anthropic-beta", "interleaved-thinking-2025-05-14"))
			}

			anthropicStream := a.client.Messages.NewStreaming(
				ctx,
				preparedMessages,
				opts...,
			)
			accumulatedMessage := anthropic.Message{}

			currentToolCallID := ""
			for anthropicStream.Next() {
				event := anthropicStream.Current()
				err := accumulatedMessage.Accumulate(event)
				if err != nil {
					slog.Warn("Error accumulating message", "error", err)
					continue
				}

				switch event := event.AsAny().(type) {
				case anthropic.ContentBlockStartEvent:
					switch event.ContentBlock.Type {
					case "text":
						eventChan <- ProviderEvent{Type: EventContentStart}
					case "tool_use":
						currentToolCallID = event.ContentBlock.ID
						eventChan <- ProviderEvent{
							Type: EventToolUseStart,
							ToolCall: &message.ToolCall{
								ID:       event.ContentBlock.ID,
								Name:     event.ContentBlock.Name,
								Finished: false,
							},
						}
					}

				case anthropic.ContentBlockDeltaEvent:
					if event.Delta.Type == "thinking_delta" && event.Delta.Thinking != "" {
						eventChan <- ProviderEvent{
							Type:     EventThinkingDelta,
							Thinking: event.Delta.Thinking,
						}
					} else if event.Delta.Type == "signature_delta" && event.Delta.Signature != "" {
						eventChan <- ProviderEvent{
							Type:      EventSignatureDelta,
							Signature: event.Delta.Signature,
						}
					} else if event.Delta.Type == "text_delta" && event.Delta.Text != "" {
						eventChan <- ProviderEvent{
							Type:    EventContentDelta,
							Content: event.Delta.Text,
						}
					} else if event.Delta.Type == "input_json_delta" {
						if currentToolCallID != "" {
							eventChan <- ProviderEvent{
								Type: EventToolUseDelta,
								ToolCall: &message.ToolCall{
									ID:       currentToolCallID,
									Finished: false,
									Input:    event.Delta.PartialJSON,
								},
							}
						}
					}
				case anthropic.ContentBlockStopEvent:
					if currentToolCallID != "" {
						eventChan <- ProviderEvent{
							Type: EventToolUseStop,
							ToolCall: &message.ToolCall{
								ID: currentToolCallID,
							},
						}
						currentToolCallID = ""
					} else {
						eventChan <- ProviderEvent{Type: EventContentStop}
					}

				case anthropic.MessageStopEvent:
					content := ""
					for _, block := range accumulatedMessage.Content {
						if text, ok := block.AsAny().(anthropic.TextBlock); ok {
							content += text.Text
						}
					}

					eventChan <- ProviderEvent{
						Type: EventComplete,
						Response: &ProviderResponse{
							Content:      content,
							ToolCalls:    a.toolCalls(accumulatedMessage),
							Usage:        a.usage(accumulatedMessage),
							FinishReason: a.finishReason(string(accumulatedMessage.StopReason)),
						},
						Content: content,
					}
				}
			}

			err := anthropicStream.Err()
			if err == nil || errors.Is(err, io.EOF) {
				close(eventChan)
				return
			}

			// If there is an error we are going to see if we can retry the call
			retry, after, retryErr := a.shouldRetry(attempts, err)
			if retryErr != nil {
				eventChan <- ProviderEvent{Type: EventError, Error: retryErr}
				close(eventChan)
				return
			}
			if retry {
				slog.Warn("Retrying due to rate limit", "attempt", attempts, "max_retries", maxRetries)
				select {
				case <-ctx.Done():
					// context cancelled
					if ctx.Err() != nil {
						eventChan <- ProviderEvent{Type: EventError, Error: ctx.Err()}
					}
					close(eventChan)
					return
				case <-time.After(time.Duration(after) * time.Millisecond):
					continue
				}
			}
			if ctx.Err() != nil {
				eventChan <- ProviderEvent{Type: EventError, Error: ctx.Err()}
			}

			close(eventChan)
			return
		}
	}()
	return eventChan
}

func (a *anthropicProvider) shouldRetry(attempts int, err error) (bool, int64, error) {
	var apiErr *anthropic.Error
	if !errors.As(err, &apiErr) {
		return false, 0, err
	}

	if attempts > maxRetries {
		return false, 0, fmt.Errorf("maximum retry attempts reached for rate limit: %d retries", maxRetries)
	}

	if apiErr.StatusCode == 401 {
		a.apiKey, err = a.resolver.ResolveValue(a.config.APIKey)
		if err != nil {
			return false, 0, fmt.Errorf("failed to resolve API key: %w", err)
		}

		a.client = createAnthropicClient(a.baseProvider, a.useBedrock)
		return true, 0, nil
	}

	// Handle context limit exceeded error (400 Bad Request)
	if apiErr.StatusCode == 400 {
		if adjusted, ok := a.handleContextLimitError(apiErr); ok {
			a.adjustedMaxTokens = adjusted
			slog.Debug("Adjusted max_tokens due to context limit", "new_max_tokens", adjusted)
			return true, 0, nil
		}
	}

	isOverloaded := strings.Contains(apiErr.Error(), "overloaded") || strings.Contains(apiErr.Error(), "rate limit exceeded")
	if apiErr.StatusCode != 429 && apiErr.StatusCode != 529 && !isOverloaded {
		return false, 0, err
	}

	retryMs := 0
	retryAfterValues := apiErr.Response.Header.Values("Retry-After")

	backoffMs := 2000 * (1 << (attempts - 1))
	jitterMs := int(float64(backoffMs) * 0.2)
	retryMs = backoffMs + jitterMs
	if len(retryAfterValues) > 0 {
		if _, err := fmt.Sscanf(retryAfterValues[0], "%d", &retryMs); err == nil {
			retryMs = retryMs * 1000
		}
	}
	return true, int64(retryMs), nil
}

// handleContextLimitError parses context limit error and returns adjusted max_tokens
func (a *anthropicProvider) handleContextLimitError(apiErr *anthropic.Error) (int, bool) {
	// Parse error message like: "input length and max_tokens exceed context limit: 154978 + 50000 > 200000"
	errorMsg := apiErr.Error()

	re := regexp.MustCompile("input length and `max_tokens` exceed context limit: (\\d+) \\+ (\\d+) > (\\d+)")
	matches := re.FindStringSubmatch(errorMsg)

	if len(matches) != 4 {
		return 0, false
	}

	inputTokens, err1 := strconv.Atoi(matches[1])
	contextLimit, err2 := strconv.Atoi(matches[3])

	if err1 != nil || err2 != nil {
		return 0, false
	}

	// Calculate safe max_tokens with a buffer of 1000 tokens
	safeMaxTokens := contextLimit - inputTokens - 1000

	// Ensure we don't go below a minimum threshold
	safeMaxTokens = max(safeMaxTokens, 1000)

	return safeMaxTokens, true
}

func (a *anthropicProvider) toolCalls(msg anthropic.Message) []message.ToolCall {
	var toolCalls []message.ToolCall

	for _, block := range msg.Content {
		switch variant := block.AsAny().(type) {
		case anthropic.ToolUseBlock:
			toolCall := message.ToolCall{
				ID:       variant.ID,
				Name:     variant.Name,
				Input:    string(variant.Input),
				Type:     string(variant.Type),
				Finished: true,
			}
			toolCalls = append(toolCalls, toolCall)
		}
	}

	return toolCalls
}

func (a *anthropicProvider) usage(msg anthropic.Message) TokenUsage {
	return TokenUsage{
		InputTokens:         msg.Usage.InputTokens,
		OutputTokens:        msg.Usage.OutputTokens,
		CacheCreationTokens: msg.Usage.CacheCreationInputTokens,
		CacheReadTokens:     msg.Usage.CacheReadInputTokens,
	}
}
