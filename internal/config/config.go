package config

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/charmbracelet/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/llm/agent"
	"github.com/charmbracelet/crush/internal/llm/provider"
	"github.com/charmbracelet/crush/internal/resolver"
	"github.com/tidwall/sjson"
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

type MCPs map[string]agent.MCPConfig

type MCP struct {
	Name string          `json:"name"`
	MCP  agent.MCPConfig `json:"mcp"`
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
	Models map[SelectedModelType]agent.Model `json:"models,omitempty"`

	// The providers that are configured
	Providers *csync.Map[string, provider.Config] `json:"providers,omitempty"`

	MCP MCPs `json:"mcp,omitempty"`

	LSP LSPs `json:"lsp,omitempty"`

	Options *Options `json:"options,omitempty"`

	Permissions *Permissions `json:"permissions,omitempty"`

	// Internal
	workingDir     string `json:"-"`
	resolver       resolver.Resolver
	dataConfigDir  string             `json:"-"`
	knownProviders []catwalk.Provider `json:"-"`
}

func (c *Config) WorkingDir() string {
	return c.workingDir
}

func (c *Config) EnabledProviders() []provider.Config {
	var enabled []provider.Config
	for p := range c.Providers.Seq() {
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
	if providerConfig, ok := c.Providers.Get(provider); ok {
		for _, m := range providerConfig.Models {
			if m.ID == model {
				return &m
			}
		}
	}
	return nil
}

func (c *Config) GetProviderForModel(modelType SelectedModelType) *provider.Config {
	model, ok := c.Models[modelType]
	if !ok {
		return nil
	}
	if providerConfig, ok := c.Providers.Get(model.Provider); ok {
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

func (c *Config) UpdatePreferredModel(modelType SelectedModelType, model agent.Model) error {
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

	providerConfig, exists := c.Providers.Get(providerID)
	if exists {
		providerConfig.APIKey = apiKey
		c.Providers.Set(providerID, providerConfig)
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
		providerConfig = provider.Config{
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
	c.Providers.Set(providerID, providerConfig)
	return nil
}

func (c *Config) Resolver() resolver.Resolver {
	return c.resolver
}
