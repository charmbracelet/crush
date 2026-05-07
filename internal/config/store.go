package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"

	"charm.land/catwalk/pkg/catwalk"
	hyperp "github.com/charmbracelet/crush/internal/agent/hyper"
	"github.com/charmbracelet/crush/internal/env"
	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/charmbracelet/crush/internal/oauth/copilot"
	"github.com/charmbracelet/crush/internal/oauth/hyper"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// fileSnapshot captures metadata about a config file at a point in time.
type fileSnapshot struct {
	Path    string
	Exists  bool
	Size    int64
	ModTime int64 // UnixNano
}

// configFileCacheEntry caches the raw bytes of a config file along with the
// stat info used to detect changes.
type configFileCacheEntry struct {
	modTime int64 // UnixNano
	size    int64
	data    []byte
}

// RuntimeOverrides holds per-session settings that are never persisted to
// disk. They are applied on top of the loaded Config and survive only for
// the lifetime of the process (or workspace).
type RuntimeOverrides struct {
	SkipPermissionRequests bool
}

// ConfigStore is the single entry point for all config access. It owns the
// pure-data Config, runtime state (working directory, resolver, known
// providers), and persistence to both global and workspace config files.
type ConfigStore struct {
	config             *Config
	workingDir         string
	resolver           VariableResolver
	globalDataPath     string   // ~/.local/share/crush/crush.json
	workspacePath      string   // .crush/crush.json
	loadedPaths        []string // config files that were successfully loaded
	knownProviders     []catwalk.Provider
	overrides          RuntimeOverrides
	trackedConfigPaths []string                // unique, normalized config file paths
	snapshots          map[string]fileSnapshot // path -> snapshot at last capture
	autoReloadDisabled bool                    // set during load/reload to prevent re-entrancy
	reloadInProgress   bool                    // set during reload to avoid disk writes mid-reload

	// configFileCache caches raw config file bytes keyed by absolute path.
	// Entries are validated against (size, mtime) on read; mismatches force
	// a fresh ReadFile. SetConfigField*/RemoveConfigField invalidate the
	// affected scope's entry.
	configFileCache map[string]configFileCacheEntry
}

// Config returns the pure-data config struct (read-only after load).
func (s *ConfigStore) Config() *Config {
	return s.config
}

// WorkingDir returns the current working directory.
func (s *ConfigStore) WorkingDir() string {
	return s.workingDir
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

// SetupAgents configures the coder and task agents on the config.
func (s *ConfigStore) SetupAgents() {
	s.config.SetupAgents()
}

// Overrides returns the runtime overrides for this store.
func (s *ConfigStore) Overrides() *RuntimeOverrides {
	return &s.overrides
}

// LoadedPaths returns the config file paths that were successfully loaded.
func (s *ConfigStore) LoadedPaths() []string {
	return slices.Clone(s.loadedPaths)
}

// configPath returns the file path for the given scope.
func (s *ConfigStore) configPath(scope Scope) (string, error) {
	switch scope {
	case ScopeWorkspace:
		if s.workspacePath == "" {
			return "", ErrNoWorkspaceConfig
		}
		return s.workspacePath, nil
	default:
		return s.globalDataPath, nil
	}
}

// readCachedConfigFile reads the config file at path, returning cached bytes
// when (size, mtime) match. Empty result with nil error means the file does
// not exist. Callers must not mutate the returned slice.
func (s *ConfigStore) readCachedConfigFile(path string) ([]byte, error) {
	if path == "" {
		return nil, nil
	}
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			delete(s.configFileCache, path)
			return nil, nil
		}
		return nil, err
	}
	if entry, ok := s.configFileCache[path]; ok {
		if entry.size == info.Size() && entry.modTime == info.ModTime().UnixNano() {
			return entry.data, nil
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if s.configFileCache == nil {
		s.configFileCache = make(map[string]configFileCacheEntry)
	}
	s.configFileCache[path] = configFileCacheEntry{
		modTime: info.ModTime().UnixNano(),
		size:    info.Size(),
		data:    data,
	}
	return data, nil
}

// invalidateConfigFileCache drops the cache entry for the given scope's file.
// Called after writes/removals to force the next read to refresh from disk.
func (s *ConfigStore) invalidateConfigFileCache(scope Scope) {
	path, err := s.configPath(scope)
	if err != nil || path == "" {
		return
	}
	delete(s.configFileCache, path)
}

// writableScopeForModel returns the scope (workspace or global) whose data
// file currently has `models.<modelType>` set, preferring workspace. Returns
// ok=false when neither writable scope contains the key, indicating the
// selection came from a non-writable location (e.g. a project-level
// crush.json) and that callers should skip persistence.
func (s *ConfigStore) writableScopeForModel(modelType SelectedModelType) (Scope, bool) {
	key := fmt.Sprintf("models.%s", modelType)
	if s.workspacePath != "" && s.HasConfigField(ScopeWorkspace, key) {
		return ScopeWorkspace, true
	}
	if s.HasConfigField(ScopeGlobal, key) {
		return ScopeGlobal, true
	}
	return ScopeGlobal, false
}

// HasConfigField checks whether a key exists in the config file for the given
// scope.
func (s *ConfigStore) HasConfigField(scope Scope, key string) bool {
	path, err := s.configPath(scope)
	if err != nil {
		return false
	}
	data, err := s.readCachedConfigFile(path)
	if err != nil || len(data) == 0 {
		return false
	}
	return gjson.GetBytes(data, key).Exists()
}

// SetConfigField sets a key/value pair in the config file for the given scope.
// After a successful write, it automatically reloads config to keep in-memory
// state fresh.
func (s *ConfigStore) SetConfigField(scope Scope, key string, value any) error {
	return s.SetConfigFields(scope, map[string]any{key: value})
}

// SetConfigFields sets multiple key/value pairs in the config file for the given
// scope in a single write. After a successful write, it automatically reloads
// config to keep in-memory state fresh. This is preferred over multiple
// SetConfigField calls when writing several fields atomically to avoid
// intermediate reloads with partial state.
func (s *ConfigStore) SetConfigFields(scope Scope, kv map[string]any) error {
	path, err := s.configPath(scope)
	if err != nil {
		return fmt.Errorf("%v: %w", kv, err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			data = []byte("{}")
		} else {
			return fmt.Errorf("failed to read config file: %w", err)
		}
	}

	newValue := string(data)
	for key, value := range kv {
		newValue, err = sjson.Set(newValue, key, value)
		if err != nil {
			return fmt.Errorf("failed to set config field %s: %w", key, err)
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("failed to create config directory %q: %w", path, err)
	}
	if err := os.WriteFile(path, []byte(newValue), 0o600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}
	s.invalidateConfigFileCache(scope)

	// Auto-reload to keep in-memory state fresh after config edits.
	// We use context.Background() since this is an internal operation that
	// shouldn't be cancelled by user context.
	if err := s.autoReload(context.Background()); err != nil {
		// Log warning but don't fail the write - disk is already updated.
		slog.Warn("Config file updated but failed to reload in-memory state", "error", err)
	}

	return nil
}

// RemoveConfigField removes a key from the config file for the given scope.
// After a successful write, it automatically reloads config to keep in-memory
// state fresh.
func (s *ConfigStore) RemoveConfigField(scope Scope, key string) error {
	path, err := s.configPath(scope)
	if err != nil {
		return fmt.Errorf("%s: %w", key, err)
	}
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
	s.invalidateConfigFileCache(scope)

	// Auto-reload to keep in-memory state fresh after config edits.
	if err := s.autoReload(context.Background()); err != nil {
		slog.Warn("Config file updated but failed to reload in-memory state", "error", err)
	}

	return nil
}

// UpdatePreferredModel updates the preferred model for the given type and
// persists it to the config file at the given scope.
func (s *ConfigStore) UpdatePreferredModel(scope Scope, modelType SelectedModelType, model SelectedModel) error {
	s.config.Models[modelType] = model
	if err := s.SetConfigField(scope, fmt.Sprintf("models.%s", modelType), model); err != nil {
		return fmt.Errorf("failed to update preferred model: %w", err)
	}
	if err := s.recordRecentModel(scope, modelType, model); err != nil {
		return err
	}
	return nil
}

var ErrNoModelChoicesToSave = errors.New("no model choices to save")

// pruneInvalidRecentModels drops entries from `recent_models.<type>` whose
// (provider, model) is no longer resolvable in the current configuration.
// Writes go to whichever writable scope (workspace or global) currently owns
// the array; if neither scope has it on disk, no write is performed.
func (s *ConfigStore) pruneInvalidRecentModels() error {
	if s.config == nil {
		return nil
	}
	for modelType, recents := range s.config.RecentModels {
		valid := make([]SelectedModel, 0, len(recents))
		for _, r := range recents {
			if s.config.GetModel(r.Provider, r.Model) != nil {
				valid = append(valid, r)
			}
		}
		if len(valid) == len(recents) {
			continue
		}
		// In-memory always reflects the pruned list.
		s.config.RecentModels[modelType] = valid
		// Determine which scope owns the on-disk recent list and write
		// only to that scope. Skip persistence if neither scope has the
		// field (e.g. the array originated from a project-level config).
		key := fmt.Sprintf("recent_models.%s", modelType)
		var scope Scope
		switch {
		case s.workspacePath != "" && s.HasConfigField(ScopeWorkspace, key):
			scope = ScopeWorkspace
		case s.HasConfigField(ScopeGlobal, key):
			scope = ScopeGlobal
		default:
			continue
		}
		if err := s.SetConfigField(scope, key, valid); err != nil {
			return fmt.Errorf("failed to prune recent_models.%s in %s scope: %w", modelType, scope, err)
		}
	}
	return nil
}

// SaveModelChoicesAsDefault persists the current effective model choices to the
// global data config. Writes are merged into the existing global file
// per-key (e.g. `models.large`, `recent_models.small`) so that pre-existing
// keys not present in the in-memory maps are preserved.
func (s *ConfigStore) SaveModelChoicesAsDefault() error {
	if len(s.config.Models) == 0 && len(s.config.RecentModels) == 0 {
		return ErrNoModelChoicesToSave
	}
	fields := make(map[string]any, len(s.config.Models)+len(s.config.RecentModels))
	for modelType, model := range s.config.Models {
		fields[fmt.Sprintf("models.%s", modelType)] = model
	}
	for modelType, recents := range s.config.RecentModels {
		fields[fmt.Sprintf("recent_models.%s", modelType)] = recents
	}
	if err := s.SetConfigFields(ScopeGlobal, fields); err != nil {
		return fmt.Errorf("failed to save model choices as defaults: %w", err)
	}
	return nil
}

// SetCompactMode sets the compact mode setting and persists it.
func (s *ConfigStore) SetCompactMode(scope Scope, enabled bool) error {
	if s.config.Options == nil {
		s.config.Options = &Options{}
	}
	s.config.Options.TUI.CompactMode = enabled
	return s.SetConfigField(scope, "options.tui.compact_mode", enabled)
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
		if err := s.SetConfigFields(scope, map[string]any{
			fmt.Sprintf("providers.%s.api_key", providerID): v.AccessToken,
			fmt.Sprintf("providers.%s.oauth", providerID):   v,
		}); err != nil {
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

// RefreshOAuthToken refreshes the OAuth token for the given provider.
// Before making an external refresh request, it checks the config file on
// disk to see if another Crush session has already refreshed the token. If
// a newer token is found, it is used instead of refreshing.
func (s *ConfigStore) RefreshOAuthToken(ctx context.Context, scope Scope, providerID string) error {
	providerConfig, exists := s.config.Providers.Get(providerID)
	if !exists {
		return fmt.Errorf("provider %s not found", providerID)
	}

	if providerConfig.OAuthToken == nil {
		return fmt.Errorf("provider %s does not have an OAuth token", providerID)
	}

	// Check if another session refreshed the token recently by reading
	// the current token from the config file on disk.
	newToken, err := s.loadTokenFromDisk(scope, providerID)
	if err != nil {
		slog.Warn("Failed to read token from config file, proceeding with refresh", "provider", providerID, "error", err)
	} else if newToken != nil && newToken.AccessToken != providerConfig.OAuthToken.AccessToken {
		slog.Info("Using token refreshed by another session", "provider", providerID)
		providerConfig.OAuthToken = newToken
		providerConfig.APIKey = newToken.AccessToken
		if providerID == string(catwalk.InferenceProviderCopilot) {
			providerConfig.SetupGitHubCopilot()
		}
		s.config.Providers.Set(providerID, providerConfig)
		return nil
	}

	var refreshedToken *oauth.Token
	var refreshErr error
	switch providerID {
	case string(catwalk.InferenceProviderCopilot):
		refreshedToken, refreshErr = copilot.RefreshToken(ctx, providerConfig.OAuthToken.RefreshToken)
	case hyperp.Name:
		refreshedToken, refreshErr = hyper.ExchangeToken(ctx, providerConfig.OAuthToken.RefreshToken)
	default:
		return fmt.Errorf("OAuth refresh not supported for provider %s", providerID)
	}
	if refreshErr != nil {
		return fmt.Errorf("failed to refresh OAuth token for provider %s: %w", providerID, refreshErr)
	}

	slog.Info("Successfully refreshed OAuth token", "provider", providerID)
	providerConfig.OAuthToken = refreshedToken
	providerConfig.APIKey = refreshedToken.AccessToken

	switch providerID {
	case string(catwalk.InferenceProviderCopilot):
		providerConfig.SetupGitHubCopilot()
	}

	s.config.Providers.Set(providerID, providerConfig)

	if err := s.SetConfigFields(scope, map[string]any{
		fmt.Sprintf("providers.%s.api_key", providerID): refreshedToken.AccessToken,
		fmt.Sprintf("providers.%s.oauth", providerID):   refreshedToken,
	}); err != nil {
		return fmt.Errorf("failed to persist refreshed token: %w", err)
	}

	return nil
}

// loadTokenFromDisk reads the OAuth token for the given provider from the
// config file on disk. Returns nil if the token is not found or matches the
// current in-memory token.
func (s *ConfigStore) loadTokenFromDisk(scope Scope, providerID string) (*oauth.Token, error) {
	path, err := s.configPath(scope)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	oauthKey := fmt.Sprintf("providers.%s.oauth", providerID)
	oauthResult := gjson.Get(string(data), oauthKey)
	if !oauthResult.Exists() {
		return nil, nil
	}

	var token oauth.Token
	if err := json.Unmarshal([]byte(oauthResult.Raw), &token); err != nil {
		return nil, err
	}

	if token.AccessToken == "" {
		return nil, nil
	}

	return &token, nil
}

func sameSelectedModel(a, b SelectedModel) bool {
	return a.Provider == b.Provider && a.Model == b.Model
}

func normalizeRecentModels(models []SelectedModel) []SelectedModel {
	var normalized []SelectedModel
	for _, model := range models {
		if model.Provider == "" || model.Model == "" || slices.ContainsFunc(normalized, func(existing SelectedModel) bool {
			return sameSelectedModel(existing, model)
		}) {
			continue
		}
		normalized = append(normalized, SelectedModel{Provider: model.Provider, Model: model.Model})
		if len(normalized) == maxRecentModelsPerType {
			break
		}
	}
	return normalized
}

// recordRecentModel records a model in the recent models list.
func (s *ConfigStore) recordRecentModel(scope Scope, modelType SelectedModelType, model SelectedModel) error {
	if model.Provider == "" || model.Model == "" {
		return nil
	}

	if s.config.RecentModels == nil {
		s.config.RecentModels = make(map[SelectedModelType][]SelectedModel)
	}

	entry := SelectedModel{
		Provider: model.Provider,
		Model:    model.Model,
	}

	// For computing the persisted list, base the new array on the
	// scope-specific on-disk recent_models array (not the merged in-memory
	// list). Otherwise a workspace write would inherit recent entries that
	// only exist in the global file, leaking global state into the
	// workspace file.
	stored, err := s.readRecentModelsFromScope(scope, modelType)
	if err != nil {
		return fmt.Errorf("failed to read recent models from %s scope: %w", scope, err)
	}
	updated := normalizeRecentModels(append([]SelectedModel{entry}, stored...))

	// In-memory recent list still reflects the merged view used for
	// rendering: prepend the entry to the merged list.
	current := s.config.RecentModels[modelType]
	merged := normalizeRecentModels(append([]SelectedModel{entry}, current...))

	if slices.EqualFunc(stored, updated, sameSelectedModel) &&
		slices.EqualFunc(current, merged, sameSelectedModel) {
		return nil
	}

	s.config.RecentModels[modelType] = merged

	if err := s.SetConfigField(scope, fmt.Sprintf("recent_models.%s", modelType), updated); err != nil {
		return fmt.Errorf("failed to persist recent models: %w", err)
	}

	return nil
}

// readRecentModelsFromScope reads recent_models.<modelType> from the on-disk
// JSON for the given scope. Returns an empty slice when the scope's file or
// key is absent. This is used to compute writes against the scope's own state
// rather than the merged in-memory view.
func (s *ConfigStore) readRecentModelsFromScope(scope Scope, modelType SelectedModelType) ([]SelectedModel, error) {
	path, err := s.configPath(scope)
	if err != nil {
		return nil, err
	}
	data, err := s.readCachedConfigFile(path)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, nil
	}
	raw := gjson.GetBytes(data, fmt.Sprintf("recent_models.%s", modelType))
	if !raw.Exists() {
		return nil, nil
	}
	var out []SelectedModel
	if err := json.Unmarshal([]byte(raw.Raw), &out); err != nil {
		return nil, fmt.Errorf("failed to parse recent_models.%s: %w", modelType, err)
	}
	return out, nil
}

// NewTestStore creates a ConfigStore for testing purposes.
func NewTestStore(cfg *Config, loadedPaths ...string) *ConfigStore {
	return &ConfigStore{
		config:      cfg,
		loadedPaths: loadedPaths,
	}
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

	if err := s.SetConfigFields(ScopeGlobal, map[string]any{
		"providers.copilot.api_key": token.AccessToken,
		"providers.copilot.oauth":   token,
	}); err != nil {
		slog.Error("Unable to save GitHub Copilot token to disk", "error", err)
	}

	slog.Info("GitHub Copilot successfully imported")
	return token, true
}

// StalenessResult contains the result of a staleness check.
type StalenessResult struct {
	Dirty   bool
	Changed []string
	Missing []string
	Errors  map[string]error // stat errors by path
}

// ConfigStaleness checks whether any tracked config files have changed on disk
// since the last snapshot. Returns dirty=true if any files changed or went
// missing, along with sorted lists of affected paths. Stat errors are
// captured in Errors map but still treated as non-existence for dirty detection.
func (s *ConfigStore) ConfigStaleness() StalenessResult {
	var result StalenessResult
	result.Errors = make(map[string]error)

	for _, path := range s.trackedConfigPaths {
		snapshot, hadSnapshot := s.snapshots[path]

		info, err := os.Stat(path)
		exists := err == nil && !info.IsDir()

		if err != nil && !os.IsNotExist(err) {
			// Capture permission/IO errors separately from non-existence
			result.Errors[path] = err
			result.Dirty = true
		}

		if !exists {
			if hadSnapshot && snapshot.Exists {
				// File existed before but now missing
				result.Missing = append(result.Missing, path)
				result.Dirty = true
			}
			continue
		}

		// File exists now
		if !hadSnapshot || !snapshot.Exists {
			// File didn't exist before but does now
			result.Changed = append(result.Changed, path)
			result.Dirty = true
			continue
		}

		// Check for content or metadata changes
		if snapshot.Size != info.Size() || snapshot.ModTime != info.ModTime().UnixNano() {
			result.Changed = append(result.Changed, path)
			result.Dirty = true
		}
	}

	// Sort for deterministic output
	slices.Sort(result.Changed)
	slices.Sort(result.Missing)

	return result
}

// RefreshStalenessSnapshot captures fresh snapshots of all tracked config files.
// Call this after reloading config to clear dirty state.
func (s *ConfigStore) RefreshStalenessSnapshot() error {
	if s.snapshots == nil {
		s.snapshots = make(map[string]fileSnapshot)
	}

	for _, path := range s.trackedConfigPaths {
		info, err := os.Stat(path)
		exists := err == nil && !info.IsDir()

		snapshot := fileSnapshot{
			Path:   path,
			Exists: exists,
		}

		if exists {
			snapshot.Size = info.Size()
			snapshot.ModTime = info.ModTime().UnixNano()
		}

		s.snapshots[path] = snapshot
	}

	return nil
}

// CaptureStalenessSnapshot captures snapshots for the given paths, building the
// tracked config paths list. Paths are deduplicated and normalized.
func (s *ConfigStore) CaptureStalenessSnapshot(paths []string) {
	// Build unique set of normalized paths
	seen := make(map[string]struct{})
	for _, p := range paths {
		if p == "" {
			continue
		}
		// Normalize path
		abs, err := filepath.Abs(p)
		if err != nil {
			abs = p
		}
		seen[abs] = struct{}{}
	}

	// Also track workspace and global config paths if set
	if s.workspacePath != "" {
		abs, err := filepath.Abs(s.workspacePath)
		if err == nil {
			seen[abs] = struct{}{}
		}
	}
	if s.globalDataPath != "" {
		abs, err := filepath.Abs(s.globalDataPath)
		if err == nil {
			seen[abs] = struct{}{}
		}
	}

	// Build sorted list for deterministic ordering
	s.trackedConfigPaths = make([]string, 0, len(seen))
	for p := range seen {
		s.trackedConfigPaths = append(s.trackedConfigPaths, p)
	}
	slices.Sort(s.trackedConfigPaths)

	// Capture initial snapshots
	s.RefreshStalenessSnapshot()
}

// captureStalenessSnapshot is an alias for CaptureStalenessSnapshot for internal use.
func (s *ConfigStore) captureStalenessSnapshot(paths []string) {
	s.CaptureStalenessSnapshot(paths)
}

// ReloadFromDisk re-runs the config load/merge flow and updates the in-memory
// config atomically. It rebuilds the staleness snapshot after successful reload.
// On failure, the store state is rolled back to its previous state.
func (s *ConfigStore) ReloadFromDisk(ctx context.Context) error {
	if s.workingDir == "" {
		return fmt.Errorf("cannot reload: working directory not set")
	}

	// Disable auto-reload during reload to prevent nested/re-entrant calls.
	s.autoReloadDisabled = true
	s.reloadInProgress = true
	defer func() {
		s.autoReloadDisabled = false
		s.reloadInProgress = false
	}()

	configPaths := lookupConfigs(s.workingDir)
	cfg, loadedPaths, err := loadFromConfigPaths(configPaths)
	if err != nil {
		return fmt.Errorf("failed to reload config: %w", err)
	}

	// Apply defaults (using existing data directory if set)
	var dataDir string
	if s.config != nil && s.config.Options != nil {
		dataDir = s.config.Options.DataDirectory
	}
	cfg.setDefaults(s.workingDir, dataDir)

	// Merge workspace config if present, preserving per-type recent_models
	// override semantics (workspace arrays replace global ones rather than
	// concatenate).
	workspacePath := filepath.Join(cfg.Options.DataDirectory, fmt.Sprintf("%s.json", appName))
	if wsData, err := os.ReadFile(workspacePath); err == nil && len(wsData) > 0 {
		merged, mergeErr := loadWorkspaceOverride(cfg, wsData, s.workingDir, cfg.Options.DataDirectory)
		if mergeErr == nil {
			*cfg = *merged
			loadedPaths = append(loadedPaths, workspacePath)
		}
	}

	// Validate hooks after all config merging is complete so matcher
	// regexes are recompiled on the reloaded config (mirrors Load).
	if err := cfg.ValidateHooks(); err != nil {
		return fmt.Errorf("invalid hook configuration on reload: %w", err)
	}

	// Preserve runtime overrides
	overrides := s.overrides

	// Reconfigure providers
	env := env.New()
	resolver := NewShellVariableResolver(env)
	providers, err := Providers(cfg)
	if err != nil {
		return fmt.Errorf("failed to load providers during reload: %w", err)
	}

	if err := cfg.configureProviders(s, env, resolver, providers); err != nil {
		return fmt.Errorf("failed to configure providers during reload: %w", err)
	}

	// Save current state for potential rollback
	oldConfig := s.config
	oldLoadedPaths := s.loadedPaths
	oldResolver := s.resolver
	oldKnownProviders := s.knownProviders
	oldOverrides := s.overrides
	oldWorkspacePath := s.workspacePath

	// Update store state BEFORE running model/agent setup (so they see new config)
	s.config = cfg
	s.loadedPaths = loadedPaths
	s.resolver = resolver
	s.knownProviders = providers
	s.overrides = overrides
	s.workspacePath = workspacePath

	// Mirror startup flow: setup models and agents against NEW config
	var setupErr error
	if !cfg.IsConfigured() {
		slog.Warn("No providers configured after reload")
	} else {
		if err := configureSelectedModels(s, providers, false); err != nil {
			setupErr = fmt.Errorf("failed to configure selected models during reload: %w", err)
		} else {
			s.SetupAgents()
		}
	}

	// Rollback on setup failure
	if setupErr != nil {
		s.config = oldConfig
		s.loadedPaths = oldLoadedPaths
		s.resolver = oldResolver
		s.knownProviders = oldKnownProviders
		s.overrides = oldOverrides
		s.workspacePath = oldWorkspacePath
		return setupErr
	}

	// Rebuild staleness tracking
	s.captureStalenessSnapshot(loadedPaths)

	return nil
}

// autoReload conditionally reloads config from disk after writes.
// It returns nil (no error) for expected skip cases: when auto-reload is
// disabled during load/reload flows, or when working directory is not set
// (e.g., during testing). Only actual reload failures return an error.
func (s *ConfigStore) autoReload(ctx context.Context) error {
	if s.autoReloadDisabled {
		return nil // Expected skip: already in load/reload flow
	}
	if s.workingDir == "" {
		return nil // Expected skip: working directory not set
	}
	return s.ReloadFromDisk(ctx)
}
