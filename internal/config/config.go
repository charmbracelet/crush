package config

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/charmbracelet/catwalk/pkg/catwalk"
	"github.com/lacymorrow/lash/internal/csync"
	"github.com/lacymorrow/lash/internal/env"
	"github.com/tidwall/sjson"
)

const (
	// AppName determines the base name of the configuration file looked up by the loader
	// e.g. it will search for "crush.json" and ".crush.json" in the working directory
	AppName                  = "lash"
	DefaultDataDirectoryName = ".lash"
	// LegacyAppName is the previous application name kept for backward-compatibility
	LegacyAppName = "crush"

	// ProvidersCacheFilename is the filename used to cache known providers
	ProvidersCacheFilename = "providers.json"

	// ProvidersCacheTTL defines how long the providers cache is considered fresh
	ProvidersCacheTTL = 24 * time.Hour

	// Current and legacy config filenames for data/config stores
	CurrentConfigFilename = "lash.json"
	LegacyConfigFilename  = "crush.json"

	// Environment variable prefix used for scoped overrides
	AppEnvPrefix = "LASH_"

	// Standard HTTP headers and values used across providers
	HeaderAuthorization    = "Authorization"
	HeaderXAPIKey          = "x-api-key"
	HeaderAnthropicVersion = "anthropic-version"
	HeaderRetryAfter       = "Retry-After"
	BearerPrefix           = "Bearer "

	// Default provider base URLs and versions
	DefaultOpenAIBaseURL    = "https://api.openai.com/v1"
	DefaultAnthropicBaseURL = "https://api.anthropic.com"
	DefaultGeminiBaseURL    = "https://generativelanguage.googleapis.com"
	DefaultAnthropicAPIVer  = "2023-06-01"

	// Default timeout for simple provider connectivity checks
	DefaultHTTPTestTimeout = 5 * time.Second

	// UI verification spinner minimum duration to ensure visible feedback
	VerificationMinSpinnerDuration = 750 * time.Millisecond

	// Onboarding delay before auto-submitting a verified API key
	OnboardingAPIKeySubmitDelay = 5 * time.Second

	// Safety buffers for context-limit handling
	ContextLimitBufferTokens = 1000
	MinSafeMaxTokens         = 1000

	// Logs and UI defaults
	DefaultTailLines = 1000

	// Tool defaults
	DefaultBashTimeoutMs = 1 * 60 * 1000  // 1 minute in ms
	MaxBashTimeoutMs     = 10 * 60 * 1000 // 10 minutes in ms
	MaxBashOutputChars   = 30000

	// System buffers
	DefaultDBPageSize  = 4096
	DefaultPTYReadSize = 4096

	// Storage filenames
	DefaultDBFilename = "lash.db"
)

var defaultContextPaths = []string{
	".github/copilot-instructions.md",
	".cursorrules",
	".cursor/rules/",
	"CLAUDE.md",
	"CLAUDE.local.md",
	"Claude.md",
	"Claude.local.md",
	"claude.md",
	"claude.local.md",
	"GEMINI.md",
	"gemini.md",
	"lash.md",
	"lash.local.md",
	"Crush.md",
	"Crush.local.md",
	"CRUSH.md",
	"CRUSH.local.md",
	"crush.md",
	"crush.local.md",
	"AGENT.md",
	"AGENT.local.md",
	"Agent.md",
	"Agent.local.md",
	"agent.md",
	"agent.local.md",
	"AGENTS.md",
	"agents.md",
	"Agents.md",
}

type SelectedModelType string

const (
	SelectedModelTypeLarge SelectedModelType = "large"
	SelectedModelTypeSmall SelectedModelType = "small"
)

type SelectedModel struct {
	// The model id as used by the provider API.
	// Required.
	Model string `json:"model" jsonschema:"required,description=The model ID as used by the provider API,example=gpt-4o"`
	// The model provider, same as the key/id used in the providers config.
	// Required.
	Provider string `json:"provider" jsonschema:"required,description=The model provider ID that matches a key in the providers config,example=openai"`

	// Only used by models that use the openai provider and need this set.
	ReasoningEffort string `json:"reasoning_effort,omitempty" jsonschema:"description=Reasoning effort level for OpenAI models that support it,enum=low,enum=medium,enum=high"`

	// Overrides the default model configuration.
	MaxTokens int64 `json:"max_tokens,omitempty" jsonschema:"description=Maximum number of tokens for model responses,minimum=1,maximum=200000,example=4096"`

	// Used by anthropic models that can reason to indicate if the model should think.
	Think bool `json:"think,omitempty" jsonschema:"description=Enable thinking mode for Anthropic models that support reasoning"`
}

type ProviderConfig struct {
	// The provider's id.
	ID string `json:"id,omitempty" jsonschema:"description=Unique identifier for the provider,example=openai"`
	// The provider's name, used for display purposes.
	Name string `json:"name,omitempty" jsonschema:"description=Human-readable name for the provider,example=OpenAI"`
	// The provider's API endpoint.
	BaseURL string `json:"base_url,omitempty" jsonschema:"description=Base URL for the provider's API,format=uri,example=https://api.openai.com/v1"`
	// The provider type, e.g. "openai", "anthropic", etc. if empty it defaults to openai.
	Type catwalk.Type `json:"type,omitempty" jsonschema:"description=Provider type that determines the API format,enum=openai,enum=anthropic,enum=gemini,enum=azure,enum=vertexai,default=openai"`
	// The provider's API key.
	APIKey string `json:"api_key,omitempty" jsonschema:"description=API key for authentication with the provider,example=$OPENAI_API_KEY"`
	// Marks the provider as disabled.
	Disable bool `json:"disable,omitempty" jsonschema:"description=Whether this provider is disabled,default=false"`

	// Custom system prompt prefix.
	SystemPromptPrefix string `json:"system_prompt_prefix,omitempty" jsonschema:"description=Custom prefix to add to system prompts for this provider"`

	// Extra headers to send with each request to the provider.
	ExtraHeaders map[string]string `json:"extra_headers,omitempty" jsonschema:"description=Additional HTTP headers to send with requests"`
	// Extra body
	ExtraBody map[string]any `json:"extra_body,omitempty" jsonschema:"description=Additional fields to include in request bodies"`

	// Used to pass extra parameters to the provider.
	ExtraParams map[string]string `json:"-"`

	// The provider models
	Models []catwalk.Model `json:"models,omitempty" jsonschema:"description=List of models available from this provider"`
}

type MCPType string

const (
	MCPStdio MCPType = "stdio"
	MCPSse   MCPType = "sse"
	MCPHttp  MCPType = "http"
)

type MCPConfig struct {
	Command  string            `json:"command,omitempty" jsonschema:"description=Command to execute for stdio MCP servers,example=npx"`
	Env      map[string]string `json:"env,omitempty" jsonschema:"description=Environment variables to set for the MCP server"`
	Args     []string          `json:"args,omitempty" jsonschema:"description=Arguments to pass to the MCP server command"`
	Type     MCPType           `json:"type,omitempty" jsonschema:"description=Type of MCP connection,enum=stdio,enum=sse,enum=http,default=stdio"`
	URL      string            `json:"url,omitempty" jsonschema:"description=URL for HTTP or SSE MCP servers,format=uri,example=http://localhost:3000/mcp"`
	Disabled bool              `json:"disabled,omitempty" jsonschema:"description=Whether this MCP server is disabled,default=false"`

	// TODO: maybe make it possible to get the value from the env
	Headers map[string]string `json:"headers,omitempty" jsonschema:"description=HTTP headers for HTTP/SSE MCP servers"`
}

type LSPConfig struct {
	Disabled bool     `json:"enabled,omitempty" jsonschema:"description=Whether this LSP server is disabled,default=false"`
	Command  string   `json:"command" jsonschema:"required,description=Command to execute for the LSP server,example=gopls"`
	Args     []string `json:"args,omitempty" jsonschema:"description=Arguments to pass to the LSP server command"`
	Options  any      `json:"options,omitempty" jsonschema:"description=LSP server-specific configuration options"`
}

type TUIOptions struct {
	CompactMode bool `json:"compact_mode,omitempty" jsonschema:"description=Enable compact mode for the TUI interface,default=false"`
	// Here we can add themes later or any TUI related options
}

type Permissions struct {
	AllowedTools []string `json:"allowed_tools,omitempty" jsonschema:"description=List of tools that don't require permission prompts,example=bash,example=view"` // Tools that don't require permission prompts
	SkipRequests bool     `json:"-"`                                                                                                                              // Automatically accept all permissions (YOLO mode)
}

// LashSafety holds Lash-specific safety toggles.
type LashSafety struct {
	// If true (default), agent-suggested shell executions require confirmation.
	// If false, agent-suggested commands can execute without prompting.
	ConfirmAgentExec *bool `json:"confirm_agent_exec,omitempty" jsonschema:"description=Require confirmation before executing agent-suggested shell commands (bash:execute),default=true"`
}

// LashConfig is the optional Lash-specific configuration namespace.
type LashConfig struct {
	// Mode persists the last selected app mode: Shell, Agent, or Auto
	Mode string `json:"mode,omitempty" jsonschema:"description=Last selected app mode (Shell, Agent, or Auto),enum=Shell,enum=Agent,enum=Auto,default=Auto"`
	// YOLO enables skipping all permission prompts (global auto-approve)
	Yolo   bool       `json:"yolo,omitempty" jsonschema:"description=Skip all permission prompts (YOLO mode),default=false"`
	Safety LashSafety `json:"safety,omitempty" jsonschema:"description=Lash-specific safety options"`
}

type Options struct {
	ContextPaths         []string    `json:"context_paths,omitempty" jsonschema:"description=Paths to files containing context information for the AI,example=.cursorrules,example=CRUSH.md"`
	TUI                  *TUIOptions `json:"tui,omitempty" jsonschema:"description=Terminal user interface options"`
	Debug                bool        `json:"debug,omitempty" jsonschema:"description=Enable debug logging,default=false"`
	DebugLSP             bool        `json:"debug_lsp,omitempty" jsonschema:"description=Enable debug logging for LSP servers,default=false"`
	DisableAutoSummarize bool        `json:"disable_auto_summarize,omitempty" jsonschema:"description=Disable automatic conversation summarization,default=false"`
	DataDirectory        string      `json:"data_directory,omitempty" jsonschema:"description=Directory for storing application data (relative to working directory),default=.lash,example=.lash"` // Relative to the cwd
	// Maximum duration for a single agent request before it is canceled. If 0, no global request timeout is applied.
	RequestTimeoutSeconds int `json:"request_timeout_seconds,omitempty" jsonschema:"description=Max duration in seconds for a single agent request; when set, requests are canceled after this time"`
	// Maximum duration for each individual tool call unless the tool specifies a shorter timeout. If 0, no extra per-tool cap is applied.
	ToolCallTimeoutSeconds int `json:"tool_call_timeout_seconds,omitempty" jsonschema:"description=Max duration in seconds for each tool call unless the tool specifies a shorter timeout"`
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
	// Start from the current process environment so stdio MCPs inherit PATH, HOME, etc.
	baseEnv := os.Environ()

	// Resolve and overlay user-provided env vars
	resolver := NewShellVariableResolver(env.New())
	overrides := make(map[string]string, len(m.Env))
	for key, value := range m.Env {
		resolved, err := resolver.ResolveValue(value)
		if err != nil {
			slog.Error("error resolving environment variable", "error", err, "variable", key, "value", value)
			continue
		}
		overrides[key] = resolved
	}

	// Apply overrides to baseEnv, preserving other variables
	// exec.Cmd.Env expects KEY=VALUE pairs; replacing in-place avoids duplicates
	for key, value := range overrides {
		replaced := false
		prefix := key + "="
		for i := range baseEnv {
			if strings.HasPrefix(baseEnv[i], prefix) {
				baseEnv[i] = prefix + value
				replaced = true
				break
			}
		}
		if !replaced {
			baseEnv = append(baseEnv, prefix+value)
		}
	}

	return baseEnv
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

	Model SelectedModelType `json:"model" jsonschema:"required,description=The model type to use for this agent,enum=large,enum=small,default=large"`

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

// Config holds the configuration for lash.
type Config struct {
	Schema string `json:"$schema,omitempty"`

	// We currently only support large/small as values here.
	Models map[SelectedModelType]SelectedModel `json:"models,omitempty" jsonschema:"description=Model configurations for different model types,example={\"large\":{\"model\":\"gpt-4o\",\"provider\":\"openai\"}}"`

	// The providers that are configured
	Providers *csync.Map[string, ProviderConfig] `json:"providers,omitempty" jsonschema:"description=AI provider configurations"`

	MCP MCPs `json:"mcp,omitempty" jsonschema:"description=Model Context Protocol server configurations"`

	LSP LSPs `json:"lsp,omitempty" jsonschema:"description=Language Server Protocol configurations"`

	Options *Options `json:"options,omitempty" jsonschema:"description=General application options"`

	Permissions *Permissions `json:"permissions,omitempty" jsonschema:"description=Permission settings for tool usage"`

	// Lash-specific configuration
	Lash *LashConfig `json:"lash,omitempty" jsonschema:"description=Lash-specific options"`

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

// ActiveMode returns the persisted mode or "Auto" when unset.
func (c *Config) ActiveMode() string {
	if c == nil {
		return "Auto"
	}
	if c.Lash == nil {
		return "Auto"
	}
	if strings.TrimSpace(c.Lash.Mode) == "" {
		return "Auto"
	}
	return c.Lash.Mode
}

// SetActiveMode persists the given mode under lash.mode and updates in-memory state.
func (c *Config) SetActiveMode(mode string) error {
	if c.Lash == nil {
		c.Lash = &LashConfig{}
	}
	c.Lash.Mode = mode
	return c.SetConfigField("lash.mode", mode)
}

func (c *Config) EnabledProviders() []ProviderConfig {
	var enabled []ProviderConfig
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
	// Support synthetic Anthropic Max provider by falling back to real Anthropic catalog
	if provider == "anthropic-max" {
		if providerConfig, ok := c.Providers.Get(string(catwalk.InferenceProviderAnthropic)); ok {
			for _, m := range providerConfig.Models {
				if m.ID == model {
					return &m
				}
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
	if c.Options.TUI == nil {
		c.Options.TUI = &TUIOptions{}
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
	if err := os.WriteFile(c.dataConfigDir, []byte(newValue), 0o600); err != nil {
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
	c.Providers.Set(providerID, providerConfig)
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
	
	// Note: OAuth tokens for anthropic-max might be expired, but we can't refresh here
	// due to import cycle. The provider will handle refresh on actual API calls.
	
	switch c.Type {
	case catwalk.TypeOpenAI:
		baseURL, _ := resolver.ResolveValue(c.BaseURL)
		if baseURL == "" {
			baseURL = DefaultOpenAIBaseURL
		}
		testURL = baseURL + "/models"
		headers[HeaderAuthorization] = BearerPrefix + apiKey
	case catwalk.TypeAnthropic:
		baseURL, _ := resolver.ResolveValue(c.BaseURL)
		if baseURL == "" {
			baseURL = DefaultAnthropicBaseURL
		}
		// Anthropic doesn't have a /models endpoint, so we skip the connectivity test
		// The actual API calls will handle authentication errors
		return nil
	case catwalk.TypeGemini:
		baseURL, _ := resolver.ResolveValue(c.BaseURL)
		if baseURL == "" {
			baseURL = DefaultGeminiBaseURL
		}
		testURL = baseURL + "/v1beta/models?key=" + url.QueryEscape(apiKey)
	}
	ctx, cancel := context.WithTimeout(context.Background(), DefaultHTTPTestTimeout)
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
