package config

import (
	"slices"
	"strings"

	"github.com/charmbracelet/crush/internal/fur/provider"
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
	// The provider's API endpoint.
	BaseURL string `json:"base_url,omitempty"`
	// The provider type, e.g. "openai", "anthropic", etc. if empty it defaults to openai.
	Type provider.Type `json:"type,omitempty"`
	// The provider's API key.
	APIKey string `json:"api_key,omitempty"`
	// Marks the provider as disabled.
	Disable bool `json:"disable,omitempty"`

	// Extra headers to send with each request to the provider.
	ExtraHeaders map[string]string

	// Used to pass extra parameters to the provider.
	ExtraParams map[string]string `json:"-"`

	// The provider models
	Models []provider.Model `json:"models,omitempty"`
}

type MCPType string

const (
	MCPStdio MCPType = "stdio"
	MCPSse   MCPType = "sse"
	MCPHttp  MCPType = "http"
)

type MCPConfig struct {
	Command string   `json:"command,omitempty" `
	Env     []string `json:"env,omitempty"`
	Args    []string `json:"args,omitempty"`
	Type    MCPType  `json:"type"`
	URL     string   `json:"url,omitempty"`

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

type Options struct {
	ContextPaths         []string    `json:"context_paths,omitempty"`
	TUI                  *TUIOptions `json:"tui,omitempty"`
	Debug                bool        `json:"debug,omitempty"`
	DebugLSP             bool        `json:"debug_lsp,omitempty"`
	DisableAutoSummarize bool        `json:"disable_auto_summarize,omitempty"`
	// Relative to the cwd
	DataDirectory string `json:"data_directory,omitempty"`
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

// Config holds the configuration for crush.
type Config struct {
	// We currently only support large/small as values here.
	Models map[SelectedModelType]SelectedModel `json:"models,omitempty"`

	// The providers that are configured
	Providers map[string]ProviderConfig `json:"providers,omitempty"`

	MCP MCPs `json:"mcp,omitempty"`

	LSP LSPs `json:"lsp,omitempty"`

	Options *Options `json:"options,omitempty"`

	// Internal
	workingDir string `json:"-"`
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

func (c *Config) GetModel(provider, model string) *provider.Model {
	if providerConfig, ok := c.Providers[provider]; ok {
		for _, m := range providerConfig.Models {
			if m.ID == model {
				return &m
			}
		}
	}
	return nil
}

func (c *Config) LargeModel() *provider.Model {
	model, ok := c.Models[SelectedModelTypeLarge]
	if !ok {
		return nil
	}
	return c.GetModel(model.Provider, model.Model)
}

func (c *Config) SmallModel() *provider.Model {
	model, ok := c.Models[SelectedModelTypeSmall]
	if !ok {
		return nil
	}
	return c.GetModel(model.Provider, model.Model)
}
