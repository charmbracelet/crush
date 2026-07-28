package mcp

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/modelcontextprotocol/go-sdk/auth"
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

// TestTokenStore_SaveMergesWithDisk pins the multi-server persistence fix:
// each HTTP MCP server gets its own handler with its own store snapshot, and
// a save used to rewrite the whole file from that snapshot — server B's save
// erased server A's token. Save now merges with what's on disk.
func TestTokenStore_SaveMergesWithDisk(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "tokens.json")

	// Handler A's store saves server A.
	storeA := &tokenStore{path: path, data: make(map[string]mcptoken)}
	storeA.load()
	storeA.save("https://a.example.com/mcp", mcptoken{Token: &oauth2.Token{AccessToken: "tok-a"}})

	// Handler B's store loaded before/without A's save (its own snapshot).
	storeB := &tokenStore{path: path, data: make(map[string]mcptoken)}
	storeB.save("https://b.example.com/mcp", mcptoken{Token: &oauth2.Token{AccessToken: "tok-b"}})

	// Both tokens must survive on disk.
	check := &tokenStore{path: path, data: make(map[string]mcptoken)}
	check.load()
	gotA, okA := check.get("https://a.example.com/mcp")
	require.True(t, okA, "server A's token was erased by server B's save")
	require.Equal(t, "tok-a", gotA.Token.AccessToken)
	gotB, okB := check.get("https://b.example.com/mcp")
	require.True(t, okB)
	require.Equal(t, "tok-b", gotB.Token.AccessToken)
}

// TestPersistingTokenSource_RepersistsRotatedToken pins the refresh fix:
// OAuth 2.1 public clients rotate the refresh token on use; a refresh that
// stayed only in memory invalidated the copy on disk, so the NEXT restart
// fell back to the browser flow. The wrapping source re-persists on change.
func TestPersistingTokenSource_RepersistsRotatedToken(t *testing.T) {
	globalDir := t.TempDir()
	t.Setenv("CRUSH_GLOBAL_CONFIG", globalDir)

	serverURL := "https://rotate.example.com/mcp"
	h := newMCPOAuthHandler(serverURL)
	h.clientID = "client-1"
	h.endpoints = oauthEndpoints{AuthURL: "https://as.example.com/auth", TokenURL: "https://as.example.com/token"}

	rotated := &oauth2.Token{AccessToken: "new-access", RefreshToken: "new-refresh"}
	src := &persistingTokenSource{
		h:    h,
		src:  oauth2.StaticTokenSource(rotated),
		last: &oauth2.Token{AccessToken: "old-access", RefreshToken: "old-refresh"},
	}

	tok, err := src.Token()
	require.NoError(t, err)
	require.Equal(t, "new-access", tok.AccessToken)

	// The rotated token must be on disk, with the client ID and endpoints.
	check := newTokenStore()
	check.load()
	got, ok := check.get(serverURL)
	require.True(t, ok, "rotated token was not re-persisted")
	require.Equal(t, "new-access", got.Token.AccessToken)
	require.Equal(t, "new-refresh", got.Token.RefreshToken)
	require.Equal(t, "client-1", got.ClientID)
	require.Equal(t, "https://as.example.com/token", got.Endpoints.TokenURL)
}

// TestCaptureFromAuthURL pins the client-ID persistence fix: the SDK never
// exposes the dynamically registered client ID, but it necessarily appears
// as client_id in the authorization URL handed to the code fetcher. Before
// the fix, ClientID was persisted as "" on every path, so refresh after
// restart could never work (a public client must send client_id).
func TestCaptureFromAuthURL(t *testing.T) {
	t.Setenv("CRUSH_GLOBAL_CONFIG", t.TempDir())
	h := newMCPOAuthHandler("https://cap.example.com/mcp")

	h.captureFromAuthURL("https://as.example.com/authorize?client_id=dyn-client-42&response_type=code&state=xyz")

	require.Equal(t, "dyn-client-42", h.clientID)
	require.Equal(t, "https://as.example.com/authorize", h.endpoints.AuthURL)

	// persistToken must write the captured ID (TokenURL pre-set so the
	// test performs no network discovery).
	h.endpoints.TokenURL = "https://as.example.com/token"
	h.persistToken(&oauth2.Token{AccessToken: "t"})
	check := newTokenStore()
	check.load()
	got, ok := check.get("https://cap.example.com/mcp")
	require.True(t, ok)
	require.Equal(t, "dyn-client-42", got.ClientID, "registered client ID must be persisted")
}

// TestCallbackHandler_DuplicateHitDoesNotBlock pins the wedge fix: a second
// /callback request (browser refresh, prefetcher, AS retry) used to block
// its handler goroutine forever on the full capacity-1 channel; the deferred
// srv.Shutdown then waited for that handler indefinitely and the whole
// authorization flow leaked.
func TestCallbackHandler_DuplicateHitDoesNotBlock(t *testing.T) {
	t.Parallel()
	resultCh := make(chan auth.AuthorizationResult, 1)
	errCh := make(chan error, 1)
	handler := callbackHandler("s1", resultCh, errCh)

	hit := func() {
		req := httptest.NewRequestWithContext(t.Context(), "GET", "/callback?code=abc&state=s1", nil)
		handler(httptest.NewRecorder(), req)
	}

	hit() // first delivery fills the channel

	done := make(chan struct{})
	go func() {
		hit() // duplicate — must not block
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("duplicate callback hit blocked (would wedge srv.Shutdown and leak the auth flow)")
	}

	require.Len(t, resultCh, 1, "exactly one result delivered")
	got := <-resultCh
	require.Equal(t, "abc", got.Code)
}

// TestCallbackHandler_RejectsStateMismatch pins the state-validation fix:
// the first /callback hit used to win regardless of the state parameter, so
// a stray prefetch or browser-refresh request with the wrong (or missing)
// state could fill the result channel and cause the legitimate callback to
// be dropped. State is now validated against the value the SDK baked into
// the authorization URL.
func TestCallbackHandler_RejectsStateMismatch(t *testing.T) {
	t.Parallel()
	resultCh := make(chan auth.AuthorizationResult, 1)
	errCh := make(chan error, 1)
	handler := callbackHandler("expected-state", resultCh, errCh)

	// Wrong state: must respond 400, must NOT deliver a result, and must NOT
	// signal errCh. errCh is terminal for openBrowserAndCapture's select, so
	// a wrong-state hit that reported there would abort the whole flow — the
	// opposite of "cannot crowd out the legitimate callback". It is dropped.
	rec := httptest.NewRecorder()
	handler(rec, httptest.NewRequestWithContext(t.Context(), "GET", "/callback?code=abc&state=wrong-state", nil))

	require.Equal(t, http.StatusBadRequest, rec.Code,
		"state mismatch should respond with 400 Bad Request")
	require.Empty(t, resultCh, "state mismatch must not deliver a result")
	require.Empty(t, errCh, "state mismatch must not signal errCh (that would abort the flow)")

	// The legitimate callback that follows must still win the result slot —
	// the stray hit neither consumed it nor tore the flow down.
	handler(httptest.NewRecorder(),
		httptest.NewRequestWithContext(t.Context(), "GET", "/callback?code=abc&state=expected-state", nil))
	require.Len(t, resultCh, 1, "legitimate callback after a stray must still deliver")
	got := <-resultCh
	require.Equal(t, "abc", got.Code)
}

// TestCallbackHandler_RejectsMissingState covers the unhappy path where the
// callback carries no state at all (defensive: the AS should always echo it,
// but a malformed redirect or a hand-crafted URL could omit it).
func TestCallbackHandler_RejectsMissingState(t *testing.T) {
	t.Parallel()
	resultCh := make(chan auth.AuthorizationResult, 1)
	errCh := make(chan error, 1)
	handler := callbackHandler("expected-state", resultCh, errCh)

	rec := httptest.NewRecorder()
	handler(rec, httptest.NewRequestWithContext(t.Context(), "GET", "/callback?code=abc", nil))

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Empty(t, resultCh, "missing state must not deliver a result")
	require.Empty(t, errCh, "missing state must not signal errCh (that would abort the flow)")
}

// TestCallbackHandler_EmptyExpectedStateSkipsValidation covers the backward-
// compat path: when no expected state is configured (e.g. the authorization
// URL lacked a state parameter), validation is skipped so the flow degrades
// to the prior first-hit-wins behavior rather than rejecting everything.
func TestCallbackHandler_EmptyExpectedStateSkipsValidation(t *testing.T) {
	t.Parallel()
	resultCh := make(chan auth.AuthorizationResult, 1)
	errCh := make(chan error, 1)
	handler := callbackHandler("", resultCh, errCh)

	handler(httptest.NewRecorder(),
		httptest.NewRequestWithContext(t.Context(), "GET", "/callback?code=abc&state=anything", nil))

	require.Len(t, resultCh, 1, "with no expected state, any callback is accepted")
	got := <-resultCh
	require.Equal(t, "abc", got.Code)
}

// TestCallbackHandler_RepeatedStateMismatchDropsCleanly pins that stray
// wrong-state callbacks are dropped (400) without ever signaling resultCh or
// errCh — so no number of them can consume the result slot or abort the flow
// via errCh — and a later legitimate callback still succeeds.
func TestCallbackHandler_RepeatedStateMismatchDropsCleanly(t *testing.T) {
	t.Parallel()
	resultCh := make(chan auth.AuthorizationResult, 1)
	errCh := make(chan error, 1)
	handler := callbackHandler("expected", resultCh, errCh)

	for range 3 {
		rec := httptest.NewRecorder()
		handler(rec, httptest.NewRequestWithContext(t.Context(), "GET", "/callback?code=abc&state=wrong", nil))
		require.Equal(t, http.StatusBadRequest, rec.Code)
	}
	require.Empty(t, resultCh, "stray wrong-state hits must not deliver a result")
	require.Empty(t, errCh, "stray wrong-state hits must not signal errCh")

	// The legitimate callback still wins after any number of strays.
	handler(httptest.NewRecorder(),
		httptest.NewRequestWithContext(t.Context(), "GET", "/callback?code=real&state=expected", nil))
	require.Len(t, resultCh, 1)
	got := <-resultCh
	require.Equal(t, "real", got.Code)
}

// TestStateFromAuthURL covers the helper that pulls the OAuth state out of
// the authorization URL the SDK builds. The state is generated by the SDK
// and echoed back by the AS; our callback validates against it.
func TestStateFromAuthURL(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		authURL string
		want    string
	}{
		{
			name:    "typical URL with state",
			authURL: "https://as.example.com/authorize?response_type=code&client_id=cid&state=abc123",
			want:    "abc123",
		},
		{
			name:    "state with special chars",
			authURL: "https://as.example.com/auth?state=abc-_123",
			want:    "abc-_123",
		},
		{
			name:    "missing state",
			authURL: "https://as.example.com/authorize?response_type=code&client_id=cid",
			want:    "",
		},
		{
			name:    "malformed URL",
			authURL: "://not-a-url",
			want:    "",
		},
		{
			name:    "empty URL",
			authURL: "",
			want:    "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, stateFromAuthURL(tt.authURL))
		})
	}
}

// TestTokenStore_ConcurrentCrossStoreSavesKeepAllTokens pins that saves
// racing across DIFFERENT store instances (one per HTTP MCP handler in
// production) cannot erase each other's entries. With only the per-store
// mutex, both stores could read the file before either wrote it, and the
// last writer dropped the other's token; the shared file mutex serializes
// them so the read-merge-write in save always sees the previous winner.
func TestTokenStore_ConcurrentCrossStoreSavesKeepAllTokens(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "tokens.json")
	const n = 8

	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range n {
		store := &tokenStore{path: path, data: make(map[string]mcptoken)}
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			store.save(
				fmt.Sprintf("https://mcp%d.example.com/mcp", i),
				mcptoken{Token: &oauth2.Token{AccessToken: fmt.Sprintf("tok-%d", i)}},
			)
		}()
	}
	close(start)
	wg.Wait()

	check := &tokenStore{path: path, data: make(map[string]mcptoken)}
	check.load()
	for i := range n {
		_, ok := check.get(fmt.Sprintf("https://mcp%d.example.com/mcp", i))
		require.True(t, ok, "server %d's token was erased by a concurrent save from another store", i)
	}
}

// TestCaptureFromAuthURL_OverwritesStaleAuthEndpoint pins the re-authorize
// healing path: a live authorization is the freshest source of the auth
// endpoint, so a previously cached (possibly stale) value must be replaced,
// not kept forever.
func TestCaptureFromAuthURL_OverwritesStaleAuthEndpoint(t *testing.T) {
	t.Setenv("CRUSH_GLOBAL_CONFIG", t.TempDir())
	h := newMCPOAuthHandler("https://stale.example.com/mcp")
	h.endpoints.AuthURL = "https://old-as.example.com/authorize"
	h.endpoints.TokenURL = "https://old-as.example.com/token"

	h.captureFromAuthURL("https://new-as.example.com/authorize?client_id=cid&response_type=code")

	require.Equal(t, "https://new-as.example.com/authorize", h.endpoints.AuthURL,
		"a live authorization must replace the cached auth endpoint")
}

// TestTokenStore_SaveLeavesNoTempFile pins the atomic-write fix: save used
// to call os.WriteFile directly, so a crash mid-write left a truncated
// mcp-oauth-tokens.json that wiped tokens for EVERY server (the corrupt-
// file load path degrades to "no tokens", forcing a browser re-auth for
// all of them). The fix writes to a temp file in the same directory and
// renames it over the target, so the on-disk file is either the previous
// contents or the new contents — never a partial write. This test asserts
// the observable contract: after save returns, no temp file litters the
// directory.
func TestTokenStore_SaveLeavesNoTempFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "tokens.json")
	store := &tokenStore{path: path, data: make(map[string]mcptoken)}

	store.save("https://mcp.example.com", mcptoken{
		Token: &oauth2.Token{AccessToken: "tok"},
	})

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	for _, e := range entries {
		// The only regular file should be the final token file. Any
		// leftover *.tmp / hidden file means the atomic rename leaked.
		require.False(t, isLikelyTempFile(e.Name()),
			"unexpected temp file left in token dir: %s", e.Name())
	}
}

// isLikelyTempFile reports whether a filename looks like an intermediate
// temp file used during an atomic write. config.AtomicWriteFile names them
// "<base>.<random>.tmp" (e.g. "tokens.json.123456.tmp"); the ".tmp-" form is
// also matched defensively. The final token file itself is NOT a temp file.
func isLikelyTempFile(name string) bool {
	return strings.HasSuffix(name, ".tmp") ||
		strings.Contains(name, ".tmp-")
}

// TestTokenStore_SaveAtomicOverwrite pins that a save over an existing file
// replaces it atomically: after the call, the file contains the new
// contents in full and is still readable as valid JSON. (Before the fix,
// os.WriteFile truncated-then-wrote, so a crash between truncate and the
// final write byte left a syntactically invalid prefix on disk.)
func TestTokenStore_SaveAtomicOverwrite(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "tokens.json")
	store := &tokenStore{path: path, data: make(map[string]mcptoken)}

	// Seed the file.
	store.save("https://a.example.com", mcptoken{
		Token: &oauth2.Token{AccessToken: "tok-a"},
	})

	// Overwrite with a different entry.
	store2 := &tokenStore{path: path, data: make(map[string]mcptoken)}
	store2.load()
	store2.save("https://b.example.com", mcptoken{
		Token: &oauth2.Token{AccessToken: "tok-b"},
	})

	// The file on disk must be valid JSON and contain both entries (the
	// save merges with disk; see TestTokenStore_SaveMergesWithDisk).
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	require.NotEmpty(t, raw, "token file must not be empty after overwrite")

	check := &tokenStore{path: path, data: make(map[string]mcptoken)}
	check.load()
	gotA, okA := check.get("https://a.example.com")
	require.True(t, okA, "previous entry lost during atomic overwrite")
	require.Equal(t, "tok-a", gotA.Token.AccessToken)
	gotB, okB := check.get("https://b.example.com")
	require.True(t, okB)
	require.Equal(t, "tok-b", gotB.Token.AccessToken)
}

// TestTokenStore_SaveKeepsModeOnOverwrite pins the 0600 mode is preserved
// across an atomic overwrite. Rename inherits the source file's mode, so
// the temp file must be chmod'd to 0600 before the rename; otherwise the
// final file would silently inherit os.CreateTemp's default mode and leak
// refresh tokens to other users on the host.
func TestTokenStore_SaveKeepsModeOnOverwrite(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("Unix file permissions not supported on Windows")
	}
	path := filepath.Join(t.TempDir(), "tokens.json")

	first := &tokenStore{path: path, data: make(map[string]mcptoken)}
	first.save("https://a.example.com", mcptoken{
		Token: &oauth2.Token{AccessToken: "tok-a"},
	})

	// Second save should preserve 0600 even though it goes through a temp
	// file + rename.
	second := &tokenStore{path: path, data: make(map[string]mcptoken)}
	second.load()
	second.save("https://b.example.com", mcptoken{
		Token: &oauth2.Token{AccessToken: "tok-b"},
	})

	info, err := os.Stat(path)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm(),
		"token file mode must remain 0600 after an atomic overwrite")
}
