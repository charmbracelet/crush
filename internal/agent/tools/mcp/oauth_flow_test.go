package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/oauthex"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

// oauthServerOpts configures the fake authorization server so each test can
// exercise a specific happy or unhappy branch of discovery + registration.
type oauthServerOpts struct {
	servePRM             bool   // serve protected-resource metadata
	serveASM             bool   // serve authorization-server metadata
	registrationEndpoint bool   // advertise a registration_endpoint in the ASM
	registerStatus       int    // status returned by /register (0 => 200)
	clientID             string // client_id returned by /register
	accessToken          string // access_token returned by /token (enables /token)
}

// newOAuthTestServer starts an httptest server that speaks just enough of the
// OAuth discovery + dynamic-registration protocol for discoverAndRegister to
// run against it. It returns the base URL and the MCP server URL (base + /mcp).
func newOAuthTestServer(t *testing.T, opts oauthServerOpts) (base, mcpURL string) {
	t.Helper()
	var baseURL string
	writeJSON := func(w http.ResponseWriter, v any) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(v)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/oauth-protected-resource/mcp", func(w http.ResponseWriter, r *http.Request) {
		if !opts.servePRM {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, map[string]any{
			"resource":              baseURL + "/mcp",
			"authorization_servers": []string{baseURL},
		})
	})
	mux.HandleFunc("/.well-known/oauth-authorization-server", func(w http.ResponseWriter, r *http.Request) {
		if !opts.serveASM {
			http.NotFound(w, r)
			return
		}
		m := map[string]any{
			"issuer":                           baseURL,
			"authorization_endpoint":           baseURL + "/authorize",
			"token_endpoint":                   baseURL + "/token",
			"code_challenge_methods_supported": []string{"S256"},
		}
		if opts.registrationEndpoint {
			m["registration_endpoint"] = baseURL + "/register"
		}
		writeJSON(w, m)
	})
	mux.HandleFunc("/register", func(w http.ResponseWriter, r *http.Request) {
		if opts.registerStatus != 0 {
			w.WriteHeader(opts.registerStatus)
			return
		}
		writeJSON(w, map[string]any{
			"client_id":                  opts.clientID,
			"token_endpoint_auth_method": "none",
		})
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		if opts.accessToken == "" {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, map[string]any{
			"access_token":  opts.accessToken,
			"refresh_token": "e2e-refresh",
			"token_type":    "Bearer",
			"expires_in":    3600,
		})
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	baseURL = srv.URL
	return srv.URL, srv.URL + "/mcp"
}

func TestDiscoverAndRegister(t *testing.T) {
	t.Parallel()

	t.Run("happy path captures client id and endpoints", func(t *testing.T) {
		t.Parallel()
		base, mcpURL := newOAuthTestServer(t, oauthServerOpts{
			servePRM: true, serveASM: true, registrationEndpoint: true,
			clientID: "captured-client-id",
		})
		h := &mcpOAuthHandler{serverURL: mcpURL}
		reg, err := h.discoverAndRegister(t.Context(), "http://localhost:1/callback")
		require.NoError(t, err)
		require.Equal(t, "captured-client-id", reg.clientID)
		require.Equal(t, base+"/authorize", reg.authURL)
		require.Equal(t, base+"/token", reg.tokenURL)
	})

	t.Run("no OAuth metadata anywhere", func(t *testing.T) {
		t.Parallel()
		_, mcpURL := newOAuthTestServer(t, oauthServerOpts{})
		h := &mcpOAuthHandler{serverURL: mcpURL}
		_, err := h.discoverAndRegister(t.Context(), "http://localhost:1/callback")
		require.Error(t, err)
		require.Contains(t, err.Error(), "authorization server")
	})

	t.Run("auth server without registration endpoint", func(t *testing.T) {
		t.Parallel()
		_, mcpURL := newOAuthTestServer(t, oauthServerOpts{
			servePRM: true, serveASM: true, registrationEndpoint: false,
		})
		h := &mcpOAuthHandler{serverURL: mcpURL}
		_, err := h.discoverAndRegister(t.Context(), "http://localhost:1/callback")
		require.Error(t, err)
		require.Contains(t, err.Error(), "does not support dynamic client registration")
	})

	t.Run("registration endpoint rejects the request", func(t *testing.T) {
		t.Parallel()
		_, mcpURL := newOAuthTestServer(t, oauthServerOpts{
			servePRM: true, serveASM: true, registrationEndpoint: true,
			registerStatus: http.StatusBadRequest,
		})
		h := &mcpOAuthHandler{serverURL: mcpURL}
		_, err := h.discoverAndRegister(t.Context(), "http://localhost:1/callback")
		require.Error(t, err)
		require.Contains(t, err.Error(), "dynamic client registration failed")
	})

	t.Run("registration returns an empty client id", func(t *testing.T) {
		t.Parallel()
		_, mcpURL := newOAuthTestServer(t, oauthServerOpts{
			servePRM: true, serveASM: true, registrationEndpoint: true,
			clientID: "",
		})
		h := &mcpOAuthHandler{serverURL: mcpURL}
		_, err := h.discoverAndRegister(t.Context(), "http://localhost:1/callback")
		require.Error(t, err)
	})

	t.Run("invalid MCP server URL", func(t *testing.T) {
		t.Parallel()
		h := &mcpOAuthHandler{serverURL: "://not-a-url"}
		_, err := h.discoverAndRegister(t.Context(), "http://localhost:1/callback")
		require.Error(t, err)
	})
}

// TestAuthorize_EndToEnd drives the entire authorization-code flow against a
// fake OAuth server, simulating the browser by having the injected opener hit
// the callback with a valid code. It proves the token is obtained, cached, and
// persisted together with the dynamically registered client ID (so a later
// restart can refresh), and that the connection timeout is suspended along the
// way.
func TestAuthorize_EndToEnd(t *testing.T) {
	_, mcpURL := newOAuthTestServer(t, oauthServerOpts{
		servePRM: true, serveASM: true, registrationEndpoint: true,
		clientID: "e2e-client-id", accessToken: "e2e-access",
	})

	// Simulate the user's browser: extract the redirect_uri and state from the
	// authorization URL and redirect back to the local callback with a code.
	simulateBrowser := func(rawAuthURL string) error {
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
		cbq.Set("code", "e2e-code")
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

	t.Setenv("CRUSH_GLOBAL_CONFIG", t.TempDir())

	stopped := false
	h := newMCPOAuthHandler(mcpURL, func() { stopped = true })
	h.openURL = simulateBrowser

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, mcpURL, nil)
	require.NoError(t, err)
	resp := &http.Response{StatusCode: http.StatusUnauthorized, Header: http.Header{}, Body: http.NoBody}

	require.NoError(t, h.Authorize(t.Context(), req, resp))
	require.True(t, stopped, "connection timeout must be suspended during the flow")

	// The freshly obtained token is cached.
	ts, err := h.TokenSource(t.Context())
	require.NoError(t, err)
	require.NotNil(t, ts)
	tok, err := ts.Token()
	require.NoError(t, err)
	require.Equal(t, "e2e-access", tok.AccessToken)

	// And persisted with the registered client ID so refresh works after a
	// restart. A brand-new handler restores it without any browser.
	h2 := newMCPOAuthHandler(mcpURL, nil)
	saved, ok := h2.store.get(mcpURL)
	require.True(t, ok, "token must be persisted")
	require.Equal(t, "e2e-client-id", saved.ClientID, "the registered client ID must be persisted for refresh")
	require.Equal(t, "e2e-access", saved.Token.AccessToken)
	require.NotEmpty(t, saved.Endpoints.TokenURL)
}

// TestAuthorize_StopsConnTimeout proves the interactive flow suspends the MCP
// connection timeout (so login isn't cancelled after 15s), and that a genuine
// permission 403 short-circuits *before* touching the timeout or the token.
func TestAuthorize_StopsConnTimeout(t *testing.T) {
	t.Parallel()

	t.Run("401 stops the connection timeout", func(t *testing.T) {
		t.Parallel()
		// A server that 404s all discovery makes Authorize fail fast, but only
		// after it has already suspended the connection timeout.
		_, mcpURL := newOAuthTestServer(t, oauthServerOpts{})
		stopped := false
		h := &mcpOAuthHandler{serverURL: mcpURL, stopConnTimeout: func() { stopped = true }}
		resp := &http.Response{StatusCode: http.StatusUnauthorized, Header: http.Header{}, Body: http.NoBody}
		err := h.Authorize(t.Context(), &http.Request{}, resp)
		require.Error(t, err) // discovery fails, but that's not what we're asserting
		require.True(t, stopped, "the connection timeout must be suspended once auth begins")
	})

	t.Run("genuine 403 does not stop the timeout and returns nil", func(t *testing.T) {
		t.Parallel()
		stopped := false
		h := &mcpOAuthHandler{serverURL: "https://mcp.example.com/mcp", stopConnTimeout: func() { stopped = true }}
		resp := &http.Response{
			StatusCode: http.StatusForbidden,
			Header:     http.Header{"Www-Authenticate": []string{`Bearer error="access_denied"`}},
			Body:       http.NoBody,
		}
		require.NoError(t, h.Authorize(t.Context(), &http.Request{}, resp))
		require.False(t, stopped, "a genuine 403 must not disturb the connection timeout")
	})

	t.Run("nil stopConnTimeout does not panic", func(t *testing.T) {
		t.Parallel()
		_, mcpURL := newOAuthTestServer(t, oauthServerOpts{})
		h := &mcpOAuthHandler{serverURL: mcpURL, stopConnTimeout: nil}
		resp := &http.Response{StatusCode: http.StatusUnauthorized, Header: http.Header{}, Body: http.NoBody}
		require.NotPanics(t, func() { _ = h.Authorize(t.Context(), &http.Request{}, resp) })
	})
}

func TestAuthStyleForRegistration(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		secret  string
		methods []string
		want    oauth2.AuthStyle
	}{
		{"public client uses params", "", nil, oauth2.AuthStyleInParams},
		{"confidential secret_post", "s", []string{"client_secret_post"}, oauth2.AuthStyleInParams},
		{"confidential secret_basic", "s", []string{"client_secret_basic"}, oauth2.AuthStyleInHeader},
		{"confidential unknown method", "s", []string{"private_key_jwt"}, oauth2.AuthStyleAutoDetect},
		{"confidential no methods advertised", "s", nil, oauth2.AuthStyleAutoDetect},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			reg := &oauthex.ClientRegistrationResponse{ClientSecret: tt.secret}
			asm := &oauthex.AuthServerMeta{TokenEndpointAuthMethodsSupported: tt.methods}
			require.Equal(t, tt.want, authStyleForRegistration(reg, asm))
		})
	}
}

// TestBuildInner covers wiring the SDK handler for both public and confidential
// clients.
func TestBuildInner(t *testing.T) {
	t.Parallel()
	h := &mcpOAuthHandler{serverURL: "https://mcp.example.com/mcp"}

	t.Run("public client", func(t *testing.T) {
		t.Parallel()
		inner, err := h.buildInner(&clientRegistration{clientID: "pub"}, 12345, "http://localhost:12345/callback")
		require.NoError(t, err)
		require.NotNil(t, inner)
	})

	t.Run("confidential client", func(t *testing.T) {
		t.Parallel()
		inner, err := h.buildInner(&clientRegistration{clientID: "conf", clientSecret: "shh"}, 12345, "http://localhost:12345/callback")
		require.NoError(t, err)
		require.NotNil(t, inner)
	})
}

func TestIsInsufficientScope(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		header []string
		want   bool
	}{
		{"insufficient_scope", []string{`Bearer error="insufficient_scope"`}, true},
		{"other bearer error", []string{`Bearer error="access_denied"`}, false},
		{"no header", nil, false},
		{"non-bearer scheme", []string{`Basic realm="x"`}, false},
		{"malformed header", []string{`Bearer error=`}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			resp := &http.Response{Header: http.Header{}}
			if tt.header != nil {
				resp.Header["Www-Authenticate"] = tt.header
			}
			require.Equal(t, tt.want, isInsufficientScope(resp))
		})
	}
}

// hitCallback repeatedly GETs the callback URL until the listener accepts the
// connection, then returns. It gives the capture server time to bind.
func hitCallback(t *testing.T, port int, query string) {
	t.Helper()
	callbackURL := fmt.Sprintf("http://127.0.0.1:%d/callback?%s", port, query)
	deadline := time.Now().Add(2 * time.Second)
	for {
		resp, err := http.Get(callbackURL) //nolint:noctx
		if err == nil {
			resp.Body.Close()
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("callback listener never came up: %v", err)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestOpenBrowserAndCapture(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		openErr  error
		query    string
		wantCode string
		wantErr  string
	}{
		{
			name:     "browser opens and code is captured",
			openErr:  nil,
			query:    "code=good-code&state=st",
			wantCode: "good-code",
		},
		{
			// The P2 regression: even when the browser cannot be launched, the
			// listener stays up so a manually-opened URL still completes.
			name:     "browser fails but manual completion still works",
			openErr:  errors.New("no browser available"),
			query:    "code=manual-code&state=st",
			wantCode: "manual-code",
		},
		{
			name:    "callback reports an authorization error",
			openErr: nil,
			query:   "error=access_denied&error_description=nope",
			wantErr: "nope",
		},
		{
			name:    "callback is missing the code",
			openErr: nil,
			query:   "state=st",
			wantErr: "missing code",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			open := func(string) error { return tt.openErr }

			port, err := allocateCallbackPort(t.Context())
			require.NoError(t, err)

			type outcome struct {
				res *auth.AuthorizationResult
				err error
			}
			done := make(chan outcome, 1)
			go func() {
				res, err := openBrowserAndCapture(t.Context(), "https://auth.example.com/authorize", port, "test-server", open)
				done <- outcome{res, err}
			}()

			hitCallback(t, port, tt.query)

			select {
			case got := <-done:
				if tt.wantErr != "" {
					require.Error(t, got.err)
					require.Contains(t, got.err.Error(), tt.wantErr)
					require.Nil(t, got.res)
					return
				}
				require.NoError(t, got.err)
				require.NotNil(t, got.res)
				require.Equal(t, tt.wantCode, got.res.Code)
			case <-time.After(3 * time.Second):
				t.Fatal("openBrowserAndCapture did not return")
			}
		})
	}
}

func TestOpenBrowserAndCapture_ContextCancelled(t *testing.T) {
	t.Parallel()
	port, err := allocateCallbackPort(context.Background())
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before we start so the select takes the ctx.Done branch

	open := func(string) error { return nil }
	res, err := openBrowserAndCapture(ctx, "https://auth.example.com/authorize", port, "test-server", open)
	require.Error(t, err)
	require.ErrorIs(t, err, context.Canceled)
	require.Nil(t, res)
}

// TestTokenStore_SaveMergesConcurrentDiskWrites proves save() preserves entries
// another writer added to the file after this store last read it, rather than
// clobbering them with a stale in-memory snapshot.
func TestTokenStore_SaveMergesConcurrentDiskWrites(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := dir + "/tokens.json"

	// Writer A persists its entry.
	a := &tokenStore{path: path, data: map[string]mcptoken{}}
	a.save("https://a.example.com", mcptoken{ClientID: "a"})

	// Writer B starts from an empty snapshot (never loaded A) and saves its own
	// entry. The merge-on-write must not drop A.
	b := &tokenStore{path: path, data: map[string]mcptoken{}}
	b.save("https://b.example.com", mcptoken{ClientID: "b"})

	final := &tokenStore{path: path, data: map[string]mcptoken{}}
	final.load()
	ga, oka := final.get("https://a.example.com")
	gb, okb := final.get("https://b.example.com")
	require.True(t, oka, "entry A must survive B's save")
	require.True(t, okb, "entry B must be written")
	require.Equal(t, "a", ga.ClientID)
	require.Equal(t, "b", gb.ClientID)
}
