package openai

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/stretchr/testify/require"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestTransportOAuthCredentialsAreScopedToOfficialHost(t *testing.T) {
	t.Parallel()

	token := &oauth.Token{AccessToken: "oauth-token", AccountID: "account", Residency: "us"}
	tests := []struct {
		name     string
		endpoint string
		official bool
	}{
		{name: "official", endpoint: "https://API.OPENAI.COM:443/v1/responses", official: true},
		{name: "custom", endpoint: "https://api.openai.com.example/v1/responses"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got *http.Request
			transport := Transport{Token: token, Base: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				got = r
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
			})}
			u, err := url.Parse(tt.endpoint)
			require.NoError(t, err)
			req := &http.Request{Method: http.MethodPost, URL: u, Header: make(http.Header)}
			req.Header.Set("Authorization", "Bearer oauth-token")
			req.Header.Set("ChatGPT-Account-Id", "account")
			req.Header.Set("x-openai-internal-codex-residency", "us")
			_, err = transport.RoundTrip(req)
			require.NoError(t, err)
			if tt.official {
				require.Equal(t, "chatgpt.com", got.URL.Hostname())
				require.Equal(t, "Bearer oauth-token", got.Header.Get("Authorization"))
				require.Equal(t, "account", got.Header.Get("ChatGPT-Account-Id"))
				require.Equal(t, "us", got.Header.Get("x-openai-internal-codex-residency"))
			} else {
				require.Equal(t, "api.openai.com.example", got.URL.Hostname())
				require.Empty(t, got.Header.Get("Authorization"))
				require.Empty(t, got.Header.Get("ChatGPT-Account-Id"))
				require.Empty(t, got.Header.Get("x-openai-internal-codex-residency"))
			}
		})
	}
}

func TestTransportRemovesPreviousOAuthTokenFromCustomHost(t *testing.T) {
	t.Parallel()

	var got *http.Request
	transport := Transport{
		Token: &oauth.Token{AccessToken: "current-oauth-token"},
		Base: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			got = r
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
		}),
	}
	u, err := url.Parse("https://custom.example/v1/responses")
	require.NoError(t, err)
	for _, authorization := range []string{"Bearer old-oauth-token", "Bearer current-oauth-token"} {
		req := &http.Request{Method: http.MethodPost, URL: u, Header: make(http.Header)}
		req.Header.Set("Authorization", authorization)
		_, err = transport.RoundTrip(req)
		require.NoError(t, err)
		require.Empty(t, got.Header.Get("Authorization"), authorization)
	}
}
