package generic

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/stretchr/testify/require"
)

func TestResolvePrefersExplicitEndpoints(t *testing.T) {
	t.Parallel()

	endpoints, err := Resolve(context.Background(), &oauth.AuthSpec{
		AuthorizeURL:  "https://auth.example.com/authorize",
		TokenURL:      "https://auth.example.com/token",
		DeviceAuthURL: "https://auth.example.com/device",
	})
	require.NoError(t, err)
	require.Equal(t, "https://auth.example.com/token", endpoints.TokenURL)
	require.Equal(t, "https://auth.example.com/device", endpoints.DeviceAuthURL)
}

func TestResolveDiscoversFromIssuer(t *testing.T) {
	t.Parallel()

	var path string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"authorization_endpoint": "https://auth.example.com/authorize",
			"token_endpoint": "https://auth.example.com/token",
			"device_authorization_endpoint": "https://auth.example.com/device"
		}`)
	}))
	defer server.Close()

	endpoints, err := Resolve(context.Background(), &oauth.AuthSpec{
		Issuer:   server.URL,
		TokenURL: "https://auth.example.com/token",
	})
	require.NoError(t, err)
	require.Equal(t, "/.well-known/oauth-authorization-server", path)
	require.Equal(t, "https://auth.example.com/authorize", endpoints.AuthorizeURL)
	require.Equal(t, "https://auth.example.com/device", endpoints.DeviceAuthURL)
}

func TestResolveWithoutEndpointsOrIssuerFails(t *testing.T) {
	t.Parallel()

	_, err := Resolve(context.Background(), &oauth.AuthSpec{ClientID: "crush"})
	require.ErrorContains(t, err, "needs an issuer or explicit token and authorization URLs")
}

func TestRefreshToken(t *testing.T) {
	t.Parallel()

	var (
		form url.Values
		ua   string
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		form = r.PostForm
		ua = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/json")
		// No refresh_token in the response: the previous one is kept.
		fmt.Fprint(w, `{"access_token":"fresh","expires_in":3600}`)
	}))
	defer server.Close()

	spec := &oauth.AuthSpec{
		ClientID:     "crush",
		TokenURL:     server.URL + "/token",
		TokenHeaders: map[string]string{"User-Agent": "example-cli/2.1.0"},
	}
	token, err := RefreshToken(context.Background(), spec, "old-refresh")
	require.NoError(t, err)
	require.Equal(t, "fresh", token.AccessToken)
	require.Equal(t, "old-refresh", token.RefreshToken)
	require.Equal(t, "refresh_token", form.Get("grant_type"))
	require.Equal(t, "old-refresh", form.Get("refresh_token"))
	require.Equal(t, "crush", form.Get("client_id"))
	// Declared headers ride on the token call; some servers only answer a
	// request identifying the client they know.
	require.Equal(t, "example-cli/2.1.0", ua)
}

// A provider that returns an ID token labels the signed-in account, which is
// what the UI shows beside the provider.
func TestTokenCarriesAccountFromIDToken(t *testing.T) {
	t.Parallel()

	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"email":"dev@example.com"}`))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"access_token":"fresh","id_token":"e30.%s.sig","expires_in":60}`, payload)
	}))
	defer server.Close()

	token, err := RefreshToken(context.Background(), &oauth.AuthSpec{
		ClientID: "crush",
		TokenURL: server.URL + "/token",
	}, "old-refresh")
	require.NoError(t, err)
	require.Equal(t, "dev@example.com", token.AccountID)
}

// The shipped gemini-sub plugin runs this exact spec shape against Google's
// Antigravity client; pinning the request it builds keeps the plugin and the
// engine in step.
func TestGoogleSubscriptionAuthorizeRequest(t *testing.T) {
	t.Parallel()

	spec := &oauth.AuthSpec{
		ClientID:     "1071006060591-tmhssin2h21lcre235vtolojh4g403ep.apps.googleusercontent.com",
		ClientSecret: "shared-secret",
		Scopes: []string{
			"https://www.googleapis.com/auth/cloud-platform",
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
			"https://www.googleapis.com/auth/cclog",
			"https://www.googleapis.com/auth/experimentsandconfigs",
			"https://www.googleapis.com/auth/aicode",
			"openid",
		},
		AuthorizeURL: "https://accounts.google.com/o/oauth2/auth",
		TokenURL:     "https://oauth2.googleapis.com/token",
		RedirectURI:  "http://localhost:0/oauth2callback",
		ExtraParams:  map[string]string{"access_type": "offline", "prompt": "consent"},
	}

	flow, err := StartBrowser(spec, "Google AI Subscription")
	require.NoError(t, err)
	defer flow.Close()

	_, _, err = flow.Start(context.Background())
	require.NoError(t, err)

	parsed, err := url.Parse(flow.URL())
	require.NoError(t, err)
	require.Equal(t, "accounts.google.com", parsed.Host)
	require.Equal(t, "/o/oauth2/auth", parsed.Path)

	query := parsed.Query()
	require.Equal(t, "code", query.Get("response_type"))
	require.Equal(t, spec.ClientID, query.Get("client_id"))
	require.Equal(t, "S256", query.Get("code_challenge_method"))
	require.NotEmpty(t, query.Get("code_challenge"))
	require.NotEmpty(t, query.Get("state"))
	// Without these two Google hands back an access token with no refresh
	// token, or no token at all on a repeat sign-in.
	require.Equal(t, "offline", query.Get("access_type"))
	require.Equal(t, "consent", query.Get("prompt"))
	require.Contains(t, query.Get("scope"), "auth/aicode")
	require.Contains(t, query.Get("scope"), "openid")

	redirect, err := url.Parse(flow.RedirectURI())
	require.NoError(t, err)
	require.Equal(t, "localhost", redirect.Hostname(), "Google registers the localhost host, not 127.0.0.1")
	require.Equal(t, "/oauth2callback", redirect.Path)
}

func TestRefreshTokenWithBasicAuth(t *testing.T) {
	t.Parallel()

	var headerAuth, bodySecret string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headerAuth = r.Header.Get("Authorization")
		require.NoError(t, r.ParseForm())
		bodySecret = r.PostForm.Get("client_secret")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"access_token":"fresh","refresh_token":"rotated","expires_in":60}`)
	}))
	defer server.Close()

	spec := &oauth.AuthSpec{
		ClientID:          "crush",
		ClientSecret:      "s3cret",
		ClientSecretBasic: true,
		TokenURL:          server.URL + "/token",
	}
	token, err := RefreshToken(context.Background(), spec, "old-refresh")
	require.NoError(t, err)
	require.Equal(t, "rotated", token.RefreshToken)
	require.True(t, strings.HasPrefix(headerAuth, "Basic "))
	require.Empty(t, bodySecret, "the secret must not be duplicated in the body")
}

func TestRefreshTokenReportsRevokedGrant(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"error":"invalid_grant","error_description":"token revoked"}`)
	}))
	defer server.Close()

	spec := &oauth.AuthSpec{ClientID: "crush", TokenURL: server.URL + "/token"}
	_, err := RefreshToken(context.Background(), spec, "dead")
	var exchangeErr *oauth.TokenExchangeError
	require.ErrorAs(t, err, &exchangeErr)
	require.Equal(t, http.StatusBadRequest, exchangeErr.StatusCode)
	require.True(t, exchangeErr.IsRefreshTokenRevoked())
}

func TestBrowserFlow(t *testing.T) {
	t.Parallel()

	var flow *BrowserFlow
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/authorize":
			w.WriteHeader(http.StatusOK)
		case "/token":
			require.NoError(t, r.ParseForm())
			require.Equal(t, "authorization_code", r.PostForm.Get("grant_type"))
			require.Equal(t, "abc", r.PostForm.Get("code"))
			require.Equal(t, flow.RedirectURI(), r.PostForm.Get("redirect_uri"))
			// The exchange presents the raw verifier, which the server
			// hashes to match the challenge from the authorize request.
			require.Equal(t, flow.verifier, r.PostForm.Get("code_verifier"))
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"access_token":"fresh","refresh_token":"rt","expires_in":60}`)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	spec := &oauth.AuthSpec{
		ClientID:     "crush",
		Scopes:       []string{"openid", "offline_access"},
		AuthorizeURL: server.URL + "/authorize",
		TokenURL:     server.URL + "/token",
	}

	flow, err := StartBrowser(spec, "Example")
	require.NoError(t, err)
	defer flow.Close()

	startURL, userCode, err := flow.Start(context.Background())
	require.NoError(t, err)
	require.Empty(t, userCode, "the browser flow shows no code")
	require.Contains(t, startURL, "/auth/start")

	authURL, err := url.Parse(flow.URL())
	require.NoError(t, err)
	require.Equal(t, "code", authURL.Query().Get("response_type"))
	require.Equal(t, "S256", authURL.Query().Get("code_challenge_method"))
	require.Equal(t, challengeS256(flow.verifier), authURL.Query().Get("code_challenge"))
	require.Equal(t, "openid offline_access", authURL.Query().Get("scope"))
	require.Equal(t, flow.RedirectURI(), authURL.Query().Get("redirect_uri"))

	// The browser redirecting back with the code and state finishes the
	// flow, exactly as the consent page would.
	callback := flow.RedirectURI() + "?code=abc&state=" + url.QueryEscape(authURL.Query().Get("state"))
	resp, err := get(t, callback)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	token, err := flow.Wait(context.Background())
	require.NoError(t, err)
	require.Equal(t, "fresh", token.AccessToken)
	require.Equal(t, 60, token.ExpiresIn)
}

// A wrong client secret is a configuration mistake, and the error has to say
// where the credential comes from rather than only what the server thought of
// it.
func TestRefreshTokenReportsRejectedClient(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error":"invalid_client","error_description":"The provided client secret is invalid."}`)
	}))
	defer server.Close()

	spec := &oauth.AuthSpec{ClientID: "crush", ClientSecret: "wrong", TokenURL: server.URL + "/token"}
	_, err := RefreshToken(context.Background(), spec, "refresh")
	require.ErrorContains(t, err, "invalid_client")
	require.ErrorContains(t, err, "auth.client_secret")

	var exchangeErr *oauth.TokenExchangeError
	require.ErrorAs(t, err, &exchangeErr)
	require.Equal(t, http.StatusUnauthorized, exchangeErr.StatusCode)
}

func TestBrowserFlowStateMismatch(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	flow, err := StartBrowser(&oauth.AuthSpec{
		ClientID:     "crush",
		AuthorizeURL: server.URL + "/authorize",
		TokenURL:     server.URL + "/token",
	}, "Example")
	require.NoError(t, err)
	defer flow.Close()

	_, _, err = flow.Start(context.Background())
	require.NoError(t, err)

	resp, err := get(t, flow.RedirectURI()+"?code=abc&state=forged")
	require.NoError(t, err)
	resp.Body.Close()

	_, err = flow.Wait(context.Background())
	require.ErrorContains(t, err, "state mismatch")
}

func TestBrowserFlowCompleteWithCode(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/authorize":
			w.WriteHeader(http.StatusOK)
		case "/token":
			require.NoError(t, r.ParseForm())
			require.Equal(t, "pasted-code", r.PostForm.Get("code"))
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"access_token":"fresh","expires_in":60}`)
		}
	}))
	defer server.Close()

	flow, err := StartBrowser(&oauth.AuthSpec{
		ClientID:     "crush",
		AuthorizeURL: server.URL + "/authorize",
		TokenURL:     server.URL + "/token",
	}, "Example")
	require.NoError(t, err)
	defer flow.Close()

	_, _, err = flow.Start(context.Background())
	require.NoError(t, err)

	// A bare code has no state to check, matching the paste fallback the
	// built-in providers offer.
	token, err := flow.CompleteWithCode(context.Background(), "pasted-code")
	require.NoError(t, err)
	require.Equal(t, "fresh", token.AccessToken)
}

func TestDeviceFlow(t *testing.T) {
	t.Parallel()

	pollInterval = 10 * time.Millisecond

	var polls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/device":
			require.NoError(t, r.ParseForm())
			require.Equal(t, "crush", r.PostForm.Get("client_id"))
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"device_code":"dc","user_code":"ABCD-1234","verification_uri":"https://example.com/activate","expires_in":600,"interval":1}`)
		case "/token":
			polls++
			if polls == 1 {
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprint(w, `{"error":"authorization_pending"}`)
				return
			}
			require.NoError(t, r.ParseForm())
			require.Equal(t, deviceGrant, r.PostForm.Get("grant_type"))
			require.Equal(t, "dc", r.PostForm.Get("device_code"))
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"access_token":"fresh","refresh_token":"rt","expires_in":60}`)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	flow := StartDevice(&oauth.AuthSpec{
		ClientID:      "crush",
		AuthorizeURL:  server.URL + "/authorize",
		TokenURL:      server.URL + "/token",
		DeviceAuthURL: server.URL + "/device",
	})

	verifyURL, userCode, err := flow.Start(context.Background())
	require.NoError(t, err)
	require.Equal(t, "https://example.com/activate", verifyURL)
	require.Equal(t, "ABCD-1234", userCode)

	token, err := flow.Wait(context.Background())
	require.NoError(t, err)
	require.Equal(t, "fresh", token.AccessToken)
	require.Equal(t, 2, polls, "the pending response must be retried")
}

func TestDeviceFlowDenied(t *testing.T) {
	t.Parallel()

	pollInterval = 10 * time.Millisecond

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/device":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"device_code":"dc","user_code":"ABCD","verification_uri":"https://example.com/activate","expires_in":5}`)
		case "/token":
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, `{"error":"access_denied"}`)
		}
	}))
	defer server.Close()

	flow := StartDevice(&oauth.AuthSpec{
		ClientID:      "crush",
		TokenURL:      server.URL + "/token",
		DeviceAuthURL: server.URL + "/device",
	})
	_, _, err := flow.Start(context.Background())
	require.NoError(t, err)

	_, err = flow.Wait(context.Background())
	require.ErrorContains(t, err, "denied")
}

func TestStartFlowSelectsDeclaredMode(t *testing.T) {
	t.Parallel()

	device, err := StartFlow(&oauth.AuthSpec{
		Flow:          oauth.AuthFlowDevice,
		ClientID:      "crush",
		TokenURL:      "https://auth.example.com/token",
		DeviceAuthURL: "https://auth.example.com/device",
	}, "Example")
	require.NoError(t, err)
	_, isDevice := device.(*DeviceFlow)
	require.True(t, isDevice, "an explicit device spec must not start a listener")

	_, err = StartFlow(nil, "Example")
	require.True(t, errors.Is(err, errNoSpec))
}

func TestMetadataURLs(t *testing.T) {
	t.Parallel()

	require.Equal(t, []string{
		"https://auth.example.com/.well-known/oauth-authorization-server",
		"https://auth.example.com/.well-known/openid-configuration",
	}, metadataURLs("https://auth.example.com/"))

	// An issuer with a path gets both the inserted and appended forms.
	urls := metadataURLs("https://example.com/tenant")
	require.Equal(t, []string{
		"https://example.com/.well-known/oauth-authorization-server",
		"https://example.com/tenant/.well-known/oauth-authorization-server",
		"https://example.com/.well-known/openid-configuration",
		"https://example.com/tenant/.well-known/openid-configuration",
	}, urls)
}

// Anthropic's OAuth redirects back without echoing state: the exchange is
// still bound to this flow by the single-use code and the PKCE verifier, so
// its absence must not refuse the sign-in.
func TestBrowserFlowAcceptsAbsentState(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/authorize":
			w.WriteHeader(http.StatusOK)
		case "/token":
			require.NoError(t, r.ParseForm())
			require.Equal(t, "real-code", r.PostForm.Get("code"))
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"access_token":"fresh","refresh_token":"rt","expires_in":60}`)
		}
	}))
	defer server.Close()

	flow, err := StartBrowser(&oauth.AuthSpec{
		ClientID:     "crush",
		AuthorizeURL: server.URL + "/authorize",
		TokenURL:     server.URL + "/token",
	}, "Claude")
	require.NoError(t, err)
	defer flow.Close()

	_, _, err = flow.Start(context.Background())
	require.NoError(t, err)

	resp, err := get(t, flow.RedirectURI()+"?code=real-code")
	require.NoError(t, err)
	resp.Body.Close()

	token, err := flow.Wait(context.Background())
	require.NoError(t, err)
	require.Equal(t, "fresh", token.AccessToken)
}

// A JSON token endpoint gets an object body with the flow's state, which is
// how the servers behind that shape check the exchange belongs to the
// authorization request.
func TestJSONTokenEncoding(t *testing.T) {
	t.Parallel()

	var (
		gotCT    string
		received map[string]any
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCT = r.Header.Get("Content-Type")
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &received)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"access_token":"fresh","refresh_token":"rt","expires_in":3600}`)
	}))
	defer server.Close()

	spec := &oauth.AuthSpec{
		ClientID:      "crush",
		TokenURL:      server.URL + "/token",
		TokenEncoding: "json",
		Scopes:        []string{"user:inference", "user:profile"},
	}
	_, err := ExchangeCode(context.Background(), spec, spec.TokenURL, "the-code",
		"http://localhost:54545/callback", "the-verifier", "the-state")
	require.NoError(t, err)

	require.Equal(t, "application/json", gotCT)
	require.Equal(t, "authorization_code", received["grant_type"])
	require.Equal(t, "the-code", received["code"])
	require.Equal(t, "the-state", received["state"])
	require.Equal(t, "the-verifier", received["code_verifier"])
	require.Equal(t, "crush", received["client_id"])

	// A refresh against the same endpoint re-sends the declared scopes.
	received = nil
	_, err = RefreshToken(context.Background(), spec, "rt")
	require.NoError(t, err)
	require.Equal(t, "refresh_token", received["grant_type"])
	require.Equal(t, "user:inference user:profile", received["scope"])
}

// get fetches url from the flow's own loopback listener.
func get(t *testing.T, rawURL string) (*http.Response, error) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, rawURL, nil)
	require.NoError(t, err)
	return http.DefaultClient.Do(req) //nolint:gosec // test reaches its own loopback listener
}
