package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/charmbracelet/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/llm/tools"
	"github.com/charmbracelet/crush/internal/log"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/google/uuid"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/param"
	"github.com/openai/openai-go/shared"
)

type openaiClient struct {
	providerOptions providerClientOptions
	client          openai.Client
}

type OpenAIClient ProviderClient

func newOpenAIClient(opts providerClientOptions) OpenAIClient {
	return &openaiClient{
		providerOptions: opts,
		client:          createOpenAIClient(opts),
	}
}

func createOpenAIClient(opts providerClientOptions) openai.Client {
	openaiClientOptions := []option.RequestOption{}
	if opts.apiKey != "" {
		openaiClientOptions = append(openaiClientOptions, option.WithAPIKey(opts.apiKey))
	}
	if opts.baseURL != "" {
		resolvedBaseURL, err := config.Get().Resolve(opts.baseURL)
		if err == nil && resolvedBaseURL != "" {
			openaiClientOptions = append(openaiClientOptions, option.WithBaseURL(resolvedBaseURL))
		}
	}

	if config.Get().Options.Debug {
		httpClient := log.NewHTTPClient()
		openaiClientOptions = append(openaiClientOptions, option.WithHTTPClient(httpClient))
	}

	for key, value := range opts.extraHeaders {
		openaiClientOptions = append(openaiClientOptions, option.WithHeader(key, value))
	}

	for extraKey, extraValue := range opts.extraBody {
		openaiClientOptions = append(openaiClientOptions, option.WithJSONSet(extraKey, extraValue))
	}

	return openai.NewClient(openaiClientOptions...)
}

func (o *openaiClient) convertMessages(messages []message.Message) (openaiMessages []openai.ChatCompletionMessageParamUnion) {
	isAnthropicModel := o.providerOptions.config.ID == string(catwalk.InferenceProviderOpenRouter) && strings.HasPrefix(o.Model().ID, "anthropic/")
	// Add system message first
	systemMessage := o.providerOptions.systemMessage
	if o.providerOptions.systemPromptPrefix != "" {
		systemMessage = o.providerOptions.systemPromptPrefix + "\n" + systemMessage
	}

	system := openai.SystemMessage(systemMessage)
	if isAnthropicModel && !o.providerOptions.disableCache {
		systemTextBlock := openai.ChatCompletionContentPartTextParam{Text: systemMessage}
		systemTextBlock.SetExtraFields(
			map[string]any{
				"cache_control": map[string]string{
					"type": "ephemeral",
				},
			},
		)
		var content []openai.ChatCompletionContentPartTextParam
		content = append(content, systemTextBlock)
		system = openai.SystemMessage(content)
	}
	openaiMessages = append(openaiMessages, system)

	for i, msg := range messages {
		cache := false
		if i > len(messages)-3 {
			cache = true
		}
		switch msg.Role {
		case message.User:
			var content []openai.ChatCompletionContentPartUnionParam

			textBlock := openai.ChatCompletionContentPartTextParam{Text: msg.Content().String()}
			content = append(content, openai.ChatCompletionContentPartUnionParam{OfText: &textBlock})
			hasBinaryContent := false
			for _, binaryContent := range msg.BinaryContent() {
				hasBinaryContent = true
				imageURL := openai.ChatCompletionContentPartImageImageURLParam{URL: binaryContent.String(catwalk.InferenceProviderOpenAI)}
				imageBlock := openai.ChatCompletionContentPartImageParam{ImageURL: imageURL}

				content = append(content, openai.ChatCompletionContentPartUnionParam{OfImageURL: &imageBlock})
			}
			if cache && !o.providerOptions.disableCache && isAnthropicModel {
				textBlock.SetExtraFields(map[string]any{
					"cache_control": map[string]string{
						"type": "ephemeral",
					},
				})
			}
			if hasBinaryContent || (isAnthropicModel && !o.providerOptions.disableCache) {
				openaiMessages = append(openaiMessages, openai.UserMessage(content))
			} else {
				openaiMessages = append(openaiMessages, openai.UserMessage(msg.Content().String()))
			}

		case message.Assistant:
			assistantMsg := openai.ChatCompletionAssistantMessageParam{
				Role: "assistant",
			}

			// Only include finished tool calls; interrupted tool calls must not be resent.
			if len(msg.ToolCalls()) > 0 {
				finished := make([]message.ToolCall, 0, len(msg.ToolCalls()))
				for _, call := range msg.ToolCalls() {
					if call.Finished {
						finished = append(finished, call)
					}
				}
				if len(finished) > 0 {
					assistantMsg.ToolCalls = make([]openai.ChatCompletionMessageToolCallParam, len(finished))
					for i, call := range finished {
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
			}
			if msg.Content().String() != "" {
				assistantMsg.Content = openai.ChatCompletionAssistantMessageParamContentUnion{
					OfString: param.NewOpt(msg.Content().Text),
				}
			}

			if cache && !o.providerOptions.disableCache && isAnthropicModel {
				assistantMsg.SetExtraFields(map[string]any{
					"cache_control": map[string]string{
						"type": "ephemeral",
					},
				})
			}
			// Skip empty assistant messages (no content and no finished tool calls)
			if msg.Content().String() == "" && len(assistantMsg.ToolCalls) == 0 {
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

	return openaiMessages
}

func (o *openaiClient) convertTools(tools []tools.BaseTool) []openai.ChatCompletionToolParam {
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

func (o *openaiClient) finishReason(reason string) message.FinishReason {
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

func (o *openaiClient) preparedParams(messages []openai.ChatCompletionMessageParamUnion, tools []openai.ChatCompletionToolParam) openai.ChatCompletionNewParams {
	model := o.providerOptions.model(o.providerOptions.modelType)
	cfg := config.Get()

	modelConfig := cfg.Models[config.SelectedModelTypeLarge]
	if o.providerOptions.modelType == config.SelectedModelTypeSmall {
		modelConfig = cfg.Models[config.SelectedModelTypeSmall]
	}

	reasoningEffort := modelConfig.ReasoningEffort

	params := openai.ChatCompletionNewParams{
		Model:    openai.ChatModel(model.ID),
		Messages: messages,
		Tools:    tools,
	}
	
	// Check if tools should be disabled for this provider
	disableTools := o.providerOptions.extraParams["disable_tools"]
	if disableTools == "true" {
		params.Tools = nil
	}

	maxTokens := model.DefaultMaxTokens
	if modelConfig.MaxTokens > 0 {
		maxTokens = modelConfig.MaxTokens
	}

	// Override max tokens if set in provider options
	if o.providerOptions.maxTokens > 0 {
		maxTokens = o.providerOptions.maxTokens
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
		case "minimal":
			params.ReasoningEffort = shared.ReasoningEffort("minimal")
		default:
			params.ReasoningEffort = shared.ReasoningEffort(reasoningEffort)
		}
	} else {
		// For non-reasoning models, only set MaxTokens (don't set MaxCompletionTokens at all)
		params.MaxTokens = openai.Int(maxTokens)
	}

	return params
}

// convertMessagesSimple converts messages to simple map format for compatibility mode
func (o *openaiClient) convertMessagesSimple(messages []message.Message) []map[string]interface{} {
	var result []map[string]interface{}
	
	// Add system message
	systemMessage := o.providerOptions.systemMessage
	if o.providerOptions.systemPromptPrefix != "" {
		systemMessage = o.providerOptions.systemPromptPrefix + "\n" + systemMessage
	}
	if systemMessage != "" {
		result = append(result, map[string]interface{}{
			"role":    "system",
			"content": systemMessage,
		})
	}
	
	for _, msg := range messages {
		switch msg.Role {
		case message.User:
			result = append(result, map[string]interface{}{
				"role":    "user",
				"content": msg.Content().String(),
			})
		case message.Assistant:
			result = append(result, map[string]interface{}{
				"role":    "assistant",
				"content": msg.Content().String(),
			})
		}
	}
	
	return result
}


// sendCompatible sends a request using a custom HTTP call to avoid OpenAI client limitations
func (o *openaiClient) sendCompatible(ctx context.Context, messages []message.Message, tools []tools.BaseTool) (response *ProviderResponse, err error) {
	
	model := o.providerOptions.model(o.providerOptions.modelType)
	cfg := config.Get()
	
	modelConfig := cfg.Models[config.SelectedModelTypeLarge]
	if o.providerOptions.modelType == config.SelectedModelTypeSmall {
		modelConfig = cfg.Models[config.SelectedModelTypeSmall]
	}
	
	maxTokens := model.DefaultMaxTokens
	if modelConfig.MaxTokens > 0 {
		maxTokens = modelConfig.MaxTokens
	}
	if o.providerOptions.maxTokens > 0 {
		maxTokens = o.providerOptions.maxTokens
	}
	
	// Create a simple request payload without problematic fields
	payload := map[string]interface{}{
		"model":      model.ID,
		"messages":   o.convertMessagesSimple(messages),
		"max_tokens": maxTokens,
	}
	
	// Only add tools if not disabled
	disableTools := o.providerOptions.extraParams["disable_tools"]
	if disableTools != "true" {
		convertedTools := o.convertTools(tools)
		if len(convertedTools) > 0 {
			payload["tools"] = convertedTools
		}
	}
	
	
	// Make HTTP request
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	
	baseURL := o.providerOptions.baseURL
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	
	req, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+o.providerOptions.apiKey)
	
	// Add extra headers
	for key, value := range o.providerOptions.extraHeaders {
		req.Header.Set(key, value)
	}
	
	client := &http.Client{}
	if cfg.Options.Debug {
		client = log.NewHTTPClient()
	}
	
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}
	
	// Read the response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	
	// Use flexible JSON parsing for compatibility mode
	var genericResponse map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &genericResponse); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	
	// Extract choices
	choicesInterface, ok := genericResponse["choices"]
	if !ok {
		return nil, fmt.Errorf("no choices in response")
	}
	
	choices, ok := choicesInterface.([]interface{})
	if !ok || len(choices) == 0 {
		return nil, fmt.Errorf("received empty choices from API")
	}
	
	firstChoice, ok := choices[0].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid choice format")
	}
	
	// Extract message content
	messageInterface, ok := firstChoice["message"]
	if !ok {
		return nil, fmt.Errorf("no message in choice")
	}
	
	messageMap, ok := messageInterface.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid message format")
	}
	
	content := ""
	if contentInterface, exists := messageMap["content"]; exists {
		if contentStr, ok := contentInterface.(string); ok {
			content = contentStr
		}
	}
	
	// Extract finish reason
	finishReasonStr := "stop" // default
	if finishReasonInterface, exists := firstChoice["finish_reason"]; exists {
		if finishReasonVal, ok := finishReasonInterface.(string); ok {
			finishReasonStr = finishReasonVal
		}
	}
	
	finishReason := o.finishReason(finishReasonStr)
	
	// Create usage info - simplified for compatibility mode
	usage := TokenUsage{
		InputTokens:  0,
		OutputTokens: 0,
	}
	
	if usageInterface, exists := genericResponse["usage"]; exists {
		if usageMap, ok := usageInterface.(map[string]interface{}); ok {
			if promptTokens, exists := usageMap["prompt_tokens"]; exists {
				if promptTokensInt, ok := promptTokens.(float64); ok {
					usage.InputTokens = int64(promptTokensInt)
				}
			}
			if completionTokens, exists := usageMap["completion_tokens"]; exists {
				if completionTokensInt, ok := completionTokens.(float64); ok {
					usage.OutputTokens = int64(completionTokensInt)
				}
			}
		}
	}
	
	// No tool calls in compatibility mode for now  
	var toolCalls []message.ToolCall
	
	if len(toolCalls) > 0 {
		finishReason = message.FinishReasonToolUse
	}
	
	return &ProviderResponse{
		Content:      content,
		ToolCalls:    toolCalls,
		Usage:        usage,
		FinishReason: finishReason,
	}, nil
}

// streamCompatible sends a streaming request using custom HTTP call to avoid OpenAI client limitations
func (o *openaiClient) streamCompatible(ctx context.Context, messages []message.Message, tools []tools.BaseTool) <-chan ProviderEvent {
	eventChan := make(chan ProviderEvent)
	
	go func() {
		defer close(eventChan)
		
		model := o.providerOptions.model(o.providerOptions.modelType)
		cfg := config.Get()
		
		modelConfig := cfg.Models[config.SelectedModelTypeLarge]
		if o.providerOptions.modelType == config.SelectedModelTypeSmall {
			modelConfig = cfg.Models[config.SelectedModelTypeSmall]
		}
		
		maxTokens := model.DefaultMaxTokens
		if modelConfig.MaxTokens > 0 {
			maxTokens = modelConfig.MaxTokens
		}
		if o.providerOptions.maxTokens > 0 {
			maxTokens = o.providerOptions.maxTokens
		}
		
		// Create streaming request payload
		payload := map[string]interface{}{
			"model":      model.ID,
			"messages":   o.convertMessagesSimple(messages),
			"max_tokens": maxTokens,
			"stream":     true,
		}
		
		// Only add tools if not disabled
		disableTools := o.providerOptions.extraParams["disable_tools"]
		if disableTools != "true" {
			convertedTools := o.convertTools(tools)
			if len(convertedTools) > 0 {
				payload["tools"] = convertedTools
			}
		}
		
		jsonData, err := json.Marshal(payload)
		if err != nil {
			eventChan <- ProviderEvent{Type: EventError, Error: fmt.Errorf("failed to marshal request: %w", err)}
			return
		}
		
		baseURL := o.providerOptions.baseURL
		if baseURL == "" {
			baseURL = "https://api.openai.com/v1"
		}
		
		req, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/chat/completions", bytes.NewBuffer(jsonData))
		if err != nil {
			eventChan <- ProviderEvent{Type: EventError, Error: fmt.Errorf("failed to create request: %w", err)}
			return
		}
		
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+o.providerOptions.apiKey)
		req.Header.Set("Accept", "text/event-stream")
		req.Header.Set("Cache-Control", "no-cache")
		
		// Add extra headers
		for key, value := range o.providerOptions.extraHeaders {
			req.Header.Set(key, value)
		}
		
		client := &http.Client{}
		if cfg.Options.Debug {
			client = log.NewHTTPClient()
		}
		
		resp, err := client.Do(req)
		if err != nil {
			eventChan <- ProviderEvent{Type: EventError, Error: fmt.Errorf("request failed: %w", err)}
			return
		}
		defer resp.Body.Close()
		
		if resp.StatusCode != 200 {
			eventChan <- ProviderEvent{Type: EventError, Error: fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)}
			return
		}
		
		// Parse Server-Sent Events
		scanner := bufio.NewScanner(resp.Body)
		currentContent := ""
		var usage TokenUsage
		finishReason := message.FinishReasonEndTurn
		
		for scanner.Scan() {
			line := scanner.Text()
			
			// Skip empty lines and non-data lines
			if line == "" || !strings.HasPrefix(line, "data: ") {
				continue
			}
			
			// Extract JSON from "data: " prefix
			jsonStr := strings.TrimPrefix(line, "data: ")
			
			// Skip [DONE] message
			if jsonStr == "[DONE]" {
				break
			}
			
			var chunk map[string]interface{}
			if err := json.Unmarshal([]byte(jsonStr), &chunk); err != nil {
				continue // Skip malformed chunks
			}
			
			// Extract choices
			choicesInterface, ok := chunk["choices"]
			if !ok {
				continue
			}
			
			choices, ok := choicesInterface.([]interface{})
			if !ok || len(choices) == 0 {
				continue
			}
			
			firstChoice, ok := choices[0].(map[string]interface{})
			if !ok {
				continue
			}
			
			// Check for finish reason
			if finishReasonInterface, exists := firstChoice["finish_reason"]; exists && finishReasonInterface != nil {
				if finishReasonVal, ok := finishReasonInterface.(string); ok {
					finishReason = o.finishReason(finishReasonVal)
				}
			}
			
			// Extract delta content
			deltaInterface, ok := firstChoice["delta"]
			if !ok {
				continue
			}
			
			delta, ok := deltaInterface.(map[string]interface{})
			if !ok {
				continue
			}
			
			if contentInterface, exists := delta["content"]; exists {
				if contentStr, ok := contentInterface.(string); ok && contentStr != "" {
					currentContent += contentStr
					eventChan <- ProviderEvent{
						Type:    EventContentDelta,
						Content: contentStr,
					}
				}
			}
			
			// Extract usage from final chunk
			if usageInterface, exists := chunk["usage"]; exists {
				if usageMap, ok := usageInterface.(map[string]interface{}); ok {
					if promptTokens, exists := usageMap["prompt_tokens"]; exists {
						if promptTokensInt, ok := promptTokens.(float64); ok {
							usage.InputTokens = int64(promptTokensInt)
						}
					}
					if completionTokens, exists := usageMap["completion_tokens"]; exists {
						if completionTokensInt, ok := completionTokens.(float64); ok {
							usage.OutputTokens = int64(completionTokensInt)
						}
					}
				}
			}
		}
		
		if err := scanner.Err(); err != nil {
			eventChan <- ProviderEvent{Type: EventError, Error: fmt.Errorf("stream reading error: %w", err)}
			return
		}
		
		// Send completion event
		eventChan <- ProviderEvent{
			Type: EventComplete,
			Response: &ProviderResponse{
				Content:      currentContent,
				ToolCalls:    nil, // No tool calls in compatibility mode for now
				Usage:        usage,
				FinishReason: finishReason,
			},
		}
	}()
	
	return eventChan
}

func (o *openaiClient) send(ctx context.Context, messages []message.Message, tools []tools.BaseTool) (response *ProviderResponse, err error) {
	// Check if compatibility mode is enabled for providers that don't support all OpenAI parameters
	compatibilityMode := o.providerOptions.extraParams["compatibility_mode"]
	if compatibilityMode == "true" {
		return o.sendCompatible(ctx, messages, tools)
	}
	
	params := o.preparedParams(o.convertMessages(messages), o.convertTools(tools))
	
	
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

func (o *openaiClient) stream(ctx context.Context, messages []message.Message, tools []tools.BaseTool) <-chan ProviderEvent {
	// Check if streaming is disabled for this provider
	disableStreaming := o.providerOptions.extraParams["disable_streaming"]
	
	if disableStreaming == "true" {
		// Use non-streaming send() and convert to streaming events
		eventChan := make(chan ProviderEvent, 1)
		go func() {
			defer close(eventChan)
			response, err := o.send(ctx, messages, tools)
			if err != nil {
				eventChan <- ProviderEvent{Type: EventError, Error: err}
				return
			}
			eventChan <- ProviderEvent{Type: EventComplete, Response: response}
		}()
		return eventChan
	}
	
	// Check if compatibility mode is enabled
	compatibilityMode := o.providerOptions.extraParams["compatibility_mode"]
	if compatibilityMode == "true" {
		return o.streamCompatible(ctx, messages, tools)
	}

	params := o.preparedParams(o.convertMessages(messages), o.convertTools(tools))
	params.StreamOptions = openai.ChatCompletionStreamOptionsParam{
		IncludeUsage: openai.Bool(true),
	}

	attempts := 0
	eventChan := make(chan ProviderEvent)

	go func() {
		for {
			attempts++
			// Kujtim: fixes an issue with anthropig models on openrouter
			if len(params.Tools) == 0 {
				params.Tools = nil
			}
			openaiStream := o.client.Chat.Completions.NewStreaming(
				ctx,
				params,
			)

			acc := openai.ChatCompletionAccumulator{}
			currentContent := ""
			toolCalls := make([]message.ToolCall, 0)
			msgToolCalls := make(map[int64]openai.ChatCompletionMessageToolCall)
			toolMap := make(map[string]openai.ChatCompletionMessageToolCall)
			toolCallIDMap := make(map[string]string)
			for openaiStream.Next() {
				chunk := openaiStream.Current()
				// Kujtim: this is an issue with openrouter qwen, its sending -1 for the tool index
				if len(chunk.Choices) != 0 && len(chunk.Choices[0].Delta.ToolCalls) > 0 && chunk.Choices[0].Delta.ToolCalls[0].Index == -1 {
					chunk.Choices[0].Delta.ToolCalls[0].Index = 0
				}
				acc.AddChunk(chunk)
				for i, choice := range chunk.Choices {
					reasoning, ok := choice.Delta.JSON.ExtraFields["reasoning"]
					if ok && reasoning.Raw() != "" {
						reasoningStr := ""
						json.Unmarshal([]byte(reasoning.Raw()), &reasoningStr)
						if reasoningStr != "" {
							eventChan <- ProviderEvent{
								Type:     EventThinkingDelta,
								Thinking: reasoningStr,
							}
						}
					}
					if choice.Delta.Content != "" {
						eventChan <- ProviderEvent{
							Type:    EventContentDelta,
							Content: choice.Delta.Content,
						}
						currentContent += choice.Delta.Content
					} else if len(choice.Delta.ToolCalls) > 0 {
						toolCall := choice.Delta.ToolCalls[0]
						if strings.HasPrefix(toolCall.ID, "functions.") {
							exID, ok := toolCallIDMap[toolCall.ID]
							if !ok {
								newID := uuid.NewString()
								toolCallIDMap[toolCall.ID] = newID
								toolCall.ID = newID
							} else {
								toolCall.ID = exID
							}
						}
						newToolCall := false
						if existingToolCall, ok := msgToolCalls[toolCall.Index]; ok { // tool call exists
							if toolCall.ID != "" && toolCall.ID != existingToolCall.ID {
								found := false
								// try to find the tool based on the ID
								for _, tool := range msgToolCalls {
									if tool.ID == toolCall.ID {
										existingToolCall.Function.Arguments += toolCall.Function.Arguments
										msgToolCalls[toolCall.Index] = existingToolCall
										toolMap[existingToolCall.ID] = existingToolCall
										found = true
									}
								}
								if !found {
									newToolCall = true
								}
							} else {
								existingToolCall.Function.Arguments += toolCall.Function.Arguments
								msgToolCalls[toolCall.Index] = existingToolCall
								toolMap[existingToolCall.ID] = existingToolCall
							}
						} else {
							newToolCall = true
						}
						if newToolCall { // new tool call
							if toolCall.ID == "" {
								toolCall.ID = uuid.NewString()
							}
							eventChan <- ProviderEvent{
								Type: EventToolUseStart,
								ToolCall: &message.ToolCall{
									ID:       toolCall.ID,
									Name:     toolCall.Function.Name,
									Finished: false,
								},
							}
							msgToolCalls[toolCall.Index] = openai.ChatCompletionMessageToolCall{
								ID:   toolCall.ID,
								Type: "function",
								Function: openai.ChatCompletionMessageToolCallFunction{
									Name:      toolCall.Function.Name,
									Arguments: toolCall.Function.Arguments,
								},
							}
							toolMap[toolCall.ID] = msgToolCalls[toolCall.Index]
						}
						toolCalls := []openai.ChatCompletionMessageToolCall{}
						for _, tc := range toolMap {
							toolCalls = append(toolCalls, tc)
						}
						acc.Choices[i].Message.ToolCalls = toolCalls
					}
				}
			}

			err := openaiStream.Err()
			if err == nil || errors.Is(err, io.EOF) {
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
				slog.Warn("Retrying due to rate limit", "attempt", attempts, "max_retries", maxRetries, "error", err)
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
			eventChan <- ProviderEvent{Type: EventError, Error: retryErr}
			close(eventChan)
			return
		}
	}()

	return eventChan
}

func (o *openaiClient) shouldRetry(attempts int, err error) (bool, int64, error) {
	if attempts > maxRetries {
		return false, 0, fmt.Errorf("maximum retry attempts reached for rate limit: %d retries", maxRetries)
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false, 0, err
	}
	var apiErr *openai.Error
	retryMs := 0
	retryAfterValues := []string{}
	if errors.As(err, &apiErr) {
		// Check for token expiration (401 Unauthorized)
		if apiErr.StatusCode == http.StatusUnauthorized {
			prev := o.providerOptions.apiKey
			// in case the key comes from a script, we try to re-evaluate it.
			o.providerOptions.apiKey, err = config.Get().Resolve(o.providerOptions.config.APIKey)
			if err != nil {
				return false, 0, fmt.Errorf("failed to resolve API key: %w", err)
			}
			// if it didn't change, do not retry.
			if prev == o.providerOptions.apiKey {
				return false, 0, err
			}
			o.client = createOpenAIClient(o.providerOptions)
			return true, 0, nil
		}

		if apiErr.StatusCode == http.StatusTooManyRequests {
			// Check if this is an insufficient quota error (permanent)
			if apiErr.Type == "insufficient_quota" || apiErr.Code == "insufficient_quota" {
				return false, 0, fmt.Errorf("OpenAI quota exceeded: %s. Please check your plan and billing details", apiErr.Message)
			}
			// Other 429 errors (rate limiting) can be retried
		} else if apiErr.StatusCode != http.StatusInternalServerError {
			return false, 0, err
		}

		if apiErr.Response != nil {
			retryAfterValues = apiErr.Response.Header.Values("Retry-After")
		}
	}

	if apiErr != nil {
		slog.Warn("OpenAI API error", "status_code", apiErr.StatusCode, "message", apiErr.Message, "type", apiErr.Type)
		if len(retryAfterValues) > 0 {
			slog.Warn("Retry-After header", "values", retryAfterValues)
		}
	} else {
		slog.Error("OpenAI API error", "error", err.Error(), "attempt", attempts, "max_retries", maxRetries)
	}

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

func (o *openaiClient) toolCalls(completion openai.ChatCompletion) []message.ToolCall {
	var toolCalls []message.ToolCall

	if len(completion.Choices) > 0 && len(completion.Choices[0].Message.ToolCalls) > 0 {
		for _, call := range completion.Choices[0].Message.ToolCalls {
			// accumulator for some reason does this.
			if call.Function.Name == "" {
				continue
			}
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

func (o *openaiClient) usage(completion openai.ChatCompletion) TokenUsage {
	cachedTokens := completion.Usage.PromptTokensDetails.CachedTokens
	inputTokens := completion.Usage.PromptTokens - cachedTokens

	return TokenUsage{
		InputTokens:         inputTokens,
		OutputTokens:        completion.Usage.CompletionTokens,
		CacheCreationTokens: 0, // OpenAI doesn't provide this directly
		CacheReadTokens:     cachedTokens,
	}
}

func (o *openaiClient) Model() catwalk.Model {
	return o.providerOptions.model(o.providerOptions.modelType)
}
