package config

import (
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"charm.land/catwalk/pkg/catwalk"
	hyperp "github.com/charmbracelet/crush/internal/agent/hyper"
	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/charmbracelet/crush/internal/oauth/copilot"
	"github.com/charmbracelet/crush/internal/oauth/hyper"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// ConfigStore is the single entry point for all config access. It owns the
// pure-data Config, runtime state (working directory, resolver, known
// providers), and persistence to both global and workspace config files.
type ConfigStore struct {
	config         *Config
	workingDir     string
	workingDirMu   sync.RWMutex // protects workingDir
	resolver       VariableResolver
	globalDataPath string // ~/.local/share/crush/crush.json
	workspacePath  string // .crush/crush.json
	knownProviders []catwalk.Provider
}

// Config returns the pure-data config struct (read-only after load).
func (s *ConfigStore) Config() *Config {
	return s.config
}

// WorkingDir returns the current working directory.
func (s *ConfigStore) WorkingDir() string {
	s.workingDirMu.RLock()
	defer s.workingDirMu.RUnlock()
	return s.workingDir
}

// SetWorkingDir updates the current working directory.
func (s *ConfigStore) SetWorkingDir(dir string) {
	s.workingDirMu.Lock()
	s.workingDir = dir
	s.workingDirMu.Unlock()
}

// Resolver returns the variable resolver.
func (s *ConfigStore) Resolver() VariableResolver {
	return s.resolver
}

// Resolve resolves a variable reference using the configured resolver.
func (s *ConfigStore) Resolve(key string) (string, error) {
	if s.resolver == nil {
		return "", fmt.Errorf("no variable resolver configured")
	}
	return s.resolver.ResolveValue(key)
}

// KnownProviders returns the list of known providers.
func (s *ConfigStore) KnownProviders() []catwalk.Provider {
	return s.knownProviders
}

// SetupAgents configures the built-in agents and merges configured overrides.
func (s *ConfigStore) SetupAgents() {
	s.config.SetupAgents()
}

// configPath returns the file path for the given scope.
func (s *ConfigStore) configPath(scope Scope) string {
	switch scope {
	case ScopeWorkspace:
		return s.workspacePath
	default:
		return s.globalDataPath
	}
}

// HasConfigField checks whether a key exists in the config file for the given
// scope.
func (s *ConfigStore) HasConfigField(scope Scope, key string) bool {
	data, err := os.ReadFile(s.configPath(scope))
	if err != nil {
		return false
	}
	return gjson.Get(string(data), key).Exists()
}

// SetConfigField sets a key/value pair in the config file for the given scope.
func (s *ConfigStore) SetConfigField(scope Scope, key string, value any) error {
	path := s.configPath(scope)
	data, err := os.ReadFile(path)
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
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("failed to create config directory %q: %w", path, err)
	}
	if err := os.WriteFile(path, []byte(newValue), 0o600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}
	return nil
}

// RemoveConfigField removes a key from the config file for the given scope.
func (s *ConfigStore) RemoveConfigField(scope Scope, key string) error {
	path := s.configPath(scope)
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	newValue, err := sjson.Delete(string(data), key)
	if err != nil {
		return fmt.Errorf("failed to delete config field %s: %w", key, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("failed to create config directory %q: %w", path, err)
	}
	if err := os.WriteFile(path, []byte(newValue), 0o600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}
	return nil
}

// UpdatePreferredModel updates the preferred model for the given type and
// persists it to the config file at the given scope.
func (s *ConfigStore) UpdatePreferredModel(scope Scope, modelType SelectedModelType, model SelectedModel) error {
	s.ApplyPreferredModel(modelType, model)
	return s.PersistPreferredModel(scope, modelType, model)
}

// ApplyPreferredModel updates the in-memory preferred model and recent models.
func (s *ConfigStore) ApplyPreferredModel(modelType SelectedModelType, model SelectedModel) {
	s.config.Models[modelType] = model

	recent, changed, ok := updatedRecentModels(s.config.RecentModels[modelType], model)
	if !ok || !changed {
		return
	}
	if s.config.RecentModels == nil {
		s.config.RecentModels = make(map[SelectedModelType][]SelectedModel)
	}
	s.config.RecentModels[modelType] = recent
}

// PersistPreferredModel persists the preferred model and current recent list
// without mutating in-memory config.
func (s *ConfigStore) PersistPreferredModel(scope Scope, modelType SelectedModelType, model SelectedModel) error {
	if err := s.SetConfigField(scope, fmt.Sprintf("models.%s", modelType), model); err != nil {
		return fmt.Errorf("failed to update preferred model: %w", err)
	}
	recent := s.config.RecentModels[modelType]
	if recent == nil {
		return nil
	}
	if err := s.SetConfigField(scope, fmt.Sprintf("recent_models.%s", modelType), recent); err != nil {
		return fmt.Errorf("failed to persist recent models: %w", err)
	}
	return nil
}

// SetCompactMode sets the compact mode setting and persists it.
func (s *ConfigStore) SetCompactMode(scope Scope, enabled bool) error {
	if s.config.Options == nil {
		s.config.Options = &Options{}
	}
	if s.config.Options.TUI == nil {
		s.config.Options.TUI = &TUIOptions{}
	}
	s.config.Options.TUI.CompactMode = enabled
	return s.SetConfigField(scope, "options.tui.compact_mode", enabled)
}

// SetSkipRequests sets the skip requests (YOLO mode) setting and persists it.
func (s *ConfigStore) SetSkipRequests(scope Scope, enabled bool) error {
	if s.config.Permissions == nil {
		s.config.Permissions = &Permissions{}
	}
	s.config.Permissions.SkipRequests = enabled
	return s.SetConfigField(scope, "permissions.skip_requests", enabled)
}

// SetPreferredCollaborationMode sets the preferred interactive collaboration
// mode and persists it.
func (s *ConfigStore) SetPreferredCollaborationMode(scope Scope, mode string) error {
	if s.config.Options == nil {
		s.config.Options = &Options{}
	}
	s.config.Options.PreferredCollaborationMode = mode
	return s.SetConfigField(scope, "options.preferred_collaboration_mode", mode)
}

// SetMCPDisabled sets the disabled state for an MCP server and persists it.
func (s *ConfigStore) SetMCPDisabled(scope Scope, name string, disabled bool) error {
	mcpConfig, ok := s.config.MCP[name]
	if !ok {
		return fmt.Errorf("mcp %s not found", name)
	}
	mcpConfig.Disabled = disabled
	s.config.MCP[name] = mcpConfig
	return s.SetConfigField(scope, fmt.Sprintf("mcp.%s.disabled", name), disabled)
}

// SetTransparentBackground sets the transparent background setting and persists it.
func (s *ConfigStore) SetTransparentBackground(scope Scope, enabled bool) error {
	if s.config.Options == nil {
		s.config.Options = &Options{}
	}
	s.config.Options.TUI.Transparent = &enabled
	return s.SetConfigField(scope, "options.tui.transparent", enabled)
}

// SetProviderAPIKey sets the API key for a provider and persists it.
func (s *ConfigStore) SetProviderAPIKey(scope Scope, providerID string, apiKey any) error {
	var providerConfig ProviderConfig
	var exists bool
	var setKeyOrToken func()

	switch v := apiKey.(type) {
	case string:
		if err := s.SetConfigField(scope, fmt.Sprintf("providers.%s.api_key", providerID), v); err != nil {
			return fmt.Errorf("failed to save api key to config file: %w", err)
		}
		setKeyOrToken = func() { providerConfig.APIKey = v }
	case *oauth.Token:
		if err := cmp.Or(
			s.SetConfigField(scope, fmt.Sprintf("providers.%s.api_key", providerID), v.AccessToken),
			s.SetConfigField(scope, fmt.Sprintf("providers.%s.oauth", providerID), v),
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

	providerConfig, exists = s.config.Providers.Get(providerID)
	if exists {
		setKeyOrToken()
		s.config.Providers.Set(providerID, providerConfig)
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
	s.config.Providers.Set(providerID, providerConfig)
	return nil
}

func (s *ConfigStore) SetMCPOAuthConfig(scope Scope, mcpName string, oauthCfg *MCPOAuthConfig) error {
	mcpConfig, ok := s.config.MCP[mcpName]
	if !ok {
		return fmt.Errorf("mcp %s not found", mcpName)
	}

	if oauthCfg == nil {
		mcpConfig.OAuth = nil
		s.config.MCP[mcpName] = mcpConfig
		if !s.HasConfigField(scope, fmt.Sprintf("mcp.%s.oauth", mcpName)) {
			return nil
		}
		return s.RemoveConfigField(scope, fmt.Sprintf("mcp.%s.oauth", mcpName))
	}

	mcpConfig.OAuth = cloneMCPOAuthConfig(oauthCfg)
	s.config.MCP[mcpName] = mcpConfig
	if err := s.SetConfigField(scope, fmt.Sprintf("mcp.%s.oauth", mcpName), mcpConfig.OAuth); err != nil {
		return fmt.Errorf("failed to save mcp oauth config: %w", err)
	}
	return nil
}

func (s *ConfigStore) SetMCPOAuthToken(scope Scope, mcpName string, token *oauth.Token) error {
	oauthCfg, err := s.cloneCurrentMCPOAuthConfig(mcpName)
	if err != nil {
		return err
	}
	if token == nil && oauthCfg == nil {
		return nil
	}
	if oauthCfg == nil {
		oauthCfg = &MCPOAuthConfig{}
	}
	oauthCfg.Token = token
	return s.SetMCPOAuthConfig(scope, mcpName, oauthCfg)
}

func (s *ConfigStore) cloneCurrentMCPOAuthConfig(mcpName string) (*MCPOAuthConfig, error) {
	mcpConfig, ok := s.config.MCP[mcpName]
	if !ok {
		return nil, fmt.Errorf("mcp %s not found", mcpName)
	}
	if mcpConfig.OAuth == nil {
		return nil, nil
	}
	return cloneMCPOAuthConfig(mcpConfig.OAuth), nil
}

func cloneMCPOAuthConfig(in *MCPOAuthConfig) *MCPOAuthConfig {
	if in == nil {
		return nil
	}
	out := *in
	if in.Token != nil {
		token := *in.Token
		out.Token = &token
	}
	if in.Registration != nil {
		registration := *in.Registration
		out.Registration = &registration
	}
	if in.AuthServer != nil {
		authServer := *in.AuthServer
		out.AuthServer = &authServer
	}
	if in.Scopes != nil {
		out.Scopes = slices.Clone(in.Scopes)
	}
	return &out
}

// RefreshOAuthToken refreshes the OAuth token for the given provider.
func (s *ConfigStore) RefreshOAuthToken(ctx context.Context, scope Scope, providerID string) error {
	providerConfig, exists := s.config.Providers.Get(providerID)
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

	s.config.Providers.Set(providerID, providerConfig)

	if err := cmp.Or(
		s.SetConfigField(scope, fmt.Sprintf("providers.%s.api_key", providerID), newToken.AccessToken),
		s.SetConfigField(scope, fmt.Sprintf("providers.%s.oauth", providerID), newToken),
	); err != nil {
		return fmt.Errorf("failed to persist refreshed token: %w", err)
	}

	return nil
}

// recordRecentModel records a model in the recent models list.
func (s *ConfigStore) recordRecentModel(scope Scope, modelType SelectedModelType, model SelectedModel) error {
	updated, changed, ok := updatedRecentModels(s.config.RecentModels[modelType], model)
	if !ok || !changed {
		return nil
	}

	if s.config.RecentModels == nil {
		s.config.RecentModels = make(map[SelectedModelType][]SelectedModel)
	}
	s.config.RecentModels[modelType] = updated

	if err := s.SetConfigField(scope, fmt.Sprintf("recent_models.%s", modelType), updated); err != nil {
		return fmt.Errorf("failed to persist recent models: %w", err)
	}

	return nil
}

func updatedRecentModels(current []SelectedModel, model SelectedModel) ([]SelectedModel, bool, bool) {
	if model.Provider == "" || model.Model == "" {
		return nil, false, false
	}

	eq := func(a, b SelectedModel) bool {
		return a.Provider == b.Provider && a.Model == b.Model
	}

	entry := SelectedModel{
		Provider: model.Provider,
		Model:    model.Model,
	}

	withoutCurrent := slices.DeleteFunc(slices.Clone(current), func(existing SelectedModel) bool {
		return eq(existing, entry)
	})

	updated := append([]SelectedModel{entry}, withoutCurrent...)
	if len(updated) > maxRecentModelsPerType {
		updated = updated[:maxRecentModelsPerType]
	}

	if slices.EqualFunc(current, updated, eq) {
		return current, false, true
	}
	return updated, true, true
}

// ImportCopilot attempts to import a GitHub Copilot token from disk.
func (s *ConfigStore) ImportCopilot() (*oauth.Token, bool) {
	if s.HasConfigField(ScopeGlobal, "providers.copilot.api_key") || s.HasConfigField(ScopeGlobal, "providers.copilot.oauth") {
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

	if err := s.SetProviderAPIKey(ScopeGlobal, string(catwalk.InferenceProviderCopilot), token); err != nil {
		return token, false
	}

	if err := cmp.Or(
		s.SetConfigField(ScopeGlobal, "providers.copilot.api_key", token.AccessToken),
		s.SetConfigField(ScopeGlobal, "providers.copilot.oauth", token),
	); err != nil {
		slog.Error("Unable to save GitHub Copilot token to disk", "error", err)
	}

	slog.Info("GitHub Copilot successfully imported")
	return token, true
}
