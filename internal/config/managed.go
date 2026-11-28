package config

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"sync"
	"time"

	"charm.land/catwalk/v3/pkg/catwalk"

	"github.com/tidwall/gjson"
)

// ManagedCredentialKind says how a pushed credential is presented to the
// provider.
type ManagedCredentialKind string

const (
	// ManagedKindAPIKey presents the credential as the provider's API key,
	// for providers configured with a key. Any OAuth login the provider
	// had is set aside while it is managed.
	ManagedKindAPIKey ManagedCredentialKind = "api-key"
	// ManagedKindOAuth presents the credential as the bearer access token
	// of a provider that normally signs in by OAuth. There is never a
	// refresh token: the owner pushes a new access token instead.
	ManagedKindOAuth ManagedCredentialKind = "oauth"
)

// ErrCredentialManaged is returned by refresh paths for a provider whose
// credential is owned by an external process. Crush never refreshes such a
// credential itself; it asks for a new one instead.
var ErrCredentialManaged = errors.New("credential is managed externally")

// ErrNotManaged is returned by [ManagedCredentials.Wait] when the provider
// has no managed credential (it was never pushed, or was deleted while
// waiting).
var ErrNotManaged = errors.New("provider has no managed credential")

// ManagedCredential is a credential pushed by an external owner. It lives
// in memory only and is never written to disk.
type ManagedCredential struct {
	AccessToken string
	// ExpiresAt is zero when the owner did not say.
	ExpiresAt time.Time
	Kind      ManagedCredentialKind
	// Version increases on every push across all providers, so a holder
	// can tell whether what it built against is still current.
	Version uint64
}

// ExpiresWithin reports whether the credential has a known expiry that
// falls within d of now.
func (c ManagedCredential) ExpiresWithin(d time.Duration) bool {
	return !c.ExpiresAt.IsZero() && time.Until(c.ExpiresAt) <= d
}

// ErrUnsupportedManagedKind is returned when a credential's kind cannot be
// presented for its provider.
var ErrUnsupportedManagedKind = errors.New("unsupported credential kind for provider")

// ParseManagedKind validates a kind named on the credentials API; empty
// means "infer".
func ParseManagedKind(s string) (ManagedCredentialKind, error) {
	switch k := ManagedCredentialKind(s); k {
	case "", ManagedKindOAuth, ManagedKindAPIKey:
		return k, nil
	default:
		return "", fmt.Errorf("%w: %q", ErrUnsupportedManagedKind, s)
	}
}

// managedOAuthProviders are the providers this build can present an OAuth
// access token to: each takes it as a bearer token.
var managedOAuthProviders = []string{
	string(catwalk.InferenceProviderAnthropic),
	string(catwalk.InferenceProviderCopilot),
	string(catwalk.InferenceProviderOpenAI),
	string(catwalk.InferenceProviderXAI),
	"hyper",
}

// SupportsManagedKind reports whether a credential of kind can be
// presented to providerID in this build.
func SupportsManagedKind(providerID string, kind ManagedCredentialKind) bool {
	return kind != ManagedKindOAuth || slices.Contains(managedOAuthProviders, providerID)
}

// ManagedKindFor picks how a credential pushed for providerID is
// presented: as an OAuth access token when the provider is configured for
// OAuth here, as an API key otherwise.
func (s *ConfigStore) ManagedKindFor(providerID string) (ManagedCredentialKind, bool) {
	cfg := s.Config()
	if cfg == nil || cfg.Providers == nil {
		return "", false
	}
	pc, ok := cfg.Providers.Get(providerID)
	if !ok {
		return "", false
	}
	if pc.OAuthToken != nil {
		return ManagedKindOAuth, true
	}
	return ManagedKindAPIKey, true
}

type managedEntry struct {
	cred ManagedCredential
	// changed is closed (and replaced) whenever cred changes or the entry
	// is deleted, waking every waiter.
	changed chan struct{}
}

// ManagedCredentials holds externally managed provider credentials for a
// whole process. It is safe for concurrent use. The zero value is not
// usable; use [NewManagedCredentials].
type ManagedCredentials struct {
	mu      sync.Mutex
	version uint64
	entries map[string]*managedEntry
}

// NewManagedCredentials returns an empty set.
func NewManagedCredentials() *ManagedCredentials {
	return &ManagedCredentials{entries: make(map[string]*managedEntry)}
}

// Set stores a credential for providerID, wakes all waiters, and returns
// the stored value (with its new version).
func (m *ManagedCredentials) Set(providerID string, cred ManagedCredential) ManagedCredential {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.version++
	cred.Version = m.version
	if cred.Kind == "" {
		cred.Kind = ManagedKindAPIKey
	}
	e, ok := m.entries[providerID]
	if !ok {
		e = &managedEntry{changed: make(chan struct{})}
		m.entries[providerID] = e
	} else {
		close(e.changed)
		e.changed = make(chan struct{})
	}
	e.cred = cred
	return cred
}

// Delete drops the credential for providerID, returning the provider to
// normal behaviour. Waiters wake and see [ErrNotManaged]. It reports
// whether there was anything to delete.
func (m *ManagedCredentials) Delete(providerID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.entries[providerID]
	if !ok {
		return false
	}
	m.version++
	close(e.changed)
	delete(m.entries, providerID)
	return true
}

// Version increases on every push and every delete, so a holder can tell
// whether anything changed since it last looked.
func (m *ManagedCredentials) Version() uint64 {
	if m == nil {
		return 0
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.version
}

// Get returns the credential for providerID.
func (m *ManagedCredentials) Get(providerID string) (ManagedCredential, bool) {
	if m == nil {
		return ManagedCredential{}, false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.entries[providerID]
	if !ok {
		return ManagedCredential{}, false
	}
	return e.cred, true
}

// IsManaged reports whether providerID currently has a managed credential.
func (m *ManagedCredentials) IsManaged(providerID string) bool {
	_, ok := m.Get(providerID)
	return ok
}

// Providers returns the IDs of every managed provider.
func (m *ManagedCredentials) Providers() []string {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return slices.Sorted(maps.Keys(m.entries))
}

// Wait blocks until providerID holds a credential whose access token
// differs from notToken, and returns it. Pass an empty notToken to wait
// for any newer push than the one identified by minVersion; a credential
// is returned as soon as its version exceeds minVersion or its token
// differs from notToken (when notToken is non-empty). It returns
// ErrNotManaged if the provider is not (or stops being) managed, and the
// context error if ctx ends first.
func (m *ManagedCredentials) Wait(ctx context.Context, providerID, notToken string, minVersion uint64) (ManagedCredential, error) {
	for {
		m.mu.Lock()
		e, ok := m.entries[providerID]
		if !ok {
			m.mu.Unlock()
			return ManagedCredential{}, ErrNotManaged
		}
		cred := e.cred
		ch := e.changed
		m.mu.Unlock()

		if cred.Version > minVersion && (notToken == "" || cred.AccessToken != notToken) {
			return cred, nil
		}
		select {
		case <-ch:
		case <-ctx.Done():
			return ManagedCredential{}, ctx.Err()
		}
	}
}

// AttachManagedCredentials makes the store consult m for externally
// managed provider credentials and marks every provider m already holds.
// It is safe to call on a live store.
func (s *ConfigStore) AttachManagedCredentials(m *ManagedCredentials) {
	s.managed.Store(m)
	s.SyncManagedCredentials()
}

// ManagedCredentials returns the set attached to the store, or nil.
func (s *ConfigStore) ManagedCredentials() *ManagedCredentials {
	return s.managed.Load()
}

// ManagedCredential returns the managed credential for providerID, if the
// provider is managed.
func (s *ConfigStore) ManagedCredential(providerID string) (ManagedCredential, bool) {
	if s == nil {
		return ManagedCredential{}, false
	}
	return s.managed.Load().Get(providerID)
}

// IsManagedProvider reports whether providerID's credential is owned by an
// external process.
func (s *ConfigStore) IsManagedProvider(providerID string) bool {
	_, ok := s.ManagedCredential(providerID)
	return ok
}

// SyncManagedProvider brings providerID's in-memory entry in line with the
// managed set: marked as managed while a credential is held, back to its
// normal configuration once it is not. Nothing is persisted, and the
// credential itself is never copied into the config. It is idempotent, so
// concurrent pushes, deletes and workspace creation all converge.
//
// It reports whether the provider is managed but has no entry to mark:
// it was dropped at load for lacking a key, so only a reload (which keeps
// managed providers) can bring it in.
func (s *ConfigStore) SyncManagedProvider(providerID string) (needsReload bool) {
	if s.IsManagedProvider(providerID) {
		// New exchanges are already refused; let any admitted one finish
		// and publish before the provider is marked.
		s.waitForExchanges(providerID)
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	s.tokenMu.Lock()
	defer s.tokenMu.Unlock()
	if cfg := s.Config(); cfg != nil {
		return s.syncManagedLocked(cfg, providerID)
	}
	return false
}

// SyncManagedCredentials syncs every provider that is managed or still
// marked as managed. See SyncManagedProvider.
func (s *ConfigStore) SyncManagedCredentials() (needsReload bool) {
	for _, id := range s.ManagedCredentials().Providers() {
		s.waitForExchanges(id)
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	s.tokenMu.Lock()
	defer s.tokenMu.Unlock()
	if cfg := s.Config(); cfg != nil {
		return s.syncAllManagedLocked(cfg)
	}
	return false
}

// syncAllManagedLocked syncs every managed or marked provider in cfg. The
// caller must hold writeMu.
func (s *ConfigStore) syncAllManagedLocked(cfg *Config) (needsReload bool) {
	if cfg.Providers == nil {
		return false
	}
	ids := s.managed.Load().Providers()
	for id, pc := range cfg.Providers.Seq2() {
		if pc.ManagedKind != "" && !slices.Contains(ids, id) {
			ids = append(ids, id)
		}
	}
	for _, id := range ids {
		if s.syncManagedLocked(cfg, id) {
			needsReload = true
		}
	}
	return needsReload
}

// syncManagedLocked is the single place a provider entry gains or loses
// its managed marker. The caller must hold writeMu.
func (s *ConfigStore) syncManagedLocked(cfg *Config, providerID string) (needsReload bool) {
	if cfg.Providers == nil {
		return false
	}
	pc, exists := cfg.Providers.Get(providerID)
	cred, managed := s.managed.Load().Get(providerID)
	switch {
	case managed && !exists:
		return true
	case managed && pc.ManagedKind != cred.Kind:
		pc.ManagedKind = cred.Kind
		cfg.Providers.Set(providerID, pc)
	case !managed && exists && pc.managedSynthetic:
		// Configured only because it was managed.
		cfg.Providers.Del(providerID)
	case !managed && exists && pc.ManagedKind != "":
		pc.ManagedKind = ""
		cfg.Providers.Set(providerID, pc)
	}
	return false
}

// keepManaged reports whether a provider that would be skipped for lacking
// a credential stays configured because one is managed for it, marking it
// so it is dropped again when the credential is.
func (s *ConfigStore) keepManaged(prepared *ProviderConfig) bool {
	cred, ok := s.ManagedCredential(prepared.ID)
	if !ok {
		return false
	}
	prepared.ManagedKind = cred.Kind
	prepared.managedSynthetic = true
	return true
}

// IsKnownProvider reports whether id names a configured provider, one in
// the catalog, or one a JSON config file declares (it may have been
// dropped at load for lacking a key).
func (s *ConfigStore) IsKnownProvider(id string) bool {
	if cfg := s.Config(); cfg != nil && cfg.Providers != nil {
		if _, ok := cfg.Providers.Get(id); ok {
			return true
		}
	}
	for _, p := range s.KnownProviders() {
		if string(p.ID) == id {
			return true
		}
	}
	s.writeMu.RLock()
	paths := slices.Clone(s.loadedPaths)
	s.writeMu.RUnlock()
	for _, path := range paths {
		if data, err := readFile(path); err == nil && gjson.GetBytes(data, "providers."+gjson.Escape(id)).Exists() {
			return true
		}
	}
	return false
}

// managedRefreshError is the error refresh paths return for a managed
// provider.
func managedRefreshError(providerID string) error {
	return fmt.Errorf("refresh for provider %s: %w", providerID, ErrCredentialManaged)
}
