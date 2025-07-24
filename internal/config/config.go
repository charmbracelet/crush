package config

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/charmbracelet/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/env"
	"github.com/tidwall/sjson"
	"golang.org/x/exp/slog"
)

const (
	appName              = "crush"
	defaultDataDirectory = ".crush"
	defaultLogLevel      = "info"
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

type SelectedModelType string

const (
	SelectedModelTypeLarge SelectedModelType = "large"
	SelectedModelTypeSmall SelectedModelType = "small"
)

type SelectedModel struct {
	// The model id as used by the provider API.
	// Required.
	Model string `json:"model"`
	// The model provider, same as the key/id used in the providers config.
	// Required.
	Provider string `json:"provider"`

	// Only used by models that use the openai provider and need this set.
	ReasoningEffort string `json:"reasoning_effort,omitempty"`

	// Overrides the default model configuration.
	MaxTokens int64 `json:"max_tokens,omitempty"`

	// Used by anthropic models that can reason to indicate if the model should think.
	Think bool `json:"think,omitempty"`
}

type ProviderConfig struct {
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

	// Extra headers to send with each request to the provider.
	ExtraHeaders map[string]string `json:"extra_headers,omitempty"`
	// Extra body
	ExtraBody map[string]any `json:"extra_body,omitempty"`

	// Used to pass extra parameters to the provider.
	ExtraParams map[string]string `json:"-"`

	// The provider models
	Models []catwalk.Model `json:"models,omitempty"`
}

type MCPType string

const (
	MCPStdio MCPType = "stdio"
	MCPSse   MCPType = "sse"
	MCPHttp  MCPType = "http"
)

type MCPConfig struct {
	Command  string            `json:"command,omitempty" `
	Env      map[string]string `json:"env,omitempty"`
	Args     []string          `json:"args,omitempty"`
	Type     MCPType           `json:"type"`
	URL      string            `json:"url,omitempty"`
	Disabled bool              `json:"disabled,omitempty"`

	// TODO: maybe make it possible to get the value from the env
	Headers map[string]string `json:"headers,omitempty"`
}

type LSPConfig struct {
	Disabled bool     `json:"enabled,omitempty"`
	Command  string   `json:"command"`
	Args     []string `json:"args,omitempty"`
	Options  any      `json:"options,omitempty"`
}

type TUIOptions struct {
	CompactMode bool `json:"compact_mode,omitempty"`
	// Here we can add themes later or any TUI related options
}

type Permissions struct {
	AllowedTools []string `json:"allowed_tools,omitempty"` // Tools that don't require permission prompts
	SkipRequests bool     `json:"-"`                       // Automatically accept all permissions (YOLO mode)
}

type Options struct {
	ContextPaths         []string    `json:"context_paths,omitempty"`
	TUI                  *TUIOptions `json:"tui,omitempty"`
	Debug                bool        `json:"debug,omitempty"`
	DebugLSP             bool        `json:"debug_lsp,omitempty"`
	DisableAutoSummarize bool        `json:"disable_auto_summarize,omitempty"`
	DataDirectory        string      `json:"data_directory,omitempty"` // Relative to the cwd
}

type MCPs map[string]MCPConfig

type MCP struct {
	Name string    `json:"name"`
	MCP  MCPConfig `json:"mcp"`
}

func (m MCPs) Sorted() []MCP {
	sorted := make([]MCP, 0, len(m))
	for k, v := range m {
		sorted = append(sorted, MCP{
			Name: k,
			MCP:  v,
		})
	}
	slices.SortFunc(sorted, func(a, b MCP) int {
		return strings.Compare(a.Name, b.Name)
	})
	return sorted
}

type LSPs map[string]LSPConfig

type LSP struct {
	Name string    `json:"name"`
	LSP  LSPConfig `json:"lsp"`
}

func (l LSPs) Sorted() []LSP {
	sorted := make([]LSP, 0, len(l))
	for k, v := range l {
		sorted = append(sorted, LSP{
			Name: k,
			LSP:  v,
		})
	}
	slices.SortFunc(sorted, func(a, b LSP) int {
		return strings.Compare(a.Name, b.Name)
	})
	return sorted
}

func (m MCPConfig) ResolvedEnv() []string {
	resolver := NewShellVariableResolver(env.New())
	for e, v := range m.Env {
		var err error
		m.Env[e], err = resolver.ResolveValue(v)
		if err != nil {
			slog.Error("error resolving environment variable", "error", err, "variable", e, "value", v)
			continue
		}
	}

	env := make([]string, 0, len(m.Env))
	for k, v := range m.Env {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	return env
}

func (m MCPConfig) ResolvedHeaders() map[string]string {
	resolver := NewShellVariableResolver(env.New())
	for e, v := range m.Headers {
		var err error
		m.Headers[e], err = resolver.ResolveValue(v)
		if err != nil {
			slog.Error("error resolving header variable", "error", err, "variable", e, "value", v)
			continue
		}
	}
	return m.Headers
}

type Agent struct {
	ID          string `json:"id,omitempty"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	// This is the id of the system prompt used by the agent
	Disabled bool `json:"disabled,omitempty"`

	Model SelectedModelType `json:"model"`

	// The available tools for the agent
	//  if this is nil, all tools are available
	AllowedTools []string `json:"allowed_tools,omitempty"`

	// this tells us which MCPs are available for this agent
	//  if this is empty all mcps are available
	//  the string array is the list of tools from the AllowedMCP the agent has available
	//  if the string array is nil, all tools from the AllowedMCP are available
	AllowedMCP map[string][]string `json:"allowed_mcp,omitempty"`

	// The list of LSPs that this agent can use
	//  if this is nil, all LSPs are available
	AllowedLSP []string `json:"allowed_lsp,omitempty"`

	// Overrides the context paths for this agent
	ContextPaths []string `json:"context_paths,omitempty"`
}

// Config holds the configuration for crush.
type Config struct {
	// We currently only support large/small as values here.
	Models map[SelectedModelType]SelectedModel `json:"models,omitempty"`

	// The providers that are configured
	Providers map[string]ProviderConfig `json:"providers,omitempty"`

	MCP MCPs `json:"mcp,omitempty"`

	LSP LSPs `json:"lsp,omitempty"`

	Options *Options `json:"options,omitempty"`

	Permissions *Permissions `json:"permissions,omitempty"`

	// Internal
	workingDir string `json:"-"`
	// TODO: most likely remove this concept when I come back to it
	Agents map[string]Agent `json:"-"`
	// TODO: find a better way to do this this should probably not be part of the config
	resolver       VariableResolver
	dataConfigDir  string             `json:"-"`
	knownProviders []catwalk.Provider `json:"-"`
}

func (c *Config) WorkingDir() string {
	return c.workingDir
}

func (c *Config) EnabledProviders() []ProviderConfig {
	enabled := make([]ProviderConfig, 0, len(c.Providers))
	for _, p := range c.Providers {
		if !p.Disable {
			enabled = append(enabled, p)
		}
	}
	return enabled
}

// IsConfigured  return true if at least one provider is configured
func (c *Config) IsConfigured() bool {
	return len(c.EnabledProviders()) > 0
}

func (c *Config) GetModel(provider, model string) *catwalk.Model {
	if providerConfig, ok := c.Providers[provider]; ok {
		for _, m := range providerConfig.Models {
			if m.ID == model {
				return &m
			}
		}
	}
	return nil
}

func (c *Config) GetProviderForModel(modelType SelectedModelType) *ProviderConfig {
	model, ok := c.Models[modelType]
	if !ok {
		return nil
	}
	if providerConfig, ok := c.Providers[model.Provider]; ok {
		return &providerConfig
	}
	return nil
}

func (c *Config) GetModelByType(modelType SelectedModelType) *catwalk.Model {
	model, ok := c.Models[modelType]
	if !ok {
		return nil
	}
	return c.GetModel(model.Provider, model.Model)
}

func (c *Config) LargeModel() *catwalk.Model {
	model, ok := c.Models[SelectedModelTypeLarge]
	if !ok {
		return nil
	}
	return c.GetModel(model.Provider, model.Model)
}

func (c *Config) SmallModel() *catwalk.Model {
	model, ok := c.Models[SelectedModelTypeSmall]
	if !ok {
		return nil
	}
	return c.GetModel(model.Provider, model.Model)
}

func (c *Config) SetCompactMode(enabled bool) error {
	if c.Options == nil {
		c.Options = &Options{}
	}
	c.Options.TUI.CompactMode = enabled
	return c.SetConfigField("options.tui.compact_mode", enabled)
}

func (c *Config) Resolve(key string) (string, error) {
	if c.resolver == nil {
		return "", fmt.Errorf("no variable resolver configured")
	}
	return c.resolver.ResolveValue(key)
}

func (c *Config) UpdatePreferredModel(modelType SelectedModelType, model SelectedModel) error {
	c.Models[modelType] = model
	if err := c.SetConfigField(fmt.Sprintf("models.%s", modelType), model); err != nil {
		return fmt.Errorf("failed to update preferred model: %w", err)
	}
	return nil
}

func (c *Config) SetConfigField(key string, value any) error {
	// read the data
	data, err := os.ReadFile(c.dataConfigDir)
	if err != nil {
		if os.IsNotExist(err) {
			data = []byte("{}")
		} else {
			return fmt.Errorf("failed to read config file: %w", err)
		}
	}

	newValue, err := sjson.Set(string(data), key, value)
	if err != nil {
		return fmt.Errorf("failed to set config field %s: %w", key, err)
	}
	if err := os.WriteFile(c.dataConfigDir, []byte(newValue), 0o644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}
	return nil
}

func (c *Config) SetProviderAPIKey(providerID, apiKey string) error {
	// First save to the config file
	err := c.SetConfigField("providers."+providerID+".api_key", apiKey)
	if err != nil {
		return fmt.Errorf("failed to save API key to config file: %w", err)
	}

	if c.Providers == nil {
		c.Providers = make(map[string]ProviderConfig)
	}

	providerConfig, exists := c.Providers[providerID]
	if exists {
		providerConfig.APIKey = apiKey
		c.Providers[providerID] = providerConfig
		return nil
	}

	var foundProvider *catwalk.Provider
	for _, p := range c.knownProviders {
		if string(p.ID) == providerID {
			foundProvider = &p
			break
		}
	}

	if foundProvider != nil {
		// Create new provider config based on known provider
		providerConfig = ProviderConfig{
			ID:           providerID,
			Name:         foundProvider.Name,
			BaseURL:      foundProvider.APIEndpoint,
			Type:         foundProvider.Type,
			APIKey:       apiKey,
			Disable:      false,
			ExtraHeaders: make(map[string]string),
			ExtraParams:  make(map[string]string),
			Models:       foundProvider.Models,
		}
	} else {
		return fmt.Errorf("provider with ID %s not found in known providers", providerID)
	}
	// Store the updated provider config
	c.Providers[providerID] = providerConfig
	return nil
}

func (c *Config) SetupAgents() {
	agents := map[string]Agent{
		"coder": {
			ID:           "coder",
			Name:         "Coder",
			Description:  "An agent that helps with executing coding tasks.",
			Model:        SelectedModelTypeLarge,
			ContextPaths: c.Options.ContextPaths,
			// All tools allowed
		},
		"task": {
			ID:           "task",
			Name:         "Task",
			Description:  "An agent that helps with searching for context and finding implementation details.",
			Model:        SelectedModelTypeLarge,
			ContextPaths: c.Options.ContextPaths,
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
	c.Agents = agents
}

func (c *Config) Resolver() VariableResolver {
	return c.resolver
}

func (c *ProviderConfig) TestConnection(resolver VariableResolver) error {
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
