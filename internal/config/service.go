package config

import (
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"slices"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	hyperp "github.com/charmbracelet/crush/internal/agent/hyper"
	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/charmbracelet/crush/internal/oauth/copilot"
	"github.com/charmbracelet/crush/internal/oauth/hyper"
)

// Service is the central access point for configuration. It wraps the
// raw Config data and owns all internal state that was previously held
// as unexported fields on Config (resolver, store, known providers,
// working directory).
type Service struct {
	cfg            *Config
	store          Store
	resolver       VariableResolver
	workingDir     string
	knownProviders []catwalk.Provider
	agents         map[string]Agent
}

// WorkingDir returns the working directory.
func (s *Service) WorkingDir() string {
	return s.workingDir
}

// EnabledProviders returns all non-disabled provider configs.
func (s *Service) EnabledProviders() []ProviderConfig {
	return s.cfg.EnabledProviders()
}

// IsConfigured returns true if at least one provider is enabled.
func (s *Service) IsConfigured() bool {
	return s.cfg.IsConfigured()
}

// GetModel returns the catwalk model for the given provider and model
// ID, or nil if not found.
func (s *Service) GetModel(provider, model string) *catwalk.Model {
	return s.cfg.GetModel(provider, model)
}

// GetProviderForModel returns the provider config for the given model
// type, or nil.
func (s *Service) GetProviderForModel(modelType SelectedModelType) *ProviderConfig {
	return s.cfg.GetProviderForModel(modelType)
}

// GetModelByType returns the catwalk model for the given model type,
// or nil.
func (s *Service) GetModelByType(modelType SelectedModelType) *catwalk.Model {
	return s.cfg.GetModelByType(modelType)
}

// LargeModel returns the catwalk model for the large model type.
func (s *Service) LargeModel() *catwalk.Model {
	return s.cfg.LargeModel()
}

// SmallModel returns the catwalk model for the small model type.
func (s *Service) SmallModel() *catwalk.Model {
	return s.cfg.SmallModel()
}

// Resolve resolves a variable value using the configured resolver.
func (s *Service) Resolve(key string) (string, error) {
	if s.resolver == nil {
		return "", fmt.Errorf("no variable resolver configured")
	}
	return s.resolver.ResolveValue(key)
}

// Resolver returns the variable resolver.
func (s *Service) Resolver() VariableResolver {
	return s.resolver
}

// SetupAgents builds the agent configurations from the current config
// options.
func (s *Service) SetupAgents() {
	allowedTools := resolveAllowedTools(allToolNames(), s.cfg.Options.DisabledTools)

	s.agents = map[string]Agent{
		AgentCoder: {
			ID:           AgentCoder,
			Name:         "Coder",
			Description:  "An agent that helps with executing coding tasks.",
			Model:        SelectedModelTypeLarge,
			ContextPaths: s.cfg.Options.ContextPaths,
			AllowedTools: allowedTools,
		},

		AgentTask: {
			ID:           AgentCoder,
			Name:         "Task",
			Description:  "An agent that helps with searching for context and finding implementation details.",
			Model:        SelectedModelTypeLarge,
			ContextPaths: s.cfg.Options.ContextPaths,
			AllowedTools: resolveReadOnlyTools(allowedTools),
			AllowedMCP:   map[string][]string{},
		},
	}
}

// Agents returns the agent configuration map.
func (s *Service) Agents() map[string]Agent {
	return s.agents
}

// Agent returns the agent configuration for the given name and
// whether it exists.
func (s *Service) Agent(name string) (Agent, bool) {
	a, ok := s.agents[name]
	return a, ok
}

// DataDirectory returns the data directory path.
func (s *Service) DataDirectory() string {
	return s.cfg.Options.DataDirectory
}

// Debug returns whether debug mode is enabled.
func (s *Service) Debug() bool {
	return s.cfg.Options.Debug
}

// DebugLSP returns whether LSP debug mode is enabled.
func (s *Service) DebugLSP() bool {
	return s.cfg.Options.DebugLSP
}

// DisableAutoSummarize returns whether auto-summarization is
// disabled.
func (s *Service) DisableAutoSummarize() bool {
	return s.cfg.Options.DisableAutoSummarize
}

// Attribution returns the attribution settings.
func (s *Service) Attribution() *Attribution {
	return s.cfg.Options.Attribution
}

// ContextPaths returns the configured context paths.
func (s *Service) ContextPaths() []string {
	return s.cfg.Options.ContextPaths
}

// SkillsPaths returns the configured skills paths.
func (s *Service) SkillsPaths() []string {
	return s.cfg.Options.SkillsPaths
}

// Progress returns the progress setting pointer.
func (s *Service) Progress() *bool {
	return s.cfg.Options.Progress
}

// DisableMetrics returns whether metrics are disabled.
func (s *Service) DisableMetrics() bool {
	return s.cfg.Options.DisableMetrics
}

// SelectedModel returns the selected model for the given type and
// whether it exists.
func (s *Service) SelectedModel(modelType SelectedModelType) (SelectedModel, bool) {
	m, ok := s.cfg.Models[modelType]
	return m, ok
}

// Provider returns the provider config for the given ID and whether
// it exists.
func (s *Service) Provider(id string) (ProviderConfig, bool) {
	p, ok := s.cfg.Providers[id]
	return p, ok
}

// SetProvider sets the provider config for the given ID.
func (s *Service) SetProvider(id string, p ProviderConfig) {
	s.cfg.Providers[id] = p
}

// Providers returns all provider configs.
func (s *Service) AllProviders() map[string]ProviderConfig {
	return s.cfg.Providers
}

// MCP returns the MCP configurations.
func (s *Service) MCP() MCPs {
	return s.cfg.MCP
}

// LSP returns the LSP configurations.
func (s *Service) LSP() LSPs {
	return s.cfg.LSP
}

// Permissions returns the permissions configuration.
func (s *Service) Permissions() *Permissions {
	return s.cfg.Permissions
}

// SetAttribution sets the attribution settings.
func (s *Service) SetAttribution(a *Attribution) {
	s.cfg.Options.Attribution = a
}

// SetSkillsPaths sets the skills paths.
func (s *Service) SetSkillsPaths(paths []string) {
	s.cfg.Options.SkillsPaths = paths
}

// SetLSP sets the LSP configurations.
func (s *Service) SetLSP(lsp LSPs) {
	s.cfg.LSP = lsp
}

// SetPermissions sets the permissions configuration.
func (s *Service) SetPermissions(p *Permissions) {
	s.cfg.Permissions = p
}

// OverrideModel overrides the in-memory model for the given type
// without persisting. Used for non-interactive model overrides.
func (s *Service) OverrideModel(modelType SelectedModelType, model SelectedModel) {
	s.cfg.Models[modelType] = model
}

// ToolLsConfig returns the ls tool configuration.
func (s *Service) ToolLsConfig() ToolLs {
	return s.cfg.Tools.Ls
}

// CompactMode returns whether compact mode is enabled.
func (s *Service) CompactMode() bool {
	if s.cfg.Options.TUI == nil {
		return false
	}
	return s.cfg.Options.TUI.CompactMode
}

// DiffMode returns the diff mode setting.
func (s *Service) DiffMode() string {
	if s.cfg.Options.TUI == nil {
		return ""
	}
	return s.cfg.Options.TUI.DiffMode
}

// CompletionLimits returns the completion depth and items limits.
func (s *Service) CompletionLimits() (depth, items int) {
	if s.cfg.Options.TUI == nil {
		return 0, 0
	}
	return s.cfg.Options.TUI.Completions.Limits()
}

// DisableDefaultProviders returns whether default providers are
// disabled.
func (s *Service) DisableDefaultProviders() bool {
	return s.cfg.Options.DisableDefaultProviders
}

// DisableProviderAutoUpdate returns whether provider auto-update is
// disabled.
func (s *Service) DisableProviderAutoUpdate() bool {
	return s.cfg.Options.DisableProviderAutoUpdate
}

// InitializeAs returns the initialization file name.
func (s *Service) InitializeAs() string {
	return s.cfg.Options.InitializeAs
}

// AutoLSP returns the auto-LSP setting pointer.
func (s *Service) AutoLSP() *bool {
	return s.cfg.Options.AutoLSP
}

// RecentModels returns recent models for the given type.
func (s *Service) RecentModels(modelType SelectedModelType) []SelectedModel {
	return s.cfg.RecentModels[modelType]
}

// Options returns the full options struct. This is a temporary
// accessor for callers that need multiple option fields.
func (s *Service) Options() *Options {
	return s.cfg.Options
}

// HasConfigField returns true if the given dotted key path exists in
// the persisted config data.
func (s *Service) HasConfigField(key string) bool {
	return HasField(s.store, key)
}

// SetConfigField sets a value at the given dotted key path and
// persists it.
func (s *Service) SetConfigField(key string, value any) error {
	return SetField(s.store, key, value)
}

// RemoveConfigField deletes a value at the given dotted key path and
// persists it.
func (s *Service) RemoveConfigField(key string) error {
	return RemoveField(s.store, key)
}

// SetCompactMode toggles compact mode and persists the change.
func (s *Service) SetCompactMode(enabled bool) error {
	cfg := s.cfg
	if cfg.Options == nil {
		cfg.Options = &Options{}
	}
	if cfg.Options.TUI == nil {
		cfg.Options.TUI = &TUIOptions{}
	}
	cfg.Options.TUI.CompactMode = enabled
	return s.SetConfigField("options.tui.compact_mode", enabled)
}

// UpdatePreferredModel updates the selected model for the given type
// and persists the change, also recording it in the recent models
// list.
func (s *Service) UpdatePreferredModel(modelType SelectedModelType, model SelectedModel) error {
	s.cfg.Models[modelType] = model
	if err := s.SetConfigField(fmt.Sprintf("models.%s", modelType), model); err != nil {
		return fmt.Errorf("failed to update preferred model: %w", err)
	}
	if err := s.recordRecentModel(modelType, model); err != nil {
		return err
	}
	return nil
}

const maxRecentModelsPerType = 5

func (s *Service) recordRecentModel(modelType SelectedModelType, model SelectedModel) error {
	if model.Provider == "" || model.Model == "" {
		return nil
	}

	cfg := s.cfg
	if cfg.RecentModels == nil {
		cfg.RecentModels = make(map[SelectedModelType][]SelectedModel)
	}

	eq := func(a, b SelectedModel) bool {
		return a.Provider == b.Provider && a.Model == b.Model
	}

	entry := SelectedModel{
		Provider: model.Provider,
		Model:    model.Model,
	}

	current := cfg.RecentModels[modelType]
	withoutCurrent := slices.DeleteFunc(slices.Clone(current), func(existing SelectedModel) bool {
		return eq(existing, entry)
	})

	updated := append([]SelectedModel{entry}, withoutCurrent...)
	if len(updated) > maxRecentModelsPerType {
		updated = updated[:maxRecentModelsPerType]
	}

	if slices.EqualFunc(current, updated, eq) {
		return nil
	}

	cfg.RecentModels[modelType] = updated

	if err := s.SetConfigField(fmt.Sprintf("recent_models.%s", modelType), updated); err != nil {
		return fmt.Errorf("failed to persist recent models: %w", err)
	}

	return nil
}

// RefreshOAuthToken refreshes the OAuth token for the given provider.
func (s *Service) RefreshOAuthToken(ctx context.Context, providerID string) error {
	cfg := s.cfg
	providerConfig, exists := cfg.Providers[providerID]
	if !exists {
		return fmt.Errorf("provider %s not found", providerID)
	}

	if providerConfig.OAuthToken == nil {
		return fmt.Errorf("provider %s does not have an OAuth token", providerID)
	}

	var newToken *oauth.Token
	var refreshErr error
	switch providerID {
	case string(catwalk.InferenceProviderCopilot):
		newToken, refreshErr = copilot.RefreshToken(ctx, providerConfig.OAuthToken.RefreshToken)
	case hyperp.Name:
		newToken, refreshErr = hyper.ExchangeToken(ctx, providerConfig.OAuthToken.RefreshToken)
	default:
		return fmt.Errorf("OAuth refresh not supported for provider %s", providerID)
	}
	if refreshErr != nil {
		return fmt.Errorf("failed to refresh OAuth token for provider %s: %w", providerID, refreshErr)
	}

	slog.Info("Successfully refreshed OAuth token", "provider", providerID)
	providerConfig.OAuthToken = newToken
	providerConfig.APIKey = newToken.AccessToken

	switch providerID {
	case string(catwalk.InferenceProviderCopilot):
		providerConfig.SetupGitHubCopilot()
	}

	cfg.Providers[providerID] = providerConfig

	if err := cmp.Or(
		s.SetConfigField(fmt.Sprintf("providers.%s.api_key", providerID), newToken.AccessToken),
		s.SetConfigField(fmt.Sprintf("providers.%s.oauth", providerID), newToken),
	); err != nil {
		return fmt.Errorf("failed to persist refreshed token: %w", err)
	}

	return nil
}

// SetProviderAPIKey sets the API key (string or *oauth.Token) for a
// provider and persists the change.
func (s *Service) SetProviderAPIKey(providerID string, apiKey any) error {
	cfg := s.cfg
	var providerConfig ProviderConfig
	var exists bool
	var setKeyOrToken func()

	switch v := apiKey.(type) {
	case string:
		if err := s.SetConfigField(fmt.Sprintf("providers.%s.api_key", providerID), v); err != nil {
			return fmt.Errorf("failed to save api key to config file: %w", err)
		}
		setKeyOrToken = func() { providerConfig.APIKey = v }
	case *oauth.Token:
		if err := cmp.Or(
			s.SetConfigField(fmt.Sprintf("providers.%s.api_key", providerID), v.AccessToken),
			s.SetConfigField(fmt.Sprintf("providers.%s.oauth", providerID), v),
		); err != nil {
			return err
		}
		setKeyOrToken = func() {
			providerConfig.APIKey = v.AccessToken
			providerConfig.OAuthToken = v
			switch providerID {
			case string(catwalk.InferenceProviderCopilot):
				providerConfig.SetupGitHubCopilot()
			}
		}
	}

	providerConfig, exists = cfg.Providers[providerID]
	if exists {
		setKeyOrToken()
		cfg.Providers[providerID] = providerConfig
		return nil
	}

	var foundProvider *catwalk.Provider
	for _, p := range s.knownProviders {
		if string(p.ID) == providerID {
			foundProvider = &p
			break
		}
	}

	if foundProvider != nil {
		providerConfig = ProviderConfig{
			ID:           providerID,
			Name:         foundProvider.Name,
			BaseURL:      foundProvider.APIEndpoint,
			Type:         foundProvider.Type,
			Disable:      false,
			ExtraHeaders: make(map[string]string),
			ExtraParams:  make(map[string]string),
			Models:       foundProvider.Models,
		}
		setKeyOrToken()
	} else {
		return fmt.Errorf("provider with ID %s not found in known providers", providerID)
	}
	cfg.Providers[providerID] = providerConfig
	return nil
}

// ImportCopilot imports an existing GitHub Copilot token from disk if
// available and not already configured.
func (s *Service) ImportCopilot() (*oauth.Token, bool) {
	if testing.Testing() {
		return nil, false
	}

	if s.HasConfigField("providers.copilot.api_key") || s.HasConfigField("providers.copilot.oauth") {
		return nil, false
	}

	diskToken, hasDiskToken := copilot.RefreshTokenFromDisk()
	if !hasDiskToken {
		return nil, false
	}

	slog.Info("Found existing GitHub Copilot token on disk. Authenticating...")
	token, err := copilot.RefreshToken(context.TODO(), diskToken)
	if err != nil {
		slog.Error("Unable to import GitHub Copilot token", "error", err)
		return nil, false
	}

	if err := s.SetProviderAPIKey(string(catwalk.InferenceProviderCopilot), token); err != nil {
		return token, false
	}

	if err := cmp.Or(
		s.SetConfigField("providers.copilot.api_key", token.AccessToken),
		s.SetConfigField("providers.copilot.oauth", token),
	); err != nil {
		slog.Error("Unable to save GitHub Copilot token to disk", "error", err)
	}

	slog.Info("GitHub Copilot successfully imported")
	return token, true
}
