package antigravity

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubTokenEndpoint points the package's token endpoint at a local server that
// answers with body, and returns a pointer to the form it was given.
func stubTokenEndpoint(t *testing.T, body string) *url.Values {
	t.Helper()

	form := &url.Values{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("parse form: %v", err)
		}
		*form = r.PostForm
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)

	previous := tokenEndpoint
	tokenEndpoint = server.URL + "/token"
	t.Cleanup(func() { tokenEndpoint = previous })

	return form
}

func TestNormalizeClientSecret(t *testing.T) {
	secret, err := normalizeClientSecret("  a-secret\n")
	require.NoError(t, err)
	assert.Equal(t, "a-secret", secret)

	// The error has to name every way of supplying the value, since this is
	// the only place a user learns the requirement exists.
	for _, empty := range []string{"", "   "} {
		_, err := normalizeClientSecret(empty)
		require.ErrorIs(t, err, ErrClientSecretRequired)
		assert.Contains(t, err.Error(), ClientSecretEnv)
		assert.Contains(t, err.Error(), "oauth_client_secret")
		assert.Contains(t, err.Error(), "crush login gemini")
	}
}

// Both grants need the secret: the token endpoint answers
// "client_secret is missing" without it.
func TestRefreshTokenSendsClientSecret(t *testing.T) {
	form := stubTokenEndpoint(t, `{"access_token":"fresh","expires_in":3600}`)

	token, err := RefreshToken(context.Background(), "original-refresh", "a-secret")
	require.NoError(t, err)

	assert.Equal(t, "a-secret", form.Get("client_secret"))
	assert.Equal(t, "refresh_token", form.Get("grant_type"))
	assert.Equal(t, "original-refresh", form.Get("refresh_token"))
	// Google omits the refresh token when it is not rotated; keep the old one.
	assert.Equal(t, "original-refresh", token.RefreshToken)
	assert.Equal(t, "fresh", token.AccessToken)
}

func TestRefreshTokenWithoutClientSecretNeverRequests(t *testing.T) {
	form := stubTokenEndpoint(t, `{"access_token":"should-not-be-used"}`)

	_, err := RefreshToken(context.Background(), "refresh", "")
	require.Error(t, err)
	assert.Empty(t, *form)
}

func TestExchangeCodeSendsClientSecretAndVerifier(t *testing.T) {
	form := stubTokenEndpoint(t, `{"access_token":"fresh","refresh_token":"rotated","expires_in":3600}`)

	_, err := ExchangeCode(context.Background(), "auth-code", "http://localhost:8085/", PKCE{
		Verifier:  "verifier",
		Challenge: "challenge",
	}, "a-secret")
	require.NoError(t, err)

	assert.Equal(t, "a-secret", form.Get("client_secret"))
	assert.Equal(t, "authorization_code", form.Get("grant_type"))
	assert.Equal(t, "auth-code", form.Get("code"))
	assert.Equal(t, "verifier", form.Get("code_verifier"))
	assert.Equal(t, ClientID, form.Get("client_id"))
}

// The browser flow must refuse to start without the secret, so that a user is
// never asked to grant consent for a code that cannot be exchanged.
func TestStartBrowserFlowRequiresClientSecret(t *testing.T) {
	flow, err := StartBrowserFlow("")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrClientSecretRequired))
	assert.Nil(t, flow)
}
