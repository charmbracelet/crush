package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/charmbracelet/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/llm/tools"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/shared"
)

type openaiProvider struct {
	*baseProvider
	client openai.Client
}

func NewOpenAIProvider(base *baseProvider) Provider {
	return &openaiProvider{
		baseProvider: base,
		client:       createOpenAIClient(base),
	}
}

func createOpenAIClient(opts *baseProvider) openai.Client {
	openaiClientOptions := []option.RequestOption{}
	if opts.apiKey != "" {
		openaiClientOptions = append(openaiClientOptions, option.WithAPIKey(opts.apiKey))
	}
	if opts.baseURL != "" {
		openaiClientOptions = append(openaiClientOptions, option.WithBaseURL(opts.baseURL))
	}

	for key, value := range opts.extraHeaders {
		openaiClientOptions = append(openaiClientOptions, option.WithHeader(key, value))
	}

	for extraKey, extraValue := range opts.extraBody {
		openaiClientOptions = append(openaiClientOptions, option.WithJSONSet(extraKey, extraValue))
	}

	return openai.NewClient(openaiClientOptions...)
}

func (o *openaiProvider) convertMessages(messages []message.Message) (openaiMessages []openai.ChatCompletionMessageParamUnion) {
	// Add system message first
	systemMessage := o.systemMessage
	if o.systemPromptPrefix != "" {
		systemMessage = o.systemPromptPrefix + "\n" + systemMessage
	}
	openaiMessages = append(openaiMessages, openai.SystemMessage(systemMessage))

	for _, msg := range messages {
		switch msg.Role {
		case message.User:
			var content []openai.ChatCompletionContentPartUnionParam
			textBlock := openai.ChatCompletionContentPartTextParam{Text: msg.Content().String()}
			content = append(content, openai.ChatCompletionContentPartUnionParam{OfText: &textBlock})
			for _, binaryContent := range msg.BinaryContent() {
				imageURL := openai.ChatCompletionContentPartImageImageURLParam{URL: binaryContent.String(catwalk.InferenceProviderOpenAI)}
				imageBlock := openai.ChatCompletionContentPartImageParam{ImageURL: imageURL}

				content = append(content, openai.ChatCompletionContentPartUnionParam{OfImageURL: &imageBlock})
			}

			openaiMessages = append(openaiMessages, openai.UserMessage(content))

		case message.Assistant:
			assistantMsg := openai.ChatCompletionAssistantMessageParam{
				Role: "assistant",
			}

			hasContent := false
			if msg.Content().String() != "" {
				hasContent = true
				assistantMsg.Content = openai.ChatCompletionAssistantMessageParamContentUnion{
					OfString: openai.String(msg.Content().String()),
				}
			}

			if len(msg.ToolCalls()) > 0 {
				hasContent = true
				assistantMsg.ToolCalls = make([]openai.ChatCompletionMessageToolCallParam, len(msg.ToolCalls()))
				for i, call := range msg.ToolCalls() {
					assistantMsg.ToolCalls[i] = openai.ChatCompletionMessageToolCallParam{
						ID:   call.ID,
						Type: "function",
						Function: openai.ChatCompletionMessageToolCallFunctionParam{
							Name:      call.Name,
							Arguments: call.Input,
						},
					}
				}
			}
			if !hasContent {
				slog.Warn("There is a message without content, investigate, this should not happen")
				continue
			}

			openaiMessages = append(openaiMessages, openai.ChatCompletionMessageParamUnion{
				OfAssistant: &assistantMsg,
			})

		case message.Tool:
			for _, result := range msg.ToolResults() {
				openaiMessages = append(openaiMessages,
					openai.ToolMessage(result.Content, result.ToolCallID),
				)
			}
		}
	}

	return
}

func (o *openaiProvider) convertTools(tools []tools.BaseTool) []openai.ChatCompletionToolParam {
	openaiTools := make([]openai.ChatCompletionToolParam, len(tools))

	for i, tool := range tools {
		info := tool.Info()
		openaiTools[i] = openai.ChatCompletionToolParam{
			Function: openai.FunctionDefinitionParam{
				Name:        info.Name,
				Description: openai.String(info.Description),
				Parameters: openai.FunctionParameters{
					"type":       "object",
					"properties": info.Parameters,
					"required":   info.Required,
				},
			},
		}
	}

	return openaiTools
}

func (o *openaiProvider) finishReason(reason string) message.FinishReason {
	switch reason {
	case "stop":
		return message.FinishReasonEndTurn
	case "length":
		return message.FinishReasonMaxTokens
	case "tool_calls":
		return message.FinishReasonToolUse
	default:
		return message.FinishReasonUnknown
	}
}

func (o *openaiProvider) preparedParams(modelID string, messages []openai.ChatCompletionMessageParamUnion, tools []openai.ChatCompletionToolParam) openai.ChatCompletionNewParams {
	model := o.Model(modelID)

	reasoningEffort := o.reasoningEffort
	if reasoningEffort == "" {
		reasoningEffort = model.DefaultReasoningEffort
	}

	params := openai.ChatCompletionNewParams{
		Model:    openai.ChatModel(model.ID),
		Messages: messages,
		Tools:    tools,
	}

	maxTokens := model.DefaultMaxTokens
	if o.maxTokens > 0 {
		maxTokens = o.maxTokens
	}

	if model.CanReason {
		params.MaxCompletionTokens = openai.Int(maxTokens)
		switch reasoningEffort {
		case "low":
			params.ReasoningEffort = shared.ReasoningEffortLow
		case "medium":
			params.ReasoningEffort = shared.ReasoningEffortMedium
		case "high":
			params.ReasoningEffort = shared.ReasoningEffortHigh
		default:
			params.ReasoningEffort = shared.ReasoningEffort(reasoningEffort)
		}
	} else {
		params.MaxTokens = openai.Int(maxTokens)
	}

	return params
}

func (o *openaiProvider) Send(ctx context.Context, model string, messages []message.Message, tools []tools.BaseTool) (*ProviderResponse, error) {
	messages = o.cleanMessages(messages)
	return o.send(ctx, model, messages, tools)
}

func (o *openaiProvider) send(ctx context.Context, model string, messages []message.Message, tools []tools.BaseTool) (response *ProviderResponse, err error) {
	params := o.preparedParams(model, o.convertMessages(messages), o.convertTools(tools))
	if o.debug {
		jsonData, _ := json.Marshal(params)
		slog.Debug("Prepared messages", "messages", string(jsonData))
	}
	attempts := 0
	for {
		attempts++
		openaiResponse, err := o.client.Chat.Completions.New(
			ctx,
			params,
		)
		// If there is an error we are going to see if we can retry the call
		if err != nil {
			retry, after, retryErr := o.shouldRetry(attempts, err)
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

		if len(openaiResponse.Choices) == 0 {
			return nil, fmt.Errorf("received empty response from OpenAI API - check endpoint configuration")
		}

		content := ""
		if openaiResponse.Choices[0].Message.Content != "" {
			content = openaiResponse.Choices[0].Message.Content
		}

		toolCalls := o.toolCalls(*openaiResponse)
		finishReason := o.finishReason(string(openaiResponse.Choices[0].FinishReason))

		if len(toolCalls) > 0 {
			finishReason = message.FinishReasonToolUse
		}

		return &ProviderResponse{
			Content:      content,
			ToolCalls:    toolCalls,
			Usage:        o.usage(*openaiResponse),
			FinishReason: finishReason,
		}, nil
	}
}

func (o *openaiProvider) Stream(ctx context.Context, model string, messages []message.Message, tools []tools.BaseTool) <-chan ProviderEvent {
	messages = o.cleanMessages(messages)
	return o.stream(ctx, model, messages, tools)
}

func (o *openaiProvider) stream(ctx context.Context, model string, messages []message.Message, tools []tools.BaseTool) <-chan ProviderEvent {
	params := o.preparedParams(model, o.convertMessages(messages), o.convertTools(tools))
	params.StreamOptions = openai.ChatCompletionStreamOptionsParam{
		IncludeUsage: openai.Bool(true),
	}

	if o.debug {
		jsonData, _ := json.Marshal(params)
		slog.Debug("Prepared messages", "messages", string(jsonData))
	}

	attempts := 0
	eventChan := make(chan ProviderEvent)

	go func() {
		for {
			attempts++
			openaiStream := o.client.Chat.Completions.NewStreaming(
				ctx,
				params,
			)

			acc := openai.ChatCompletionAccumulator{}
			currentContent := ""
			toolCalls := make([]message.ToolCall, 0)

			var currentToolCallID string
			var currentToolCall openai.ChatCompletionMessageToolCall
			var msgToolCalls []openai.ChatCompletionMessageToolCall
			for openaiStream.Next() {
				chunk := openaiStream.Current()
				acc.AddChunk(chunk)
				// This fixes multiple tool calls for some providers
				for _, choice := range chunk.Choices {
					if choice.Delta.Content != "" {
						eventChan <- ProviderEvent{
							Type:    EventContentDelta,
							Content: choice.Delta.Content,
						}
						currentContent += choice.Delta.Content
					} else if len(choice.Delta.ToolCalls) > 0 {
						toolCall := choice.Delta.ToolCalls[0]
						// Detect tool use start
						if currentToolCallID == "" {
							if toolCall.ID != "" {
								currentToolCallID = toolCall.ID
								currentToolCall = openai.ChatCompletionMessageToolCall{
									ID:   toolCall.ID,
									Type: "function",
									Function: openai.ChatCompletionMessageToolCallFunction{
										Name:      toolCall.Function.Name,
										Arguments: toolCall.Function.Arguments,
									},
								}
							}
						} else {
							// Delta tool use
							if toolCall.ID == "" {
								currentToolCall.Function.Arguments += toolCall.Function.Arguments
							} else {
								// Detect new tool use
								if toolCall.ID != currentToolCallID {
									msgToolCalls = append(msgToolCalls, currentToolCall)
									currentToolCallID = toolCall.ID
									currentToolCall = openai.ChatCompletionMessageToolCall{
										ID:   toolCall.ID,
										Type: "function",
										Function: openai.ChatCompletionMessageToolCallFunction{
											Name:      toolCall.Function.Name,
											Arguments: toolCall.Function.Arguments,
										},
									}
								}
							}
						}
					}
					if choice.FinishReason == "tool_calls" {
						msgToolCalls = append(msgToolCalls, currentToolCall)
						if len(acc.Choices) > 0 {
							acc.Choices[0].Message.ToolCalls = msgToolCalls
						}
					}
				}
			}

			err := openaiStream.Err()
			if err == nil || errors.Is(err, io.EOF) {
				if o.debug {
					jsonData, _ := json.Marshal(acc.ChatCompletion)
					slog.Debug("Response", "messages", string(jsonData))
				}

				if len(acc.Choices) == 0 {
					eventChan <- ProviderEvent{
						Type:  EventError,
						Error: fmt.Errorf("received empty streaming response from OpenAI API - check endpoint configuration"),
					}
					return
				}

				resultFinishReason := acc.Choices[0].FinishReason
				if resultFinishReason == "" {
					// If the finish reason is empty, we assume it was a successful completion
					// INFO: this is happening for openrouter for some reason
					resultFinishReason = "stop"
				}
				// Stream completed successfully
				finishReason := o.finishReason(resultFinishReason)
				if len(acc.Choices[0].Message.ToolCalls) > 0 {
					toolCalls = append(toolCalls, o.toolCalls(acc.ChatCompletion)...)
				}
				if len(toolCalls) > 0 {
					finishReason = message.FinishReasonToolUse
				}

				eventChan <- ProviderEvent{
					Type: EventComplete,
					Response: &ProviderResponse{
						Content:      currentContent,
						ToolCalls:    toolCalls,
						Usage:        o.usage(acc.ChatCompletion),
						FinishReason: finishReason,
					},
				}
				close(eventChan)
				return
			}

			// If there is an error we are going to see if we can retry the call
			retry, after, retryErr := o.shouldRetry(attempts, err)
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
					if ctx.Err() == nil {
						eventChan <- ProviderEvent{Type: EventError, Error: ctx.Err()}
					}
					close(eventChan)
					return
				case <-time.After(time.Duration(after) * time.Millisecond):
					continue
				}
			}
			eventChan <- ProviderEvent{Type: EventError, Error: retryErr}
			close(eventChan)
			return
		}
	}()

	return eventChan
}

func (o *openaiProvider) shouldRetry(attempts int, err error) (bool, int64, error) {
	var apiErr *openai.Error
	if !errors.As(err, &apiErr) {
		return false, 0, err
	}

	if attempts > maxRetries {
		return false, 0, fmt.Errorf("maximum retry attempts reached for rate limit: %d retries", maxRetries)
	}

	// Check for token expiration (401 Unauthorized)
	if apiErr.StatusCode == 401 {
		o.apiKey, err = o.resolver.ResolveValue(o.config.APIKey)
		if err != nil {
			return false, 0, fmt.Errorf("failed to resolve API key: %w", err)
		}
		o.client = createOpenAIClient(o.baseProvider)
		return true, 0, nil
	}

	if apiErr.StatusCode != 429 && apiErr.StatusCode != 500 {
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

func (o *openaiProvider) toolCalls(completion openai.ChatCompletion) []message.ToolCall {
	var toolCalls []message.ToolCall

	if len(completion.Choices) > 0 && len(completion.Choices[0].Message.ToolCalls) > 0 {
		for _, call := range completion.Choices[0].Message.ToolCalls {
			toolCall := message.ToolCall{
				ID:       call.ID,
				Name:     call.Function.Name,
				Input:    call.Function.Arguments,
				Type:     "function",
				Finished: true,
			}
			toolCalls = append(toolCalls, toolCall)
		}
	}

	return toolCalls
}

func (o *openaiProvider) usage(completion openai.ChatCompletion) TokenUsage {
	cachedTokens := completion.Usage.PromptTokensDetails.CachedTokens
	inputTokens := completion.Usage.PromptTokens - cachedTokens

	return TokenUsage{
		InputTokens:         inputTokens,
		OutputTokens:        completion.Usage.CompletionTokens,
		CacheCreationTokens: 0, // OpenAI doesn't provide this directly
		CacheReadTokens:     cachedTokens,
	}
}
