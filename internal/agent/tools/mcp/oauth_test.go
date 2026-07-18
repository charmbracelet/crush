package mcp

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

func TestTokenStore_SaveLoad(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	store := &tokenStore{
		path: filepath.Join(dir, "tokens.json"),
		data: make(map[string]mcptoken),
	}

	tok := mcptoken{
		Token: &oauth2.Token{
			AccessToken:  "abc123",
			RefreshToken: "refresh456",
			TokenType:    "Bearer",
		},
		ClientID: "test-client",
	}
	tok.Endpoints.AuthURL = "https://auth.example.com/authorize"
	tok.Endpoints.TokenURL = "https://auth.example.com/token"

	store.save("https://mcp.example.com/mcp", tok)

	// Reload from disk into a fresh store.
	store2 := &tokenStore{
		path: store.path,
		data: make(map[string]mcptoken),
	}
	store2.load()

	got, ok := store2.get("https://mcp.example.com/mcp")
	require.True(t, ok)
	require.Equal(t, "abc123", got.Token.AccessToken)
	require.Equal(t, "refresh456", got.Token.RefreshToken)
	require.Equal(t, "test-client", got.ClientID)
	require.Equal(t, "https://auth.example.com/authorize", got.Endpoints.AuthURL)
	require.Equal(t, "https://auth.example.com/token", got.Endpoints.TokenURL)
}

func TestTokenStore_FilePermissions(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("Unix file permissions not supported on Windows")
	}
	dir := t.TempDir()
	store := &tokenStore{
		path: filepath.Join(dir, "tokens.json"),
		data: make(map[string]mcptoken),
	}

	store.save("https://server.example.com", mcptoken{
		Token: &oauth2.Token{AccessToken: "secret"},
	})

	info, err := os.Stat(store.path)
	require.NoError(t, err)
	// Token files should only be readable by the owner.
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestHasAuthHeader(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		headers map[string]string
		want    bool
	}{
		{"nil headers", nil, false},
		{"empty headers", map[string]string{}, false},
		{"non-auth headers only", map[string]string{"Content-Type": "application/json"}, false},
		{"Authorization header", map[string]string{"Authorization": "Bearer token"}, true},
		{"lowercase authorization", map[string]string{"authorization": "Bearer token"}, true},
		{"AUTHORIZATION uppercase", map[string]string{"AUTHORIZATION": "Bearer token"}, true},
		{"mixed with auth", map[string]string{"Content-Type": "application/json", "Authorization": "Bearer token"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, hasAuthHeader(tt.headers))
		})
	}
}

func TestMCPOAuthHandler_TokenSourceEmptyByDefault(t *testing.T) {
	// A handler for a server with no saved tokens should return a nil
	// token source, which tells the transport to send requests without
	// an Authorization header (triggering the 401 → Authorize flow).
	t.Setenv("CRUSH_GLOBAL_CONFIG", t.TempDir())
	h := newMCPOAuthHandler("https://mcp.example.com/mcp", nil)
	ts, err := h.TokenSource(t.Context())
	require.NoError(t, err)
	require.Nil(t, ts)
}

func TestTokenStore_GetUnknownServer(t *testing.T) {
	t.Parallel()
	store := &tokenStore{
		path: filepath.Join(t.TempDir(), "tokens.json"),
		data: make(map[string]mcptoken),
	}
	_, ok := store.get("https://never-saved.example.com")
	require.False(t, ok, "get should report ok=false for an unknown server")
}

func TestTokenStore_LoadMissingFile(t *testing.T) {
	t.Parallel()
	// Pointing at a non-existent file must not panic and must leave the
	// store empty (first-run / no-tokens-yet case).
	store := &tokenStore{
		path: filepath.Join(t.TempDir(), "does-not-exist.json"),
		data: make(map[string]mcptoken),
	}
	store.load()
	require.Empty(t, store.data, "load of a missing file should leave the store empty")
	_, ok := store.get("https://x.example.com")
	require.False(t, ok)
}

func TestTokenStore_LoadCorruptFile(t *testing.T) {
	t.Parallel()
	// A corrupt token file must be ignored gracefully rather than crashing
	// Crush on startup.
	path := filepath.Join(t.TempDir(), "tokens.json")
	require.NoError(t, os.WriteFile(path, []byte("{ not valid json"), 0o600))
	store := &tokenStore{path: path, data: make(map[string]mcptoken)}
	require.NotPanics(t, func() { store.load() })
	require.Empty(t, store.data, "a corrupt token file should be ignored, leaving the store empty")
}

func TestMCPOAuthHandler_RestoresSavedToken(t *testing.T) {
	// With a token already persisted for the server, a new handler should
	// restore a non-nil token source so the browser flow is skipped.
	globalDir := t.TempDir()
	t.Setenv("CRUSH_GLOBAL_CONFIG", globalDir)

	serverURL := "https://mcp.example.com/mcp"
	seed := &tokenStore{
		path: filepath.Join(filepath.Dir(config.GlobalConfig()), tokenFileName),
		data: make(map[string]mcptoken),
	}
	saved := mcptoken{
		Token:    &oauth2.Token{AccessToken: "abc123", RefreshToken: "refresh456", TokenType: "Bearer"},
		ClientID: "test-client",
	}
	saved.Endpoints.AuthURL = "https://auth.example.com/authorize"
	saved.Endpoints.TokenURL = "https://auth.example.com/token"
	seed.save(serverURL, saved)

	h := newMCPOAuthHandler(serverURL, nil)
	ts, err := h.TokenSource(t.Context())
	require.NoError(t, err)
	require.NotNil(t, ts, "a saved token should produce a non-nil token source")

	tok, err := ts.Token()
	require.NoError(t, err)
	require.Equal(t, "abc123", tok.AccessToken, "restored token source should yield the saved (unexpired) token")
}

// TestMCPOAuthHandler_RestoredTokenRefreshes proves the persisted client ID and
// token endpoint are enough to refresh an expired token without going back
// through the browser. This is the lifecycle case that fails if the client ID
// is not captured at authorization time.
func TestMCPOAuthHandler_RestoredTokenRefreshes(t *testing.T) {
	var gotGrant, gotRefresh, gotClientID string
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		gotGrant = r.Form.Get("grant_type")
		gotRefresh = r.Form.Get("refresh_token")
		gotClientID = r.Form.Get("client_id")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"fresh-access","token_type":"Bearer","expires_in":3600}`))
	}))
	defer tokenSrv.Close()

	globalDir := t.TempDir()
	t.Setenv("CRUSH_GLOBAL_CONFIG", globalDir)

	serverURL := "https://mcp.example.com/mcp"
	seed := &tokenStore{
		path: filepath.Join(filepath.Dir(config.GlobalConfig()), tokenFileName),
		data: make(map[string]mcptoken),
	}
	saved := mcptoken{
		// An already-expired access token with a refresh token forces the
		// oauth2 library to refresh on the first Token() call.
		Token: &oauth2.Token{
			AccessToken:  "stale-access",
			RefreshToken: "refresh456",
			TokenType:    "Bearer",
			Expiry:       time.Now().Add(-time.Hour),
		},
		ClientID:  "dynamic-client-id",
		AuthStyle: int(oauth2.AuthStyleInParams),
	}
	saved.Endpoints.AuthURL = "https://auth.example.com/authorize"
	saved.Endpoints.TokenURL = tokenSrv.URL
	seed.save(serverURL, saved)

	h := newMCPOAuthHandler(serverURL, nil)
	ts, err := h.TokenSource(t.Context())
	require.NoError(t, err)
	require.NotNil(t, ts)

	tok, err := ts.Token()
	require.NoError(t, err, "an expired token with a saved client ID and token URL must refresh")
	require.Equal(t, "fresh-access", tok.AccessToken)
	require.Equal(t, "refresh_token", gotGrant)
	require.Equal(t, "refresh456", gotRefresh)
	require.Equal(t, "dynamic-client-id", gotClientID, "the persisted client ID must be sent on refresh")
}

// TestMCPOAuthHandler_ForbiddenPreservesToken proves that a genuine
// (non-insufficient_scope) 403 does not wipe a restored token source: the
// handler returns nil so the request is retried and the real error surfaces,
// while TokenSource keeps yielding the saved token.
func TestMCPOAuthHandler_ForbiddenPreservesToken(t *testing.T) {
	globalDir := t.TempDir()
	t.Setenv("CRUSH_GLOBAL_CONFIG", globalDir)

	serverURL := "https://mcp.example.com/mcp"
	seed := &tokenStore{
		path: filepath.Join(filepath.Dir(config.GlobalConfig()), tokenFileName),
		data: make(map[string]mcptoken),
	}
	saved := mcptoken{Token: &oauth2.Token{AccessToken: "keep-me", TokenType: "Bearer"}}
	saved.Endpoints.TokenURL = "https://auth.example.com/token"
	seed.save(serverURL, saved)

	h := newMCPOAuthHandler(serverURL, nil)

	resp := &http.Response{
		StatusCode: http.StatusForbidden,
		Header:     http.Header{"Www-Authenticate": []string{`Bearer error="access_denied"`}},
		Body:       http.NoBody,
	}
	require.NoError(t, h.Authorize(t.Context(), &http.Request{}, resp),
		"a genuine 403 should return nil (retry), not an error")

	ts, err := h.TokenSource(t.Context())
	require.NoError(t, err)
	require.NotNil(t, ts, "the restored token source must survive a genuine 403")
	tok, err := ts.Token()
	require.NoError(t, err)
	require.Equal(t, "keep-me", tok.AccessToken)
}

// TestTokenStore_ConcurrentSavePreservesEntries proves that handlers for
// different servers authorizing concurrently do not clobber each other. In
// production every handler shares one store (see sharedTokenStore), so its
// mutex serializes the read-modify-write. Before that fix each handler held an
// independent snapshot and the last save erased the others' tokens.
func TestTokenStore_ConcurrentSavePreservesEntries(t *testing.T) {
	t.Setenv("CRUSH_GLOBAL_CONFIG", t.TempDir())

	servers := []string{"https://a.example.com", "https://b.example.com", "https://c.example.com"}
	var wg sync.WaitGroup
	for _, server := range servers {
		wg.Add(1)
		go func(server string) {
			defer wg.Done()
			// Each handler resolves its store the same way production does.
			store := sharedTokenStore()
			store.save(server, mcptoken{Token: &oauth2.Token{AccessToken: server}})
		}(server)
	}
	wg.Wait()

	// A fresh read from disk must see every entry.
	final := &tokenStore{
		path: filepath.Join(filepath.Dir(config.GlobalConfig()), tokenFileName),
		data: make(map[string]mcptoken),
	}
	final.load()
	for _, server := range servers {
		got, ok := final.get(server)
		require.True(t, ok, "entry for %s should survive concurrent saves", server)
		require.Equal(t, server, got.Token.AccessToken)
	}
}

// TestSharedTokenStore_SameInstancePerPath proves handlers in one process share
// a single store (and its mutex) rather than each loading an independent
// snapshot of the token file.
func TestSharedTokenStore_SameInstancePerPath(t *testing.T) {
	t.Setenv("CRUSH_GLOBAL_CONFIG", t.TempDir())
	require.Same(t, sharedTokenStore(), sharedTokenStore(),
		"sharedTokenStore must return the same instance for the same config dir")
}
