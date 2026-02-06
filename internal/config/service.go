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
}

// Config returns the underlying Config struct. This is a temporary
// escape hatch that will be removed once all callers migrate to
// Service getter methods.
func (s *Service) Config() *Config {
	return s.cfg
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
	providerConfig, exists := cfg.Providers.Get(providerID)
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

	cfg.Providers.Set(providerID, providerConfig)

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

	providerConfig, exists = cfg.Providers.Get(providerID)
	if exists {
		setKeyOrToken()
		cfg.Providers.Set(providerID, providerConfig)
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
	cfg.Providers.Set(providerID, providerConfig)
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
