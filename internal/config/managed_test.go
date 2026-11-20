package config

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/lock"
	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/stretchr/testify/require"
)

func TestManagedCredentials_SetGetDelete(t *testing.T) {
	t.Parallel()
	m := NewManagedCredentials()

	_, ok := m.Get("hyper")
	require.False(t, ok)

	first := m.Set("hyper", ManagedCredential{AccessToken: "a"})
	require.Equal(t, ManagedKindAPIKey, first.Kind, "kind defaults to api key")
	second := m.Set("hyper", ManagedCredential{AccessToken: "b", Kind: ManagedKindOAuth})
	require.Greater(t, second.Version, first.Version)

	got, ok := m.Get("hyper")
	require.True(t, ok)
	require.Equal(t, "b", got.AccessToken)
	require.Equal(t, []string{"hyper"}, m.Providers())

	require.True(t, m.Delete("hyper"))
	require.False(t, m.Delete("hyper"))
	require.False(t, m.IsManaged("hyper"))
}

func TestManagedCredentials_NilIsEmpty(t *testing.T) {
	t.Parallel()
	var m *ManagedCredentials
	require.False(t, m.IsManaged("hyper"))
	require.Empty(t, m.Providers())
}

func TestManagedCredentials_WaitWakesOnDifferentToken(t *testing.T) {
	t.Parallel()
	m := NewManagedCredentials()
	m.Set("hyper", ManagedCredential{AccessToken: "old"})

	const waiters = 3
	results := make(chan ManagedCredential, waiters)
	for range waiters {
		go func() {
			cred, err := m.Wait(t.Context(), "hyper", "old", 0)
			if err == nil {
				results <- cred
			}
		}()
	}

	// Re-pushing the same token must not wake a waiter asking for a
	// different one.
	time.Sleep(20 * time.Millisecond)
	m.Set("hyper", ManagedCredential{AccessToken: "old"})
	select {
	case <-results:
		t.Fatal("a push of the rejected token woke a waiter")
	case <-time.After(50 * time.Millisecond):
	}

	m.Set("hyper", ManagedCredential{AccessToken: "new"})
	for range waiters {
		select {
		case cred := <-results:
			require.Equal(t, "new", cred.AccessToken)
		case <-time.After(5 * time.Second):
			t.Fatal("waiter was not woken by the push")
		}
	}
}

func TestManagedCredentials_WaitByVersion(t *testing.T) {
	t.Parallel()
	m := NewManagedCredentials()
	cur := m.Set("hyper", ManagedCredential{AccessToken: "same"})

	done := make(chan ManagedCredential, 1)
	go func() {
		cred, err := m.Wait(t.Context(), "hyper", "", cur.Version)
		if err == nil {
			done <- cred
		}
	}()
	m.Set("hyper", ManagedCredential{AccessToken: "same", ExpiresAt: time.Now().Add(time.Hour)})
	select {
	case cred := <-done:
		require.Greater(t, cred.Version, cur.Version)
	case <-time.After(5 * time.Second):
		t.Fatal("a newer push did not wake a version waiter")
	}
}

func TestManagedCredentials_WaitTimeoutAndDelete(t *testing.T) {
	t.Parallel()
	m := NewManagedCredentials()

	_, err := m.Wait(t.Context(), "hyper", "", 0)
	require.ErrorIs(t, err, ErrNotManaged)

	m.Set("hyper", ManagedCredential{AccessToken: "old"})
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Millisecond)
	defer cancel()
	_, err = m.Wait(ctx, "hyper", "old", 0)
	require.ErrorIs(t, err, context.DeadlineExceeded)

	errs := make(chan error, 1)
	go func() {
		_, err := m.Wait(t.Context(), "hyper", "old", 0)
		errs <- err
	}()
	time.Sleep(20 * time.Millisecond)
	m.Delete("hyper")
	select {
	case err := <-errs:
		require.ErrorIs(t, err, ErrNotManaged)
	case <-time.After(5 * time.Second):
		t.Fatal("delete did not wake the waiter")
	}
}

func TestManagedCredential_ExpiresWithin(t *testing.T) {
	t.Parallel()
	require.False(t, ManagedCredential{}.ExpiresWithin(time.Hour), "unknown expiry never expires")
	require.True(t, ManagedCredential{ExpiresAt: time.Now().Add(30 * time.Second)}.ExpiresWithin(time.Minute))
	require.False(t, ManagedCredential{ExpiresAt: time.Now().Add(time.Hour)}.ExpiresWithin(time.Minute))
}

// The kind follows how the provider is configured: an OAuth login means
// the pushed token is an OAuth access token, a key means an API key.
func TestManagedKindFor(t *testing.T) {
	t.Parallel()
	providers := csync.NewMap[string, ProviderConfig]()
	providers.Set("hyper", ProviderConfig{ID: "hyper", OAuthToken: &oauth.Token{AccessToken: "x"}})
	providers.Set("anthropic", ProviderConfig{ID: "anthropic", APIKey: "$ANTHROPIC_API_KEY"})
	store := &ConfigStore{config: &Config{Providers: providers}}
	kind, ok := store.ManagedKindFor("hyper")
	require.True(t, ok)
	require.Equal(t, ManagedKindOAuth, kind)
	kind, ok = store.ManagedKindFor("anthropic")
	require.True(t, ok)
	require.Equal(t, ManagedKindAPIKey, kind)
	_, ok = store.ManagedKindFor("nope")
	require.False(t, ok)
}

// A managed credential beats both the in-memory OAuth token and a newer
// one on disk, and no refresh-token exchange is ever attempted for it.
func TestManagedCredential_BeatsDiskAndSkipsRefresh(t *testing.T) {
	isolateHyperCredentials(t)

	configPath := filepath.Join(t.TempDir(), "crush.json")
	store := newRefreshTestStore(t, configPath, noExchange(t))
	m := NewManagedCredentials()
	store.AttachManagedCredentials(m)

	m.Set("hyper", ManagedCredential{AccessToken: "pushed", ExpiresAt: time.Now().Add(time.Hour)})
	store.SyncManagedProvider("hyper")

	pc, ok := store.Config().Providers.Get("hyper")
	require.True(t, ok)
	require.Equal(t, ManagedKindAPIKey, pc.ManagedKind)
	require.False(t, pc.UsesOAuth(), "an api-key credential overrides the oauth login")
	require.NotEqual(t, "pushed", pc.APIKey, "the token never lands on the provider config")

	// A newer, usable peer token on disk would normally be adopted.
	writeTokenToDisk(t, configPath, &oauth.Token{
		AccessToken:  "peer-at",
		RefreshToken: "peer-rt",
		ExpiresIn:    3600,
		ExpiresAt:    time.Now().Add(time.Hour).Unix(),
	})
	adopted, err := store.AdoptNewerDiskToken(ScopeGlobal, "hyper")
	require.NoError(t, err)
	require.False(t, adopted)
	adopted, err = store.AdoptPeerRefreshedToken(t.Context(), ScopeGlobal, "hyper")
	require.NoError(t, err)
	require.False(t, adopted)

	err = store.RefreshOAuthToken(t.Context(), ScopeGlobal, "hyper")
	require.ErrorIs(t, err, ErrCredentialManaged)

	pc, _ = store.Config().Providers.Get("hyper")
	require.Equal(t, "at0", pc.OAuthToken.AccessToken, "disk token was not adopted")
	require.Equal(t, ManagedKindAPIKey, pc.ManagedKind)
}

// An exchange or adoption already in flight when the provider became
// managed must not overwrite the managed entry when it lands.
func TestManagedCredential_InFlightTokenDiscarded(t *testing.T) {
	isolateHyperCredentials(t)
	configPath := filepath.Join(t.TempDir(), "crush.json")
	store := newRefreshTestStore(t, configPath, noExchange(t))
	stale, _ := store.Config().Providers.Get("hyper") // snapshot before the push

	m := NewManagedCredentials()
	store.AttachManagedCredentials(m)
	m.Set("hyper", ManagedCredential{AccessToken: "pushed"})
	store.SyncManagedProvider("hyper")

	_ = stale
	store.applyToken("hyper", &oauth.Token{AccessToken: "late", RefreshToken: "rt"})
	require.False(t, store.applyTokenIfUnchanged("hyper", stale.OAuthToken, &oauth.Token{AccessToken: "late2"}))
	pc, _ := store.Config().Providers.Get("hyper")
	require.Equal(t, ManagedKindAPIKey, pc.ManagedKind, "the managed marker survives")
	require.Equal(t, "at0", pc.OAuthToken.AccessToken)
}

// The OAuth presentation marks the provider as OAuth without copying the
// token anywhere serializable; a provider kept only because it
// was managed goes away with its credential.
func TestManagedCredential_OAuthKindAndDelete(t *testing.T) {
	t.Parallel()
	providers := csync.NewMap[string, ProviderConfig]()
	providers.Set("anthropic", ProviderConfig{ID: "anthropic", managedSynthetic: true})
	store := &ConfigStore{config: &Config{Providers: providers}}
	m := NewManagedCredentials()
	const secret = "oauth-access-secret"
	m.Set("anthropic", ManagedCredential{AccessToken: secret, ExpiresAt: time.Now().Add(time.Hour), Kind: ManagedKindOAuth})
	store.AttachManagedCredentials(m)

	pc, ok := store.Config().Providers.Get("anthropic")
	require.True(t, ok)
	require.Equal(t, ManagedKindOAuth, pc.ManagedKind)
	require.True(t, pc.UsesOAuth())
	require.Empty(t, pc.APIKey)
	require.Nil(t, pc.OAuthToken)

	data, err := json.Marshal(store.Config())
	require.NoError(t, err)
	require.NotContains(t, string(data), secret, "serialized config must not carry the managed token")

	m.Delete("anthropic")
	require.False(t, store.SyncManagedProvider("anthropic"))
	_, ok = store.Config().Providers.Get("anthropic")
	require.False(t, ok, "a provider configured only by the push goes away with it")
}

// A managed provider with no config entry asks for a reload, and a
// provider declared only in a config file counts as known.
func TestManagedCredential_AbsentProviderNeedsReload(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "crush.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"providers":{"mock":{"base_url":"http://x"}}}`), 0o600))
	store := &ConfigStore{config: &Config{Providers: csync.NewMap[string, ProviderConfig]()}, loadedPaths: []string{path}}
	m := NewManagedCredentials()
	store.AttachManagedCredentials(m)
	require.True(t, store.IsKnownProvider("mock"))
	require.False(t, store.IsKnownProvider("nope"))

	m.Set("mock", ManagedCredential{AccessToken: "tok"})
	require.True(t, store.SyncManagedProvider("mock"))
}

// A refresh queued behind the cross-process lock re-checks after getting
// it: a provider that became managed meanwhile is never exchanged.
func TestRefreshOAuthToken_ManagedWhileQueuedForLock(t *testing.T) {
	isolateHyperCredentials(t)
	configPath := filepath.Join(t.TempDir(), "crush.json")
	store := newRefreshTestStore(t, configPath, noExchange(t))
	m := NewManagedCredentials()
	store.AttachManagedCredentials(m)

	release, err := lock.File(t.Context(), store.refreshLockPath("hyper"))
	require.NoError(t, err)
	done := make(chan error, 1)
	go func() { done <- store.RefreshOAuthToken(t.Context(), ScopeGlobal, "hyper") }()
	time.Sleep(100 * time.Millisecond) // let it pass the early check and queue
	m.Set("hyper", ManagedCredential{AccessToken: "pushed"})
	release()

	select {
	case err := <-done:
		require.ErrorIs(t, err, ErrCredentialManaged)
	case <-time.After(10 * time.Second):
		t.Fatal("refresh did not return")
	}
}

// Deleting a credential returns a configured provider to normal without a
// reload, and a full sync catches up a delete it never saw.
func TestManagedCredential_DeleteRestoresConfiguredProvider(t *testing.T) {
	isolateHyperCredentials(t)
	configPath := filepath.Join(t.TempDir(), "crush.json")
	store := newRefreshTestStore(t, configPath, noExchange(t))
	m := NewManagedCredentials()
	m.Set("hyper", ManagedCredential{AccessToken: "pushed"})
	store.AttachManagedCredentials(m)
	require.True(t, store.IsManagedProvider("hyper"))

	m.Delete("hyper") // no per-provider sync, as if the fan-out missed us
	store.SyncManagedCredentials()
	pc, ok := store.Config().Providers.Get("hyper")
	require.True(t, ok)
	require.Empty(t, pc.ManagedKind)
	require.True(t, pc.UsesOAuth(), "the oauth login is back")
}

// A store without a managed set behaves exactly as before.
func TestManagedCredential_UnmanagedStoreUnaffected(t *testing.T) {
	isolateHyperCredentials(t)
	configPath := filepath.Join(t.TempDir(), "crush.json")
	store := newRefreshTestStore(t, configPath, noExchange(t))
	require.False(t, store.IsManagedProvider("hyper"))
	writeTokenToDisk(t, configPath, &oauth.Token{
		AccessToken:  "peer-at",
		RefreshToken: "peer-rt",
		ExpiresIn:    3600,
		ExpiresAt:    time.Now().Add(time.Hour).Unix(),
	})
	adopted, err := store.AdoptNewerDiskToken(ScopeGlobal, "hyper")
	require.NoError(t, err)
	require.True(t, adopted)
}

// The Hyper catalog fetch presents a managed token and does not refresh
// on 401 when the credential is managed.
func TestHyperCatalog_ManagedTokenNoRefresh(t *testing.T) {
	t.Parallel()
	var auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(srv.Close)

	store := &ConfigStore{config: &Config{Providers: csync.NewMap[string, ProviderConfig]()}}
	m := NewManagedCredentials()
	m.Set("hyper", ManagedCredential{AccessToken: "managed-hyper"})
	store.managed.Store(m)
	a := store.hyperAuth(func() string { return "disk-key" })
	client := realHyperClient{
		baseURL:    srv.URL,
		resolveKey: a.resolveKey,
		refreshToken: func(context.Context) error {
			t.Error("a managed credential must not be refreshed")
			return nil
		},
		managed: a.managed,
	}
	_, err := client.Get(t.Context(), "")
	require.Error(t, err)
	require.Equal(t, "Bearer managed-hyper", auth)
}

// A refresh answering a rejected request is refused for a managed provider.
func TestRefreshRejectedOAuthToken_Managed(t *testing.T) {
	isolateHyperCredentials(t)
	configPath := filepath.Join(t.TempDir(), "crush.json")
	store := newRefreshTestStore(t, configPath, noExchange(t))
	m := NewManagedCredentials()
	store.AttachManagedCredentials(m)
	m.Set("hyper", ManagedCredential{AccessToken: "pushed"})
	err := store.RefreshRejectedOAuthToken(t.Context(), ScopeGlobal, "hyper", &oauth.Token{AccessToken: "at0"})
	require.ErrorIs(t, err, ErrCredentialManaged)
}

// A push that lands while an exchange is in flight waits for that
// exchange (and its publish) before marking the provider managed, and no
// second exchange starts once the push has begun.
func TestManagedTransition_WaitsForInFlightExchange(t *testing.T) {
	isolateHyperCredentials(t)
	configPath := filepath.Join(t.TempDir(), "crush.json")
	started := make(chan struct{})
	release := make(chan struct{})
	var exchanges atomic.Int32
	store := newRefreshTestStore(t, configPath, func(context.Context, string, string) (*oauth.Token, error) {
		if exchanges.Add(1) == 1 {
			close(started)
			<-release
		}
		return &oauth.Token{AccessToken: "at1", RefreshToken: "rt1", ExpiresIn: 3600, ExpiresAt: time.Now().Add(time.Hour).Unix()}, nil
	})
	m := NewManagedCredentials()
	store.AttachManagedCredentials(m)

	refreshed := make(chan error, 1)
	go func() { refreshed <- store.RefreshOAuthToken(context.Background(), ScopeGlobal, "hyper") }()
	<-started

	pushed := make(chan struct{})
	go func() {
		m.Set("hyper", ManagedCredential{AccessToken: "pushed"})
		store.SyncManagedProvider("hyper")
		close(pushed)
	}()
	select {
	case <-pushed:
		t.Fatal("the push completed while an exchange was in flight")
	case <-time.After(100 * time.Millisecond):
	}
	// A refresh arriving now is refused, not exchanged.
	require.ErrorIs(t, store.RefreshOAuthToken(t.Context(), ScopeGlobal, "hyper"), ErrCredentialManaged)

	close(release)
	select {
	case <-pushed:
	case <-time.After(10 * time.Second):
		t.Fatal("the push did not complete after the exchange")
	}
	require.NoError(t, <-refreshed)
	require.Equal(t, int32(1), exchanges.Load(), "no second exchange")
	pc, _ := store.Config().Providers.Get("hyper")
	require.Equal(t, ManagedKindAPIKey, pc.ManagedKind)
	cred, ok := store.ManagedCredential("hyper")
	require.True(t, ok)
	require.Equal(t, "pushed", cred.AccessToken, "the push wins; the late token is only saved")
}

func TestParseAndSupportManagedKind(t *testing.T) {
	t.Parallel()
	for _, s := range []string{"", "oauth", "api-key"} {
		_, err := ParseManagedKind(s)
		require.NoError(t, err, s)
	}
	_, err := ParseManagedKind("api_key")
	require.ErrorIs(t, err, ErrUnsupportedManagedKind)
	require.True(t, SupportsManagedKind("anthropic", ManagedKindOAuth))
	require.True(t, SupportsManagedKind("hyper", ManagedKindOAuth))
	require.False(t, SupportsManagedKind("gemini", ManagedKindOAuth))
	require.True(t, SupportsManagedKind("gemini", ManagedKindAPIKey))
}
