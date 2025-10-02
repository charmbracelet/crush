package agent

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/charmbracelet/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/agent/prompt"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/history"
	"github.com/charmbracelet/crush/internal/log"
	"github.com/charmbracelet/crush/internal/lsp"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/fantasy/ai"
	"github.com/charmbracelet/fantasy/anthropic"
	"github.com/charmbracelet/fantasy/google"
	"github.com/charmbracelet/fantasy/openai"
	"github.com/charmbracelet/fantasy/openaicompat"
	"github.com/charmbracelet/fantasy/openrouter"
)

type Coordinator interface {
	// INFO: (kujtim) this is not used yet we will use this when we have multiple agents
	// SetMainAgent(string)
	Run(ctx context.Context, sessionID, prompt string, attachments ...message.Attachment) (*ai.AgentResult, error)
	Cancel(sessionID string)
	CancelAll()
	IsSessionBusy(sessionID string) bool
	IsBusy() bool
	QueuedPrompts(sessionID string) int
	ClearQueue(sessionID string)
	Summarize(context.Context, string) error
	Model() Model
	UpdateModels() error
}

type coordinator struct {
	cfg         *config.Config
	sessions    session.Service
	messages    message.Service
	permissions permission.Service
	history     history.Service
	lspClients  *csync.Map[string, *lsp.Client]

	currentAgent SessionAgent
	agents       map[string]SessionAgent
}

func NewCoordinator(
	cfg *config.Config,
	sessions session.Service,
	messages message.Service,
	permissions permission.Service,
	history history.Service,
	lspClients *csync.Map[string, *lsp.Client],
) (Coordinator, error) {
	c := &coordinator{
		cfg:         cfg,
		sessions:    sessions,
		messages:    messages,
		permissions: permissions,
		history:     history,
		lspClients:  lspClients,
		agents:      make(map[string]SessionAgent),
	}

	agentCfg, ok := cfg.Agents[config.AgentCoder]
	if !ok {
		return nil, errors.New("coder agent not configured")
	}

	// TODO: make this dynamic when we support multiple agents
	prompt, err := coderPrompt(prompt.WithWorkingDir(c.cfg.WorkingDir()))
	if err != nil {
		return nil, err
	}

	agent, err := c.buildAgent(prompt, agentCfg)
	if err != nil {
		return nil, err
	}
	c.currentAgent = agent
	c.agents[config.AgentCoder] = agent
	return c, nil
}

// Run implements Coordinator.
func (c *coordinator) Run(ctx context.Context, sessionID string, prompt string, attachments ...message.Attachment) (*ai.AgentResult, error) {
	model := c.currentAgent.Model()
	maxTokens := model.CatwalkCfg.DefaultMaxTokens
	if model.ModelCfg.MaxTokens != 0 {
		maxTokens = model.ModelCfg.MaxTokens
	}

	if !model.CatwalkCfg.SupportsImages && attachments != nil {
		attachments = nil
	}

	return c.currentAgent.Run(ctx, SessionAgentCall{
		SessionID:        sessionID,
		Prompt:           prompt,
		Attachments:      attachments,
		MaxOutputTokens:  maxTokens,
		ProviderOptions:  c.getProviderOptions(model),
		Temperature:      model.ModelCfg.Temperature,
		TopP:             model.ModelCfg.TopP,
		TopK:             model.ModelCfg.TopK,
		FrequencyPenalty: model.ModelCfg.FrequencyPenalty,
		PresencePenalty:  model.ModelCfg.PresencePenalty,
	})
}

func (c *coordinator) getProviderOptions(model Model) ai.ProviderOptions {
	options := ai.ProviderOptions{}

	switch model.Model.Provider() {
	case openai.Name:
		parsed, err := openai.ParseOptions(model.ModelCfg.ProviderOptions)
		if err == nil {
			options[openai.Name] = parsed
		}
	case anthropic.Name:
		parsed, err := anthropic.ParseOptions(model.ModelCfg.ProviderOptions)
		if err == nil {
			options[anthropic.Name] = parsed
		}
	case openrouter.Name:
		parsed, err := openrouter.ParseOptions(model.ModelCfg.ProviderOptions)
		if err == nil {
			options[openrouter.Name] = parsed
		}
	case google.Name:
		parsed, err := google.ParseOptions(model.ModelCfg.ProviderOptions)
		if err == nil {
			options[google.Name] = parsed
		}
	case openaicompat.Name:
		parsed, err := openaicompat.ParseOptions(model.ModelCfg.ProviderOptions)
		if err == nil {
			options[openaicompat.Name] = parsed
		}
	}

	return options
}

func (c *coordinator) buildAgent(prompt *prompt.Prompt, agent config.Agent) (SessionAgent, error) {
	large, small, err := c.buildAgentModels()
	if err != nil {
		return nil, err
	}

	systemPrompt, err := prompt.Build(large.Model.Provider(), large.Model.Model(), *c.cfg)
	if err != nil {
		return nil, err
	}

	tools, err := c.buildTools(agent)
	if err != nil {
		return nil, err
	}
	return NewSessionAgent(large, small, systemPrompt, c.sessions, c.messages, tools...), nil
}

func (c *coordinator) buildTools(agent config.Agent) ([]ai.AgentTool, error) {
	var allTools []ai.AgentTool
	if slices.Contains(agent.AllowedTools, AgentToolName) {
		agentTool, err := c.agentTool()
		if err != nil {
			return nil, err
		}
		allTools = append(allTools, agentTool)
	}

	allTools = append(allTools,
		tools.NewBashTool(c.permissions, c.cfg.WorkingDir(), c.cfg.Options.Attribution),
		tools.NewDownloadTool(c.permissions, c.cfg.WorkingDir(), nil),
		tools.NewEditTool(c.lspClients, c.permissions, c.history, c.cfg.WorkingDir()),
		tools.NewMultiEditTool(c.lspClients, c.permissions, c.history, c.cfg.WorkingDir()),
		tools.NewFetchTool(c.permissions, c.cfg.WorkingDir(), nil),
		tools.NewGlobTool(c.cfg.WorkingDir()),
		tools.NewGrepTool(c.cfg.WorkingDir()),
		tools.NewLsTool(c.permissions, c.cfg.WorkingDir(), c.cfg.Tools.Ls),
		tools.NewSourcegraphTool(nil),
		tools.NewViewTool(c.lspClients, c.permissions, c.cfg.WorkingDir()),
		tools.NewWriteTool(c.lspClients, c.permissions, c.history, c.cfg.WorkingDir()),
	)

	var filteredTools []ai.AgentTool
	for _, tool := range allTools {
		if slices.Contains(agent.AllowedTools, tool.Info().Name) {
			filteredTools = append(filteredTools, tool)
		}
	}

	mcpTools := tools.GetMCPTools(context.Background(), c.permissions, c.cfg)

	for _, mcpTool := range mcpTools {
		if agent.AllowedMCP == nil {
			// No MCP restrictions
			filteredTools = append(filteredTools, mcpTool)
		} else if len(agent.AllowedMCP) == 0 {
			// no mcps allowed
			break
		}

		for mcp, tools := range agent.AllowedMCP {
			if mcp == mcpTool.MCP() {
				if len(tools) == 0 {
					filteredTools = append(filteredTools, mcpTool)
				}
				for _, t := range tools {
					if t == mcpTool.MCPToolName() {
						filteredTools = append(filteredTools, mcpTool)
					}
				}
				break
			}
		}
	}

	return filteredTools, nil
}

// TODO: when we support multiple agents we need to change this so that we pass in the agent specific model config
func (c *coordinator) buildAgentModels() (Model, Model, error) {
	largeModelCfg, ok := c.cfg.Models[config.SelectedModelTypeLarge]
	if !ok {
		return Model{}, Model{}, errors.New("large model not selected")
	}
	smallModelCfg, ok := c.cfg.Models[config.SelectedModelTypeSmall]
	if !ok {
		return Model{}, Model{}, errors.New("small model not selected")
	}

	largeProviderCfg, ok := c.cfg.Providers.Get(largeModelCfg.Provider)
	if !ok {
		return Model{}, Model{}, errors.New("large model provider not configured")
	}

	largeProvider, err := c.buildProvider(largeProviderCfg, largeModelCfg)
	if err != nil {
		return Model{}, Model{}, err
	}

	smallProviderCfg, ok := c.cfg.Providers.Get(smallModelCfg.Provider)
	if !ok {
		return Model{}, Model{}, errors.New("large model provider not configured")
	}

	smallProvider, err := c.buildProvider(smallProviderCfg, largeModelCfg)
	if err != nil {
		return Model{}, Model{}, err
	}

	var largeCatwalkModel *catwalk.Model
	var smallCatwalkModel *catwalk.Model

	for _, m := range largeProviderCfg.Models {
		if m.ID == largeModelCfg.Model {
			largeCatwalkModel = &m
		}
	}
	for _, m := range smallProviderCfg.Models {
		if m.ID == smallModelCfg.Model {
			smallCatwalkModel = &m
		}
	}

	if largeCatwalkModel == nil {
		return Model{}, Model{}, errors.New("large model not found in provider config")
	}

	if smallCatwalkModel == nil {
		return Model{}, Model{}, errors.New("snall model not found in provider config")
	}

	largeModel, err := largeProvider.LanguageModel(largeModelCfg.Model)
	if err != nil {
		return Model{}, Model{}, err
	}
	smallModel, err := smallProvider.LanguageModel(smallModelCfg.Model)
	if err != nil {
		return Model{}, Model{}, err
	}

	return Model{
			Model:      largeModel,
			CatwalkCfg: *largeCatwalkModel,
			ModelCfg:   largeModelCfg,
		}, Model{
			Model:      smallModel,
			CatwalkCfg: *smallCatwalkModel,
			ModelCfg:   smallModelCfg,
		}, nil
}

func (c *coordinator) buildAnthropicProvider(baseURL, apiKey string, headers map[string]string) ai.Provider {
	hasBearerAuth := false
	for key := range headers {
		if strings.ToLower(key) == "authorization" {
			hasBearerAuth = true
			break
		}
	}
	if hasBearerAuth {
		apiKey = "" // clear apiKey to avoid using X-Api-Key header
	}

	var opts []anthropic.Option

	if apiKey != "" {
		// Use standard X-Api-Key header
		opts = append(opts, anthropic.WithAPIKey(apiKey))
	}

	if len(headers) > 0 {
		opts = append(opts, anthropic.WithHeaders(headers))
	}

	if baseURL != "" {
		opts = append(opts, anthropic.WithBaseURL(baseURL))
	}

	if c.cfg.Options.Debug {
		httpClient := log.NewHTTPClient()
		opts = append(opts, anthropic.WithHTTPClient(httpClient))
	}

	return anthropic.New(opts...)
}

func (c *coordinator) buildOpenaiProvider(baseURL, apiKey string, headers map[string]string) ai.Provider {
	opts := []openai.Option{
		openai.WithAPIKey(apiKey),
	}
	if c.cfg.Options.Debug {
		httpClient := log.NewHTTPClient()
		opts = append(opts, openai.WithHTTPClient(httpClient))
	}
	if len(headers) > 0 {
		opts = append(opts, openai.WithHeaders(headers))
	}
	if baseURL != "" {
		opts = append(opts, openai.WithBaseURL(baseURL))
	}
	return openai.New(opts...)
}

func (c *coordinator) buildOpenrouterProvider(_, apiKey string, headers map[string]string) ai.Provider {
	opts := []openrouter.Option{
		openrouter.WithAPIKey(apiKey),
		openrouter.WithLanguageUniqueToolCallIds(),
	}
	if c.cfg.Options.Debug {
		httpClient := log.NewHTTPClient()
		opts = append(opts, openrouter.WithHTTPClient(httpClient))
	}
	if len(headers) > 0 {
		opts = append(opts, openrouter.WithHeaders(headers))
	}
	return openrouter.New(opts...)
}

func (c *coordinator) buildOpenaiCompatProvider(baseURL, apiKey string, headers map[string]string) ai.Provider {
	opts := []openaicompat.Option{
		openaicompat.WithAPIKey(apiKey),
	}
	if c.cfg.Options.Debug {
		httpClient := log.NewHTTPClient()
		opts = append(opts, openaicompat.WithHTTPClient(httpClient))
	}
	if len(headers) > 0 {
		opts = append(opts, openaicompat.WithHeaders(headers))
	}

	return openaicompat.New(baseURL, opts...)
}

// TODO: add baseURL for google
func (c *coordinator) buildGoogleProvider(baseURL, apiKey string, headers map[string]string) ai.Provider {
	opts := []google.Option{
		google.WithAPIKey(apiKey),
	}
	if c.cfg.Options.Debug {
		httpClient := log.NewHTTPClient()
		opts = append(opts, google.WithHTTPClient(httpClient))
	}
	if len(headers) > 0 {
		opts = append(opts, google.WithHeaders(headers))
	}
	return google.New(opts...)
}

func (c *coordinator) isAnthropicThinking(model config.SelectedModel) bool {
	if model.Think {
		return true
	}

	if model.ProviderOptions == nil {
		return false
	}

	opts, err := anthropic.ParseOptions(model.ProviderOptions)
	if err != nil {
		return false
	}
	if opts.Thinking != nil {
		return true
	}
	return false
}

func (c *coordinator) buildProvider(providerCfg config.ProviderConfig, model config.SelectedModel) (ai.Provider, error) {
	headers := providerCfg.ExtraHeaders

	// handle special headers for anthropic
	if providerCfg.Type == anthropic.Name && c.isAnthropicThinking(model) {
		headers["anthropic-beta"] = "interleaved-thinking-2025-05-14"
	}

	// TODO: make sure we have
	apiKey, _ := c.cfg.Resolve(providerCfg.APIKey)
	baseURL, _ := c.cfg.Resolve(providerCfg.BaseURL)
	var provider ai.Provider
	switch providerCfg.Type {
	case openai.Name:
		provider = c.buildOpenaiProvider(baseURL, apiKey, headers)
	case anthropic.Name:
		provider = c.buildAnthropicProvider(baseURL, apiKey, headers)
	case openrouter.Name:
		provider = c.buildOpenrouterProvider(baseURL, apiKey, headers)
	case google.Name:
		provider = c.buildGoogleProvider(baseURL, apiKey, headers)
	case openaicompat.Name:
		provider = c.buildOpenaiCompatProvider(baseURL, apiKey, headers)
	default:
		return nil, errors.New("provider type not supported")
	}
	return provider, nil
}

func (c *coordinator) Cancel(sessionID string) {
	c.currentAgent.Cancel(sessionID)
}

func (c *coordinator) CancelAll() {
	c.currentAgent.CancelAll()
}

func (c *coordinator) ClearQueue(sessionID string) {
	c.currentAgent.ClearQueue(sessionID)
}

func (c *coordinator) IsBusy() bool {
	return c.currentAgent.IsBusy()
}

func (c *coordinator) IsSessionBusy(sessionID string) bool {
	return c.currentAgent.IsSessionBusy(sessionID)
}

func (c *coordinator) Model() Model {
	return c.currentAgent.Model()
}

func (c *coordinator) UpdateModels() error {
	// build the models again so we make sure we get the latest config
	large, small, err := c.buildAgentModels()
	if err != nil {
		return err
	}
	c.currentAgent.SetModels(large, small)

	agentCfg, ok := c.cfg.Agents[config.AgentCoder]
	if !ok {
		return errors.New("coder agent not configured")
	}

	tools, err := c.buildTools(agentCfg)
	if err != nil {
		return err
	}
	c.currentAgent.SetTools(tools)
	return nil
}

func (c *coordinator) QueuedPrompts(sessionID string) int {
	return c.currentAgent.QueuedPrompts(sessionID)
}

func (c *coordinator) Summarize(ctx context.Context, sessionID string) error {
	return c.currentAgent.Summarize(ctx, sessionID)
}
