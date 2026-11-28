package config

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// newSignOutTestStore is newRefreshTestStore without the reload that a
// working directory triggers, so assertions see only what SignOut did.
func newSignOutTestStore(t *testing.T, configPath string, exchange func(context.Context, string, string) (*oauth.Token, error)) *ConfigStore {
	t.Helper()
	isolateHyperCredentials(t)
	store := newRefreshTestStore(t, configPath, exchange)
	store.workingDir = ""
	return store
}

// requireSignedOut asserts the hyper credential is gone from disk and memory.
func requireSignedOut(t *testing.T, store *ConfigStore) {
	t.Helper()
	onDisk, err := store.loadTokenFromDisk(ScopeGlobal, "hyper")
	require.NoError(t, err)
	require.Nil(t, onDisk, "the token must not be on disk after sign-out")
	require.False(t, store.HasConfigField(ScopeGlobal, "providers.hyper.api_key"))
	pc, ok := store.Config().Providers.Get("hyper")
	require.True(t, ok)
	require.Nil(t, pc.OAuthToken)
	require.Empty(t, pc.APIKey)
	_, pending := store.pendingToken("hyper")
	require.False(t, pending)
}

// A refresh already exchanging when sign-out starts must not save its token
// back once sign-out has finished.
func TestSignOut_WaitsOutARefreshInFlight(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "crush.json")
	started, release := make(chan struct{}), make(chan struct{})
	store := newSignOutTestStore(t, configPath, func(context.Context, string, string) (*oauth.Token, error) {
		close(started)
		<-release
		return freshToken("at1", "rt1"), nil
	})

	refreshed := make(chan error, 1)
	go func() { refreshed <- store.RefreshOAuthToken(context.Background(), ScopeGlobal, "hyper") }()
	<-started

	signedOut := make(chan error, 1)
	go func() { signedOut <- store.SignOut(context.Background(), ScopeGlobal, "hyper") }()
	select {
	case err := <-signedOut:
		t.Fatalf("sign-out finished while a refresh was still exchanging: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	close(release)
	require.NoError(t, <-refreshed)
	require.NoError(t, <-signedOut)
	requireSignedOut(t, store)
}

// A refreshed token that could not be saved is dropped, so no later refresh
// or reload puts it back.
func TestSignOut_DropsAnUnsavedRefreshedToken(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "crush.json")
	var exchanges atomic.Int64
	store := newSignOutTestStore(t, configPath, func(context.Context, string, string) (*oauth.Token, error) {
		exchanges.Add(1)
		return freshToken("at1", "rt1"), nil
	})
	store.persistTokenFields = func(Scope, map[string]any) error { return errors.New("disk full") }

	require.NoError(t, store.RefreshOAuthToken(context.Background(), ScopeGlobal, "hyper"))
	_, pending := store.pendingToken("hyper")
	require.True(t, pending)

	require.NoError(t, store.SignOut(context.Background(), ScopeGlobal, "hyper"))
	requireSignedOut(t, store)

	store.persistTokenFields = nil
	require.Error(t, store.RefreshOAuthToken(context.Background(), ScopeGlobal, "hyper"))
	reloaded := &Config{Providers: csync.NewMap[string, ProviderConfig]()}
	reloaded.Providers.Set("hyper", ProviderConfig{ID: "hyper"})
	store.reapplyPendingTokens(reloaded)
	pc, _ := reloaded.Providers.Get("hyper")
	require.Nil(t, pc.OAuthToken, "a reload must not re-apply the dropped token")
	require.Equal(t, int64(1), exchanges.Load())
}

// Extra fields go in the same write; the rest of the provider stays.
func TestSignOut_RemovesExtraFieldsOnly(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "crush.json")
	store := newSignOutTestStore(t, configPath, nil)
	require.NoError(t, store.atomicEdit(ScopeGlobal, map[string]any{
		"providers.hyper.grok_models": []string{"m"},
		"providers.hyper.base_url":    "https://example.com",
	}, nil))

	require.NoError(t, store.SignOut(context.Background(), ScopeGlobal, "hyper", "grok_models"))
	requireSignedOut(t, store)
	require.False(t, store.HasConfigField(ScopeGlobal, "providers.hyper.grok_models"))
	require.True(t, store.HasConfigField(ScopeGlobal, "providers.hyper.base_url"))
}

// A sign-out whose write fails leaves the credential as it was.
func TestSignOut_FailedWriteKeepsTheCredential(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs a directory the test cannot write to")
	}
	dir := t.TempDir()
	configPath := filepath.Join(dir, "crush.json")
	store := newSignOutTestStore(t, configPath, nil)
	// Create the lock files first so only the config write itself fails.
	store.refreshLockPath("hyper")
	require.NoError(t, os.WriteFile(configPath+".lock", nil, 0o600))
	require.NoError(t, os.Chmod(dir, 0o500))
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	require.Error(t, store.SignOut(context.Background(), ScopeGlobal, "hyper"))

	onDisk, err := store.loadTokenFromDisk(ScopeGlobal, "hyper")
	require.NoError(t, err)
	require.Equal(t, "rt0", onDisk.RefreshToken)
	pc, _ := store.Config().Providers.Get("hyper")
	require.NotNil(t, pc.OAuthToken)
	require.Equal(t, "at0", pc.APIKey)
}

// A credential pushed by an external owner is not the user's to sign out of.
func TestSignOut_RefusesAManagedProvider(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "crush.json")
	store := newSignOutTestStore(t, configPath, nil)
	m := NewManagedCredentials()
	m.Set("hyper", ManagedCredential{AccessToken: "managed"})
	store.managed.Store(m)

	require.ErrorIs(t, store.SignOut(context.Background(), ScopeGlobal, "hyper"), ErrCredentialManaged)
	onDisk, err := store.loadTokenFromDisk(ScopeGlobal, "hyper")
	require.NoError(t, err)
	require.NotNil(t, onDisk)
}

// A reload that read disk before a sign-out can put the token back in
// memory; the next refresh must not spend it and write the login back.
func TestRefreshOAuthToken_RefusesALoginSignedOutUnderAReload(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "crush.json")
	var exchanges atomic.Int64
	store := newSignOutTestStore(t, configPath, func(context.Context, string, string) (*oauth.Token, error) {
		exchanges.Add(1)
		return freshToken("at1", "rt1"), nil
	})
	before, _ := store.Config().Providers.Get("hyper")
	stale := &Config{Providers: csync.NewMap[string, ProviderConfig]()}
	stale.Providers.Set("hyper", before)

	store.writeMu.Lock()
	require.NoError(t, store.SignOut(context.Background(), ScopeGlobal, "hyper"))
	store.setConfig(stale)
	store.writeMu.Unlock()

	err := store.RefreshOAuthToken(context.Background(), ScopeGlobal, "hyper")
	require.ErrorIs(t, err, ErrSignedOut)
	require.Zero(t, exchanges.Load())
	requireSignedOut(t, store)
}

// A login saved in the workspace file is still signed in when the refresh
// names the global scope.
func TestRefreshOAuthToken_FindsALoginInTheOtherScope(t *testing.T) {
	dir := t.TempDir()
	globalPath, workspacePath := filepath.Join(dir, "global.json"), filepath.Join(dir, "workspace.json")
	var exchanges atomic.Int64
	store := newSignOutTestStore(t, workspacePath, func(context.Context, string, string) (*oauth.Token, error) {
		exchanges.Add(1)
		return freshToken("at1", "rt1"), nil
	})
	store.globalDataPath, store.workspacePath = globalPath, workspacePath

	require.NoError(t, store.RefreshOAuthToken(context.Background(), ScopeGlobal, "hyper"))
	require.Equal(t, int64(1), exchanges.Load())
}

// An API key typed while a refresh is exchanging replaces the login on disk
// once that refresh lands, rather than being overwritten by it.
func TestSetProviderAPIKey_WaitsOutARefreshInFlight(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "crush.json")
	expired := &oauth.Token{AccessToken: "at0", RefreshToken: "rt0", ExpiresAt: time.Now().Add(-time.Hour).Unix()}
	require.NoError(t, os.WriteFile(configPath, []byte(`{"providers":{"xai":{"api_key":"at0","oauth":{"access_token":"at0","refresh_token":"rt0"}}}}`), 0o600))
	providers := csync.NewMap[string, ProviderConfig]()
	providers.Set("xai", ProviderConfig{ID: "xai", APIKey: "at0", OAuthToken: expired})
	started, release := make(chan struct{}), make(chan struct{})
	store := &ConfigStore{
		config:         &Config{Providers: providers},
		globalDataPath: configPath,
		exchangeToken: func(context.Context, string, string) (*oauth.Token, error) {
			close(started)
			<-release
			return freshToken("at1", "rt1"), nil
		},
	}

	refreshed := make(chan error, 1)
	go func() { refreshed <- store.RefreshOAuthToken(context.Background(), ScopeGlobal, "xai") }()
	<-started
	saved := make(chan error, 1)
	go func() { saved <- store.SetProviderAPIKey(ScopeGlobal, "xai", "key") }()
	select {
	case err := <-saved:
		t.Fatalf("the key was saved while a refresh was still exchanging: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	close(release)
	require.NoError(t, <-refreshed)
	require.NoError(t, <-saved)
	disk, err := os.ReadFile(configPath)
	require.NoError(t, err)
	require.Equal(t, "key", gjson.GetBytes(disk, "providers.xai.api_key").String())
	require.False(t, gjson.GetBytes(disk, "providers.xai.oauth").Exists())
	pc, _ := store.Config().Providers.Get("xai")
	require.Nil(t, pc.OAuthToken)
	require.Equal(t, "key", pc.APIKey)
}
