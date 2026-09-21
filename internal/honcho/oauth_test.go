package honcho

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// oaFake is an in-memory authorization server: an RFC 8414 discovery
// document, an RFC 7591 registration endpoint, and a token endpoint.
// Every OAuth test drives one of these, so nothing ever reaches the
// network.
type oaFake struct {
	t   *testing.T
	srv *httptest.Server

	mu sync.Mutex

	// Discovery behaviour. A non-zero status or a non-empty body
	// replaces the generated document, which is how the 404 and
	// malformed-JSON cases are expressed.
	discoveryStatus  int
	discoveryBody    string
	omitPKCE         bool
	omitRegistration bool

	// Registration behaviour.
	registerStatus int
	registerBody   string

	// Token behaviour.
	tokenStatus int
	tokenBody   string

	// Recorded traffic.
	registerCount   int
	registerPayload map[string]any
	tokenCount      int
	tokenForm       url.Values
}

// oaNewFake starts a fake authorization server and shuts it down when
// the test finishes.
func oaNewFake(t *testing.T) *oaFake {
	t.Helper()
	f := &oaFake{t: t}
	mux := http.NewServeMux()
	mux.HandleFunc(discoveryPath, f.handleDiscovery)
	mux.HandleFunc("/register", f.handleRegister)
	mux.HandleFunc("/token", f.handleToken)
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

// handleDiscovery serves the authorization server metadata.
func (f *oaFake) handleDiscovery(w http.ResponseWriter, _ *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.discoveryStatus != 0 && f.discoveryStatus != http.StatusOK {
		w.WriteHeader(f.discoveryStatus)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if f.discoveryBody != "" {
		io.WriteString(w, f.discoveryBody) //nolint:errcheck
		return
	}

	doc := map[string]any{
		"issuer":                 f.srv.URL,
		"authorization_endpoint": f.srv.URL + "/authorize",
		"token_endpoint":         f.srv.URL + "/token",
		"registration_endpoint":  f.srv.URL + "/register",
		"scopes_supported":       []string{"read", "write"},
		"grant_types_supported":  []string{"authorization_code", "refresh_token"},
	}
	if !f.omitPKCE {
		doc["code_challenge_methods_supported"] = []string{"plain", "S256"}
	}
	if f.omitRegistration {
		delete(doc, "registration_endpoint")
	}
	json.NewEncoder(w).Encode(doc) //nolint:errcheck
}

// handleRegister records the registration payload and answers with a
// client ID.
func (f *oaFake) handleRegister(w http.ResponseWriter, r *http.Request) {
	raw, _ := io.ReadAll(r.Body)

	f.mu.Lock()
	defer f.mu.Unlock()

	f.registerCount++
	f.registerPayload = map[string]any{}
	json.Unmarshal(raw, &f.registerPayload) //nolint:errcheck

	if f.registerStatus != 0 && f.registerStatus != http.StatusOK {
		w.WriteHeader(f.registerStatus)
		io.WriteString(w, f.registerBody) //nolint:errcheck
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if f.registerBody != "" {
		io.WriteString(w, f.registerBody) //nolint:errcheck
		return
	}
	io.WriteString(w, `{"client_id":"client-123"}`) //nolint:errcheck
}

// handleToken records the form it was posted and answers with a token.
func (f *oaFake) handleToken(w http.ResponseWriter, r *http.Request) {
	raw, _ := io.ReadAll(r.Body)
	form, _ := url.ParseQuery(string(raw))

	f.mu.Lock()
	defer f.mu.Unlock()

	f.tokenCount++
	f.tokenForm = form

	if f.tokenStatus != 0 && f.tokenStatus != http.StatusOK {
		w.WriteHeader(f.tokenStatus)
		io.WriteString(w, f.tokenBody) //nolint:errcheck
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if f.tokenBody != "" {
		io.WriteString(w, f.tokenBody) //nolint:errcheck
		return
	}
	io.WriteString(w, `{"access_token":"access-abc","refresh_token":"refresh-abc","token_type":"Bearer","scope":"read write","expires_in":3600}`) //nolint:errcheck
}

// registered returns the recorded registration payload.
func (f *oaFake) registered() map[string]any {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.registerPayload
}

// registrations returns how many registration requests arrived.
func (f *oaFake) registrations() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.registerCount
}

// posted returns the form the token endpoint last received.
func (f *oaFake) posted() url.Values {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.tokenForm
}

// tokens returns how many token requests arrived.
func (f *oaFake) tokens() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.tokenCount
}

// oaStart begins a browser flow and returns it with the query
// parameters of the authorize URL.
func oaStart(t *testing.T, f *oaFake) (*AuthFlow, url.Values) {
	t.Helper()
	flow, err := StartAuth(t.Context(), f.srv.URL)
	require.NoError(t, err)
	t.Cleanup(flow.Close)

	u, err := url.Parse(flow.URL)
	require.NoError(t, err)
	return flow, u.Query()
}

// oaCallback plays the part of the browser returning to the loopback
// redirect with the given query parameters.
func oaCallback(t *testing.T, redirect string, params url.Values) {
	t.Helper()
	resp, err := http.Get(redirect + "?" + params.Encode()) //nolint:noctx
	require.NoError(t, err)
	io.Copy(io.Discard, resp.Body) //nolint:errcheck
	require.NoError(t, resp.Body.Close())
}

// oaRequirePortReleased asserts the loopback redirect no longer
// accepts connections, which is how a leaked listener shows up.
func oaRequirePortReleased(t *testing.T, redirect string) {
	t.Helper()
	u, err := url.Parse(redirect)
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		var d net.Dialer
		d.Timeout = 250 * time.Millisecond
		conn, err := d.DialContext(t.Context(), "tcp", u.Host)
		if err != nil {
			return true
		}
		conn.Close() //nolint:errcheck
		return false
	}, 5*time.Second, 25*time.Millisecond, "loopback listener on %s was never released", u.Host)
}

// oaChallengeFor returns the S256 challenge for a verifier.
func oaChallengeFor(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func TestDiscover(t *testing.T) {
	t.Parallel()

	t.Run("valid document", func(t *testing.T) {
		t.Parallel()
		f := oaNewFake(t)

		meta, err := Discover(t.Context(), f.srv.URL)
		require.NoError(t, err)
		require.Equal(t, f.srv.URL, meta.Issuer)
		require.Equal(t, f.srv.URL+"/authorize", meta.AuthorizationEndpoint)
		require.Equal(t, f.srv.URL+"/token", meta.TokenEndpoint)
		require.Equal(t, f.srv.URL+"/register", meta.RegistrationEndpoint)
		require.Equal(t, []string{"read", "write"}, meta.ScopesSupported)
		require.Equal(t, []string{"authorization_code", "refresh_token"}, meta.GrantTypes)
		require.Equal(t, []string{"plain", "S256"}, meta.CodeChallengeMethods)
	})

	t.Run("trailing slash is trimmed", func(t *testing.T) {
		t.Parallel()
		f := oaNewFake(t)

		meta, err := Discover(t.Context(), "  "+f.srv.URL+"/  ")
		require.NoError(t, err)
		require.Equal(t, f.srv.URL+"/token", meta.TokenEndpoint)
	})

	t.Run("404 points at API keys", func(t *testing.T) {
		t.Parallel()
		f := oaNewFake(t)
		f.discoveryStatus = http.StatusNotFound

		_, err := Discover(t.Context(), f.srv.URL)
		require.Error(t, err)
		require.Contains(t, err.Error(), "API key")
		require.Contains(t, err.Error(), "404")
	})

	t.Run("malformed JSON", func(t *testing.T) {
		t.Parallel()
		f := oaNewFake(t)
		f.discoveryBody = `{"issuer": "oops"`

		_, err := Discover(t.Context(), f.srv.URL)
		require.Error(t, err)
		require.Contains(t, err.Error(), "decode discovery document")
	})

	t.Run("missing authorization endpoint", func(t *testing.T) {
		t.Parallel()
		f := oaNewFake(t)
		f.discoveryBody = `{"issuer":"x","token_endpoint":"x/token"}`

		_, err := Discover(t.Context(), f.srv.URL)
		require.Error(t, err)
		require.Contains(t, err.Error(), "missing an authorization or token endpoint")
	})

	t.Run("missing token endpoint", func(t *testing.T) {
		t.Parallel()
		f := oaNewFake(t)
		f.discoveryBody = `{"issuer":"x","authorization_endpoint":"x/authorize"}`

		_, err := Discover(t.Context(), f.srv.URL)
		require.Error(t, err)
		require.Contains(t, err.Error(), "missing an authorization or token endpoint")
	})
}

// TestDiscoverEmptyBaseURLFallsBack proves an empty base URL resolves
// to Honcho Cloud. The context is cancelled up front so the request
// fails before any packet leaves, and the URL is read out of the
// resulting error.
func TestDiscoverEmptyBaseURLFallsBack(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	for _, base := range []string{"", "   "} {
		_, err := Discover(ctx, base)
		require.Error(t, err)
		require.Contains(t, err.Error(), DefaultBaseURL+discoveryPath)
	}
}

func TestAuthServerSupportsPKCE(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		methods []string
		want    bool
	}{
		{name: "S256 present", methods: []string{"S256"}, want: true},
		{name: "S256 among others", methods: []string{"plain", "S256"}, want: true},
		{name: "plain only", methods: []string{"plain"}, want: false},
		{name: "empty", methods: []string{}, want: false},
		{name: "nil", methods: nil, want: false},
		{name: "case sensitive", methods: []string{"s256"}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			a := &AuthServer{CodeChallengeMethods: tt.methods}
			require.Equal(t, tt.want, a.SupportsPKCE())
		})
	}
}

func TestRegister(t *testing.T) {
	t.Parallel()
	f := oaNewFake(t)

	meta, err := Discover(t.Context(), f.srv.URL)
	require.NoError(t, err)

	clientID, err := Register(t.Context(), meta, "http://127.0.0.1:9999/callback")
	require.NoError(t, err)
	require.Equal(t, "client-123", clientID)
	require.Equal(t, 1, f.registrations())

	payload := f.registered()
	require.Equal(t, []any{"http://127.0.0.1:9999/callback"}, payload["redirect_uris"])
	require.Equal(t, "none", payload["token_endpoint_auth_method"])
	require.Equal(t, []any{"authorization_code", "refresh_token"}, payload["grant_types"])
	require.Equal(t, []any{"code"}, payload["response_types"])
	require.Equal(t, "Crush", payload["client_name"])
	require.Equal(t, "native", payload["application_type"])
	require.Equal(t, oauthScopes, payload["scope"])
}

func TestRegisterErrors(t *testing.T) {
	t.Parallel()

	t.Run("server rejects registration", func(t *testing.T) {
		t.Parallel()
		f := oaNewFake(t)
		f.registerStatus = http.StatusBadRequest
		f.registerBody = `{"error":"invalid_client_metadata","error_description":"unsupported grant_types"}`

		meta, err := Discover(t.Context(), f.srv.URL)
		require.NoError(t, err)

		_, err = Register(t.Context(), meta, "http://127.0.0.1:9999/callback")
		require.Error(t, err)
		require.Contains(t, err.Error(), "client registration failed")
		require.Contains(t, err.Error(), "unsupported grant_types")
	})

	t.Run("success with no client id", func(t *testing.T) {
		t.Parallel()
		f := oaNewFake(t)
		f.registerBody = `{"client_name":"Crush"}`

		meta, err := Discover(t.Context(), f.srv.URL)
		require.NoError(t, err)

		_, err = Register(t.Context(), meta, "http://127.0.0.1:9999/callback")
		require.Error(t, err)
		require.Contains(t, err.Error(), "registration returned no client id")
	})

	t.Run("no registration endpoint makes no request", func(t *testing.T) {
		t.Parallel()
		f := oaNewFake(t)
		f.omitRegistration = true

		meta, err := Discover(t.Context(), f.srv.URL)
		require.NoError(t, err)
		require.Empty(t, meta.RegistrationEndpoint)

		_, err = Register(t.Context(), meta, "http://127.0.0.1:9999/callback")
		require.Error(t, err)
		require.Contains(t, err.Error(), "dynamic client registration")
		require.Zero(t, f.registrations())
	})
}

// TestStartAuthAndWait walks the whole browser flow: discovery,
// registration, the authorize URL, the loopback callback, and the code
// exchange. The PKCE assertion is the point of the test, since the
// verifier is the only thing protecting a client with no secret.
func TestStartAuthAndWait(t *testing.T) {
	t.Parallel()
	f := oaNewFake(t)

	flow, q := oaStart(t, f)

	require.True(t, strings.HasPrefix(flow.URL, f.srv.URL+"/authorize?"))
	require.Equal(t, "code", q.Get("response_type"))
	require.Equal(t, "client-123", q.Get("client_id"))
	require.Equal(t, oauthScopes, q.Get("scope"))
	require.NotEmpty(t, q.Get("state"))
	require.NotEmpty(t, q.Get("code_challenge"))
	require.Equal(t, "S256", q.Get("code_challenge_method"))

	redirect := q.Get("redirect_uri")
	redirectURL, err := url.Parse(redirect)
	require.NoError(t, err)
	require.Equal(t, "http", redirectURL.Scheme)
	require.Equal(t, "127.0.0.1", redirectURL.Hostname())
	require.Equal(t, "/callback", redirectURL.Path)
	require.NotEmpty(t, redirectURL.Port())

	// Play the browser: approve and come back with a code.
	oaCallback(t, redirect, url.Values{
		"code":  {"abc"},
		"state": {q.Get("state")},
	})

	tok, err := flow.Wait(t.Context())
	require.NoError(t, err)
	require.Equal(t, "access-abc", tok.AccessToken)
	require.Equal(t, "refresh-abc", tok.RefreshToken)
	require.Equal(t, "Bearer", tok.TokenType)
	require.Equal(t, "client-123", tok.ClientID)
	require.Equal(t, f.srv.URL, tok.Issuer)
	require.Greater(t, tok.ExpiresAt, time.Now().Unix())
	require.LessOrEqual(t, tok.ExpiresAt, time.Now().Add(time.Hour).Unix())

	// The token endpoint must have seen a verifier that hashes to the
	// challenge Crush published in the authorize URL.
	form := f.posted()
	require.Equal(t, "authorization_code", form.Get("grant_type"))
	require.Equal(t, "abc", form.Get("code"))
	require.Equal(t, redirect, form.Get("redirect_uri"))
	require.Equal(t, "client-123", form.Get("client_id"))
	require.NotEmpty(t, form.Get("code_verifier"))
	require.Equal(t, q.Get("code_challenge"), oaChallengeFor(form.Get("code_verifier")),
		"code_verifier does not hash to the published code_challenge")

	oaRequirePortReleased(t, redirect)
}

func TestWaitCallbackErrors(t *testing.T) {
	t.Parallel()

	t.Run("authorization denied", func(t *testing.T) {
		t.Parallel()
		f := oaNewFake(t)
		flow, q := oaStart(t, f)
		redirect := q.Get("redirect_uri")

		oaCallback(t, redirect, url.Values{
			"error":             {"access_denied"},
			"error_description": {"user said no"},
			"state":             {q.Get("state")},
		})

		_, err := flow.Wait(t.Context())
		require.Error(t, err)
		require.Contains(t, err.Error(), "access_denied")
		require.Zero(t, f.tokens())
		oaRequirePortReleased(t, redirect)
	})

	t.Run("state mismatch", func(t *testing.T) {
		t.Parallel()
		f := oaNewFake(t)
		flow, q := oaStart(t, f)
		redirect := q.Get("redirect_uri")

		// The CSRF guard: a code delivered under someone else's state
		// is not ours to exchange.
		oaCallback(t, redirect, url.Values{
			"code":  {"abc"},
			"state": {q.Get("state") + "-tampered"},
		})

		_, err := flow.Wait(t.Context())
		require.Error(t, err)
		require.Contains(t, err.Error(), "state")
		require.Zero(t, f.tokens())
		oaRequirePortReleased(t, redirect)
	})

	t.Run("missing state", func(t *testing.T) {
		t.Parallel()
		f := oaNewFake(t)
		flow, q := oaStart(t, f)
		redirect := q.Get("redirect_uri")

		oaCallback(t, redirect, url.Values{"code": {"abc"}})

		_, err := flow.Wait(t.Context())
		require.Error(t, err)
		require.Contains(t, err.Error(), "state")
		require.Zero(t, f.tokens())
	})

	t.Run("no code", func(t *testing.T) {
		t.Parallel()
		f := oaNewFake(t)
		flow, q := oaStart(t, f)
		redirect := q.Get("redirect_uri")

		oaCallback(t, redirect, url.Values{"state": {q.Get("state")}})

		_, err := flow.Wait(t.Context())
		require.Error(t, err)
		require.Contains(t, err.Error(), "no code")
		require.Zero(t, f.tokens())
		oaRequirePortReleased(t, redirect)
	})

	t.Run("token endpoint rejects the code", func(t *testing.T) {
		t.Parallel()
		f := oaNewFake(t)
		f.tokenStatus = http.StatusBadRequest
		f.tokenBody = `{"error":"invalid_grant","error_description":"code expired"}`

		flow, q := oaStart(t, f)
		redirect := q.Get("redirect_uri")

		oaCallback(t, redirect, url.Values{
			"code":  {"abc"},
			"state": {q.Get("state")},
		})

		_, err := flow.Wait(t.Context())
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid_grant")
		require.Contains(t, err.Error(), "code expired")
		oaRequirePortReleased(t, redirect)
	})
}

// TestWaitTimesOutOnCancelledContext checks that a cancelled parent
// context ends the wait and still frees the port.
func TestWaitTimesOutOnCancelledContext(t *testing.T) {
	t.Parallel()
	f := oaNewFake(t)
	flow, q := oaStart(t, f)
	redirect := q.Get("redirect_uri")

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err := flow.Wait(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "timed out waiting for browser sign-in")
	oaRequirePortReleased(t, redirect)
}

// TestStartAuthRequiresPKCE checks Crush refuses a deployment that
// cannot bind the code exchange, and bails before registering or
// binding a port.
func TestStartAuthRequiresPKCE(t *testing.T) {
	t.Parallel()
	f := oaNewFake(t)
	f.omitPKCE = true

	flow, err := StartAuth(t.Context(), f.srv.URL)
	require.Error(t, err)
	require.Nil(t, flow)
	require.Contains(t, err.Error(), "PKCE")
	// No registration means no listener was ever bound, since
	// StartAuth binds only on its way to registering.
	require.Zero(t, f.registrations())
}

// TestStartAuthReleasesListenerOnRegistrationFailure proves the
// loopback port bound before registration is handed back when
// registration fails. The port is read out of the redirect URI the
// server saw.
func TestStartAuthReleasesListenerOnRegistrationFailure(t *testing.T) {
	t.Parallel()
	f := oaNewFake(t)
	f.registerStatus = http.StatusForbidden
	f.registerBody = `{"error":"access_denied"}`

	_, err := StartAuth(t.Context(), f.srv.URL)
	require.Error(t, err)

	uris, ok := f.registered()["redirect_uris"].([]any)
	require.True(t, ok)
	require.Len(t, uris, 1)
	oaRequirePortReleased(t, uris[0].(string))
}

func TestRefreshToken(t *testing.T) {
	t.Parallel()

	t.Run("rotating server", func(t *testing.T) {
		t.Parallel()
		f := oaNewFake(t)
		f.tokenBody = `{"access_token":"access-2","refresh_token":"refresh-2","expires_in":120}`

		fresh, err := RefreshToken(t.Context(), f.srv.URL, &OAuthToken{
			AccessToken:  "access-1",
			RefreshToken: "refresh-1",
			ClientID:     "client-123",
		})
		require.NoError(t, err)
		require.Equal(t, "access-2", fresh.AccessToken)
		require.Equal(t, "refresh-2", fresh.RefreshToken)
		require.Equal(t, "client-123", fresh.ClientID)
		require.Equal(t, f.srv.URL, fresh.Issuer)
		require.Greater(t, fresh.ExpiresAt, time.Now().Unix())

		form := f.posted()
		require.Equal(t, "refresh_token", form.Get("grant_type"))
		require.Equal(t, "refresh-1", form.Get("refresh_token"))
		require.Equal(t, "client-123", form.Get("client_id"))
	})

	t.Run("non-rotating server keeps the old refresh token", func(t *testing.T) {
		t.Parallel()
		f := oaNewFake(t)
		f.tokenBody = `{"access_token":"access-2","expires_in":120}`

		fresh, err := RefreshToken(t.Context(), f.srv.URL, &OAuthToken{
			AccessToken:  "access-1",
			RefreshToken: "refresh-1",
			ClientID:     "client-123",
		})
		require.NoError(t, err)
		require.Equal(t, "access-2", fresh.AccessToken)
		require.Equal(t, "refresh-1", fresh.RefreshToken,
			"a server that does not rotate refresh tokens must not strip the existing one")
	})

	t.Run("server rejects the grant", func(t *testing.T) {
		t.Parallel()
		f := oaNewFake(t)
		f.tokenStatus = http.StatusBadRequest
		f.tokenBody = `{"error":"invalid_grant","error_description":"revoked"}`

		_, err := RefreshToken(t.Context(), f.srv.URL, &OAuthToken{
			RefreshToken: "refresh-1",
			ClientID:     "client-123",
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid_grant")
	})

	t.Run("no token makes no request", func(t *testing.T) {
		t.Parallel()
		f := oaNewFake(t)

		_, err := RefreshToken(t.Context(), f.srv.URL, nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "no refresh token available")

		_, err = RefreshToken(t.Context(), f.srv.URL, &OAuthToken{AccessToken: "access-1"})
		require.Error(t, err)
		require.Contains(t, err.Error(), "no refresh token available")

		require.Zero(t, f.tokens())
	})
}

func TestOAuthTokenExpired(t *testing.T) {
	t.Parallel()

	now := time.Now()
	tests := []struct {
		name string
		tok  *OAuthToken
		want bool
	}{
		{name: "nil token", tok: nil, want: true},
		{name: "empty access token", tok: &OAuthToken{}, want: true},
		{
			name: "no expiry never expires",
			tok:  &OAuthToken{AccessToken: "a", ExpiresAt: 0},
			want: false,
		},
		{
			name: "expired an hour ago",
			tok:  &OAuthToken{AccessToken: "a", ExpiresAt: now.Add(-time.Hour).Unix()},
			want: true,
		},
		{
			name: "inside the sixty second slack",
			tok:  &OAuthToken{AccessToken: "a", ExpiresAt: now.Add(30 * time.Second).Unix()},
			want: true,
		},
		{
			name: "five minutes of life left",
			tok:  &OAuthToken{AccessToken: "a", ExpiresAt: now.Add(5 * time.Minute).Unix()},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, tt.tok.Expired())
		})
	}
}
