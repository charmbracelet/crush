package configv2

import (
	"encoding/json"
	"errors"
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
	"crush.md",
	"crush.local.md",
	"Crush.md",
	"Crush.local.md",
	"CRUSH.md",
	"CRUSH.local.md",
}

type AgentID string

const (
	AgentCoder     AgentID = "coder"
	AgentTask      AgentID = "task"
	AgentTitle     AgentID = "title"
	AgentSummarize AgentID = "summarize"
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

	Provider provider.InferenceProvider `json:"provider"`
	Model    string                     `json:"model"`

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

type Config struct {
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

func loadConfig(cwd string) (*Config, error) {
	// First read the global config file
	cfgPath := ConfigPath()

	cfg := defaultConfigBasedOnEnv()

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

	if preferredProvider == nil {
		return nil, errors.New("no valid providers configured")
	}

	agents := map[AgentID]Agent{
		AgentCoder: {
			ID:           AgentCoder,
			Name:         "Coder",
			Description:  "An agent that helps with executing coding tasks.",
			Provider:     preferredProvider.ID,
			Model:        preferredProvider.DefaultLargeModel,
			ContextPaths: cfg.Options.ContextPaths,
			// All tools allowed
		},
		AgentTask: {
			ID:           AgentTask,
			Name:         "Task",
			Description:  "An agent that helps with searching for context and finding implementation details.",
			Provider:     preferredProvider.ID,
			Model:        preferredProvider.DefaultLargeModel,
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
		AgentTitle: {
			ID:           AgentTitle,
			Name:         "Title",
			Description:  "An agent that helps with generating titles for sessions.",
			Provider:     preferredProvider.ID,
			Model:        preferredProvider.DefaultSmallModel,
			ContextPaths: cfg.Options.ContextPaths,
			AllowedTools: []string{},
			// NO MCPs or LSPs by default
			AllowedMCP: map[string][]string{},
			AllowedLSP: []string{},
		},
		AgentSummarize: {
			ID:           AgentSummarize,
			Name:         "Summarize",
			Description:  "An agent that helps with summarizing sessions.",
			Provider:     preferredProvider.ID,
			Model:        preferredProvider.DefaultSmallModel,
			ContextPaths: cfg.Options.ContextPaths,
			AllowedTools: []string{},
			// NO MCPs or LSPs by default
			AllowedMCP: map[string][]string{},
			AllowedLSP: []string{},
		},
	}
	cfg.Agents = agents
	mergeAgents(cfg, globalCfg, localConfig)
	mergeMCPs(cfg, globalCfg, localConfig)
	mergeLSPs(cfg, globalCfg, localConfig)

	return cfg, nil
}

func InitConfig(workingDir string) *Config {
	once.Do(func() {
		cwd = workingDir
		cfg, err := loadConfig(cwd)
		if err != nil {
			// TODO: Handle this better
			panic("Failed to load config: " + err.Error())
		}
		instance = cfg
	})

	return instance
}

func GetConfig() *Config {
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
		if len(base.ExtraHeaders) > 0 {
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
				newAgent.ID = agentID // Ensure the ID is set correctly
				base.Agents[agentID] = newAgent
			} else {
				switch agentID {
				case AgentCoder:
					baseAgent := base.Agents[agentID]
					baseAgent.Model = newAgent.Model
					baseAgent.Provider = newAgent.Provider
					baseAgent.AllowedMCP = newAgent.AllowedMCP
					baseAgent.AllowedLSP = newAgent.AllowedLSP
					base.Agents[agentID] = baseAgent
				case AgentTask:
					baseAgent := base.Agents[agentID]
					baseAgent.Model = newAgent.Model
					baseAgent.Provider = newAgent.Provider
					base.Agents[agentID] = baseAgent
				case AgentTitle:
					baseAgent := base.Agents[agentID]
					baseAgent.Model = newAgent.Model
					baseAgent.Provider = newAgent.Provider
					base.Agents[agentID] = baseAgent
				case AgentSummarize:
					baseAgent := base.Agents[agentID]
					baseAgent.Model = newAgent.Model
					baseAgent.Provider = newAgent.Provider
					base.Agents[agentID] = baseAgent
				default:
					baseAgent := base.Agents[agentID]
					baseAgent.Name = newAgent.Name
					baseAgent.Description = newAgent.Description
					baseAgent.Disabled = newAgent.Disabled
					baseAgent.Provider = newAgent.Provider
					baseAgent.Model = newAgent.Model
					baseAgent.AllowedTools = newAgent.AllowedTools
					baseAgent.AllowedMCP = newAgent.AllowedMCP
					baseAgent.AllowedLSP = newAgent.AllowedLSP
					base.Agents[agentID] = baseAgent

				}
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
		for providerName, globalProvider := range cfg.Providers {
			if _, ok := base.Providers[providerName]; !ok {
				base.Providers[providerName] = globalProvider
			} else {
				base.Providers[providerName] = mergeProviderConfig(providerName, base.Providers[providerName], globalProvider)
			}
		}
	}

	finalProviders := make(map[provider.InferenceProvider]ProviderConfig)
	for providerName, providerConfig := range base.Providers {
		err := validateProvider(providerName, providerConfig)
		if err != nil {
			logging.Warn("Skipping provider", "name", providerName, "error", err)
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
				for _, model := range p.Models {
					providerConfig.Models = append(providerConfig.Models, Model{
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
					})
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
		cfg.Providers[provider.InferenceProviderVertexAI] = providerConfig
	}

	if hasAWSCredentials() {
		providerConfig := providerDefaultConfig(provider.InferenceProviderBedrock)
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
