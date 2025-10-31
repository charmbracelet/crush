package agent

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"charm.land/fantasy"
	"charm.land/fantasy/providers/anthropic"
	"charm.land/fantasy/providers/bedrock"
	"charm.land/fantasy/providers/google"
	"charm.land/fantasy/providers/openai"
	"charm.land/fantasy/providers/openrouter"
	"github.com/charmbracelet/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/session"
)

//go:embed templates/title.md
var titlePrompt []byte

//go:embed templates/summary.md
var summaryPrompt []byte

// Errors
var (
	ErrEmptyPrompt    = errors.New("empty prompt")
	ErrSessionMissing = errors.New("session missing")
	ErrAgentCanceled  = errors.New("agent run cancelled")
)

type SessionAgentCall struct {
	SessionID        string
	Prompt           string
	ProviderOptions  fantasy.ProviderOptions
	Attachments      []message.Attachment
	MaxOutputTokens  int64
	Temperature      *float64
	TopP             *float64
	TopK             *int64
	FrequencyPenalty *float64
	PresencePenalty  *float64
}

type SessionAgent interface {
	Run(context.Context, SessionAgentCall) (*fantasy.AgentResult, error)
	SetModels(large Model, small Model)
	SetTools(tools []fantasy.AgentTool)
	Cancel(sessionID string)
	CancelAll()
	IsSessionBusy(sessionID string)
	IsBusy() bool
	QueuedPrompts(sessionID string) int
	ClearQueue(sessionID string)
	Summarize(context.Context, string, fantasy.ProviderOptions) error
	Model() Model
}

type Model struct {
	Model      fantasy.LanguageModel
	CatwalkCfg catwalk.Model
	ModelCfg   config.SelectedModel
}

type sessionAgent struct {
	largeModel           Model
	smallModel           Model
	systemPromptPrefix   string
	systemPrompt         string
	tools                []fantasy.AgentTool
	sessions             session.Service
	messages             message.Service
	disableAutoSummarize bool
	isYolo               bool

	messageQueue   *csync.Map[string, []SessionAgentCall]
	activeRequests *csync.Map[string, context.CancelFunc]
}

// SessionAgentOptions holds configuration options for a SessionAgent.
// The largeModel and smallModel are now resolved internally by NewSessionAgent
// based on overrides and ConfigService.
type SessionAgentOptions struct {
	SystemPromptPrefix   string
	SystemPrompt         string
	DisableAutoSummarize bool
	IsYolo               bool
	Sessions             session.Service
	Messages             message.Service
	Tools                []fantasy.AgentTool
	ConfigService        config.Service // New: for resolving provider and model configurations
}

// makeLanguageModelAndConfigs creates a Model struct (containing fantasy.LanguageModel, catwalk.Model, and config.SelectedModel)
// based on the provided provider and model names. If requestedProvider or requestedModel are empty,
// it falls back to the defaults defined in the config.Service for the given modelType.
func makeLanguageModelAndConfigs(
	cfgService config.Service,
	requestedProvider string,
	requestedModel string,
	modelType config.ModelType,
) (Model, error) {
	effectiveProviderName := requestedProvider
	effectiveModelName := requestedModel

	// If no provider is requested, get the default from config
	if effectiveProviderName == "" {
		switch modelType {
		case config.ModelTypeLarge:
			effectiveProviderName = cfgService.GetLargeModel().Provider
		case config.ModelTypeSmall:
			effectiveProviderName = cfgService.GetSmallModel().Provider
		}
	}

	// If no model is requested, get the default from config
	if effectiveModelName == "" {
		switch modelType {
		case config.ModelTypeLarge:
			effectiveModelName = cfgService.GetLargeModel().Model
		case config.ModelTypeSmall:
			effectiveModelName = cfgService.GetSmallModel().Model
		}
	}

	if effectiveProviderName == "" || effectiveModelName == "" {
		return Model{}, fmt.Errorf("effective provider or model name cannot be empty for %s model", modelType)
	}

	providerConfig, err := cfgService.GetProvider(effectiveProviderName)
	if err != nil {
		return Model{}, fmt.Errorf("failed to get provider config for '%s': %w", effectiveProviderName, err)
	}

	selectedModelConfig, err := cfgService.GetModel(effectiveProviderName, effectiveModelName)
	if err != nil {
		return Model{}, fmt.Errorf("failed to get model config for provider '%s' and model '%s': %w", effectiveProviderName, effectiveModelName, err)
	}

	var lm fantasy.LanguageModel
	var catwalkModel catwalk.Model

	switch providerConfig.Name {
	case openai.Name:
		lm, err = openai.NewClient(openai.Options{
			APIKey:  providerConfig.APIKey,
			Model:   selectedModelConfig.Model,
			BaseURL: providerConfig.BaseURL,
		})
		catwalkModel = catwalk.OpenAI
	case anthropic.Name:
		lm, err = anthropic.NewClient(anthropic.Options{
			APIKey:  providerConfig.APIKey,
			Model:   selectedModelConfig.Model,
			BaseURL: providerConfig.BaseURL,
		})
		catwalkModel = catwalk.Anthropic
	case google.Name:
		lm, err = google.NewClient(google.Options{
			APIKey:  providerConfig.APIKey,
			Model:   selectedModelConfig.Model,
			BaseURL: providerConfig.BaseURL,
		})
		catwalkModel = catwalk.Google
	case bedrock.Name:
		lm, err = bedrock.NewClient(bedrock.Options{
			Region: providerConfig.Region,
			Model:  selectedModelConfig.Model,
		})
		catwalkModel = catwalk.Bedrock
	case openrouter.Name:
		lm, err = openrouter.NewClient(openrouter.Options{
			APIKey:  providerConfig.APIKey,
			Model:   selectedModelConfig.Model,
			BaseURL: providerConfig.BaseURL,
		})
		catwalkModel = catwalk.OpenRouter
	default:
		return Model{}, fmt.Errorf("unsupported provider: %s", providerConfig.Name)
	}

	if err != nil {
		return Model{}, fmt.Errorf("failed to create %s language model for provider '%s', model '%s': %w", modelType, effectiveProviderName, effectiveModelName, err)
	}

	return Model{
		Model:      lm,
		CatwalkCfg: catwalkModel,
		ModelCfg:   selectedModelConfig,
	}, nil
}

// NewSessionAgent creates a new session agent, configuring the large and small language models.
// The large model is resolved using the effectiveProvider and effectiveModel which are determined
// from command-line flags, environment variables, or config file defaults.
// The small model is always resolved from the defaults specified in the config file.
func NewSessionAgent(
	opts SessionAgentOptions,
	effectiveProvider string, // Resolved provider name for the large model (CLI > ENV > Config default)
	effectiveModel string,   // Resolved model name for the large model (CLI > ENV > Config default)
) (SessionAgent, error) {
	// Resolve the large model based on overrides or config defaults
	largeModel, err := makeLanguageModelAndConfigs(
		opts.ConfigService,
		effectiveProvider,
		effectiveModel,
		config.ModelTypeLarge,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to configure large language model: %w", err)
	}

	// Resolve the small model using only the configured defaults.
	// We pass empty strings for provider/model to ensure the function uses the config defaults for the small model.
	smallModel, err := makeLanguageModelAndConfigs(
		opts.ConfigService,
		"", // Always use default provider for small model
		"", // Always use default model for small model
		config.ModelTypeSmall,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to configure default small language model: %w", err)
	}

	return &sessionAgent{
		largeModel:           largeModel,
		smallModel:           smallModel,
		systemPromptPrefix:   opts.SystemPromptPrefix,
		systemPrompt:         opts.SystemPrompt,
		sessions:             opts.Sessions,
		messages:             opts.Messages,
		disableAutoSummarize: opts.DisableAutoSummarize,
		tools:                opts.Tools,
		isYolo:               opts.IsYolo,
		messageQueue:         csync.NewMap[string, []SessionAgentCall](),
		activeRequests:       csync.NewMap[string, context.CancelFunc](),
	}, nil
}

func (a *sessionAgent) Run(ctx context.Context, call SessionAgentCall) (*fantasy.AgentResult, error) {
	if call.Prompt == "" {
		return nil, ErrEmptyPrompt
	}
	if call.SessionID == "" {
		return nil, ErrSessionMissing
	}

	// Queue the message if busy
	if a.IsSessionBusy(call.SessionID) {
		existing, ok := a.messageQueue.Get(call.SessionID)
		if !ok {
			existing = []SessionAgentCall{}
		}
		existing = append(existing, call)
		a.messageQueue.Set(call.SessionID, existing)
		return nil, nil
	}

	if len(a.tools) > 0 {
		// add anthropic caching to the last tool
		a.tools[len(a.tools)-1].SetProviderOptions(a.getCacheControlOptions())
	}

	agent := fantasy.NewAgent(
		a.largeModel.Model,
		fantasy.WithSystemPrompt(a.systemPrompt),
		fantasy.WithTools(a.tools...),
	)

	sessionLock := sync.Mutex{}
	currentSession, err := a.sessions.Get(ctx, call.SessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	msgs, err := a.getSessionMessages(ctx, currentSession)
	if err != nil {
		return nil, fmt.Errorf("failed to get session messages: %w", err)
	}

	var wg sync.WaitGroup
	// Generate title if first message
	if len(msgs) == 0 {
		wg.Go(func() {
			sessionLock.Lock()
			a.generateTitle(ctx, &currentSession, call.Prompt)
			sessionLock.Unlock()
		})
	}

	// Add the user message to the session
	_, err = a.createUserMessage(ctx, call)
	if err != nil {
		return nil, err
	}

	// add the session to the context
	ctx = context.WithValue(ctx, tools.SessionIDContextKey, call.SessionID)

	genCtx, cancel := context.WithCancel(ctx)
	a.activeRequests.Set(call.SessionID, cancel)

	defer cancel()
	defer a.activeRequests.Del(call.SessionID)

	history, files := a.preparePrompt(msgs, call.Attachments...)

	startTime := time.Now()
	a.eventPromptSent(call.SessionID)

	var currentAssistant *message.Message
	var shouldSummarize bool
	// NOTE: The remainder of this function is omitted as it was truncated and contained
	// a syntax error in the provided code sample. The core changes are in the
	// NewSessionAgent function above.
	return nil, nil // Placeholder return
}