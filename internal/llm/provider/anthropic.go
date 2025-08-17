package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/bedrock"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/anthropics/anthropic-sdk-go/vertex"
	"github.com/charmbracelet/catwalk/pkg/catwalk"
	"github.com/lacymorrow/lash/internal/auth"
	"github.com/lacymorrow/lash/internal/config"
	"github.com/lacymorrow/lash/internal/llm/tools"
	"github.com/lacymorrow/lash/internal/log"
	"github.com/lacymorrow/lash/internal/message"
)

// Pre-compiled regex for parsing context limit errors.
var contextLimitRegex = regexp.MustCompile(`input length and ` + "`max_tokens`" + ` exceed context limit: (\d+) \+ (\d+) > (\d+)`)

// oauthRoundTripper modifies requests to use OAuth instead of API key
type oauthRoundTripper struct {
	base  http.RoundTripper
	token string
}

func (t *oauthRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// Clone the request
	newReq := req.Clone(req.Context())
	
	// Create new headers map to bypass Go's canonicalization
	newHeaders := make(http.Header)
	
	// Copy only essential headers
	if v := newReq.Header.Get("Accept"); v != "" {
		newHeaders["Accept"] = []string{v}
	}
	if v := newReq.Header.Get("Content-Type"); v != "" {
		newHeaders["Content-Type"] = []string{v}
	}
	
	// Set OAuth headers using exact case - matching OpenCode exactly
	newHeaders["authorization"] = []string{"Bearer " + t.token}
	newHeaders["anthropic-beta"] = []string{"oauth-2025-04-20,claude-code-20250219,interleaved-thinking-2025-05-14,fine-grained-tool-streaming-2025-05-14"}
	newHeaders["anthropic-version"] = []string{"2023-06-01"}
	newHeaders["User-Agent"] = []string{"Claude-Code-Max/1.0"}
	
	// Replace headers
	newReq.Header = newHeaders
	
	// Debug: log the final headers
	slog.Info("OAuth request sending", 
		"url", newReq.URL.String(),
		"headers", newReq.Header)
	
	return t.base.RoundTrip(newReq)
}

type anthropicClient struct {
	providerOptions   providerClientOptions
	tp                AnthropicClientType
	client            anthropic.Client
	adjustedMaxTokens int // Used when context limit is hit
}

type AnthropicClient ProviderClient

type AnthropicClientType string

const (
	AnthropicClientTypeNormal  AnthropicClientType = "normal"
	AnthropicClientTypeBedrock AnthropicClientType = "bedrock"
	AnthropicClientTypeVertex  AnthropicClientType = "vertex"
)

func newAnthropicClient(opts providerClientOptions, tp AnthropicClientType) AnthropicClient {
	return &anthropicClient{
		providerOptions: opts,
		tp:              tp,
		client:          createAnthropicClient(opts, tp),
	}
}

func createAnthropicClient(opts providerClientOptions, tp AnthropicClientType) anthropic.Client {
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

	isBearerToken := strings.HasPrefix(opts.apiKey, config.BearerPrefix)

	// Track if we're using OAuth
	var oauthTransport *oauthRoundTripper
	
	slog.Debug("Anthropic client creation",
		"has_api_key", opts.apiKey != "",
		"has_bearer_auth", hasBearerAuth,
		"is_bearer_token", isBearerToken,
		"api_key_prefix", func() string {
			if len(opts.apiKey) > 20 {
				return opts.apiKey[:20] + "..."
			}
			return opts.apiKey
		}())
	
	if opts.apiKey != "" && !hasBearerAuth {
		if isBearerToken {
			slog.Info("Using Claude Code OAuth with custom transport", 
				"token_prefix", opts.apiKey[:min(20, len(opts.apiKey))]+"...",
				"provider_id", opts.config.ID)
			
			// Extract the token (remove Bearer prefix if present)
			token := opts.apiKey
			if strings.HasPrefix(token, config.BearerPrefix) {
				token = strings.TrimPrefix(token, config.BearerPrefix)
			}
			
			// Create OAuth transport
			oauthTransport = &oauthRoundTripper{
				base:  http.DefaultTransport,
				token: token,
			}
			
			// Set empty API key for OAuth (matching OpenCode)
			anthropicClientOptions = append(anthropicClientOptions, option.WithAPIKey(""))
			
			// Use custom base URL if provided, otherwise use default Anthropic API
			if opts.baseURL != "" {
				anthropicClientOptions = append(anthropicClientOptions, option.WithBaseURL(opts.baseURL))
			}
		} else {
			// Use standard X-Api-Key header
			anthropicClientOptions = append(anthropicClientOptions, option.WithAPIKey(opts.apiKey))
		}
	} else if hasBearerAuth {
		slog.Debug("Authorization header provided in extra headers, using Claude Code OAuth")
		
		// Extract the authorization header value
		authHeader := ""
		for key, value := range opts.extraHeaders {
			if strings.ToLower(key) == "authorization" {
				authHeader = value
				break
			}
		}
		
		// Extract the token (remove Bearer prefix if present)
		token := authHeader
		if strings.HasPrefix(token, "Bearer ") {
			token = strings.TrimPrefix(token, "Bearer ")
		}
		
		// Create OAuth transport
		oauthTransport = &oauthRoundTripper{
			base:  http.DefaultTransport,
			token: token,
		}
		
		// Set empty API key for OAuth (matching OpenCode)
		anthropicClientOptions = append(anthropicClientOptions, option.WithAPIKey(""))
		
		// Use custom base URL if provided, otherwise use default Anthropic API
		if opts.baseURL != "" {
			anthropicClientOptions = append(anthropicClientOptions, option.WithBaseURL(opts.baseURL))
		}
	} else {
		// Use the configured base URL if provided (non-OAuth case)
		if opts.baseURL != "" {
			anthropicClientOptions = append(anthropicClientOptions, option.WithBaseURL(opts.baseURL))
		}
	}

	// Configure HTTP client with OAuth transport if needed
	if oauthTransport != nil {
		if config.Get().Options.Debug {
			// Wrap OAuth transport with debug logging
			debugClient := log.NewHTTPClient()
			oauthTransport.base = debugClient.Transport
			debugClient.Transport = oauthTransport
			anthropicClientOptions = append(anthropicClientOptions, option.WithHTTPClient(debugClient))
		} else {
			// Use OAuth transport directly
			httpClient := &http.Client{
				Transport: oauthTransport,
			}
			anthropicClientOptions = append(anthropicClientOptions, option.WithHTTPClient(httpClient))
		}
	} else if config.Get().Options.Debug {
		// Use debug client without OAuth
		httpClient := log.NewHTTPClient()
		anthropicClientOptions = append(anthropicClientOptions, option.WithHTTPClient(httpClient))
	}

	switch tp {
	case AnthropicClientTypeBedrock:
		anthropicClientOptions = append(anthropicClientOptions, bedrock.WithLoadDefaultConfig(context.Background()))
	case AnthropicClientTypeVertex:
		project := opts.extraParams["project"]
		location := opts.extraParams["location"]
		anthropicClientOptions = append(anthropicClientOptions, vertex.WithGoogleAuth(context.Background(), location, project))
	}
	for key, header := range opts.extraHeaders {
		// Skip authorization header if we're using OAuth
		if oauthTransport != nil && strings.EqualFold(key, "authorization") {
			continue
		}
		anthropicClientOptions = append(anthropicClientOptions, option.WithHeaderAdd(key, header))
	}
	for key, value := range opts.extraBody {
		anthropicClientOptions = append(anthropicClientOptions, option.WithJSONSet(key, value))
	}
	return anthropic.NewClient(anthropicClientOptions...)
}

func (a *anthropicClient) convertMessages(messages []message.Message) (anthropicMessages []anthropic.MessageParam) {
	for i, msg := range messages {
		cache := false
		if i > len(messages)-3 {
			cache = true
		}
		switch msg.Role {
		case message.User:
			content := anthropic.NewTextBlock(msg.Content().String())
			if cache && !a.providerOptions.disableCache {
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
				if cache && !a.providerOptions.disableCache {
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

func (a *anthropicClient) convertTools(tools []tools.BaseTool) []anthropic.ToolUnionParam {
	anthropicTools := make([]anthropic.ToolUnionParam, len(tools))

	for i, tool := range tools {
		info := tool.Info()
		toolParam := anthropic.ToolParam{
			Name:        info.Name,
			Description: anthropic.String(info.Description),
			InputSchema: anthropic.ToolInputSchemaParam{
				Properties: info.Parameters,
				// TODO: figure out how we can tell claude the required fields?
			},
		}

		if i == len(tools)-1 && !a.providerOptions.disableCache {
			toolParam.CacheControl = anthropic.CacheControlEphemeralParam{
				Type: "ephemeral",
			}
		}

		anthropicTools[i] = anthropic.ToolUnionParam{OfTool: &toolParam}
	}

	return anthropicTools
}

func (a *anthropicClient) finishReason(reason string) message.FinishReason {
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

func (a *anthropicClient) isThinkingEnabled() bool {
	cfg := config.Get()
	modelConfig := cfg.Models[config.SelectedModelTypeLarge]
	if a.providerOptions.modelType == config.SelectedModelTypeSmall {
		modelConfig = cfg.Models[config.SelectedModelTypeSmall]
	}
	return a.Model().CanReason && modelConfig.Think
}

func (a *anthropicClient) preparedMessages(messages []anthropic.MessageParam, tools []anthropic.ToolUnionParam) anthropic.MessageNewParams {
	model := a.providerOptions.model(a.providerOptions.modelType)
	slog.Info("Retrieved model for API call", "model_id", model.ID, "model_name", model.Name, "model_type", a.providerOptions.modelType)
	var thinkingParam anthropic.ThinkingConfigParamUnion
	cfg := config.Get()
	modelConfig := cfg.Models[config.SelectedModelTypeLarge]
	if a.providerOptions.modelType == config.SelectedModelTypeSmall {
		modelConfig = cfg.Models[config.SelectedModelTypeSmall]
	}
	temperature := anthropic.Float(0)

	maxTokens := model.DefaultMaxTokens
	if modelConfig.MaxTokens > 0 {
		maxTokens = modelConfig.MaxTokens
	}
	if a.isThinkingEnabled() {
		thinkingParam = anthropic.ThinkingConfigParamOfEnabled(int64(float64(maxTokens) * 0.8))
		temperature = anthropic.Float(1)
	}
	// Override max tokens if set in provider options
	if a.providerOptions.maxTokens > 0 {
		maxTokens = a.providerOptions.maxTokens
	}

	// Use adjusted max tokens if context limit was hit
	if a.adjustedMaxTokens > 0 {
		maxTokens = int64(a.adjustedMaxTokens)
	}

	systemBlocks := []anthropic.TextBlockParam{}

	// Add Claude Code spoofing for OAuth authenticated requests
	isOAuth := strings.HasPrefix(a.providerOptions.apiKey, config.BearerPrefix)
	if isOAuth && strings.Contains(model.ID, "claude-") {
		// Prepend Claude Code identity when using OAuth (matching OpenCode)
		systemBlocks = append(systemBlocks, anthropic.TextBlockParam{
			Text: "You are Claude Code, Anthropic's official CLI for Claude.",
		})
	}

	// Add custom system prompt prefix if configured
	if a.providerOptions.systemPromptPrefix != "" {
		systemBlocks = append(systemBlocks, anthropic.TextBlockParam{
			Text: a.providerOptions.systemPromptPrefix,
		})
	}

	systemBlocks = append(systemBlocks, anthropic.TextBlockParam{
		Text: a.providerOptions.systemMessage,
		CacheControl: anthropic.CacheControlEphemeralParam{
			Type: "ephemeral",
		},
	})

	slog.Info("Preparing Anthropic API request", "model_id", model.ID, "provider_id", a.providerOptions.config.ID)
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

func (a *anthropicClient) send(ctx context.Context, messages []message.Message, tools []tools.BaseTool) (response *ProviderResponse, err error) {
	attempts := 0
	for {
		attempts++
		// Prepare messages on each attempt in case max_tokens was adjusted
		preparedMessages := a.preparedMessages(a.convertMessages(messages), a.convertTools(tools))

		anthropicResponse, err := a.client.Messages.New(
			ctx,
			preparedMessages,
		)
		// If there is an error we are going to see if we can retry the call
		if err != nil {
			slog.Error("Anthropic API error", "error", err.Error(), "attempt", attempts, "max_retries", maxRetries)
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

func (a *anthropicClient) stream(ctx context.Context, messages []message.Message, tools []tools.BaseTool) <-chan ProviderEvent {
	attempts := 0
	eventChan := make(chan ProviderEvent)
	go func() {
		for {
			attempts++
			// Prepare messages on each attempt in case max_tokens was adjusted
			preparedMessages := a.preparedMessages(a.convertMessages(messages), a.convertTools(tools))

			anthropicStream := a.client.Messages.NewStreaming(
				ctx,
				preparedMessages,
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

func (a *anthropicClient) shouldRetry(attempts int, err error) (bool, int64, error) {
	var apiErr *anthropic.Error
	if !errors.As(err, &apiErr) {
		return false, 0, err
	}

	// Handle 401 first (token refresh) before checking retry limits
	if apiErr.StatusCode == 401 {
		// Check if we're using OAuth (Bearer token)
		if strings.HasPrefix(a.providerOptions.apiKey, config.BearerPrefix) {
			// Try refresh via OAuth - always use "anthropic" as the auth provider ID
			// (both anthropic and anthropic-max store OAuth tokens under "anthropic")
			slog.Info("OAuth token expired, refreshing", "provider_id", a.providerOptions.config.ID)
			if token, terr := auth.AccessToken("anthropic"); terr == nil && token != "" {
				if !strings.HasPrefix(token, config.BearerPrefix) {
					token = config.BearerPrefix + token
				}
				slog.Info("OAuth token refreshed successfully")
				a.providerOptions.apiKey = token
				// Recreate the client with new token
				a.client = createAnthropicClient(a.providerOptions, a.tp)
				return true, 0, nil
			}
			slog.Error("Failed to refresh OAuth token")
		}
		// Fallback to resolving configured API key (might be env)
		a.providerOptions.apiKey, err = config.Get().Resolve(a.providerOptions.config.APIKey)
		if err != nil {
			return false, 0, fmt.Errorf("failed to resolve API key: %w", err)
		}
		a.client = createAnthropicClient(a.providerOptions, a.tp)
		return true, 0, nil
	}

	// Check retry limits after 401 handling
	if attempts > maxRetries {
		return false, 0, fmt.Errorf("maximum retry attempts reached for rate limit: %d retries", maxRetries)
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

	backoffMs := computeBackoffMs(attempts)
	retryMs = backoffMs
	if len(retryAfterValues) > 0 {
		if _, err := fmt.Sscanf(retryAfterValues[0], "%d", &retryMs); err == nil {
			retryMs = retryMs * 1000
		}
	}
	return true, int64(retryMs), nil
}

// handleContextLimitError parses context limit error and returns adjusted max_tokens
func (a *anthropicClient) handleContextLimitError(apiErr *anthropic.Error) (int, bool) {
	// Parse error message like: "input length and max_tokens exceed context limit: 154978 + 50000 > 200000"
	errorMsg := apiErr.Error()

	matches := contextLimitRegex.FindStringSubmatch(errorMsg)

	if len(matches) != 4 {
		return 0, false
	}

	inputTokens, err1 := strconv.Atoi(matches[1])
	contextLimit, err2 := strconv.Atoi(matches[3])

	if err1 != nil || err2 != nil {
		return 0, false
	}

	// Calculate safe max_tokens with a buffer
	safeMaxTokens := contextLimit - inputTokens - config.ContextLimitBufferTokens

	// Ensure we don't go below a minimum threshold
	safeMaxTokens = max(safeMaxTokens, config.MinSafeMaxTokens)

	return safeMaxTokens, true
}

func (a *anthropicClient) toolCalls(msg anthropic.Message) []message.ToolCall {
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

func (a *anthropicClient) usage(msg anthropic.Message) TokenUsage {
	return TokenUsage{
		InputTokens:         msg.Usage.InputTokens,
		OutputTokens:        msg.Usage.OutputTokens,
		CacheCreationTokens: msg.Usage.CacheCreationInputTokens,
		CacheReadTokens:     msg.Usage.CacheReadInputTokens,
	}
}

func (a *anthropicClient) Model() catwalk.Model {
	return a.providerOptions.model(a.providerOptions.modelType)
}
