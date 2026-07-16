package mcp

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

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
	h := newMCPOAuthHandler("https://mcp.example.com/mcp")
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

	h := newMCPOAuthHandler(serverURL)
	ts, err := h.TokenSource(t.Context())
	require.NoError(t, err)
	require.NotNil(t, ts, "a saved token should produce a non-nil token source")

	tok, err := ts.Token()
	require.NoError(t, err)
	require.Equal(t, "abc123", tok.AccessToken, "restored token source should yield the saved (unexpired) token")
}
