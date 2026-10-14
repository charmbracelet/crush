package grok

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/stretchr/testify/require"
)

// stubTokenServer serves an OAuth token endpoint that always returns the
// given token response and records the last request it received.
func stubTokenServer(t *testing.T, resp any) (*httptest.Server, *url.Values) {
	t.Helper()
	var got url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		got = r.PostForm
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(server.Close)

	orig := tokenEndpoint
	tokenEndpoint = server.URL
	t.Cleanup(func() { tokenEndpoint = orig })
	return server, &got
}

func TestAuthorizeURL(t *testing.T) {
	t.Parallel()

	pkce := PKCE{Verifier: "verifier", Challenge: "challenge"}
	got := AuthorizeURL("http://127.0.0.1:45131/callback", pkce, "state123", "nonce456")

	require.Equal(t, Issuer+"/oauth2/authorize", got[:len(Issuer)+len("/oauth2/authorize")])

	query, err := url.ParseQuery(got[strings.Index(got, "?")+1:])
	require.NoError(t, err)
	require.Equal(t, "code", query.Get("response_type"))
	require.Equal(t, ClientID, query.Get("client_id"))
	require.Equal(t, "http://127.0.0.1:45131/callback", query.Get("redirect_uri"))
	require.Equal(t, Scope, query.Get("scope"))
	require.Equal(t, "challenge", query.Get("code_challenge"))
	require.Equal(t, "S256", query.Get("code_challenge_method"))
	require.Equal(t, "state123", query.Get("state"))
	require.Equal(t, "nonce456", query.Get("nonce"))
	require.Equal(t, "crush", query.Get("referrer"))
}

func TestNewPKCE(t *testing.T) {
	t.Parallel()

	pkce, err := NewPKCE()
	require.NoError(t, err)
	require.NotEmpty(t, pkce.Verifier)
	require.NotEmpty(t, pkce.Challenge)
	require.NotEqual(t, pkce.Verifier, pkce.Challenge)

	// Two generations must not collide.
	again, err := NewPKCE()
	require.NoError(t, err)
	require.NotEqual(t, pkce.Verifier, again.Verifier)
}

func TestExchangeCode(t *testing.T) {
	_, got := stubTokenServer(t, tokenResponse{
		AccessToken:  "at-abc",
		RefreshToken: "rt-abc",
		IDToken:      "id-abc",
		ExpiresIn:    3600,
	})

	pkce := PKCE{Verifier: "v", Challenge: "c"}
	token, err := ExchangeCode(context.Background(), "the-code", "http://127.0.0.1:45131/callback", pkce)
	require.NoError(t, err)

	require.Equal(t, "authorization_code", got.Get("grant_type"))
	require.Equal(t, "the-code", got.Get("code"))
	require.Equal(t, "http://127.0.0.1:45131/callback", got.Get("redirect_uri"))
	require.Equal(t, ClientID, got.Get("client_id"))
	require.Equal(t, "v", got.Get("code_verifier"))

	require.Equal(t, "at-abc", token.AccessToken)
	require.Equal(t, "rt-abc", token.RefreshToken)
	require.Equal(t, "id-abc", token.IDToken)
	require.WithinDuration(t, time.Now().Add(time.Hour), time.Unix(token.ExpiresAt, 0), time.Minute)
}

func TestRefreshToken(t *testing.T) {
	_, got := stubTokenServer(t, tokenResponse{
		AccessToken: "at-new",
		ExpiresIn:   600,
	})

	token, err := RefreshToken(context.Background(), "rt-old")
	require.NoError(t, err)

	require.Equal(t, "refresh_token", got.Get("grant_type"))
	require.Equal(t, "rt-old", got.Get("refresh_token"))
	require.Equal(t, ClientID, got.Get("client_id"))

	require.Equal(t, "at-new", token.AccessToken)
	// The server did not rotate the refresh token, so keep the old one.
	require.Equal(t, "rt-old", token.RefreshToken)
}

func TestRefreshToken_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
	}))
	t.Cleanup(server.Close)

	orig := tokenEndpoint
	tokenEndpoint = server.URL
	t.Cleanup(func() { tokenEndpoint = orig })

	token, err := RefreshToken(context.Background(), "rt-revoked")
	require.Nil(t, token)

	var exchangeErr *oauth.TokenExchangeError
	require.ErrorAs(t, err, &exchangeErr)
	require.True(t, exchangeErr.IsRefreshTokenRevoked())
}
