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
	ID                 string  `json:"id"`
	Name               string  `json:"model"`
	CostPer1MIn        float64 `json:"cost_per_1m_in"`
	CostPer1MOut       float64 `json:"cost_per_1m_out"`
	CostPer1MInCached  float64 `json:"cost_per_1m_in_cached"`
	CostPer1MOutCached float64 `json:"cost_per_1m_out_cached"`
	ContextWindow      int64   `json:"context_window"`
	DefaultMaxTokens   int64   `json:"default_max_tokens"`
	CanReason          bool    `json:"can_reason"`
	ReasoningEffort    string  `json:"reasoning_effort"`
	HasReasoningEffort bool    `json:"has_reasoning_effort"`
	SupportsImages     bool    `json:"supports_attachments"`
}

type VertexAIOptions struct {
	APIKey   string `json:"api_key,omitempty"`
	Project  string `json:"project,omitempty"`
	Location string `json:"location,omitempty"`
}

type ProviderConfig struct {
	ID           provider.InferenceProvider `json:"id"`
	BaseURL      string                     `json:"base_url,omitempty"`
	ProviderType provider.Type              `json:"provider_type"`
	APIKey       string                     `json:"api_key,omitempty"`
	Disabled     bool                       `json:"disabled"`
	ExtraHeaders map[string]string          `json:"extra_headers,omitempty"`
	// used for e.x for vertex to set the project
	ExtraParams map[string]string `json:"extra_params,omitempty"`

	DefaultLargeModel string `json:"default_large_model,omitempty"`
	DefaultSmallModel string `json:"default_small_model,omitempty"`

	Models []Model `json:"models,omitempty"`
}

type Agent struct {
	ID          AgentID `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description,omitempty"`
	// This is the id of the system prompt used by the agent
	Disabled bool `json:"disabled"`

	Model ModelType `json:"model"`

	// The available tools for the agent
	//  if this is nil, all tools are available
	AllowedTools []string `json:"allowed_tools"`

	// this tells us which MCPs are available for this agent
	//  if this is empty all mcps are available
	//  the string array is the list of tools from the AllowedMCP the agent has available
	//  if the string array is nil, all tools from the AllowedMCP are available
	AllowedMCP map[string][]string `json:"allowed_mcp"`

	// The list of LSPs that this agent can use
	//  if this is nil, all LSPs are available
	AllowedLSP []string `json:"allowed_lsp"`

	// Overrides the context paths for this agent
	ContextPaths []string `json:"context_paths"`
}

type MCPType string

const (
	MCPStdio MCPType = "stdio"
	MCPSse   MCPType = "sse"
)

type MCP struct {
	Command string            `json:"command"`
	Env     []string          `json:"env"`
	Args    []string          `json:"args"`
	Type    MCPType           `json:"type"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
}

type LSPConfig struct {
	Disabled bool     `json:"enabled"`
	Command  string   `json:"command"`
	Args     []string `json:"args"`
	Options  any      `json:"options"`
}

type TUIOptions struct {
	CompactMode bool `json:"compact_mode"`
	// Here we can add themes later or any TUI related options
}

type Options struct {
	ContextPaths         []string   `json:"context_paths"`
	TUI                  TUIOptions `json:"tui"`
	Debug                bool       `json:"debug"`
	DebugLSP             bool       `json:"debug_lsp"`
	DisableAutoSummarize bool       `json:"disable_auto_summarize"`
	// Relative to the cwd
	DataDirectory string `json:"data_directory"`
}

type PreferredModel struct {
	ModelID  string                     `json:"model_id"`
	Provider provider.InferenceProvider `json:"provider"`
	// ReasoningEffort overrides the default reasoning effort for this model
	ReasoningEffort string `json:"reasoning_effort,omitempty"`
	// MaxTokens overrides the default max tokens for this model
	MaxTokens int64 `json:"max_tokens,omitempty"`

	// Think indicates if the model should think, only applicable for anthropic reasoning models
	Think bool `json:"think,omitempty"`
}

type PreferredModels struct {
	Large PreferredModel `json:"large"`
	Small PreferredModel `json:"small"`
}

type Config struct {
	Models PreferredModels `json:"models"`
	// List of configured providers
	Providers map[provider.InferenceProvider]ProviderConfig `json:"providers,omitempty"`

	// List of configured agents
	Agents map[AgentID]Agent `json:"agents,omitempty"`

	// List of configured MCPs
	MCP map[string]MCP `json:"mcp,omitempty"`

	// List of configured LSPs
	LSP map[string]LSPConfig `json:"lsp,omitempty"`

	// Miscellaneous options
	Options Options `json:"options"`
}

var (
	instance *Config // The single instance of the Singleton
	cwd      string
	once     sync.Once // Ensures the initialization happens only once

)

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
	var globalCfg *Config
	if _, err := os.Stat(cfgPath); err != nil && !os.IsNotExist(err) {
		// some other error occurred while checking the file
		return nil, err
	} else if err == nil {
		// config file exists, read it
		file, err := os.ReadFile(cfgPath)
		if err != nil {
			return nil, err
		}
		globalCfg = &Config{}
		if err := json.Unmarshal(file, globalCfg); err != nil {
			return nil, err
		}
	} else {
		// config file does not exist, create a new one
		globalCfg = &Config{}
	}

	var localConfig *Config
	// Global config loaded, now read the local config file
	localConfigPath := filepath.Join(cwd, "crush.json")
	if _, err := os.Stat(localConfigPath); err != nil && !os.IsNotExist(err) {
		// some other error occurred while checking the file
		return nil, err
	} else if err == nil {
		// local config file exists, read it
		file, err := os.ReadFile(localConfigPath)
		if err != nil {
			return nil, err
		}
		localConfig = &Config{}
		if err := json.Unmarshal(file, localConfig); err != nil {
			return nil, err
		}
	}

	// merge options
	mergeOptions(cfg, globalCfg, localConfig)

	mergeProviderConfigs(cfg, globalCfg, localConfig)
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

	mergeModels(cfg, globalCfg, localConfig)

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
	mergeAgents(cfg, globalCfg, localConfig)
	mergeMCPs(cfg, globalCfg, localConfig)
	mergeLSPs(cfg, globalCfg, localConfig)

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

func mergeModels(base, global, local *Config) {
	for _, cfg := range []*Config{global, local} {
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

func mergeOptions(base, global, local *Config) {
	for _, cfg := range []*Config{global, local} {
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

func mergeAgents(base, global, local *Config) {
	for _, cfg := range []*Config{global, local} {
		if cfg == nil {
			continue
		}
		for agentID, newAgent := range cfg.Agents {
			if _, ok := base.Agents[agentID]; !ok {
				// New agent - apply defaults
				newAgent.ID = agentID // Ensure the ID is set correctly
				if newAgent.Model == "" {
					newAgent.Model = LargeModel // Default model type
				}
				// Context paths are always additive - start with global, then add custom
				if len(newAgent.ContextPaths) > 0 {
					newAgent.ContextPaths = append(base.Options.ContextPaths, newAgent.ContextPaths...)
				} else {
					newAgent.ContextPaths = base.Options.ContextPaths // Use global context paths only
				}
				base.Agents[agentID] = newAgent
			} else {
				baseAgent := base.Agents[agentID]

				// Special handling for known agents - only allow model changes
				if agentID == AgentCoder || agentID == AgentTask {
					if newAgent.Model != "" {
						baseAgent.Model = newAgent.Model
					}
					// For known agents, only allow MCP and LSP configuration
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
					// Custom agents - allow full merging
					if newAgent.Name != "" {
						baseAgent.Name = newAgent.Name
					}
					if newAgent.Description != "" {
						baseAgent.Description = newAgent.Description
					}
					if newAgent.Model != "" {
						baseAgent.Model = newAgent.Model
					} else if baseAgent.Model == "" {
						baseAgent.Model = LargeModel // Default fallback
					}

					// Boolean fields - always update (including false values)
					baseAgent.Disabled = newAgent.Disabled

					// Slice/Map fields - update if provided (including empty slices/maps)
					if newAgent.AllowedTools != nil {
						baseAgent.AllowedTools = newAgent.AllowedTools
					}
					if newAgent.AllowedMCP != nil {
						baseAgent.AllowedMCP = newAgent.AllowedMCP
					}
					if newAgent.AllowedLSP != nil {
						baseAgent.AllowedLSP = newAgent.AllowedLSP
					}
					// Context paths are additive for custom agents too
					if len(newAgent.ContextPaths) > 0 {
						baseAgent.ContextPaths = append(baseAgent.ContextPaths, newAgent.ContextPaths...)
					}
				}

				base.Agents[agentID] = baseAgent
			}
		}
	}
}

func mergeMCPs(base, global, local *Config) {
	for _, cfg := range []*Config{global, local} {
		if cfg == nil {
			continue
		}
		maps.Copy(base.MCP, cfg.MCP)
	}
}

func mergeLSPs(base, global, local *Config) {
	for _, cfg := range []*Config{global, local} {
		if cfg == nil {
			continue
		}
		maps.Copy(base.LSP, cfg.LSP)
	}
}

func mergeProviderConfigs(base, global, local *Config) {
	for _, cfg := range []*Config{global, local} {
		if cfg == nil {
			continue
		}
		for providerName, p := range cfg.Providers {
			if _, ok := base.Providers[providerName]; !ok {
				base.Providers[providerName] = p
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

func providerDefaultConfig(providerId provider.InferenceProvider) ProviderConfig {
	switch providerId {
	case provider.InferenceProviderAnthropic:
		return ProviderConfig{
			ID:           providerId,
			ProviderType: provider.TypeAnthropic,
		}
	case provider.InferenceProviderOpenAI:
		return ProviderConfig{
			ID:           providerId,
			ProviderType: provider.TypeOpenAI,
		}
	case provider.InferenceProviderGemini:
		return ProviderConfig{
			ID:           providerId,
			ProviderType: provider.TypeGemini,
		}
	case provider.InferenceProviderBedrock:
		return ProviderConfig{
			ID:           providerId,
			ProviderType: provider.TypeBedrock,
		}
	case provider.InferenceProviderAzure:
		return ProviderConfig{
			ID:           providerId,
			ProviderType: provider.TypeAzure,
		}
	case provider.InferenceProviderOpenRouter:
		return ProviderConfig{
			ID:           providerId,
			ProviderType: provider.TypeOpenAI,
			BaseURL:      "https://openrouter.ai/api/v1",
			ExtraHeaders: map[string]string{
				"HTTP-Referer": "crush.charm.land",
				"X-Title":      "Crush",
			},
		}
	case provider.InferenceProviderXAI:
		return ProviderConfig{
			ID:           providerId,
			ProviderType: provider.TypeXAI,
			BaseURL:      "https://api.x.ai/v1",
		}
	case provider.InferenceProviderVertexAI:
		return ProviderConfig{
			ID:           providerId,
			ProviderType: provider.TypeVertexAI,
		}
	default:
		return ProviderConfig{
			ID:           providerId,
			ProviderType: provider.TypeOpenAI,
		}
	}
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
				cfg.Providers[p.ID] = providerConfig
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
		if mcpConfig.Type != MCPStdio && mcpConfig.Type != MCPSse {
			errors.Add(fieldPrefix+".type", fmt.Sprintf("invalid MCP type: %s (must be 'stdio' or 'sse')", mcpConfig.Type))
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
