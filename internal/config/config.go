package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/charmbracelet/crush/internal/fur/provider"
	"github.com/charmbracelet/crush/internal/logging"
	"github.com/invopop/jsonschema"
)

const (
	defaultDataDirectory = ".crush"
	defaultLogLevel      = "info"
	appName              = "crush"

	MaxTokensFallbackDefault = 4096
)

var defaultContextPaths = []string{
	".github/copilot-instructions.md",
	".cursorrules",
	".cursor/rules/",
	"CLAUDE.md",
	"CLAUDE.local.md",
	"GEMINI.md",
	"gemini.md",
	"crush.md",
	"crush.local.md",
	"Crush.md",
	"Crush.local.md",
	"CRUSH.md",
	"CRUSH.local.md",
}

type AgentID string

const (
	AgentCoder AgentID = "coder"
	AgentTask  AgentID = "task"
)

type ModelType string

const (
	LargeModel ModelType = "large"
	SmallModel ModelType = "small"
)

type Model struct {
	ID                 string  `json:"id" jsonschema:"title=Model ID,description=Unique identifier for the model, the API model"`
	Name               string  `json:"name" jsonschema:"title=Model Name,description=Display name of the model"`
	CostPer1MIn        float64 `json:"cost_per_1m_in,omitempty" jsonschema:"title=Input Cost,description=Cost per 1 million input tokens,minimum=0"`
	CostPer1MOut       float64 `json:"cost_per_1m_out,omitempty" jsonschema:"title=Output Cost,description=Cost per 1 million output tokens,minimum=0"`
	CostPer1MInCached  float64 `json:"cost_per_1m_in_cached,omitempty" jsonschema:"title=Cached Input Cost,description=Cost per 1 million cached input tokens,minimum=0"`
	CostPer1MOutCached float64 `json:"cost_per_1m_out_cached,omitempty" jsonschema:"title=Cached Output Cost,description=Cost per 1 million cached output tokens,minimum=0"`
	ContextWindow      int64   `json:"context_window" jsonschema:"title=Context Window,description=Maximum context window size in tokens,minimum=1"`
	DefaultMaxTokens   int64   `json:"default_max_tokens" jsonschema:"title=Default Max Tokens,description=Default maximum tokens for responses,minimum=1"`
	CanReason          bool    `json:"can_reason,omitempty" jsonschema:"title=Can Reason,description=Whether the model supports reasoning capabilities"`
	ReasoningEffort    string  `json:"reasoning_effort,omitempty" jsonschema:"title=Reasoning Effort,description=Default reasoning effort level for reasoning models"`
	HasReasoningEffort bool    `json:"has_reasoning_effort,omitempty" jsonschema:"title=Has Reasoning Effort,description=Whether the model supports reasoning effort configuration"`
	SupportsImages     bool    `json:"supports_attachments,omitempty" jsonschema:"title=Supports Images,description=Whether the model supports image attachments"`
}

type VertexAIOptions struct {
	APIKey   string `json:"api_key,omitempty"`
	Project  string `json:"project,omitempty"`
	Location string `json:"location,omitempty"`
}

type ProviderConfig struct {
	ID           provider.InferenceProvider `json:"id,omitempty" jsonschema:"title=Provider ID,description=Unique identifier for the provider"`
	BaseURL      string                     `json:"base_url,omitempty" jsonschema:"title=Base URL,description=Base URL for the provider API (required for custom providers)"`
	ProviderType provider.Type              `json:"provider_type" jsonschema:"title=Provider Type,description=Type of the provider (openai, anthropic, etc.)"`
	APIKey       string                     `json:"api_key,omitempty" jsonschema:"title=API Key,description=API key for authenticating with the provider"`
	Disabled     bool                       `json:"disabled,omitempty" jsonschema:"title=Disabled,description=Whether this provider is disabled,default=false"`
	ExtraHeaders map[string]string          `json:"extra_headers,omitempty" jsonschema:"title=Extra Headers,description=Additional HTTP headers to send with requests"`
	// used for e.x for vertex to set the project
	ExtraParams map[string]string `json:"extra_params,omitempty" jsonschema:"title=Extra Parameters,description=Additional provider-specific parameters"`

	DefaultLargeModel string `json:"default_large_model,omitempty" jsonschema:"title=Default Large Model,description=Default model ID for large model type"`
	DefaultSmallModel string `json:"default_small_model,omitempty" jsonschema:"title=Default Small Model,description=Default model ID for small model type"`

	Models []Model `json:"models,omitempty" jsonschema:"title=Models,description=List of available models for this provider"`
}

type Agent struct {
	ID          AgentID `json:"id,omitempty" jsonschema:"title=Agent ID,description=Unique identifier for the agent,enum=coder,enum=task"`
	Name        string  `json:"name,omitempty" jsonschema:"title=Name,description=Display name of the agent"`
	Description string  `json:"description,omitempty" jsonschema:"title=Description,description=Description of what the agent does"`
	// This is the id of the system prompt used by the agent
	Disabled bool `json:"disabled,omitempty" jsonschema:"title=Disabled,description=Whether this agent is disabled,default=false"`

	Model ModelType `json:"model" jsonschema:"title=Model Type,description=Type of model to use (large or small),enum=large,enum=small"`

	// The available tools for the agent
	//  if this is nil, all tools are available
	AllowedTools []string `json:"allowed_tools,omitempty" jsonschema:"title=Allowed Tools,description=List of tools this agent is allowed to use (if nil all tools are allowed)"`

	// this tells us which MCPs are available for this agent
	//  if this is empty all mcps are available
	//  the string array is the list of tools from the AllowedMCP the agent has available
	//  if the string array is nil, all tools from the AllowedMCP are available
	AllowedMCP map[string][]string `json:"allowed_mcp,omitempty" jsonschema:"title=Allowed MCP,description=Map of MCP servers this agent can use and their allowed tools"`

	// The list of LSPs that this agent can use
	//  if this is nil, all LSPs are available
	AllowedLSP []string `json:"allowed_lsp,omitempty" jsonschema:"title=Allowed LSP,description=List of LSP servers this agent can use (if nil all LSPs are allowed)"`

	// Overrides the context paths for this agent
	ContextPaths []string `json:"context_paths,omitempty" jsonschema:"title=Context Paths,description=Custom context paths for this agent (additive to global context paths)"`
}

type MCPType string

const (
	MCPStdio MCPType = "stdio"
	MCPSse   MCPType = "sse"
	MCPHttp  MCPType = "http"
)

type MCP struct {
	Command string   `json:"command,omitempty" jsonschema:"title=Command,description=Command to execute for stdio MCP servers"`
	Env     []string `json:"env,omitempty" jsonschema:"title=Environment,description=Environment variables for the MCP server"`
	Args    []string `json:"args,omitempty" jsonschema:"title=Arguments,description=Command line arguments for the MCP server"`
	Type    MCPType  `json:"type" jsonschema:"title=Type,description=Type of MCP connection,enum=stdio,enum=sse,enum=http,default=stdio"`
	URL     string   `json:"url,omitempty" jsonschema:"title=URL,description=URL for SSE MCP servers"`
	// TODO: maybe make it possible to get the value from the env
	Headers map[string]string `json:"headers,omitempty" jsonschema:"title=Headers,description=HTTP headers for SSE MCP servers"`
}

type LSPConfig struct {
	Disabled bool     `json:"enabled,omitempty" jsonschema:"title=Enabled,description=Whether this LSP server is enabled,default=true"`
	Command  string   `json:"command" jsonschema:"title=Command,description=Command to execute for the LSP server"`
	Args     []string `json:"args,omitempty" jsonschema:"title=Arguments,description=Command line arguments for the LSP server"`
	Options  any      `json:"options,omitempty" jsonschema:"title=Options,description=LSP server specific options"`
}

type TUIOptions struct {
	CompactMode bool `json:"compact_mode" jsonschema:"title=Compact Mode,description=Enable compact mode for the TUI,default=false"`
	// Here we can add themes later or any TUI related options
}

type Options struct {
	ContextPaths         []string   `json:"context_paths,omitempty" jsonschema:"title=Context Paths,description=List of paths to search for context files"`
	TUI                  TUIOptions `json:"tui,omitempty" jsonschema:"title=TUI Options,description=Terminal UI configuration options"`
	Debug                bool       `json:"debug,omitempty" jsonschema:"title=Debug,description=Enable debug logging,default=false"`
	DebugLSP             bool       `json:"debug_lsp,omitempty" jsonschema:"title=Debug LSP,description=Enable LSP debug logging,default=false"`
	DisableAutoSummarize bool       `json:"disable_auto_summarize,omitempty" jsonschema:"title=Disable Auto Summarize,description=Disable automatic conversation summarization,default=false"`
	// Relative to the cwd
	DataDirectory string `json:"data_directory,omitempty" jsonschema:"title=Data Directory,description=Directory for storing application data,default=.crush"`
}

type PreferredModel struct {
	ModelID  string                     `json:"model_id" jsonschema:"title=Model ID,description=ID of the preferred model"`
	Provider provider.InferenceProvider `json:"provider" jsonschema:"title=Provider,description=Provider for the preferred model"`
	// ReasoningEffort overrides the default reasoning effort for this model
	ReasoningEffort string `json:"reasoning_effort,omitempty" jsonschema:"title=Reasoning Effort,description=Override reasoning effort for this model"`
	// MaxTokens overrides the default max tokens for this model
	MaxTokens int64 `json:"max_tokens,omitempty" jsonschema:"title=Max Tokens,description=Override max tokens for this model,minimum=1"`

	// Think indicates if the model should think, only applicable for anthropic reasoning models
	Think bool `json:"think,omitempty" jsonschema:"title=Think,description=Enable thinking for reasoning models,default=false"`
}

type PreferredModels struct {
	Large PreferredModel `json:"large,omitempty" jsonschema:"title=Large Model,description=Preferred model configuration for large model type"`
	Small PreferredModel `json:"small,omitempty" jsonschema:"title=Small Model,description=Preferred model configuration for small model type"`
}

type Config struct {
	Models PreferredModels `json:"models,omitempty" jsonschema:"title=Models,description=Preferred model configurations for large and small model types"`
	// List of configured providers
	Providers map[provider.InferenceProvider]ProviderConfig `json:"providers,omitempty" jsonschema:"title=Providers,description=LLM provider configurations"`

	// List of configured agents
	Agents map[AgentID]Agent `json:"agents,omitempty" jsonschema:"title=Agents,description=Agent configurations for different tasks"`

	// List of configured MCPs
	MCP map[string]MCP `json:"mcp,omitempty" jsonschema:"title=MCP,description=Model Control Protocol server configurations"`

	// List of configured LSPs
	LSP map[string]LSPConfig `json:"lsp,omitempty" jsonschema:"title=LSP,description=Language Server Protocol configurations"`

	// Miscellaneous options
	Options Options `json:"options,omitempty" jsonschema:"title=Options,description=General application options and settings"`
}

var (
	instance *Config // The single instance of the Singleton
	cwd      string
	once     sync.Once // Ensures the initialization happens only once

)

func readConfigFile(path string) (*Config, error) {
	var cfg *Config
	if _, err := os.Stat(path); err != nil && !os.IsNotExist(err) {
		// some other error occurred while checking the file
		return nil, err
	} else if err == nil {
		// config file exists, read it
		file, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		cfg = &Config{}
		if err := json.Unmarshal(file, cfg); err != nil {
			return nil, err
		}
	} else {
		// config file does not exist, create a new one
		cfg = &Config{}
	}
	return cfg, nil
}

func loadConfig(cwd string, debug bool) (*Config, error) {
	// First read the global config file
	cfgPath := ConfigPath()

	cfg := defaultConfigBasedOnEnv()
	cfg.Options.Debug = debug
	defaultLevel := slog.LevelInfo
	if cfg.Options.Debug {
		defaultLevel = slog.LevelDebug
	}
	if os.Getenv("CRUSH_DEV_DEBUG") == "true" {
		loggingFile := fmt.Sprintf("%s/%s", cfg.Options.DataDirectory, "debug.log")

		// if file does not exist create it
		if _, err := os.Stat(loggingFile); os.IsNotExist(err) {
			if err := os.MkdirAll(cfg.Options.DataDirectory, 0o755); err != nil {
				return cfg, fmt.Errorf("failed to create directory: %w", err)
			}
			if _, err := os.Create(loggingFile); err != nil {
				return cfg, fmt.Errorf("failed to create log file: %w", err)
			}
		}

		messagesPath := fmt.Sprintf("%s/%s", cfg.Options.DataDirectory, "messages")

		if _, err := os.Stat(messagesPath); os.IsNotExist(err) {
			if err := os.MkdirAll(messagesPath, 0o756); err != nil {
				return cfg, fmt.Errorf("failed to create directory: %w", err)
			}
		}
		logging.MessageDir = messagesPath

		sloggingFileWriter, err := os.OpenFile(loggingFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o666)
		if err != nil {
			return cfg, fmt.Errorf("failed to open log file: %w", err)
		}
		// Configure logger
		logger := slog.New(slog.NewTextHandler(sloggingFileWriter, &slog.HandlerOptions{
			Level: defaultLevel,
		}))
		slog.SetDefault(logger)
	} else {
		// Configure logger
		logger := slog.New(slog.NewTextHandler(logging.NewWriter(), &slog.HandlerOptions{
			Level: defaultLevel,
		}))
		slog.SetDefault(logger)
	}

	priorityOrderedConfigFiles := []string{
		cfgPath,                           // Global config file
		filepath.Join(cwd, "crush.json"),  // Local config file
		filepath.Join(cwd, ".crush.json"), // Local config file
	}

	configs := make([]*Config, 0)
	for _, path := range priorityOrderedConfigFiles {
		localConfig, err := readConfigFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
		}
		if localConfig != nil {
			// If the config file was read successfully, add it to the list
			configs = append(configs, localConfig)
		}
	}

	// merge options
	mergeOptions(cfg, configs...)

	mergeProviderConfigs(cfg, configs...)
	// no providers found the app is not initialized yet
	if len(cfg.Providers) == 0 {
		return cfg, nil
	}
	preferredProvider := getPreferredProvider(cfg.Providers)
	if preferredProvider != nil {
		cfg.Models = PreferredModels{
			Large: PreferredModel{
				ModelID:  preferredProvider.DefaultLargeModel,
				Provider: preferredProvider.ID,
			},
			Small: PreferredModel{
				ModelID:  preferredProvider.DefaultSmallModel,
				Provider: preferredProvider.ID,
			},
		}
	} else {
		// No valid providers found, set empty models
		cfg.Models = PreferredModels{}
	}

	mergeModels(cfg, configs...)

	agents := map[AgentID]Agent{
		AgentCoder: {
			ID:           AgentCoder,
			Name:         "Coder",
			Description:  "An agent that helps with executing coding tasks.",
			Model:        LargeModel,
			ContextPaths: cfg.Options.ContextPaths,
			// All tools allowed
		},
		AgentTask: {
			ID:           AgentTask,
			Name:         "Task",
			Description:  "An agent that helps with searching for context and finding implementation details.",
			Model:        LargeModel,
			ContextPaths: cfg.Options.ContextPaths,
			AllowedTools: []string{
				"glob",
				"grep",
				"ls",
				"sourcegraph",
				"view",
			},
			// NO MCPs or LSPs by default
			AllowedMCP: map[string][]string{},
			AllowedLSP: []string{},
		},
	}
	cfg.Agents = agents
	mergeAgents(cfg, configs...)
	mergeMCPs(cfg, configs...)
	mergeLSPs(cfg, configs...)

	// Validate the final configuration
	if err := cfg.Validate(); err != nil {
		return cfg, fmt.Errorf("configuration validation failed: %w", err)
	}

	return cfg, nil
}

func Init(workingDir string, debug bool) (*Config, error) {
	var err error
	once.Do(func() {
		cwd = workingDir
		instance, err = loadConfig(cwd, debug)
		if err != nil {
			logging.Error("Failed to load config", "error", err)
		}
	})

	return instance, err
}

func Get() *Config {
	if instance == nil {
		// TODO: Handle this better
		panic("Config not initialized. Call InitConfig first.")
	}
	return instance
}

func getPreferredProvider(configuredProviders map[provider.InferenceProvider]ProviderConfig) *ProviderConfig {
	providers := Providers()
	for _, p := range providers {
		if providerConfig, ok := configuredProviders[p.ID]; ok && !providerConfig.Disabled {
			return &providerConfig
		}
	}
	// if none found return the first configured provider
	for _, providerConfig := range configuredProviders {
		if !providerConfig.Disabled {
			return &providerConfig
		}
	}
	return nil
}

func mergeProviderConfig(p provider.InferenceProvider, base, other ProviderConfig) ProviderConfig {
	if other.APIKey != "" {
		base.APIKey = other.APIKey
	}
	// Only change these options if the provider is not a known provider
	if !slices.Contains(provider.KnownProviders(), p) {
		if other.BaseURL != "" {
			base.BaseURL = other.BaseURL
		}
		if other.ProviderType != "" {
			base.ProviderType = other.ProviderType
		}
		if len(other.ExtraHeaders) > 0 {
			if base.ExtraHeaders == nil {
				base.ExtraHeaders = make(map[string]string)
			}
			maps.Copy(base.ExtraHeaders, other.ExtraHeaders)
		}
		if len(other.ExtraParams) > 0 {
			if base.ExtraParams == nil {
				base.ExtraParams = make(map[string]string)
			}
			maps.Copy(base.ExtraParams, other.ExtraParams)
		}
	}

	if other.Disabled {
		base.Disabled = other.Disabled
	}

	if other.DefaultLargeModel != "" {
		base.DefaultLargeModel = other.DefaultLargeModel
	}
	// Add new models if they don't exist
	if other.Models != nil {
		for _, model := range other.Models {
			// check if the model already exists
			exists := false
			for _, existingModel := range base.Models {
				if existingModel.ID == model.ID {
					exists = true
					break
				}
			}
			if !exists {
				base.Models = append(base.Models, model)
			}
		}
	}

	return base
}

func validateProvider(p provider.InferenceProvider, providerConfig ProviderConfig) error {
	if !slices.Contains(provider.KnownProviders(), p) {
		if providerConfig.ProviderType != provider.TypeOpenAI {
			return errors.New("invalid provider type: " + string(providerConfig.ProviderType))
		}
		if providerConfig.BaseURL == "" {
			return errors.New("base URL must be set for custom providers")
		}
		if providerConfig.APIKey == "" {
			return errors.New("API key must be set for custom providers")
		}
	}
	return nil
}

func mergeModels(base *Config, others ...*Config) {
	for _, cfg := range others {
		if cfg == nil {
			continue
		}
		if cfg.Models.Large.ModelID != "" && cfg.Models.Large.Provider != "" {
			base.Models.Large = cfg.Models.Large
		}

		if cfg.Models.Small.ModelID != "" && cfg.Models.Small.Provider != "" {
			base.Models.Small = cfg.Models.Small
		}
	}
}

func mergeOptions(base *Config, others ...*Config) {
	for _, cfg := range others {
		if cfg == nil {
			continue
		}
		baseOptions := base.Options
		other := cfg.Options
		if len(other.ContextPaths) > 0 {
			baseOptions.ContextPaths = append(baseOptions.ContextPaths, other.ContextPaths...)
		}

		if other.TUI.CompactMode {
			baseOptions.TUI.CompactMode = other.TUI.CompactMode
		}

		if other.Debug {
			baseOptions.Debug = other.Debug
		}

		if other.DebugLSP {
			baseOptions.DebugLSP = other.DebugLSP
		}

		if other.DisableAutoSummarize {
			baseOptions.DisableAutoSummarize = other.DisableAutoSummarize
		}

		if other.DataDirectory != "" {
			baseOptions.DataDirectory = other.DataDirectory
		}
		base.Options = baseOptions
	}
}

func mergeAgents(base *Config, others ...*Config) {
	for _, cfg := range others {
		if cfg == nil {
			continue
		}
		for agentID, newAgent := range cfg.Agents {
			if _, ok := base.Agents[agentID]; !ok {
				newAgent.ID = agentID
				if newAgent.Model == "" {
					newAgent.Model = LargeModel
				}
				if len(newAgent.ContextPaths) > 0 {
					newAgent.ContextPaths = append(base.Options.ContextPaths, newAgent.ContextPaths...)
				} else {
					newAgent.ContextPaths = base.Options.ContextPaths
				}
				base.Agents[agentID] = newAgent
			} else {
				baseAgent := base.Agents[agentID]

				if agentID == AgentCoder || agentID == AgentTask {
					if newAgent.Model != "" {
						baseAgent.Model = newAgent.Model
					}
					if newAgent.AllowedMCP != nil {
						baseAgent.AllowedMCP = newAgent.AllowedMCP
					}
					if newAgent.AllowedLSP != nil {
						baseAgent.AllowedLSP = newAgent.AllowedLSP
					}
					// Context paths are additive for known agents too
					if len(newAgent.ContextPaths) > 0 {
						baseAgent.ContextPaths = append(baseAgent.ContextPaths, newAgent.ContextPaths...)
					}
				} else {
					if newAgent.Name != "" {
						baseAgent.Name = newAgent.Name
					}
					if newAgent.Description != "" {
						baseAgent.Description = newAgent.Description
					}
					if newAgent.Model != "" {
						baseAgent.Model = newAgent.Model
					} else if baseAgent.Model == "" {
						baseAgent.Model = LargeModel
					}

					baseAgent.Disabled = newAgent.Disabled

					if newAgent.AllowedTools != nil {
						baseAgent.AllowedTools = newAgent.AllowedTools
					}
					if newAgent.AllowedMCP != nil {
						baseAgent.AllowedMCP = newAgent.AllowedMCP
					}
					if newAgent.AllowedLSP != nil {
						baseAgent.AllowedLSP = newAgent.AllowedLSP
					}
					if len(newAgent.ContextPaths) > 0 {
						baseAgent.ContextPaths = append(baseAgent.ContextPaths, newAgent.ContextPaths...)
					}
				}

				base.Agents[agentID] = baseAgent
			}
		}
	}
}

func mergeMCPs(base *Config, others ...*Config) {
	for _, cfg := range others {
		if cfg == nil {
			continue
		}
		maps.Copy(base.MCP, cfg.MCP)
	}
}

func mergeLSPs(base *Config, others ...*Config) {
	for _, cfg := range others {
		if cfg == nil {
			continue
		}
		maps.Copy(base.LSP, cfg.LSP)
	}
}

func mergeProviderConfigs(base *Config, others ...*Config) {
	for _, cfg := range others {
		if cfg == nil {
			continue
		}
		for providerName, p := range cfg.Providers {
			p.ID = providerName
			if _, ok := base.Providers[providerName]; !ok {
				if slices.Contains(provider.KnownProviders(), providerName) {
					providers := Providers()
					for _, providerDef := range providers {
						if providerDef.ID == providerName {
							logging.Info("Using default provider config for", "provider", providerName)
							baseProvider := getDefaultProviderConfig(providerDef, providerDef.APIKey)
							base.Providers[providerName] = mergeProviderConfig(providerName, baseProvider, p)
							break
						}
					}
				} else {
					base.Providers[providerName] = p
				}
			} else {
				base.Providers[providerName] = mergeProviderConfig(providerName, base.Providers[providerName], p)
			}
		}
	}

	finalProviders := make(map[provider.InferenceProvider]ProviderConfig)
	for providerName, providerConfig := range base.Providers {
		err := validateProvider(providerName, providerConfig)
		if err != nil {
			logging.Warn("Skipping provider", "name", providerName, "error", err)
			continue // Skip invalid providers
		}
		finalProviders[providerName] = providerConfig
	}
	base.Providers = finalProviders
}

func providerDefaultConfig(providerID provider.InferenceProvider) ProviderConfig {
	switch providerID {
	case provider.InferenceProviderAnthropic:
		return ProviderConfig{
			ID:           providerID,
			ProviderType: provider.TypeAnthropic,
		}
	case provider.InferenceProviderOpenAI:
		return ProviderConfig{
			ID:           providerID,
			ProviderType: provider.TypeOpenAI,
		}
	case provider.InferenceProviderGemini:
		return ProviderConfig{
			ID:           providerID,
			ProviderType: provider.TypeGemini,
		}
	case provider.InferenceProviderBedrock:
		return ProviderConfig{
			ID:           providerID,
			ProviderType: provider.TypeBedrock,
		}
	case provider.InferenceProviderAzure:
		return ProviderConfig{
			ID:           providerID,
			ProviderType: provider.TypeAzure,
		}
	case provider.InferenceProviderOpenRouter:
		return ProviderConfig{
			ID:           providerID,
			ProviderType: provider.TypeOpenAI,
			BaseURL:      "https://openrouter.ai/api/v1",
			ExtraHeaders: map[string]string{
				"HTTP-Referer": "crush.charm.land",
				"X-Title":      "Crush",
			},
		}
	case provider.InferenceProviderXAI:
		return ProviderConfig{
			ID:           providerID,
			ProviderType: provider.TypeXAI,
			BaseURL:      "https://api.x.ai/v1",
		}
	case provider.InferenceProviderVertexAI:
		return ProviderConfig{
			ID:           providerID,
			ProviderType: provider.TypeVertexAI,
		}
	default:
		return ProviderConfig{
			ID:           providerID,
			ProviderType: provider.TypeOpenAI,
		}
	}
}

func getDefaultProviderConfig(p provider.Provider, apiKey string) ProviderConfig {
	providerConfig := providerDefaultConfig(p.ID)
	providerConfig.APIKey = apiKey
	providerConfig.DefaultLargeModel = p.DefaultLargeModelID
	providerConfig.DefaultSmallModel = p.DefaultSmallModelID
	baseURL := p.APIEndpoint
	if strings.HasPrefix(baseURL, "$") {
		envVar := strings.TrimPrefix(baseURL, "$")
		baseURL = os.Getenv(envVar)
	}
	providerConfig.BaseURL = baseURL
	for _, model := range p.Models {
		configModel := Model{
			ID:                 model.ID,
			Name:               model.Name,
			CostPer1MIn:        model.CostPer1MIn,
			CostPer1MOut:       model.CostPer1MOut,
			CostPer1MInCached:  model.CostPer1MInCached,
			CostPer1MOutCached: model.CostPer1MOutCached,
			ContextWindow:      model.ContextWindow,
			DefaultMaxTokens:   model.DefaultMaxTokens,
			CanReason:          model.CanReason,
			SupportsImages:     model.SupportsImages,
		}
		// Set reasoning effort for reasoning models
		if model.HasReasoningEffort && model.DefaultReasoningEffort != "" {
			configModel.HasReasoningEffort = model.HasReasoningEffort
			configModel.ReasoningEffort = model.DefaultReasoningEffort
		}
		providerConfig.Models = append(providerConfig.Models, configModel)
	}
	return providerConfig
}

func defaultConfigBasedOnEnv() *Config {
	cfg := &Config{
		Options: Options{
			DataDirectory: defaultDataDirectory,
			ContextPaths:  defaultContextPaths,
		},
		Providers: make(map[provider.InferenceProvider]ProviderConfig),
		Agents:    make(map[AgentID]Agent),
		LSP:       make(map[string]LSPConfig),
		MCP:       make(map[string]MCP),
	}

	providers := Providers()

	for _, p := range providers {
		if strings.HasPrefix(p.APIKey, "$") {
			envVar := strings.TrimPrefix(p.APIKey, "$")
			if apiKey := os.Getenv(envVar); apiKey != "" {
				cfg.Providers[p.ID] = getDefaultProviderConfig(p, apiKey)
			}
		}
	}
	// TODO: support local models

	if useVertexAI := os.Getenv("GOOGLE_GENAI_USE_VERTEXAI"); useVertexAI == "true" {
		providerConfig := providerDefaultConfig(provider.InferenceProviderVertexAI)
		providerConfig.ExtraParams = map[string]string{
			"project":  os.Getenv("GOOGLE_CLOUD_PROJECT"),
			"location": os.Getenv("GOOGLE_CLOUD_LOCATION"),
		}
		// Find the VertexAI provider definition to get default models
		for _, p := range providers {
			if p.ID == provider.InferenceProviderVertexAI {
				providerConfig.DefaultLargeModel = p.DefaultLargeModelID
				providerConfig.DefaultSmallModel = p.DefaultSmallModelID
				for _, model := range p.Models {
					configModel := Model{
						ID:                 model.ID,
						Name:               model.Name,
						CostPer1MIn:        model.CostPer1MIn,
						CostPer1MOut:       model.CostPer1MOut,
						CostPer1MInCached:  model.CostPer1MInCached,
						CostPer1MOutCached: model.CostPer1MOutCached,
						ContextWindow:      model.ContextWindow,
						DefaultMaxTokens:   model.DefaultMaxTokens,
						CanReason:          model.CanReason,
						SupportsImages:     model.SupportsImages,
					}
					// Set reasoning effort for reasoning models
					if model.HasReasoningEffort && model.DefaultReasoningEffort != "" {
						configModel.HasReasoningEffort = model.HasReasoningEffort
						configModel.ReasoningEffort = model.DefaultReasoningEffort
					}
					providerConfig.Models = append(providerConfig.Models, configModel)
				}
				break
			}
		}
		cfg.Providers[provider.InferenceProviderVertexAI] = providerConfig
	}

	if hasAWSCredentials() {
		providerConfig := providerDefaultConfig(provider.InferenceProviderBedrock)
		providerConfig.ExtraParams = map[string]string{
			"region": os.Getenv("AWS_DEFAULT_REGION"),
		}
		if providerConfig.ExtraParams["region"] == "" {
			providerConfig.ExtraParams["region"] = os.Getenv("AWS_REGION")
		}
		// Find the Bedrock provider definition to get default models
		for _, p := range providers {
			if p.ID == provider.InferenceProviderBedrock {
				providerConfig.DefaultLargeModel = p.DefaultLargeModelID
				providerConfig.DefaultSmallModel = p.DefaultSmallModelID
				for _, model := range p.Models {
					configModel := Model{
						ID:                 model.ID,
						Name:               model.Name,
						CostPer1MIn:        model.CostPer1MIn,
						CostPer1MOut:       model.CostPer1MOut,
						CostPer1MInCached:  model.CostPer1MInCached,
						CostPer1MOutCached: model.CostPer1MOutCached,
						ContextWindow:      model.ContextWindow,
						DefaultMaxTokens:   model.DefaultMaxTokens,
						CanReason:          model.CanReason,
						SupportsImages:     model.SupportsImages,
					}
					// Set reasoning effort for reasoning models
					if model.HasReasoningEffort && model.DefaultReasoningEffort != "" {
						configModel.HasReasoningEffort = model.HasReasoningEffort
						configModel.ReasoningEffort = model.DefaultReasoningEffort
					}
					providerConfig.Models = append(providerConfig.Models, configModel)
				}
				break
			}
		}
		cfg.Providers[provider.InferenceProviderBedrock] = providerConfig
	}
	return cfg
}

func hasAWSCredentials() bool {
	if os.Getenv("AWS_ACCESS_KEY_ID") != "" && os.Getenv("AWS_SECRET_ACCESS_KEY") != "" {
		return true
	}

	if os.Getenv("AWS_PROFILE") != "" || os.Getenv("AWS_DEFAULT_PROFILE") != "" {
		return true
	}

	if os.Getenv("AWS_REGION") != "" || os.Getenv("AWS_DEFAULT_REGION") != "" {
		return true
	}

	if os.Getenv("AWS_CONTAINER_CREDENTIALS_RELATIVE_URI") != "" ||
		os.Getenv("AWS_CONTAINER_CREDENTIALS_FULL_URI") != "" {
		return true
	}

	return false
}

func WorkingDirectory() string {
	return cwd
}

// TODO: Handle error state

func GetAgentModel(agentID AgentID) Model {
	cfg := Get()
	agent, ok := cfg.Agents[agentID]
	if !ok {
		logging.Error("Agent not found", "agent_id", agentID)
		return Model{}
	}

	var model PreferredModel
	switch agent.Model {
	case LargeModel:
		model = cfg.Models.Large
	case SmallModel:
		model = cfg.Models.Small
	default:
		logging.Warn("Unknown model type for agent", "agent_id", agentID, "model_type", agent.Model)
		model = cfg.Models.Large // Fallback to large model
	}
	providerConfig, ok := cfg.Providers[model.Provider]
	if !ok {
		logging.Error("Provider not found for agent", "agent_id", agentID, "provider", model.Provider)
		return Model{}
	}

	for _, m := range providerConfig.Models {
		if m.ID == model.ModelID {
			return m
		}
	}

	logging.Error("Model not found for agent", "agent_id", agentID, "model", agent.Model)
	return Model{}
}

func GetAgentProvider(agentID AgentID) ProviderConfig {
	cfg := Get()
	agent, ok := cfg.Agents[agentID]
	if !ok {
		logging.Error("Agent not found", "agent_id", agentID)
		return ProviderConfig{}
	}

	var model PreferredModel
	switch agent.Model {
	case LargeModel:
		model = cfg.Models.Large
	case SmallModel:
		model = cfg.Models.Small
	default:
		logging.Warn("Unknown model type for agent", "agent_id", agentID, "model_type", agent.Model)
		model = cfg.Models.Large // Fallback to large model
	}

	providerConfig, ok := cfg.Providers[model.Provider]
	if !ok {
		logging.Error("Provider not found for agent", "agent_id", agentID, "provider", model.Provider)
		return ProviderConfig{}
	}

	return providerConfig
}

func GetProviderModel(provider provider.InferenceProvider, modelID string) Model {
	cfg := Get()
	providerConfig, ok := cfg.Providers[provider]
	if !ok {
		logging.Error("Provider not found", "provider", provider)
		return Model{}
	}

	for _, model := range providerConfig.Models {
		if model.ID == modelID {
			return model
		}
	}

	logging.Error("Model not found for provider", "provider", provider, "model_id", modelID)
	return Model{}
}

func GetModel(modelType ModelType) Model {
	cfg := Get()
	var model PreferredModel
	switch modelType {
	case LargeModel:
		model = cfg.Models.Large
	case SmallModel:
		model = cfg.Models.Small
	default:
		model = cfg.Models.Large // Fallback to large model
	}
	providerConfig, ok := cfg.Providers[model.Provider]
	if !ok {
		return Model{}
	}

	for _, m := range providerConfig.Models {
		if m.ID == model.ModelID {
			return m
		}
	}
	return Model{}
}

func UpdatePreferredModel(modelType ModelType, model PreferredModel) error {
	cfg := Get()
	switch modelType {
	case LargeModel:
		cfg.Models.Large = model
	case SmallModel:
		cfg.Models.Small = model
	default:
		return fmt.Errorf("unknown model type: %s", modelType)
	}
	return nil
}

// ValidationError represents a configuration validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("validation error in %s: %s", e.Field, e.Message)
}

// ValidationErrors represents multiple validation errors
type ValidationErrors []ValidationError

func (e ValidationErrors) Error() string {
	if len(e) == 0 {
		return "no validation errors"
	}
	if len(e) == 1 {
		return e[0].Error()
	}

	var messages []string
	for _, err := range e {
		messages = append(messages, err.Error())
	}
	return fmt.Sprintf("multiple validation errors: %s", strings.Join(messages, "; "))
}

// HasErrors returns true if there are any validation errors
func (e ValidationErrors) HasErrors() bool {
	return len(e) > 0
}

// Add appends a new validation error
func (e *ValidationErrors) Add(field, message string) {
	*e = append(*e, ValidationError{Field: field, Message: message})
}

// Validate performs comprehensive validation of the configuration
func (c *Config) Validate() error {
	var errors ValidationErrors

	// Validate providers
	c.validateProviders(&errors)

	// Validate models
	c.validateModels(&errors)

	// Validate agents
	c.validateAgents(&errors)

	// Validate options
	c.validateOptions(&errors)

	// Validate MCP configurations
	c.validateMCPs(&errors)

	// Validate LSP configurations
	c.validateLSPs(&errors)

	// Validate cross-references
	c.validateCrossReferences(&errors)

	// Validate completeness
	c.validateCompleteness(&errors)

	if errors.HasErrors() {
		return errors
	}

	return nil
}

// validateProviders validates all provider configurations
func (c *Config) validateProviders(errors *ValidationErrors) {
	if c.Providers == nil {
		c.Providers = make(map[provider.InferenceProvider]ProviderConfig)
	}

	knownProviders := provider.KnownProviders()
	validTypes := []provider.Type{
		provider.TypeOpenAI,
		provider.TypeAnthropic,
		provider.TypeGemini,
		provider.TypeAzure,
		provider.TypeBedrock,
		provider.TypeVertexAI,
		provider.TypeXAI,
	}

	for providerID, providerConfig := range c.Providers {
		fieldPrefix := fmt.Sprintf("providers.%s", providerID)

		// Validate API key for non-disabled providers
		if !providerConfig.Disabled && providerConfig.APIKey == "" {
			// Special case for AWS Bedrock and VertexAI which may use other auth methods
			if providerID != provider.InferenceProviderBedrock && providerID != provider.InferenceProviderVertexAI {
				errors.Add(fieldPrefix+".api_key", "API key is required for non-disabled providers")
			}
		}

		// Validate provider type
		validType := slices.Contains(validTypes, providerConfig.ProviderType)
		if !validType {
			errors.Add(fieldPrefix+".provider_type", fmt.Sprintf("invalid provider type: %s", providerConfig.ProviderType))
		}

		// Validate custom providers
		isKnownProvider := slices.Contains(knownProviders, providerID)

		if !isKnownProvider {
			// Custom provider validation
			if providerConfig.BaseURL == "" {
				errors.Add(fieldPrefix+".base_url", "BaseURL is required for custom providers")
			}
			if providerConfig.ProviderType != provider.TypeOpenAI {
				errors.Add(fieldPrefix+".provider_type", "custom providers currently only support OpenAI type")
			}
		}

		// Validate models
		modelIDs := make(map[string]bool)
		for i, model := range providerConfig.Models {
			modelFieldPrefix := fmt.Sprintf("%s.models[%d]", fieldPrefix, i)

			// Check for duplicate model IDs
			if modelIDs[model.ID] {
				errors.Add(modelFieldPrefix+".id", fmt.Sprintf("duplicate model ID: %s", model.ID))
			}
			modelIDs[model.ID] = true

			// Validate required model fields
			if model.ID == "" {
				errors.Add(modelFieldPrefix+".id", "model ID is required")
			}
			if model.Name == "" {
				errors.Add(modelFieldPrefix+".name", "model name is required")
			}
			if model.ContextWindow <= 0 {
				errors.Add(modelFieldPrefix+".context_window", "context window must be positive")
			}
			if model.DefaultMaxTokens <= 0 {
				errors.Add(modelFieldPrefix+".default_max_tokens", "default max tokens must be positive")
			}
			if model.DefaultMaxTokens > model.ContextWindow {
				errors.Add(modelFieldPrefix+".default_max_tokens", "default max tokens cannot exceed context window")
			}

			// Validate cost fields
			if model.CostPer1MIn < 0 {
				errors.Add(modelFieldPrefix+".cost_per_1m_in", "cost per 1M input tokens cannot be negative")
			}
			if model.CostPer1MOut < 0 {
				errors.Add(modelFieldPrefix+".cost_per_1m_out", "cost per 1M output tokens cannot be negative")
			}
			if model.CostPer1MInCached < 0 {
				errors.Add(modelFieldPrefix+".cost_per_1m_in_cached", "cached cost per 1M input tokens cannot be negative")
			}
			if model.CostPer1MOutCached < 0 {
				errors.Add(modelFieldPrefix+".cost_per_1m_out_cached", "cached cost per 1M output tokens cannot be negative")
			}
		}

		// Validate default model references
		if providerConfig.DefaultLargeModel != "" {
			if !modelIDs[providerConfig.DefaultLargeModel] {
				errors.Add(fieldPrefix+".default_large_model", fmt.Sprintf("default large model '%s' not found in provider models", providerConfig.DefaultLargeModel))
			}
		}
		if providerConfig.DefaultSmallModel != "" {
			if !modelIDs[providerConfig.DefaultSmallModel] {
				errors.Add(fieldPrefix+".default_small_model", fmt.Sprintf("default small model '%s' not found in provider models", providerConfig.DefaultSmallModel))
			}
		}

		// Validate provider-specific requirements
		c.validateProviderSpecific(providerID, providerConfig, errors)
	}
}

// validateProviderSpecific validates provider-specific requirements
func (c *Config) validateProviderSpecific(providerID provider.InferenceProvider, providerConfig ProviderConfig, errors *ValidationErrors) {
	fieldPrefix := fmt.Sprintf("providers.%s", providerID)

	switch providerID {
	case provider.InferenceProviderVertexAI:
		if !providerConfig.Disabled {
			if providerConfig.ExtraParams == nil {
				errors.Add(fieldPrefix+".extra_params", "VertexAI requires extra_params configuration")
			} else {
				if providerConfig.ExtraParams["project"] == "" {
					errors.Add(fieldPrefix+".extra_params.project", "VertexAI requires project parameter")
				}
				if providerConfig.ExtraParams["location"] == "" {
					errors.Add(fieldPrefix+".extra_params.location", "VertexAI requires location parameter")
				}
			}
		}
	case provider.InferenceProviderBedrock:
		if !providerConfig.Disabled {
			if providerConfig.ExtraParams == nil || providerConfig.ExtraParams["region"] == "" {
				errors.Add(fieldPrefix+".extra_params.region", "Bedrock requires region parameter")
			}
			// Check for AWS credentials in environment
			if !hasAWSCredentials() {
				errors.Add(fieldPrefix, "Bedrock requires AWS credentials in environment")
			}
		}
	}
}

// validateModels validates preferred model configurations
func (c *Config) validateModels(errors *ValidationErrors) {
	// Validate large model
	if c.Models.Large.ModelID != "" || c.Models.Large.Provider != "" {
		if c.Models.Large.ModelID == "" {
			errors.Add("models.large.model_id", "large model ID is required when provider is set")
		}
		if c.Models.Large.Provider == "" {
			errors.Add("models.large.provider", "large model provider is required when model ID is set")
		}

		// Check if provider exists and is not disabled
		if providerConfig, exists := c.Providers[c.Models.Large.Provider]; exists {
			if providerConfig.Disabled {
				errors.Add("models.large.provider", "large model provider is disabled")
			}

			// Check if model exists in provider
			modelExists := false
			for _, model := range providerConfig.Models {
				if model.ID == c.Models.Large.ModelID {
					modelExists = true
					break
				}
			}
			if !modelExists {
				errors.Add("models.large.model_id", fmt.Sprintf("large model '%s' not found in provider '%s'", c.Models.Large.ModelID, c.Models.Large.Provider))
			}
		} else {
			errors.Add("models.large.provider", fmt.Sprintf("large model provider '%s' not found", c.Models.Large.Provider))
		}
	}

	// Validate small model
	if c.Models.Small.ModelID != "" || c.Models.Small.Provider != "" {
		if c.Models.Small.ModelID == "" {
			errors.Add("models.small.model_id", "small model ID is required when provider is set")
		}
		if c.Models.Small.Provider == "" {
			errors.Add("models.small.provider", "small model provider is required when model ID is set")
		}

		// Check if provider exists and is not disabled
		if providerConfig, exists := c.Providers[c.Models.Small.Provider]; exists {
			if providerConfig.Disabled {
				errors.Add("models.small.provider", "small model provider is disabled")
			}

			// Check if model exists in provider
			modelExists := false
			for _, model := range providerConfig.Models {
				if model.ID == c.Models.Small.ModelID {
					modelExists = true
					break
				}
			}
			if !modelExists {
				errors.Add("models.small.model_id", fmt.Sprintf("small model '%s' not found in provider '%s'", c.Models.Small.ModelID, c.Models.Small.Provider))
			}
		} else {
			errors.Add("models.small.provider", fmt.Sprintf("small model provider '%s' not found", c.Models.Small.Provider))
		}
	}
}

// validateAgents validates agent configurations
func (c *Config) validateAgents(errors *ValidationErrors) {
	if c.Agents == nil {
		c.Agents = make(map[AgentID]Agent)
	}

	validTools := []string{
		"bash", "edit", "fetch", "glob", "grep", "ls", "sourcegraph", "view", "write", "agent",
	}

	for agentID, agent := range c.Agents {
		fieldPrefix := fmt.Sprintf("agents.%s", agentID)

		// Validate agent ID consistency
		if agent.ID != agentID {
			errors.Add(fieldPrefix+".id", fmt.Sprintf("agent ID mismatch: expected '%s', got '%s'", agentID, agent.ID))
		}

		// Validate required fields
		if agent.ID == "" {
			errors.Add(fieldPrefix+".id", "agent ID is required")
		}
		if agent.Name == "" {
			errors.Add(fieldPrefix+".name", "agent name is required")
		}

		// Validate model type
		if agent.Model != LargeModel && agent.Model != SmallModel {
			errors.Add(fieldPrefix+".model", fmt.Sprintf("invalid model type: %s (must be 'large' or 'small')", agent.Model))
		}

		// Validate allowed tools
		if agent.AllowedTools != nil {
			for i, tool := range agent.AllowedTools {
				validTool := slices.Contains(validTools, tool)
				if !validTool {
					errors.Add(fmt.Sprintf("%s.allowed_tools[%d]", fieldPrefix, i), fmt.Sprintf("unknown tool: %s", tool))
				}
			}
		}

		// Validate MCP references
		if agent.AllowedMCP != nil {
			for mcpName := range agent.AllowedMCP {
				if _, exists := c.MCP[mcpName]; !exists {
					errors.Add(fieldPrefix+".allowed_mcp", fmt.Sprintf("referenced MCP '%s' not found", mcpName))
				}
			}
		}

		// Validate LSP references
		if agent.AllowedLSP != nil {
			for _, lspName := range agent.AllowedLSP {
				if _, exists := c.LSP[lspName]; !exists {
					errors.Add(fieldPrefix+".allowed_lsp", fmt.Sprintf("referenced LSP '%s' not found", lspName))
				}
			}
		}

		// Validate context paths (basic path validation)
		for i, contextPath := range agent.ContextPaths {
			if contextPath == "" {
				errors.Add(fmt.Sprintf("%s.context_paths[%d]", fieldPrefix, i), "context path cannot be empty")
			}
			// Check for invalid characters in path
			if strings.Contains(contextPath, "\x00") {
				errors.Add(fmt.Sprintf("%s.context_paths[%d]", fieldPrefix, i), "context path contains invalid characters")
			}
		}

		// Validate known agents maintain their core properties
		if agentID == AgentCoder {
			if agent.Name != "Coder" {
				errors.Add(fieldPrefix+".name", "coder agent name cannot be changed")
			}
			if agent.Description != "An agent that helps with executing coding tasks." {
				errors.Add(fieldPrefix+".description", "coder agent description cannot be changed")
			}
		} else if agentID == AgentTask {
			if agent.Name != "Task" {
				errors.Add(fieldPrefix+".name", "task agent name cannot be changed")
			}
			if agent.Description != "An agent that helps with searching for context and finding implementation details." {
				errors.Add(fieldPrefix+".description", "task agent description cannot be changed")
			}
			expectedTools := []string{"glob", "grep", "ls", "sourcegraph", "view"}
			if agent.AllowedTools != nil && !slices.Equal(agent.AllowedTools, expectedTools) {
				errors.Add(fieldPrefix+".allowed_tools", "task agent allowed tools cannot be changed")
			}
		}
	}
}

// validateOptions validates configuration options
func (c *Config) validateOptions(errors *ValidationErrors) {
	// Validate data directory
	if c.Options.DataDirectory == "" {
		errors.Add("options.data_directory", "data directory is required")
	}

	// Validate context paths
	for i, contextPath := range c.Options.ContextPaths {
		if contextPath == "" {
			errors.Add(fmt.Sprintf("options.context_paths[%d]", i), "context path cannot be empty")
		}
		if strings.Contains(contextPath, "\x00") {
			errors.Add(fmt.Sprintf("options.context_paths[%d]", i), "context path contains invalid characters")
		}
	}
}

// validateMCPs validates MCP configurations
func (c *Config) validateMCPs(errors *ValidationErrors) {
	if c.MCP == nil {
		c.MCP = make(map[string]MCP)
	}

	for mcpName, mcpConfig := range c.MCP {
		fieldPrefix := fmt.Sprintf("mcp.%s", mcpName)

		// Validate MCP type
		if mcpConfig.Type != MCPStdio && mcpConfig.Type != MCPSse && mcpConfig.Type != MCPHttp {
			errors.Add(fieldPrefix+".type", fmt.Sprintf("invalid MCP type: %s (must be 'stdio' or 'sse' or 'http')", mcpConfig.Type))
		}

		// Validate based on type
		if mcpConfig.Type == MCPStdio {
			if mcpConfig.Command == "" {
				errors.Add(fieldPrefix+".command", "command is required for stdio MCP")
			}
		} else if mcpConfig.Type == MCPSse {
			if mcpConfig.URL == "" {
				errors.Add(fieldPrefix+".url", "URL is required for SSE MCP")
			}
		}
	}
}

// validateLSPs validates LSP configurations
func (c *Config) validateLSPs(errors *ValidationErrors) {
	if c.LSP == nil {
		c.LSP = make(map[string]LSPConfig)
	}

	for lspName, lspConfig := range c.LSP {
		fieldPrefix := fmt.Sprintf("lsp.%s", lspName)

		if lspConfig.Command == "" {
			errors.Add(fieldPrefix+".command", "command is required for LSP")
		}
	}
}

// validateCrossReferences validates cross-references between different config sections
func (c *Config) validateCrossReferences(errors *ValidationErrors) {
	// Validate that agents can use their assigned model types
	for agentID, agent := range c.Agents {
		fieldPrefix := fmt.Sprintf("agents.%s", agentID)

		var preferredModel PreferredModel
		switch agent.Model {
		case LargeModel:
			preferredModel = c.Models.Large
		case SmallModel:
			preferredModel = c.Models.Small
		}

		if preferredModel.Provider != "" {
			if providerConfig, exists := c.Providers[preferredModel.Provider]; exists {
				if providerConfig.Disabled {
					errors.Add(fieldPrefix+".model", fmt.Sprintf("agent cannot use model type '%s' because provider '%s' is disabled", agent.Model, preferredModel.Provider))
				}
			}
		}
	}
}

// validateCompleteness validates that the configuration is complete and usable
func (c *Config) validateCompleteness(errors *ValidationErrors) {
	// Check for at least one valid, non-disabled provider
	hasValidProvider := false
	for _, providerConfig := range c.Providers {
		if !providerConfig.Disabled {
			hasValidProvider = true
			break
		}
	}
	if !hasValidProvider {
		errors.Add("providers", "at least one non-disabled provider is required")
	}

	// Check that default agents exist
	if _, exists := c.Agents[AgentCoder]; !exists {
		errors.Add("agents", "coder agent is required")
	}
	if _, exists := c.Agents[AgentTask]; !exists {
		errors.Add("agents", "task agent is required")
	}

	// Check that preferred models are set if providers exist
	if hasValidProvider {
		if c.Models.Large.ModelID == "" || c.Models.Large.Provider == "" {
			errors.Add("models.large", "large preferred model must be configured when providers are available")
		}
		if c.Models.Small.ModelID == "" || c.Models.Small.Provider == "" {
			errors.Add("models.small", "small preferred model must be configured when providers are available")
		}
	}
}

// JSONSchemaExtend adds custom schema properties for AgentID
func (AgentID) JSONSchemaExtend(schema *jsonschema.Schema) {
	schema.Enum = []any{
		string(AgentCoder),
		string(AgentTask),
	}
}

// JSONSchemaExtend adds custom schema properties for ModelType
func (ModelType) JSONSchemaExtend(schema *jsonschema.Schema) {
	schema.Enum = []any{
		string(LargeModel),
		string(SmallModel),
	}
}

// JSONSchemaExtend adds custom schema properties for MCPType
func (MCPType) JSONSchemaExtend(schema *jsonschema.Schema) {
	schema.Enum = []any{
		string(MCPStdio),
		string(MCPSse),
	}
}
