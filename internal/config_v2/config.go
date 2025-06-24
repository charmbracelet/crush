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

	"github.com/charmbracelet/crush/internal/logging"
	"github.com/charmbracelet/fur/pkg/provider"
)

const (
	defaultDataDirectory = ".crush"
	defaultLogLevel      = "info"
	appName              = "crush"

	MaxTokensFallbackDefault = 4096
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
	BaseURL      string            `json:"base_url,omitempty"`
	ProviderType provider.Type     `json:"provider_type"`
	APIKey       string            `json:"api_key,omitempty"`
	Disabled     bool              `json:"disabled"`
	ExtraHeaders map[string]string `json:"extra_headers,omitempty"`
	// used for e.x for vertex to set the project
	ExtraParams map[string]string `json:"extra_params,omitempty"`

	DefaultModel string `json:"default_model"`
}

type Agent struct {
	Name string `json:"name"`
	// This is the id of the system prompt used by the agent
	//  TODO: still needs to be implemented
	PromptID string `json:"prompt_id"`
	Disabled bool   `json:"disabled"`

	Provider provider.InferenceProvider `json:"provider"`
	Model    Model                      `json:"model"`

	// The available tools for the agent
	//  if this is empty, all tools are available
	AllowedTools []string `json:"allowed_tools"`

	// this tells us which MCPs are available for this agent
	//  if this is empty all mcps are available
	//  the string array is the list of tools from the MCP the agent has available
	//  if the string array is empty, all tools from the MCP are available
	MCP map[string][]string `json:"mcp"`

	// The list of LSPs that this agent can use
	//  if this is empty, all LSPs are available
	LSP []string `json:"lsp"`

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
	Agents map[string]Agent `json:"agents,omitempty"`

	// List of configured MCPs
	MCP map[string]MCP `json:"mcp,omitempty"`

	// List of configured LSPs
	LSP map[string]LSPConfig `json:"lsp,omitempty"`

	// Miscellaneous options
	Options Options `json:"options"`

	// Used to add models that are not already in the repository
	Models map[provider.InferenceProvider][]provider.Model `json:"models,omitempty"`
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
	cfg.Options = mergeOptions(cfg.Options, globalCfg.Options)
	cfg.Options = mergeOptions(cfg.Options, localConfig.Options)

	mergeProviderConfigs(cfg, globalCfg, localConfig)
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

func mergeOptions(base, other Options) Options {
	result := base

	if len(other.ContextPaths) > 0 {
		base.ContextPaths = append(base.ContextPaths, other.ContextPaths...)
	}

	if other.TUI.CompactMode {
		result.TUI.CompactMode = other.TUI.CompactMode
	}

	if other.Debug {
		result.Debug = other.Debug
	}

	if other.DebugLSP {
		result.DebugLSP = other.DebugLSP
	}

	if other.DisableAutoSummarize {
		result.DisableAutoSummarize = other.DisableAutoSummarize
	}

	if other.DataDirectory != "" {
		result.DataDirectory = other.DataDirectory
	}

	return result
}

func mergeProviderConfigs(base, global, local *Config) {
	if global != nil {
		for providerName, globalProvider := range global.Providers {
			if _, ok := base.Providers[providerName]; !ok {
				base.Providers[providerName] = globalProvider
			} else {
				base.Providers[providerName] = mergeProviderConfig(providerName, base.Providers[providerName], globalProvider)
			}
		}
	}
	if local != nil {
		for providerName, localProvider := range local.Providers {
			if _, ok := base.Providers[providerName]; !ok {
				base.Providers[providerName] = localProvider
			} else {
				base.Providers[providerName] = mergeProviderConfig(providerName, base.Providers[providerName], localProvider)
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

func providerDefaultConfig(providerName provider.InferenceProvider) ProviderConfig {
	switch providerName {
	case provider.InferenceProviderAnthropic:
		return ProviderConfig{
			ProviderType: provider.TypeAnthropic,
		}
	case provider.InferenceProviderOpenAI:
		return ProviderConfig{
			ProviderType: provider.TypeOpenAI,
		}
	case provider.InferenceProviderGemini:
		return ProviderConfig{
			ProviderType: provider.TypeGemini,
		}
	case provider.InferenceProviderBedrock:
		return ProviderConfig{
			ProviderType: provider.TypeBedrock,
		}
	case provider.InferenceProviderAzure:
		return ProviderConfig{
			ProviderType: provider.TypeAzure,
		}
	case provider.InferenceProviderOpenRouter:
		return ProviderConfig{
			ProviderType: provider.TypeOpenAI,
			BaseURL:      "https://openrouter.ai/api/v1",
			ExtraHeaders: map[string]string{
				"HTTP-Referer": "crush.charm.land",
				"X-Title":      "Crush",
			},
		}
	case provider.InferenceProviderXAI:
		return ProviderConfig{
			ProviderType: provider.TypeXAI,
			BaseURL:      "https://api.x.ai/v1",
		}
	case provider.InferenceProviderVertexAI:
		return ProviderConfig{
			ProviderType: provider.TypeVertexAI,
		}
	default:
		return ProviderConfig{
			ProviderType: provider.TypeOpenAI,
		}
	}
}

func defaultConfigBasedOnEnv() *Config {
	cfg := &Config{
		Options: Options{
			DataDirectory: defaultDataDirectory,
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
				providerConfig.DefaultModel = p.DefaultModelID
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
