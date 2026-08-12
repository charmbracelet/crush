package mcpoauth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
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
	// issSupported advertises RFC 9207: the server promises to name itself
	// in the authorization response, and the SDK rejects the authorization
	// if no issuer comes back.
	issSupported bool
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
		meta := map[string]any{
			"issuer":                           baseURL,
			"authorization_endpoint":           baseURL + "/authorize",
			"token_endpoint":                   baseURL + "/token",
			"registration_endpoint":            baseURL + "/register",
			"code_challenge_methods_supported": []string{"S256"},
			"scopes_supported":                 []string{"offline_access"},
		}
		if opts.issSupported {
			meta["authorization_response_iss_parameter_supported"] = true
		}
		writeJSON(w, meta)
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
	return browserRedirectIss(code, "")
}

// browserRedirectIss is browserRedirect for a server that implements RFC
// 9207: the redirect also names the issuer. Pass an empty iss for a server
// that does not.
func browserRedirectIss(code, iss string) func(string) error {
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
		if iss != "" {
			cbq.Set("iss", iss)
		}
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

// TestHandler_BrowserSuppressed proves SetBrowserSuppress prevents the
// browser from opening while still recording the authorization URL, which
// is how a remote client surfaces the flow on the user's machine. The
// returned restore function re-enables the browser.
func TestHandler_BrowserSuppressed(t *testing.T) {
	base, mcpURL := newFakeAS(t, fakeASOpts{clientID: "c", accessToken: "a"})
	h, err := NewHandler("test", mcpURL, nil, nil, func(*oauth.Token) {}, true, 0)
	require.NoError(t, err)
	t.Cleanup(h.Close)

	var opens atomic.Int64
	h.openURL = func(string) error {
		opens.Add(1)
		return browserRedirect("code")(h.AuthURL())
	}

	restore := h.SetBrowserSuppress(true)

	// Suppressed: the flow generates the URL but never opens a browser.
	// Drive the callback manually using the recorded URL.
	done := make(chan error, 1)
	go func() {
		done <- authorizeWith401(t, h, base, mcpURL)
	}()

	require.Eventually(t, func() bool { return h.AuthURL() != "" },
		2*time.Second, 10*time.Millisecond, "auth URL should be recorded")
	require.Equal(t, int64(0), opens.Load(), "browser must not open while suppressed")

	// Complete the flow by simulating the browser redirect.
	require.NoError(t, browserRedirect("code")(h.AuthURL()))
	require.NoError(t, <-done)
	require.Equal(t, int64(0), opens.Load())

	// Restoring re-enables the browser.
	restore()
	h.mu.Lock()
	suppressed := h.suppressBrowser
	h.mu.Unlock()
	require.False(t, suppressed)
}

// TestCallbackReceiver_IgnoresNonCallbackPaths is a regression test for a
// browser incidentally aborting the flow. The listener answered every path,
// so a request for something like /favicon.ico could win the one-time
// handoff and hand the flow an empty authorization code.
func TestCallbackReceiver_IgnoresNonCallbackPaths(t *testing.T) {
	t.Parallel()

	r := &callbackReceiver{}
	t.Cleanup(r.close)

	base := serveReceiver(t, r)

	flight, owned, err := r.begin()
	require.NoError(t, err)
	require.True(t, owned)

	// A stray request must not be mistaken for the redirect.
	resp, err := http.Get(base + "/favicon.ico") //nolint:noctx
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
	require.Empty(t, flight.done, "a stray request must not complete the flow")

	// The real redirect still lands.
	resp, err = http.Get(base + callbackPath + "?code=abc&state=xyz") //nolint:noctx
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Contains(t, string(body), "You’re all set")

	<-flight.done
	require.NoError(t, flight.err)
	require.Equal(t, "abc", flight.result.Code)
	require.Equal(t, "xyz", flight.result.State)
}

// TestCallbackReceiver_RendersFailurePage proves a denied authorization
// reaches the user as a readable page naming the server, not just a status
// line in the terminal.
func TestCallbackReceiver_RendersFailurePage(t *testing.T) {
	t.Parallel()

	r := &callbackReceiver{serverName: "linear"}
	t.Cleanup(r.close)

	base := serveReceiver(t, r)

	flight, owned, err := r.begin()
	require.NoError(t, err)
	require.True(t, owned)

	url := base + callbackPath +
		"?error=access_denied&error_description=user+said+no"
	resp, err := http.Get(url) //nolint:noctx
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	require.NoError(t, err)

	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	require.Contains(t, string(body), "access_denied")
	require.Contains(t, string(body), "user said no")
	require.Contains(t, string(body), "linear")

	<-flight.done
	require.ErrorContains(t, flight.err, "access_denied")
}

// TestCallbackReceiver_ConcurrentAuthorizeOpensOneTab is a regression test
// for the browser opening twice for a single login. Connecting to a server
// can put several requests in flight, and each one that meets a 401 asks to
// authorize. Every ask used to open its own tab and then contend for the
// single redirect, so the user saw two tabs and one of the two requests
// waited on a redirect that had already been consumed, hanging until its
// context expired.
func TestCallbackReceiver_ConcurrentAuthorizeOpensOneTab(t *testing.T) {
	t.Parallel()

	r := &callbackReceiver{serverName: "linear"}
	t.Cleanup(r.close)

	base := serveReceiver(t, r)

	// Stand in for the browser: record the open, then redirect back as the
	// authorization server would once the user consents.
	var opens atomic.Int64
	r.handler = &Handler{openURL: func(string) error {
		opens.Add(1)
		go func() {
			resp, gerr := http.Get(base + callbackPath + "?code=abc&state=xyz") //nolint:noctx
			if gerr == nil {
				resp.Body.Close()
			}
		}()
		return nil
	}}

	const callers = 4
	var wg sync.WaitGroup
	results := make(chan *auth.AuthorizationResult, callers)
	errs := make(chan error, callers)
	for range callers {
		wg.Go(func() {
			result, ferr := r.fetchAuthorizationCode(t.Context(), &auth.AuthorizationArgs{URL: base + "/authorize"})
			results <- result
			errs <- ferr
		})
	}
	wg.Wait()
	close(results)
	close(errs)

	require.Equal(t, int64(1), opens.Load(), "one login must open exactly one tab")

	// Every caller is served the same authorization code, and none is left
	// waiting on a redirect it will never see.
	for err := range errs {
		require.NoError(t, err)
	}
	for result := range results {
		require.NotNil(t, result)
		require.Equal(t, "abc", result.Code)
	}
}

// TestCallbackReceiver_AuthorizeTwiceInSequence proves a later
// authorization still works. The handler outlives a single login (tokens
// get revoked, servers reconnect), and the redirect handoff used to be
// once-per-handler, so any second attempt hung forever.
func TestCallbackReceiver_AuthorizeTwiceInSequence(t *testing.T) {
	t.Parallel()

	r := &callbackReceiver{serverName: "linear"}
	t.Cleanup(r.close)

	base := serveReceiver(t, r)

	var opens atomic.Int64
	code := "first"
	r.handler = &Handler{openURL: func(string) error {
		opens.Add(1)
		go func() {
			resp, gerr := http.Get(base + callbackPath + "?code=" + code) //nolint:noctx
			if gerr == nil {
				resp.Body.Close()
			}
		}()
		return nil
	}}

	args := &auth.AuthorizationArgs{URL: base + "/authorize"}

	result, err := r.fetchAuthorizationCode(t.Context(), args)
	require.NoError(t, err)
	require.Equal(t, "first", result.Code)

	code = "second"
	result, err = r.fetchAuthorizationCode(t.Context(), args)
	require.NoError(t, err)
	require.Equal(t, "second", result.Code)

	require.Equal(t, int64(2), opens.Load(), "each login opens its own tab")
}

// serveReceiver binds the receiver's listener and returns the base URL
// the authorization server would redirect to. The tests construct the
// receiver directly (fixedPort 0) and run in parallel, so pin an
// ephemeral port rather than a shared callbackPorts entry — otherwise
// parallel runs collide on the same candidate.
func serveReceiver(t *testing.T, r *callbackReceiver) string {
	t.Helper()
	if r.fixedPort == 0 {
		lc := &net.ListenConfig{}
		probe, err := lc.Listen(t.Context(), "tcp", "localhost:0")
		require.NoError(t, err)
		r.fixedPort = probe.Addr().(*net.TCPAddr).Port
		_ = probe.Close()
	}
	require.NoError(t, r.bind())
	return fmt.Sprintf("http://localhost:%d", r.port)
}

// TestHandler_PassesIssuerThrough is a regression test for logins failing
// against servers that implement RFC 9207. Such a server names itself in
// the redirect and the SDK rejects the authorization when that name does
// not come back, so dropping it broke the connection outright and sent the
// user round the browser flow again.
func TestHandler_PassesIssuerThrough(t *testing.T) {
	base, mcpURL := newFakeAS(t, fakeASOpts{
		clientID:     "c",
		accessToken:  "a",
		issSupported: true,
	})
	h, err := NewHandler("test", mcpURL, nil, nil, func(*oauth.Token) {}, true, 0)
	require.NoError(t, err)
	t.Cleanup(h.Close)

	var opens atomic.Int64
	redirect := browserRedirectIss("code123", base)
	h.openURL = func(u string) error {
		opens.Add(1)
		return redirect(u)
	}

	require.NoError(t, authorizeWith401(t, h, base, mcpURL))
	require.Equal(t, int64(1), opens.Load())
	require.NotNil(t, h.Token())
	require.Equal(t, "a", h.Token().AccessToken)
}

// TestHandler_RejectsWrongIssuer confirms the issuer is passed through for
// checking rather than merely echoed: a redirect naming a different server
// must fail the login.
func TestHandler_RejectsWrongIssuer(t *testing.T) {
	base, mcpURL := newFakeAS(t, fakeASOpts{
		clientID:     "c",
		accessToken:  "a",
		issSupported: true,
	})
	h, err := NewHandler("test", mcpURL, nil, nil, func(*oauth.Token) {}, true, 0)
	require.NoError(t, err)
	t.Cleanup(h.Close)

	h.openURL = browserRedirectIss("code123", "https://attacker.example.com")

	require.ErrorContains(t, authorizeWith401(t, h, base, mcpURL), "issuer")
}

// newFakeMCP starts a server that refuses requests until a bearer token is
// present, pointing at the given authorization server. It returns the MCP
// endpoint URL.
func newFakeMCP(t *testing.T, authServer string) string {
	t.Helper()
	var srv *httptest.Server
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/oauth-protected-resource/mcp", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"resource":              srv.URL + "/mcp",
			"authorization_servers": []string{authServer},
		})
	})
	mux.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			w.Header().Set("WWW-Authenticate",
				`Bearer resource_metadata="`+srv.URL+`/.well-known/oauth-protected-resource/mcp"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		// Authorization is all this fake is for; the MCP handshake itself
		// is left to fail so the test stays focused.
		w.WriteHeader(http.StatusNotFound)
	})
	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv.URL + "/mcp"
}

// TestConnect_OneLoginOpensOneTab drives a real client connection against a
// server that requires authorization, and is the regression test for a
// single login opening two browser tabs.
//
// Connecting makes more than one request, and each refusal asks to log in.
// The first tab's redirect was being accepted but then rejected during
// validation, so the login failed just as the user finished it and the next
// request opened another tab.
func TestConnect_OneLoginOpensOneTab(t *testing.T) {
	authServer, _ := newFakeAS(t, fakeASOpts{
		clientID:     "c",
		accessToken:  "tok",
		issSupported: true,
	})
	endpoint := newFakeMCP(t, authServer)

	h, err := NewHandler("test", endpoint, nil, nil, func(*oauth.Token) {}, true, 0)
	require.NoError(t, err)
	t.Cleanup(h.Close)

	var opens atomic.Int64
	redirect := browserRedirectIss("code123", authServer)
	h.openURL = func(u string) error {
		opens.Add(1)
		return redirect(u)
	}

	client := mcp.NewClient(&mcp.Implementation{Name: "crush", Version: "test"}, nil)
	// The handshake itself fails by design; the tab count is the subject.
	_, _ = client.Connect(t.Context(), &mcp.StreamableClientTransport{
		Endpoint:     endpoint,
		OAuthHandler: h,
	}, nil)

	require.Equal(t, int64(1), opens.Load(), "one login must open exactly one browser tab")
	require.NotNil(t, h.Token(), "the login must yield a usable token")
}
