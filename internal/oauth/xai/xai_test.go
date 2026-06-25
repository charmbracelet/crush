package xai

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestClientID(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		require.Equal(t, defaultClientID, clientID())
	})
	t.Run("env override", func(t *testing.T) {
		t.Setenv(clientIDEnv, "custom-client")
		require.Equal(t, "custom-client", clientID())
	})
}

func TestRedirectURI(t *testing.T) {
	t.Parallel()
	require.Equal(t, "http://127.0.0.1:56121/callback", redirectURI())
}

func TestTrustedEndpoint(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		endpoint string
		wantErr  bool
	}{
		{"x.ai apex", "https://x.ai/authorize", false},
		{"x.ai subdomain", "https://auth.x.ai/oauth2/auth", false},
		{"http rejected", "http://auth.x.ai/authorize", true},
		{"foreign host rejected", "https://evil.com/authorize", true},
		{"lookalike host rejected", "https://auth.x.ai.evil.com/a", true},
		{"empty rejected", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := trustedEndpoint(tt.endpoint, "test endpoint")
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestGeneratePKCE(t *testing.T) {
	t.Parallel()
	p, err := generatePKCE()
	require.NoError(t, err)
	require.Len(t, p.verifier, 64) // 32 random bytes, hex-encoded.

	sum := sha256.Sum256([]byte(p.verifier))
	require.Equal(t, base64.RawURLEncoding.EncodeToString(sum[:]), p.challenge)

	p2, err := generatePKCE()
	require.NoError(t, err)
	require.NotEqual(t, p.verifier, p2.verifier)
}

func TestBuildAuthorizeURL(t *testing.T) {
	t.Parallel()
	p := pkceChallenge{verifier: "verifier", challenge: "the-challenge"}
	raw, err := buildAuthorizeURL("https://auth.x.ai/oauth2/auth", p, "the-state", "the-nonce")
	require.NoError(t, err)

	u, err := url.Parse(raw)
	require.NoError(t, err)
	require.Equal(t, "auth.x.ai", u.Host)
	require.Equal(t, "/oauth2/auth", u.Path)

	q := u.Query()
	require.Equal(t, "code", q.Get("response_type"))
	require.Equal(t, defaultClientID, q.Get("client_id"))
	require.Equal(t, "http://127.0.0.1:56121/callback", q.Get("redirect_uri"))
	require.Equal(t, oauthScope, q.Get("scope"))
	require.Equal(t, "the-state", q.Get("state"))
	require.Equal(t, "the-nonce", q.Get("nonce"))
	require.Equal(t, "the-challenge", q.Get("code_challenge"))
	require.Equal(t, "S256", q.Get("code_challenge_method"))
	require.Equal(t, "crush", q.Get("referrer"))
}

func TestParseTokenResponse(t *testing.T) {
	t.Parallel()

	t.Run("expires_in", func(t *testing.T) {
		t.Parallel()
		body := []byte(`{"access_token":"at","refresh_token":"rt","expires_in":1800}`)
		tok, err := parseTokenResponse(body, http.StatusOK, "")
		require.NoError(t, err)
		require.Equal(t, "at", tok.AccessToken)
		require.Equal(t, "rt", tok.RefreshToken)
		require.Equal(t, 1800, tok.ExpiresIn)
		require.False(t, tok.IsExpired())
	})

	t.Run("retains prior refresh token", func(t *testing.T) {
		t.Parallel()
		body := []byte(`{"access_token":"at","expires_in":1800}`)
		tok, err := parseTokenResponse(body, http.StatusOK, "prior-rt")
		require.NoError(t, err)
		require.Equal(t, "prior-rt", tok.RefreshToken)
	})

	t.Run("oauth error", func(t *testing.T) {
		t.Parallel()
		body := []byte(`{"error":"invalid_grant","error_description":"bad code"}`)
		_, err := parseTokenResponse(body, http.StatusBadRequest, "")
		require.ErrorContains(t, err, "bad code")
	})

	t.Run("missing access token", func(t *testing.T) {
		t.Parallel()
		body := []byte(`{"refresh_token":"rt"}`)
		_, err := parseTokenResponse(body, http.StatusOK, "")
		require.ErrorContains(t, err, "missing an access token")
	})

	t.Run("jwt expiry fallback", func(t *testing.T) {
		t.Parallel()
		exp := time.Now().Add(time.Hour).Unix()
		body := []byte(`{"access_token":"` + makeJWT(t, exp) + `","refresh_token":"rt"}`)
		tok, err := parseTokenResponse(body, http.StatusOK, "")
		require.NoError(t, err)
		require.Equal(t, exp, tok.ExpiresAt)
	})

	t.Run("non-2xx without error body", func(t *testing.T) {
		t.Parallel()
		_, err := parseTokenResponse([]byte(`{}`), http.StatusServiceUnavailable, "")
		require.ErrorContains(t, err, "503")
	})
}

func TestJWTExpiry(t *testing.T) {
	t.Parallel()
	require.Equal(t, int64(0), jwtExpiry(""))
	require.Equal(t, int64(0), jwtExpiry("not-a-jwt"))
	require.Equal(t, int64(1234567890), jwtExpiry(makeJWT(t, 1234567890)))
}

func TestCallbackHandler(t *testing.T) {
	t.Parallel()

	newServer := func() *callbackServer {
		return &callbackServer{
			expectedState: "expected",
			resultCh:      make(chan callbackResult, 1),
		}
	}

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		cs := newServer()
		req := httptest.NewRequest(http.MethodGet, "/callback?code=the-code&state=expected", nil)
		rec := httptest.NewRecorder()
		cs.handle(rec, req)

		res := <-cs.resultCh
		require.NoError(t, res.err)
		require.Equal(t, "the-code", res.code)
		require.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("state mismatch", func(t *testing.T) {
		t.Parallel()
		cs := newServer()
		req := httptest.NewRequest(http.MethodGet, "/callback?code=the-code&state=wrong", nil)
		rec := httptest.NewRecorder()
		cs.handle(rec, req)

		res := <-cs.resultCh
		require.ErrorContains(t, res.err, "state mismatch")
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("oauth error", func(t *testing.T) {
		t.Parallel()
		cs := newServer()
		req := httptest.NewRequest(http.MethodGet, "/callback?error=access_denied&error_description=nope&state=expected", nil)
		rec := httptest.NewRecorder()
		cs.handle(rec, req)

		res := <-cs.resultCh
		require.ErrorContains(t, res.err, "nope")
	})

	t.Run("missing code", func(t *testing.T) {
		t.Parallel()
		cs := newServer()
		req := httptest.NewRequest(http.MethodGet, "/callback?state=expected", nil)
		rec := httptest.NewRecorder()
		cs.handle(rec, req)

		res := <-cs.resultCh
		require.ErrorContains(t, res.err, "missing")
	})

	t.Run("cors preflight echoed for trusted origin", func(t *testing.T) {
		t.Parallel()
		cs := newServer()
		req := httptest.NewRequest(http.MethodOptions, "/callback", nil)
		req.Header.Set("Origin", "https://auth.x.ai")
		rec := httptest.NewRecorder()
		cs.handle(rec, req)

		require.Equal(t, http.StatusNoContent, rec.Code)
		require.Equal(t, "https://auth.x.ai", rec.Header().Get("Access-Control-Allow-Origin"))
	})

	t.Run("cors not echoed for untrusted origin", func(t *testing.T) {
		t.Parallel()
		cs := newServer()
		req := httptest.NewRequest(http.MethodOptions, "/callback", nil)
		req.Header.Set("Origin", "https://evil.com")
		rec := httptest.NewRecorder()
		cs.handle(rec, req)

		require.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
	})
}

func TestFetchDiscovery(t *testing.T) {
	t.Run("valid document", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"authorization_endpoint":"https://auth.x.ai/authorize","token_endpoint":"https://auth.x.ai/token"}`))
		}))
		defer srv.Close()
		swapDiscoveryURL(t, srv.URL)

		doc, err := fetchDiscovery(context.Background())
		require.NoError(t, err)
		require.Equal(t, "https://auth.x.ai/authorize", doc.AuthorizationEndpoint)
		require.Equal(t, "https://auth.x.ai/token", doc.TokenEndpoint)
	})

	t.Run("rejects untrusted endpoint", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"authorization_endpoint":"https://evil.com/authorize","token_endpoint":"https://auth.x.ai/token"}`))
		}))
		defer srv.Close()
		swapDiscoveryURL(t, srv.URL)

		_, err := fetchDiscovery(context.Background())
		require.ErrorContains(t, err, "untrusted")
	})
}

// swapDiscoveryURL points discoveryURL at a test server for the duration of
// the test.
func swapDiscoveryURL(t *testing.T, u string) {
	t.Helper()
	orig := discoveryURL
	discoveryURL = u
	t.Cleanup(func() { discoveryURL = orig })
}

// makeJWT builds an unsigned JWT carrying the given exp claim.
func makeJWT(t *testing.T, exp int64) string {
	t.Helper()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`))
	payload, err := json.Marshal(map[string]int64{"exp": exp})
	require.NoError(t, err)
	return header + "." + base64.RawURLEncoding.EncodeToString(payload) + ".sig"
}
