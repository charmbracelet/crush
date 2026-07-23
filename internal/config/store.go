package config

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"charm.land/catwalk/pkg/catwalk"
	hyperp "github.com/charmbracelet/crush/internal/agent/hyper"
	"github.com/charmbracelet/crush/internal/env"
	"github.com/charmbracelet/crush/internal/lock"
	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/charmbracelet/crush/internal/oauth/copilot"
	"github.com/charmbracelet/crush/internal/oauth/hyper"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	"golang.org/x/sync/singleflight"
)

// configLockDeadline bounds how long lockConfig waits for the
// cross-process flock before giving up. A few seconds is plenty for
// honest contention; longer suggests something is wedged.
const configLockDeadline = 5 * time.Second

// refreshLockDeadline bounds how long RefreshOAuthToken waits for the
// per-provider cross-process refresh lock. It must exceed the token
// exchange HTTP timeout (30s) so that a peer mid-exchange is given time
// to finish and publish its result, which we then adopt instead of
// running our own exchange. Running our own would reuse an
// already-rotated refresh token and trip the provider's reuse detection,
// revoking the whole token family.
const refreshLockDeadline = 45 * time.Second

// fileSnapshot captures metadata about a config file at a point in time.
type fileSnapshot struct {
	Path    string
	Exists  bool
	Size    int64
	ModTime int64 // UnixNano
}

// RuntimeOverrides holds per-session settings that are never persisted to
// disk. They are applied on top of the loaded Config and survive only for
// the lifetime of the process (or workspace).
type RuntimeOverrides struct {
	SkipPermissionRequests bool
	// EnabledChannels lists the MCP servers opted in as channels for this
	// session (via the --channels flag). A server present in MCP config only
	// pushes channel events when it also appears here. Entries may be written
	// as "server:<name>" or as a bare "<name>".
	EnabledChannels []string
}

// ConfigStore is the single entry point for all config access. It owns the
// pure-data Config, runtime state (working directory, resolver, known
// providers), and persistence to both global and workspace config files.
//
// mu serialises all config file mutations (SetConfigFields,
// RemoveConfigField, RefreshOAuthToken) to prevent both in-process
// goroutine races and, together with the shared lock.File, cross-process
// races on the config file.
//
// writeMu serialises every operation that produces a new in-memory Config:
// the typed copy-on-write mutators (SetCompactMode, UpdatePreferredModel,
// ...) and ReloadFromDisk. Typed mutators take Lock; autoReload takes
// TryLock so a write triggered re-entrantly during a reload (e.g.
// configureProviders calling RemoveConfigField) skips the nested reload
// instead of deadlocking. This is what lets published Configs be treated
// as immutable: a mutator clones, mutates the clone, and swaps it in under
// writeMu rather than mutating the live Config in place.
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

	// configMu guards the config pointer field against concurrent
	// readers (Config) and the writeMu-serialised swap (setConfig). It
	// protects the pointer word only; the pointed-to Config is treated
	// as immutable once published, since both reloads and typed mutators
	// build a fresh Config rather than mutating the live one.
	configMu sync.RWMutex

	mu      sync.Mutex   // serialises config file writes
	writeMu sync.RWMutex // serialises in-memory config production (mutators + reload); RLock for readers

	// refreshSF collapses concurrent in-process OAuth refreshes for the
	// same provider into a single attempt. Combined with the per-provider
	// cross-process refresh lock, it ensures only one token exchange runs
	// at a time. See RefreshOAuthToken.
	refreshSF singleflight.Group

	// exchangeToken performs the provider-specific OAuth token exchange.
	// It is a field so tests can substitute a fake exchange without making
	// real network calls. Production code leaves it nil, and exchange falls
	// back to the real provider clients.
	exchangeToken func(ctx context.Context, providerID, refreshToken string) (*oauth.Token, error)

	// authSignalMu guards authSignals, which maps provider IDs to
	// channels that WaitForTokenChange blocks on. SignalAuthComplete
	// closes the channel to unblock waiters; a new channel is created
	// on the next wait.
	authSignalMu sync.Mutex
	authSignals  map[string]chan struct{}
}

// Config returns the pure-data config struct (read-only after load).
//
// The pointer read is guarded by configMu so it can never tear against
// the reload swap in reloadFromDiskLocked. Reloads build a brand-new
// Config and swap it in rather than mutating the live one, so holding the
// returned pointer stays safe even across a concurrent reload — the reader
// keeps reading its (now immutable) snapshot.
func (s *ConfigStore) Config() *Config {
	s.configMu.RLock()
	defer s.configMu.RUnlock()
	return s.config
}

// setConfig atomically swaps the active config pointer under configMu.
// Used by the reload path; in-place field mutators leave the pointer
// untouched and run under mu instead.
func (s *ConfigStore) setConfig(cfg *Config) {
	s.configMu.Lock()
	defer s.configMu.Unlock()
	s.config = cfg
}

// WorkingDir returns the current working directory.
func (s *ConfigStore) WorkingDir() string {
	return s.workingDir
}

// Resolver returns the variable resolver.
func (s *ConfigStore) Resolver() VariableResolver {
	s.writeMu.RLock()
	defer s.writeMu.RUnlock()
	return s.resolver
}

// Resolve resolves a variable reference using the configured resolver.
func (s *ConfigStore) Resolve(key string) (string, error) {
	s.writeMu.RLock()
	r := s.resolver
	s.writeMu.RUnlock()
	if r == nil {
		return "", fmt.Errorf("no variable resolver configured")
	}
	return r.ResolveValue(key)
}

// KnownProviders returns the list of known providers.
func (s *ConfigStore) KnownProviders() []catwalk.Provider {
	s.writeMu.RLock()
	defer s.writeMu.RUnlock()
	return s.knownProviders
}

// SetupAgents configures the coder and task agents on the config.
func (s *ConfigStore) SetupAgents() {
	s.Config().SetupAgents()
}

// Overrides returns the runtime overrides for this store.
func (s *ConfigStore) Overrides() *RuntimeOverrides {
	s.writeMu.RLock()
	defer s.writeMu.RUnlock()
	return &s.overrides
}

// LoadedPaths returns the config file paths that were successfully loaded.
func (s *ConfigStore) LoadedPaths() []string {
	s.writeMu.RLock()
	defer s.writeMu.RUnlock()
	return slices.Clone(s.loadedPaths)
}

// lockConfig acquires both the in-process mutex and a cross-process flock
// on the config file for the given scope. Callers that need to do I/O
// between reading and writing (e.g. an HTTP token exchange) must use
// lockConfig explicitly rather than atomicWrite.
//
// The returned release function drops both locks. Callers must call it
// as soon as the file access is complete — no I/O should be performed
// while the lock is held.
func (s *ConfigStore) lockConfig(scope Scope) (func(), error) {
	s.mu.Lock()
	path, err := s.configPath(scope)
	if err != nil {
		s.mu.Unlock()
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		s.mu.Unlock()
		return nil, fmt.Errorf("create config directory: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), configLockDeadline)
	defer cancel()
	release, err := lock.File(ctx, path+".lock")
	if err != nil {
		s.mu.Unlock()
		return nil, fmt.Errorf("acquire config lock: %w", err)
	}
	return func() {
		release()
		s.mu.Unlock()
	}, nil
}

// atomicWrite handles the lock-read-transform-write-unlock cycle for
// config file mutations. The fn callback receives the current file
// contents (raw bytes, or {} if the file is missing) and must return the
// new contents. fn must be pure — no I/O, no network calls.
func (s *ConfigStore) atomicWrite(scope Scope, fn func(current []byte) ([]byte, error)) error {
	unlock, err := s.lockConfig(scope)
	if err != nil {
		return err
	}
	defer unlock()

	path, err := s.configPath(scope)
	if err != nil {
		return err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			data = []byte("{}")
		} else {
			return fmt.Errorf("read config file: %w", err)
		}
	}

	newData, err := fn(data)
	if err != nil {
		return err
	}

	return atomicWriteFile(path, newData, 0o600)
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

// HasConfigField checks whether a key exists in the config file for the given
// scope.
func (s *ConfigStore) HasConfigField(scope Scope, key string) bool {
	path, err := s.configPath(scope)
	if err != nil {
		return false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return gjson.Get(string(data), key).Exists()
}

// SetConfigField sets a key/value pair in the config file for the given scope.
// After a successful write, it automatically reloads config to keep in-memory
// state fresh.
func (s *ConfigStore) SetConfigField(scope Scope, key string, value any) error {
	return s.SetConfigFields(scope, map[string]any{key: value})
}

// SetConfigFields sets multiple key/value pairs in the config file for the
// given scope in a single write, then reloads in-memory state from disk.
//
// Use this for arbitrary external edits where the in-memory effect of the
// change is not known ahead of time. The typed mutators (which know exactly
// what changed) go through update instead and skip the reload.
//
// The write is protected by an in-process mutex and a cross-process flock
// to prevent races between concurrent writers in different processes.
func (s *ConfigStore) SetConfigFields(scope Scope, kv map[string]any) error {
	if err := s.writeConfigFields(scope, kv); err != nil {
		return err
	}
	// Auto-reload to keep in-memory state fresh after config edits.
	// We use context.Background() since this is an internal operation that
	// shouldn't be cancelled by user context.
	if err := s.autoReload(context.Background()); err != nil {
		// Log warning but don't fail the write - disk is already updated.
		slog.Warn("Config file updated but failed to reload in-memory state", "error", err)
	}
	return nil
}

// writeConfigFields persists key/value pairs to the config file. It does not
// touch in-memory config state or the staleness snapshot: callers either
// reload (SetConfigFields, whose reload recaptures the snapshot) or have
// already published an updated clone and capture the snapshot themselves
// (update). Both of those run under writeMu, which is what keeps the
// snapshot map free of concurrent writers.
func (s *ConfigStore) writeConfigFields(scope Scope, kv map[string]any) error {
	// Sort keys for deterministic output regardless of map iteration
	// order. This also ensures consistent results when callers pass
	// overlapping JSONPath keys (e.g. "a" and "a.b").
	keys := make([]string, 0, len(kv))
	for k := range kv {
		keys = append(keys, k)
	}
	slices.Sort(keys)

	return s.atomicWrite(scope, func(data []byte) ([]byte, error) {
		v := string(data)
		for _, key := range keys {
			var sErr error
			if v, sErr = sjson.Set(v, key, kv[key]); sErr != nil {
				return nil, fmt.Errorf("failed to set config field %s: %w", key, sErr)
			}
		}
		return []byte(v), nil
	})
}

// mutateInMemory applies a copy-on-write change to the config without
// persisting. Under writeMu it clones the live config, lets mutate edit the
// clone, and publishes it. This is the single primitive every in-memory
// config change goes through, so a published Config is never mutated in
// place and readers always see a consistent snapshot.
func (s *ConfigStore) mutateInMemory(mutate func(*Config)) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	nc := s.Config().cloneForWrite()
	mutate(nc)
	s.setConfig(nc)
}

// update applies a copy-on-write change and persists the reported fields.
// mutate edits the clone and returns the JSON-path fields to write to disk;
// because the clone already reflects the change, no reload is needed.
// Returning an empty map publishes the clone without a disk write.
func (s *ConfigStore) update(scope Scope, mutate func(*Config) map[string]any) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return s.updateLocked(scope, mutate)
}

// updateLocked is the lock-free core of update. Caller must hold writeMu.
func (s *ConfigStore) updateLocked(scope Scope, mutate func(*Config) map[string]any) error {
	nc := s.Config().cloneForWrite()
	fields := mutate(nc)
	s.setConfig(nc)
	if len(fields) == 0 {
		return nil
	}
	if err := s.writeConfigFields(scope, fields); err != nil {
		return err
	}
	// Refresh the staleness snapshot so the file watcher does not treat
	// our own write as an external change. Safe to touch the snapshot map
	// here because we hold writeMu.
	if path, err := s.configPath(scope); err == nil {
		s.captureStalenessSnapshot(append(slices.Clone(s.loadedPaths), path))
	}
	return nil
}

// OverridePreferredModel sets the preferred model for the given type in
// memory only, without persisting. It is for per-run overrides (such as the
// non-interactive --model flags) that must not be written to the user's
// config file.
func (s *ConfigStore) OverridePreferredModel(modelType SelectedModelType, model SelectedModel) {
	s.mutateInMemory(func(c *Config) {
		if c.Models == nil {
			c.Models = make(map[SelectedModelType]SelectedModel)
		}
		c.Models[modelType] = model
	})
}

// RemoveConfigField removes a key from the config file for the given scope.
// After a successful write, it automatically reloads config to keep in-memory
// state fresh.
//
// The write is protected by an in-process mutex and a cross-process flock.
func (s *ConfigStore) RemoveConfigField(scope Scope, key string) error {
	err := s.atomicWrite(scope, func(data []byte) ([]byte, error) {
		v, sErr := sjson.Delete(string(data), key)
		if sErr != nil {
			return nil, fmt.Errorf("failed to delete config field %s: %w", key, sErr)
		}
		return []byte(v), nil
	})
	if err != nil {
		return err
	}

	if err := s.autoReload(context.Background()); err != nil {
		slog.Warn("Config file updated but failed to reload in-memory state", "error", err)
	}

	return nil
}

// UpdatePreferredModel updates the preferred model for the given type and
// persists it to the config file at the given scope. The selected model and
// the recent-models list are written together in a single config write.
//
// The write skips the full disk reparse/reload (which would rebuild the
// provider catalog and agents on every model switch and dominate selection
// latency); agents are refreshed separately by the caller (see
// UpdateAgentModel).
func (s *ConfigStore) UpdatePreferredModel(scope Scope, modelType SelectedModelType, model SelectedModel) error {
	return s.update(scope, func(c *Config) map[string]any {
		return s.updatePreferredModelFields(c, modelType, model)
	})
}

// updatePreferredModelFields builds the fields map for persisting a preferred
// model change. Shared between UpdatePreferredModel and direct updateLocked
// callers (e.g. Load).
func (s *ConfigStore) updatePreferredModelFields(c *Config, modelType SelectedModelType, model SelectedModel) map[string]any {
	if c.Models == nil {
		c.Models = make(map[SelectedModelType]SelectedModel)
	}
	c.Models[modelType] = model

	fields := map[string]any{
		fmt.Sprintf("models.%s", modelType): model,
	}
	if updated, changed := nextRecentModels(c, modelType, model); changed {
		if c.RecentModels == nil {
			c.RecentModels = make(map[SelectedModelType][]SelectedModel)
		}
		c.RecentModels[modelType] = updated
		fields[fmt.Sprintf("recent_models.%s", modelType)] = updated
	}
	return fields
}

// SetCompactMode sets the compact mode setting and persists it.
func (s *ConfigStore) SetCompactMode(scope Scope, enabled bool) error {
	return s.update(scope, func(c *Config) map[string]any {
		c.ensureTUI().CompactMode = enabled
		return map[string]any{"options.tui.compact_mode": enabled}
	})
}

// SetTransparentBackground sets the transparent background setting and persists it.
func (s *ConfigStore) SetTransparentBackground(scope Scope, enabled bool) error {
	return s.update(scope, func(c *Config) map[string]any {
		c.ensureTUI().Transparent = &enabled
		return map[string]any{"options.tui.transparent": enabled}
	})
}

// SetScrollbar sets the chat scrollbar visibility and persists it.
func (s *ConfigStore) SetScrollbar(scope Scope, style string) error {
	return s.update(scope, func(c *Config) map[string]any {
		c.ensureTUI().Scrollbar = style
		return map[string]any{"options.tui.scrollbar": style}
	})
}

// SetNotificationStyle sets the notification style and persists it.
func (s *ConfigStore) SetNotificationStyle(scope Scope, style string) error {
	return s.update(scope, func(c *Config) map[string]any {
		c.ensureOptions().NotificationStyle = style
		return map[string]any{"options.notification_style": style}
	})
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

	cfg := s.Config()
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

// RefreshOAuthToken refreshes the OAuth token for the given provider.
//
// Providers like Hyper rotate refresh tokens: each exchange consumes the
// caller's refresh token, issues a new pair, and revokes the old one. If
// two crush instances (or two goroutines) refresh concurrently with the
// same stored refresh token, the second exchange reuses an already-revoked
// token, trips the provider's reuse detection, and revokes the entire
// token family — leaving both with dead tokens even though each refresh
// "succeeded".
//
// To prevent that, refreshes are single-flighted at two levels:
//
//   - In-process: refreshSF collapses concurrent goroutines for the same
//     provider into one attempt.
//   - Cross-process: a per-provider advisory lock is held across the whole
//     read-decide-exchange-write cycle, so only one process exchanges at a
//     time. A process that acquires the lock after a peer rotated finds the
//     peer's fresh token on disk and adopts it instead of exchanging.
func (s *ConfigStore) RefreshOAuthToken(ctx context.Context, scope Scope, providerID string) error {
	key := fmt.Sprintf("%d\x00%s", scope, providerID)
	_, err, _ := s.refreshSF.Do(key, func() (any, error) {
		return nil, s.refreshOAuthTokenLocked(ctx, scope, providerID)
	})
	return err
}

// refreshOAuthTokenLocked performs the cross-process single-flighted
// refresh. It is invoked through refreshSF, so at most one goroutine per
// provider runs it at a time within this process.
func (s *ConfigStore) refreshOAuthTokenLocked(ctx context.Context, scope Scope, providerID string) error {
	cfg := s.Config()
	providerConfig, exists := cfg.Providers.Get(providerID)
	if !exists {
		return fmt.Errorf("provider %s not found", providerID)
	}
	if providerConfig.OAuthToken == nil {
		return fmt.Errorf("provider %s does not have an OAuth token", providerID)
	}
	entryToken := providerConfig.OAuthToken

	// Acquire the per-provider cross-process refresh lock. This is a
	// dedicated lock file, not the config-write lock, and it does not take
	// s.mu — so the network exchange below cannot stall unrelated config
	// operations. The deadline exceeds the exchange timeout so that a peer
	// mid-exchange has time to publish a token we can adopt. Lock ordering:
	// the refresh lock is always taken before the config-write lock (via
	// SetConfigFields), never the reverse, so no deadlock is possible.
	lockCtx, cancel := context.WithTimeout(ctx, refreshLockDeadline)
	defer cancel()
	release, lockErr := lock.File(lockCtx, s.refreshLockPath(providerID))
	if lockErr != nil {
		// Could not acquire the lock (peer wedged or deadline hit). Prefer a
		// usable token already on disk over forcing our own exchange, which
		// would risk reusing a rotated refresh token.
		if diskToken := s.adoptableDiskToken(scope, providerID, entryToken); diskToken != nil {
			slog.Warn("Refresh lock unavailable; adopting token from disk", "provider", providerID, "error", lockErr)
			return s.applyToken(providerConfig, diskToken, providerID)
		}
		return fmt.Errorf("acquire refresh lock for provider %s: %w", providerID, lockErr)
	}
	defer release()

	// Did a peer rotate the token while we waited for the lock? If disk now
	// holds a different, unexpired token, adopt it instead of exchanging.
	if diskToken := s.adoptableDiskToken(scope, providerID, entryToken); diskToken != nil {
		slog.Info("Adopting token refreshed by another session", "provider", providerID)
		return s.applyToken(providerConfig, diskToken, providerID)
	}

	// Disk still holds our token (or no usable peer token exists) and we hold
	// the lock, so we are the sole exchanger. Perform the exchange.
	refreshedToken, refreshErr := s.exchange(ctx, providerID, entryToken.RefreshToken)
	if refreshErr != nil {
		// The exchange may have failed because a peer rotated the refresh
		// token in a window we did not cover. Re-check disk and adopt.
		if diskToken := s.adoptableDiskToken(scope, providerID, entryToken); diskToken != nil {
			slog.Info("Adopting token refreshed by another session after exchange failure", "provider", providerID)
			return s.applyToken(providerConfig, diskToken, providerID)
		}
		return fmt.Errorf("failed to refresh OAuth token for provider %s: %w", providerID, refreshErr)
	}

	slog.Info("Successfully refreshed OAuth token", "provider", providerID)
	providerConfig.OAuthToken = refreshedToken
	providerConfig.APIKey = refreshedToken.AccessToken
	if providerID == string(catwalk.InferenceProviderCopilot) {
		providerConfig.SetupGitHubCopilot()
	}
	cfg.Providers.Set(providerID, providerConfig)

	if err := s.SetConfigFields(scope, map[string]any{
		fmt.Sprintf("providers.%s.api_key", providerID): refreshedToken.AccessToken,
		fmt.Sprintf("providers.%s.oauth", providerID):   refreshedToken,
	}); err != nil {
		return fmt.Errorf("failed to persist refreshed token: %w", err)
	}
	return nil
}

// WaitForTokenChange blocks until SignalAuthComplete is called for the
// given provider or the context is cancelled. It is used by OnAuthRefresh
// callbacks to wait for interactive re-authentication to complete before
// retrying a failed request. The channel is created atomically with the
// wait registration so a concurrent SignalAuthComplete cannot miss it.
func (s *ConfigStore) WaitForTokenChange(ctx context.Context, providerID string) error {
	s.authSignalMu.Lock()
	ch, ok := s.authSignals[providerID]
	if !ok {
		ch = make(chan struct{})
		if s.authSignals == nil {
			s.authSignals = make(map[string]chan struct{})
		}
		s.authSignals[providerID] = ch
	}
	s.authSignalMu.Unlock()

	select {
	case <-ch:
		// Remove the consumed signal so a subsequent
		// SignalAuthComplete does not close an already-closed
		// channel.
		s.authSignalMu.Lock()
		if s.authSignals[providerID] == ch {
			delete(s.authSignals, providerID)
		}
		s.authSignalMu.Unlock()
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// SignalAuthComplete unblocks any goroutine waiting in WaitForTokenChange
// for the given provider. If no waiter exists yet, it pre-creates and
// immediately closes the channel so a subsequent WaitForTokenChange
// returns without blocking. This eliminates the race where the signal
// fires before the waiter registers.
func (s *ConfigStore) SignalAuthComplete(providerID string) {
	s.authSignalMu.Lock()
	defer s.authSignalMu.Unlock()
	if ch, ok := s.authSignals[providerID]; ok {
		delete(s.authSignals, providerID)
		select {
		case <-ch:
			// Already closed by a previous signal; nothing to do.
		default:
			close(ch)
		}
	} else {
		// No waiter yet. Pre-create a closed channel so the next
		// WaitForTokenChange returns immediately.
		if s.authSignals == nil {
			s.authSignals = make(map[string]chan struct{})
		}
		ch := make(chan struct{})
		close(ch)
		s.authSignals[providerID] = ch
	}
}

// adoptableDiskToken returns the on-disk token for the provider when it is
// usable and differs from entryToken — i.e. when another session has
// already refreshed it and we should adopt that result rather than running
// our own exchange. It returns nil when there is nothing newer to adopt.
func (s *ConfigStore) adoptableDiskToken(scope Scope, providerID string, entryToken *oauth.Token) *oauth.Token {
	diskToken, err := s.loadTokenFromDisk(scope, providerID)
	if err != nil {
		slog.Warn("Failed to read token from config file", "provider", providerID, "error", err)
		return nil
	}
	if diskToken == nil || diskToken.IsExpired() {
		return nil
	}
	if diskToken.AccessToken == entryToken.AccessToken {
		// Same token we started with; nobody refreshed since.
		return nil
	}
	return diskToken
}

// exchange performs the provider-specific OAuth token exchange. Tests may
// override it via the exchangeToken field; production uses the real
// provider clients.
func (s *ConfigStore) exchange(ctx context.Context, providerID, refreshToken string) (*oauth.Token, error) {
	if s.exchangeToken != nil {
		return s.exchangeToken(ctx, providerID, refreshToken)
	}
	switch providerID {
	case string(catwalk.InferenceProviderCopilot):
		return copilot.RefreshToken(ctx, refreshToken)
	case hyperp.Name:
		return hyper.ExchangeToken(ctx, refreshToken)
	default:
		return nil, fmt.Errorf("OAuth refresh not supported for provider %s", providerID)
	}
}

// refreshLockPath returns the path to the per-provider cross-process refresh
// lock file. Lock files live under a dedicated locks/ subdirectory of the
// data dir so they do not clutter the config directory. The file is created
// on demand by lock.File and is never removed (flock keys on inode, not
// path).
func (s *ConfigStore) refreshLockPath(providerID string) string {
	dir := filepath.Join(filepath.Dir(s.globalDataPath), "locks")
	_ = os.MkdirAll(dir, 0o755)
	return filepath.Join(dir, fmt.Sprintf("%s.refresh.lock", providerID))
}

// applyToken updates the in-memory provider config with the given token.
func (s *ConfigStore) applyToken(providerConfig ProviderConfig, token *oauth.Token, providerID string) error {
	providerConfig.OAuthToken = token
	providerConfig.APIKey = token.AccessToken
	if providerID == string(catwalk.InferenceProviderCopilot) {
		providerConfig.SetupGitHubCopilot()
	}
	s.Config().Providers.Set(providerID, providerConfig)
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

// nextRecentModels computes the recent-models list for the given type
// after recording the supplied model at the front, operating on the
// provided config without persisting anything. It returns the new slice
// and whether it differs from cfg's current list. Callers fold the result
// into a clone they are about to publish.
func nextRecentModels(cfg *Config, modelType SelectedModelType, model SelectedModel) ([]SelectedModel, bool) {
	if model.Provider == "" || model.Model == "" {
		return nil, false
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
		return current, false
	}

	return updated, true
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
// Concurrent calls are serialised via writeMu.
func (s *ConfigStore) ReloadFromDisk(ctx context.Context) error {
	if s.workingDir == "" {
		return fmt.Errorf("cannot reload: working directory not set")
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return s.reloadFromDiskLocked(ctx)
}

// reloadFromDiskLocked performs the actual reload. Caller must hold writeMu.
func (s *ConfigStore) reloadFromDiskLocked(ctx context.Context) error {
	// Migrate deprecated disable_notifications before reloading config.
	migrateDisableNotifications()

	configPaths := lookupConfigs(s.workingDir)
	cfg, loadedPaths, err := loadFromConfigPaths(configPaths)
	if err != nil {
		return fmt.Errorf("failed to reload config: %w", err)
	}

	// Apply defaults (using existing data directory if set)
	var dataDir string
	if cur := s.Config(); cur != nil && cur.Options != nil {
		dataDir = cur.Options.DataDirectory
	}
	cfg.setDefaults(s.workingDir, dataDir)

	// Merge workspace config if present
	workspacePath := filepath.Join(cfg.Options.DataDirectory, fmt.Sprintf("%s.json", appName))
	if wsData, err := os.ReadFile(workspacePath); err == nil && len(wsData) > 0 {
		if !json.Valid(wsData) {
			return fmt.Errorf("invalid JSON in config file %s", workspacePath)
		}
		merged, mergeErr := loadFromBytes(append([][]byte{mustMarshalConfig(cfg)}, wsData))
		if mergeErr == nil {
			dataDir := cfg.Options.DataDirectory
			*cfg = *merged
			cfg.setDefaults(s.workingDir, dataDir)
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

	if err := cfg.configureProviders(ctx, s, env, resolver, providers); err != nil {
		return fmt.Errorf("failed to configure providers during reload: %w", err)
	}

	// Save current state for potential rollback
	oldConfig := s.Config()
	oldLoadedPaths := s.loadedPaths
	oldResolver := s.resolver
	oldKnownProviders := s.knownProviders
	oldOverrides := s.overrides
	oldWorkspacePath := s.workspacePath

	// Update store state BEFORE running model/agent setup (so they see new config)
	s.setConfig(cfg)
	s.loadedPaths = loadedPaths
	s.resolver = resolver
	s.knownProviders = providers
	s.overrides = overrides
	s.workspacePath = workspacePath

	// Mirror startup flow: setup models and agents against NEW config.
	var setupErr error
	if !cfg.IsConfigured() {
		slog.Warn("No providers configured after reload")
	} else {
		resolved, resolveErr := resolveSelectedModels(cfg, providers)
		if resolveErr != nil {
			setupErr = fmt.Errorf("failed to configure selected models during reload: %w", resolveErr)
		} else {
			cfg.Models[SelectedModelTypeLarge] = resolved.Large
			cfg.Models[SelectedModelTypeSmall] = resolved.Small
			s.SetupAgents()
		}
	}

	// Rollback on setup failure
	if setupErr != nil {
		s.setConfig(oldConfig)
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
	if s.workingDir == "" {
		return nil // Expected skip: working directory not set
	}
	// Skip if a reload is already in progress. This handles both
	// concurrent auto-reloads after parallel writes and re-entrant
	// calls from configureProviders during a reload.
	//
	// Note: if a write completes after the in-progress reload has
	// already read the config file, that write won't be reflected in
	// memory until the next reload. This is acceptable because writes
	// are rare and the next user action or file-watch tick will pick
	// up the change. Callers that need guaranteed fresh state after a
	// write should call ReloadFromDisk explicitly.
	if !s.writeMu.TryLock() {
		return nil
	}
	defer s.writeMu.Unlock()
	return s.reloadFromDiskLocked(ctx)
}
