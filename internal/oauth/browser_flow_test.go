package oauth_test

import (
	"context"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/stretchr/testify/require"
)

func TestBrowserFlow_Success(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var capturedState string
	session, err := oauth.StartBrowserFlow(ctx, oauth.BrowserFlowConfig{
		Port:    0,
		Subject: "Test Service",
		AuthURL: func(redirectURI string, pkce oauth.PKCE, state string) string {
			capturedState = state
			return redirectURI + "?state=" + state
		},
		Exchange: func(ctx context.Context, code, redirectURI string, pkce oauth.PKCE) (*oauth.Token, error) {
			require.Equal(t, "test-auth-code", code)
			require.NotEmpty(t, pkce.Verifier)
			return &oauth.Token{
				AccessToken:  "access-123",
				RefreshToken: "refresh-456",
			}, nil
		},
	})
	require.NoError(t, err)
	defer session.Close()

	u, err := url.Parse(session.URL())
	require.NoError(t, err)

	callbackURL := "http://" + u.Host + u.Path + "?code=test-auth-code&state=" + capturedState
	resp, err := http.Get(callbackURL)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	tok, err := session.Wait(ctx)
	require.NoError(t, err)
	require.NotNil(t, tok)
	require.Equal(t, "access-123", tok.AccessToken)
	require.Equal(t, "refresh-456", tok.RefreshToken)
}

func TestBrowserFlow_StateMismatch(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var capturedState string
	session, err := oauth.StartBrowserFlow(ctx, oauth.BrowserFlowConfig{
		Port:    0,
		Subject: "Test Service",
		AuthURL: func(redirectURI string, pkce oauth.PKCE, state string) string {
			capturedState = state
			return redirectURI + "?state=" + state
		},
		Exchange: func(ctx context.Context, code, redirectURI string, pkce oauth.PKCE) (*oauth.Token, error) {
			return &oauth.Token{AccessToken: "access-123"}, nil
		},
	})
	require.NoError(t, err)
	defer session.Close()

	u, err := url.Parse(session.URL())
	require.NoError(t, err)

	callbackURL := "http://" + u.Host + u.Path + "?code=test-auth-code&state=wrong-state"
	resp, err := http.Get(callbackURL)
	require.NoError(t, err)
	resp.Body.Close()

	callbackURL = "http://" + u.Host + u.Path + "?code=test-auth-code&state=" + capturedState
	resp, err = http.Get(callbackURL)
	require.NoError(t, err)
	resp.Body.Close()

	tok, err := session.Wait(ctx)
	require.NoError(t, err)
	require.Equal(t, "access-123", tok.AccessToken)
}
