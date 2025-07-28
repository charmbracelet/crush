package provider

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/charmbracelet/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/llm/tools"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/resolver"
)

type EventType string

const maxRetries = 8

const (
	EventContentStart   EventType = "content_start"
	EventToolUseStart   EventType = "tool_use_start"
	EventToolUseDelta   EventType = "tool_use_delta"
	EventToolUseStop    EventType = "tool_use_stop"
	EventContentDelta   EventType = "content_delta"
	EventThinkingDelta  EventType = "thinking_delta"
	EventSignatureDelta EventType = "signature_delta"
	EventContentStop    EventType = "content_stop"
	EventComplete       EventType = "complete"
	EventError          EventType = "error"
	EventWarning        EventType = "warning"
)

type TokenUsage struct {
	InputTokens         int64
	OutputTokens        int64
	CacheCreationTokens int64
	CacheReadTokens     int64
}

type ProviderResponse struct {
	Content      string
	ToolCalls    []message.ToolCall
	Usage        TokenUsage
	FinishReason message.FinishReason
}

type ProviderEvent struct {
	Type EventType

	Content   string
	Thinking  string
	Signature string
	Response  *ProviderResponse
	ToolCall  *message.ToolCall
	Error     error
}

type Config struct {
	// The provider's id.
	ID string `json:"id,omitempty"`
	// The provider's name, used for display purposes.
	Name string `json:"name,omitempty"`
	// The provider's API endpoint.
	BaseURL string `json:"base_url,omitempty"`
	// The provider type, e.g. "openai", "anthropic", etc. if empty it defaults to openai.
	Type catwalk.Type `json:"type,omitempty"`
	// The provider's API key.
	APIKey string `json:"api_key,omitempty"`
	// Marks the provider as disabled.
	Disable bool `json:"disable,omitempty"`

	// Custom system prompt prefix.
	SystemPromptPrefix string `json:"system_prompt_prefix,omitempty"`

	// Extra headers to send with each request to the provider.
	ExtraHeaders map[string]string `json:"extra_headers,omitempty"`
	// Extra body
	ExtraBody map[string]any `json:"extra_body,omitempty"`

	// Used to pass extra parameters to the provider.
	ExtraParams map[string]string `json:"-"`

	// The provider models
	Models []catwalk.Model `json:"models,omitempty"`
}

type Provider interface {
	Send(ctx context.Context, model string, messages []message.Message, tools []tools.BaseTool) (*ProviderResponse, error)

	Stream(ctx context.Context, model string, messages []message.Message, tools []tools.BaseTool) <-chan ProviderEvent

	Model(modelID string) *catwalk.Model

	SetDebug(debug bool)
}

type baseProvider struct {
	baseURL            string
	debug              bool
	config             Config
	apiKey             string
	disableCache       bool
	systemMessage      string
	systemPromptPrefix string
	maxTokens          int64
	think              bool
	reasoningEffort    string
	resolver           resolver.Resolver
	extraHeaders       map[string]string
	extraBody          map[string]any
	extraParams        map[string]string
}

type Option func(*baseProvider)

func WithDisableCache(disableCache bool) Option {
	return func(options *baseProvider) {
		options.disableCache = disableCache
	}
}

func WithSystemMessage(systemMessage string) Option {
	return func(options *baseProvider) {
		options.systemMessage = systemMessage
	}
}

func WithMaxTokens(maxTokens int64) Option {
	return func(options *baseProvider) {
		options.maxTokens = maxTokens
	}
}

func WithThinking(think bool) Option {
	return func(options *baseProvider) {
		options.think = think
	}
}

func WithReasoningEffort(reasoningEffort string) Option {
	return func(options *baseProvider) {
		options.reasoningEffort = reasoningEffort
	}
}

func WithDebug(debug bool) Option {
	return func(options *baseProvider) {
		options.debug = debug
	}
}

func WithResolver(resolver resolver.Resolver) Option {
	return func(options *baseProvider) {
		options.resolver = resolver
	}
}

func newBaseProvider(cfg Config, opts ...Option) (*baseProvider, error) {
	provider := &baseProvider{
		baseURL:            cfg.BaseURL,
		config:             cfg,
		apiKey:             cfg.APIKey,
		extraHeaders:       cfg.ExtraHeaders,
		extraBody:          cfg.ExtraBody,
		systemPromptPrefix: cfg.SystemPromptPrefix,
		resolver:           resolver.New(),
	}
	for _, o := range opts {
		o(provider)
	}

	resolvedAPIKey, err := provider.resolver.ResolveValue(cfg.APIKey)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve API key for provider %s: %w", cfg.ID, err)
	}

	resolvedBaseURL, err := provider.resolver.ResolveValue(cfg.BaseURL)
	if err != nil {
		resolvedBaseURL = ""
	}
	// Resolve extra headers
	resolvedExtraHeaders := make(map[string]string)
	for key, value := range cfg.ExtraHeaders {
		resolvedValue, err := provider.resolver.ResolveValue(value)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve extra header %s for provider %s: %w", key, cfg.ID, err)
		}
		resolvedExtraHeaders[key] = resolvedValue
	}

	provider.apiKey = resolvedAPIKey
	provider.baseURL = resolvedBaseURL
	provider.extraHeaders = resolvedExtraHeaders
	return provider, nil
}

func NewProvider(cfg Config, opts ...Option) (Provider, error) {
	base, err := newBaseProvider(cfg, opts...)
	if err != nil {
		return nil, err
	}
	switch cfg.Type {
	case catwalk.TypeAnthropic:
		return NewAnthropicProvider(base, false), nil
	case catwalk.TypeOpenAI:
		return NewOpenAIProvider(base), nil
	case catwalk.TypeGemini:
		return NewGeminiProvider(base), nil
	case catwalk.TypeBedrock:
		return NewBedrockProvider(base), nil
	case catwalk.TypeAzure:
		return NewAzureProvider(base), nil
	case catwalk.TypeVertexAI:
		return NewVertexAIProvider(base), nil
	}
	return nil, fmt.Errorf("provider not supported: %s", cfg.Type)
}

func (p *baseProvider) cleanMessages(messages []message.Message) (cleaned []message.Message) {
	for _, msg := range messages {
		// The message has no content
		if len(msg.Parts) == 0 {
			continue
		}
		cleaned = append(cleaned, msg)
	}
	return
}

func (o *baseProvider) Model(model string) *catwalk.Model {
	for _, m := range o.config.Models {
		if m.ID == model {
			return &m
		}
	}
	return nil
}

func (o *baseProvider) SetDebug(debug bool) {
	o.debug = debug
}

func (c *Config) TestConnection(resolver resolver.Resolver) error {
	testURL := ""
	headers := make(map[string]string)
	apiKey, _ := resolver.ResolveValue(c.APIKey)
	switch c.Type {
	case catwalk.TypeOpenAI:
		baseURL, _ := resolver.ResolveValue(c.BaseURL)
		if baseURL == "" {
			baseURL = "https://api.openai.com/v1"
		}
		testURL = baseURL + "/models"
		headers["Authorization"] = "Bearer " + apiKey
	case catwalk.TypeAnthropic:
		baseURL, _ := resolver.ResolveValue(c.BaseURL)
		if baseURL == "" {
			baseURL = "https://api.anthropic.com/v1"
		}
		testURL = baseURL + "/models"
		headers["x-api-key"] = apiKey
		headers["anthropic-version"] = "2023-06-01"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client := &http.Client{}
	req, err := http.NewRequestWithContext(ctx, "GET", testURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request for provider %s: %w", c.ID, err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	for k, v := range c.ExtraHeaders {
		req.Header.Set(k, v)
	}
	b, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to create request for provider %s: %w", c.ID, err)
	}
	if b.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to connect to provider %s: %s", c.ID, b.Status)
	}
	_ = b.Body.Close()
	return nil
}
