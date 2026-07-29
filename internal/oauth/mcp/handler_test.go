package mcpoauth

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

// fakeASOpts configures the fake authorization server so each test can
// exercise a specific branch of the discovery + registration + token flow.
type fakeASOpts struct {
	clientID       string // client_id returned by /register
	accessToken    string // access_token returned by /token for a code exchange
	refreshedToken string // access_token returned by /token for a refresh grant
	refreshToken   string // refresh_token returned by /token
	tokenExpiresIn int    // expires_in returned by /token (0 => 3600)
	failRegister   bool   // make /register return 500 (server has no DCR)
}

// newFakeAS starts an httptest server speaking enough of the OAuth
// discovery, dynamic-registration, and token protocol for the go-sdk
// AuthorizationCodeHandler to run end to end. It returns the base URL and
// the MCP server URL (base + /mcp).
func newFakeAS(t *testing.T, opts fakeASOpts) (base, mcpURL string) {
	t.Helper()
	var baseURL string
	writeJSON := func(w http.ResponseWriter, v any) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(v)
	}

	expiresIn := opts.tokenExpiresIn
	if expiresIn == 0 {
		expiresIn = 3600
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/oauth-protected-resource/mcp", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{
			"resource":              baseURL + "/mcp",
			"authorization_servers": []string{baseURL},
		})
	})
	mux.HandleFunc("/.well-known/oauth-authorization-server", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{
			"issuer":                           baseURL,
			"authorization_endpoint":           baseURL + "/authorize",
			"token_endpoint":                   baseURL + "/token",
			"registration_endpoint":            baseURL + "/register",
			"code_challenge_methods_supported": []string{"S256"},
			"scopes_supported":                 []string{"offline_access"},
		})
	})
	mux.HandleFunc("/register", func(w http.ResponseWriter, r *http.Request) {
		if opts.failRegister {
			http.Error(w, "registration not supported", http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{
			"client_id":                  opts.clientID,
			"token_endpoint_auth_method": "none",
		})
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		access := opts.accessToken
		if r.Form.Get("grant_type") == "refresh_token" && opts.refreshedToken != "" {
			access = opts.refreshedToken
		}
		writeJSON(w, map[string]any{
			"access_token":  access,
			"refresh_token": opts.refreshToken,
			"token_type":    "Bearer",
			"expires_in":    expiresIn,
		})
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	baseURL = srv.URL
	return srv.URL, srv.URL + "/mcp"
}

// browserRedirect simulates the user's browser: it extracts the
// redirect_uri and state from the authorization URL and calls the local
// callback with a fixed code, driving the flow forward without a real
// browser.
func browserRedirect(code string) func(string) error {
	return func(rawAuthURL string) error {
		u, err := url.Parse(rawAuthURL)
		if err != nil {
			return err
		}
		q := u.Query()
		cb, err := url.Parse(q.Get("redirect_uri"))
		if err != nil {
			return err
		}
		cbq := cb.Query()
		cbq.Set("code", code)
		cbq.Set("state", q.Get("state"))
		cb.RawQuery = cbq.Encode()
		go func() {
			resp, err := http.Get(cb.String()) //nolint:noctx
			if err == nil {
				resp.Body.Close()
			}
		}()
		return nil
	}
}

// authorizeWith401 creates a 401 response with the appropriate
// WWW-Authenticate header and passes it to the handler's Authorize
// method. The response body is consumed and closed within this function
// so callers don't need to worry about bodyclose.
func authorizeWith401(t *testing.T, h *Handler, base, mcpURL string) error {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, mcpURL, nil)
	require.NoError(t, err)
	resp := &http.Response{
		StatusCode: http.StatusUnauthorized,
		Header: http.Header{
			"Www-Authenticate": []string{
				`Bearer resource_metadata="` + base + `/.well-known/oauth-protected-resource/mcp"`,
			},
		},
		Body: io.NopCloser(bytes.NewReader(nil)),
	}
	defer resp.Body.Close()
	return h.Authorize(t.Context(), req, resp)
}

// TestHandler_FreshAuthorize drives the whole authorization-code flow and
// asserts the token is captured and persisted together with the registered
// client ID and endpoints, so a later start can refresh without a browser.
func TestHandler_FreshAuthorize(t *testing.T) {
	base, mcpURL := newFakeAS(t, fakeASOpts{
		clientID:     "fresh-client",
		accessToken:  "fresh-access",
		refreshToken: "fresh-refresh",
	})

	var (
		mu    sync.Mutex
		saved *oauth.Token
	)
	h, err := NewHandler("test", mcpURL, nil, nil, func(tok *oauth.Token) {
		mu.Lock()
		saved = tok
		mu.Unlock()
	}, true, 0)
	require.NoError(t, err)
	t.Cleanup(h.Close)
	h.openURL = browserRedirect("fresh-code")

	require.NoError(t, authorizeWith401(t, h, base, mcpURL))

	ts, err := h.TokenSource(t.Context())
	require.NoError(t, err)
	require.NotNil(t, ts)
	tok, err := ts.Token()
	require.NoError(t, err)
	require.Equal(t, "fresh-access", tok.AccessToken)

	mu.Lock()
	defer mu.Unlock()
	require.NotNil(t, saved, "token must be persisted via the saver")
	require.Equal(t, "fresh-access", saved.AccessToken)
	require.Equal(t, "fresh-refresh", saved.RefreshToken)
	require.NotNil(t, saved.Client)
	require.Equal(t, "fresh-client", saved.Client.ClientID)
	require.Equal(t, base+"/token", saved.Client.TokenURL)
}

// TestHandler_PreregisteredClientSkipsDCR proves that a configured client is
// used even when the server does not support dynamic client registration
// (as with GitHub or Slack): the flow authorizes without ever calling
// /register successfully.
func TestHandler_PreregisteredClientSkipsDCR(t *testing.T) {
	base, mcpURL := newFakeAS(t, fakeASOpts{
		accessToken:  "prereg-access",
		refreshToken: "prereg-refresh",
		failRegister: true, // server rejects DCR
	})

	preregistered := &oauth.OAuthClient{ClientID: "configured-client"}
	var saved *oauth.Token
	h, err := NewHandler("test", mcpURL, nil, preregistered, func(tok *oauth.Token) {
		saved = tok
	}, true, 0)
	require.NoError(t, err)
	t.Cleanup(h.Close)
	h.openURL = browserRedirect("prereg-code")

	require.NoError(t, authorizeWith401(t, h, base, mcpURL))

	ts, err := h.TokenSource(t.Context())
	require.NoError(t, err)
	tok, err := ts.Token()
	require.NoError(t, err)
	require.Equal(t, "prereg-access", tok.AccessToken)
	require.NotNil(t, saved)
	require.Equal(t, "configured-client", saved.Client.ClientID)
}

// TestHandler_RestoreSkipsBrowser proves a restored, unexpired token is used
// directly: TokenSource returns it and the browser is never opened.
func TestHandler_RestoreSkipsBrowser(t *testing.T) {
	base, mcpURL := newFakeAS(t, fakeASOpts{clientID: "saved-client"})

	saved := &oauth.Token{
		AccessToken:  "restored-access",
		RefreshToken: "restored-refresh",
		ExpiresAt:    time.Now().Add(time.Hour).Unix(),
		Client: &oauth.OAuthClient{
			ClientID: "saved-client",
			AuthURL:  base + "/authorize",
			TokenURL: base + "/token",
		},
	}

	h, err := NewHandler("test", mcpURL, saved, nil, func(*oauth.Token) {}, false, 0)
	require.NoError(t, err)
	t.Cleanup(h.Close)
	h.openURL = func(string) error {
		t.Error("browser must not open when a valid token is restored")
		return nil
	}

	ts, err := h.TokenSource(t.Context())
	require.NoError(t, err)
	require.NotNil(t, ts)
	tok, err := ts.Token()
	require.NoError(t, err)
	require.Equal(t, "restored-access", tok.AccessToken)
}

// TestHandler_RefreshPersists proves an expired restored token is refreshed
// via the stored token endpoint and the new token is persisted, all without a
// browser.
func TestHandler_RefreshPersists(t *testing.T) {
	base, mcpURL := newFakeAS(t, fakeASOpts{
		clientID:       "saved-client",
		refreshedToken: "refreshed-access",
		refreshToken:   "next-refresh",
	})

	saved := &oauth.Token{
		AccessToken:  "stale-access",
		RefreshToken: "old-refresh",
		ExpiresAt:    time.Now().Add(-time.Hour).Unix(), // expired
		Client: &oauth.OAuthClient{
			ClientID: "saved-client",
			AuthURL:  base + "/authorize",
			TokenURL: base + "/token",
		},
	}

	var (
		mu    sync.Mutex
		saver *oauth.Token
	)
	h, err := NewHandler("test", mcpURL, saved, nil, func(tok *oauth.Token) {
		mu.Lock()
		saver = tok
		mu.Unlock()
	}, false, 0)
	require.NoError(t, err)
	t.Cleanup(h.Close)
	h.openURL = func(string) error {
		t.Error("browser must not open when refreshing a token")
		return nil
	}

	ts, err := h.TokenSource(t.Context())
	require.NoError(t, err)
	tok, err := ts.Token()
	require.NoError(t, err)
	require.Equal(t, "refreshed-access", tok.AccessToken)

	mu.Lock()
	defer mu.Unlock()
	require.NotNil(t, saver, "refreshed token must be persisted")
	require.Equal(t, "refreshed-access", saver.AccessToken)
}

func TestHasRefreshableToken(t *testing.T) {
	t.Parallel()
	full := &oauth.Token{AccessToken: "a", Client: &oauth.OAuthClient{TokenURL: "https://x/token"}}
	tests := []struct {
		name string
		tok  *oauth.Token
		want bool
	}{
		{"nil", nil, false},
		{"no access token", &oauth.Token{Client: &oauth.OAuthClient{TokenURL: "x"}}, false},
		{"no client", &oauth.Token{AccessToken: "a"}, false},
		{"no token url", &oauth.Token{AccessToken: "a", Client: &oauth.OAuthClient{}}, false},
		{"complete", full, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, hasRefreshableToken(tt.tok))
		})
	}
}

// staticSource returns the same token every call, letting us assert the
// saver fires only when the access token actually changes.
type staticSource struct{ tok *oauth2.Token }

func (s staticSource) Token() (*oauth2.Token, error) { return s.tok, nil }

func TestSavingTokenSource_FiresOnChangeOnly(t *testing.T) {
	t.Parallel()

	tok := &oauth2.Token{AccessToken: "same"}
	var calls int
	ts := NewSavingTokenSource(staticSource{tok}, nil, tok, func(*oauth2.Config, *oauth2.Token) {
		calls++
	})

	_, err := ts.Token()
	require.NoError(t, err)
	_, err = ts.Token()
	require.NoError(t, err)
	require.Zero(t, calls, "unchanged token must not trigger the saver")

	changing := &oauth2.Token{AccessToken: "new"}
	ts2 := NewSavingTokenSource(staticSource{changing}, nil, tok, func(*oauth2.Config, *oauth2.Token) {
		calls++
	})
	_, err = ts2.Token()
	require.NoError(t, err)
	require.Equal(t, 1, calls, "changed token must trigger the saver once")
}

func TestSavingTokenSource_NilInputs(t *testing.T) {
	t.Parallel()
	require.Nil(t, NewSavingTokenSource(nil, nil, nil, func(*oauth2.Config, *oauth2.Token) {}))
	src := staticSource{&oauth2.Token{AccessToken: "x"}}
	require.Equal(t, oauth2.TokenSource(src), NewSavingTokenSource(src, nil, nil, nil))
}

// TestHandler_AuthorizeError proves an OAuth error in the callback surfaces
// as an authorization failure rather than a captured token.
func TestHandler_AuthorizeError(t *testing.T) {
	base, mcpURL := newFakeAS(t, fakeASOpts{clientID: "c", accessToken: "a"})
	h, err := NewHandler("test", mcpURL, nil, nil, func(*oauth.Token) {}, true, 0)
	require.NoError(t, err)
	t.Cleanup(h.Close)

	// Simulate the user denying consent: redirect back with an error.
	h.openURL = func(rawAuthURL string) error {
		u, _ := url.Parse(rawAuthURL)
		cb, _ := url.Parse(u.Query().Get("redirect_uri"))
		q := cb.Query()
		q.Set("error", "access_denied")
		q.Set("error_description", "user said no")
		cb.RawQuery = q.Encode()
		go func() {
			resp, gerr := http.Get(cb.String()) //nolint:noctx
			if gerr == nil {
				resp.Body.Close()
			}
		}()
		return nil
	}

	authErr := authorizeWith401(t, h, base, mcpURL)
	require.Error(t, authErr)
	require.Contains(t, authErr.Error(), "access_denied")
}

// TestHandler_BackgroundAuthorizeRefused proves a background (non-interactive)
// connection never opens a browser: Authorize fails fast with
// ErrInteractiveAuthRequired so the caller can surface a needs-auth state.
func TestHandler_BackgroundAuthorizeRefused(t *testing.T) {
	base, mcpURL := newFakeAS(t, fakeASOpts{clientID: "c", accessToken: "a"})
	h, err := NewHandler("test", mcpURL, nil, nil, func(*oauth.Token) {}, false, 0)
	require.NoError(t, err)
	t.Cleanup(h.Close)
	h.openURL = func(string) error {
		t.Error("browser must not open for a background connection")
		return nil
	}

	err = authorizeWith401(t, h, base, mcpURL)
	require.ErrorIs(t, err, ErrInteractiveAuthRequired)
}
