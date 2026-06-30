package crush

import (
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/env"
	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/charmbracelet/crush/internal/skills"
)

type (
	Config            = config.Config
	ConfigStore       = config.ConfigStore
	SelectedModel     = config.SelectedModel
	SelectedModelType = config.SelectedModelType
	ProviderConfig    = config.ProviderConfig
	MCPConfig         = config.MCPConfig
	MCPType           = config.MCPType
	LSPConfig         = config.LSPConfig
	Agent             = config.Agent
	Options           = config.Options
	TUIOptions        = config.TUIOptions
	Completions       = config.Completions
	Permissions       = config.Permissions
	Attribution       = config.Attribution
	TrailerStyle      = config.TrailerStyle
	HookConfig        = config.HookConfig
	Tools             = config.Tools
	ToolLs            = config.ToolLs
	ToolGrep          = config.ToolGrep
	Scope             = config.Scope
	VariableResolver  = config.VariableResolver
	RuntimeOverrides  = config.RuntimeOverrides
	SkillsManager     = skills.Manager
	OAuthToken        = oauth.Token
)

const (
	SelectedModelTypeLarge = config.SelectedModelTypeLarge
	SelectedModelTypeSmall = config.SelectedModelTypeSmall

	AgentCoder = config.AgentCoder
	AgentTask  = config.AgentTask

	MCPStdio = config.MCPStdio
	MCPSSE   = config.MCPSSE
	MCPHttp  = config.MCPHttp

	TrailerStyleNone         = config.TrailerStyleNone
	TrailerStyleCoAuthoredBy = config.TrailerStyleCoAuthoredBy
	TrailerStyleAssistedBy   = config.TrailerStyleAssistedBy
)

// Load loads the configuration from default paths and returns a ConfigStore.
func Load(workingDir, dataDir string, debug bool) (*ConfigStore, error) {
	return config.Load(workingDir, dataDir, debug)
}

// NewConfigStore creates a ConfigStore from a pre-built Config for
// programmatic use. It wraps the Config without loading anything from disk.
func NewConfigStore(cfg *Config, loadedPaths ...string) *ConfigStore {
	return config.NewTestStore(cfg, loadedPaths...)
}

// NewTestStore is a deprecated alias for NewConfigStore.
// Deprecated: Use NewConfigStore instead.
func NewTestStore(cfg *Config, loadedPaths ...string) *ConfigStore {
	return NewConfigStore(cfg, loadedPaths...)
}

// NewVariableResolver returns a VariableResolver that expands shell
// variables and command substitutions.
func NewVariableResolver(environment map[string]string) VariableResolver {
	return config.NewShellVariableResolver(env.NewFromMap(environment))
}

// NewShellVariableResolver is a deprecated alias for NewVariableResolver.
// Deprecated: Use NewVariableResolver instead.
func NewShellVariableResolver(environment map[string]string) VariableResolver {
	return NewVariableResolver(environment)
}
