package generic

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/stretchr/testify/require"
)

// issuerServer publishes metadata and counts how often it is asked for it.
func issuerServer(t *testing.T, meta string) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/.well-known/") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, meta)
	}))
	t.Cleanup(server.Close)
	return server, &hits
}

func TestResolve(t *testing.T) {
	t.Parallel()

	const (
		authorize = "https://auth.example.com/authorize"
		token     = "https://auth.example.com/token"
		device    = "https://auth.example.com/device"
	)
	full := `{"authorization_endpoint":"` + authorize + `","token_endpoint":"` + token + `","device_authorization_endpoint":"` + device + `"}`
	deviceOnly := `{"token_endpoint":"` + token + `","device_authorization_endpoint":"` + device + `"}`

	tests := []struct {
		name      string
		spec      oauth.AuthSpec
		meta      string
		noIssuer  bool
		grant     Grant
		want      Endpoints
		discovers bool
		err       string
	}{
		{
			name:  "refresh with an explicit token url skips discovery",
			spec:  oauth.AuthSpec{TokenURL: token},
			meta:  full,
			grant: GrantRefresh,
			want:  Endpoints{TokenURL: token},
		},
		{
			name:  "browser with explicit endpoints skips discovery",
			spec:  oauth.AuthSpec{AuthorizeURL: authorize, TokenURL: token},
			meta:  full,
			grant: GrantBrowser,
			want:  Endpoints{AuthorizeURL: authorize, TokenURL: token},
		},
		{
			name:      "browser discovers the authorize url when only the device url is explicit",
			spec:      oauth.AuthSpec{TokenURL: token, DeviceAuthURL: device},
			meta:      full,
			grant:     GrantBrowser,
			want:      Endpoints{AuthorizeURL: authorize, TokenURL: token, DeviceAuthURL: device},
			discovers: true,
		},
		{
			name:      "device discovers the device url when only the authorize url is explicit",
			spec:      oauth.AuthSpec{AuthorizeURL: authorize, TokenURL: token},
			meta:      full,
			grant:     GrantDevice,
			want:      Endpoints{AuthorizeURL: authorize, TokenURL: token, DeviceAuthURL: device},
			discovers: true,
		},
		{
			name:  "auto is covered by either grant endpoint",
			spec:  oauth.AuthSpec{TokenURL: token, DeviceAuthURL: device},
			meta:  full,
			grant: GrantAuto,
			want:  Endpoints{TokenURL: token, DeviceAuthURL: device},
		},
		{
			name:      "everything discovered from the issuer",
			meta:      full,
			grant:     GrantAuto,
			want:      Endpoints{AuthorizeURL: authorize, TokenURL: token, DeviceAuthURL: device},
			discovers: true,
		},
		{
			name:      "an issuer without the endpoint the grant needs",
			meta:      deviceOnly,
			grant:     GrantBrowser,
			discovers: true,
			err:       "publishes nothing for authorize_url",
		},
		{
			name:     "no issuer and no token url",
			noIssuer: true,
			grant:    GrantRefresh,
			err:      "needs an issuer or an explicit token_url",
		},
		{
			name:     "no issuer and no device url",
			spec:     oauth.AuthSpec{TokenURL: token},
			noIssuer: true,
			grant:    GrantDevice,
			err:      "needs an issuer or an explicit device_auth_url",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server, hits := issuerServer(t, tt.meta)
			spec := tt.spec
			if !tt.noIssuer {
				spec.Issuer = server.URL
			}
			got, err := Resolve(context.Background(), &spec, tt.grant)
			require.Equal(t, tt.discovers, hits.Load() > 0, "discovery requests: %d", hits.Load())
			if tt.err != "" {
				require.ErrorContains(t, err, tt.err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

// An issuer that is down must not break a refresh whose token URL is written
// down: the refresh never needed the issuer.
func TestRefreshTokenWithExplicitTokenURLSkipsDiscovery(t *testing.T) {
	t.Parallel()

	var discovered atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/token" {
			discovered.Add(1)
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"access_token":"fresh","expires_in":60}`)
	}))
	defer server.Close()

	token, err := RefreshToken(context.Background(), &oauth.AuthSpec{
		ClientID: "crush",
		Issuer:   server.URL,
		TokenURL: server.URL + "/token",
	}, "old-refresh")
	require.NoError(t, err)
	require.Equal(t, "fresh", token.AccessToken)
	require.Zero(t, discovered.Load())
}

// An issuer that only offers the device grant gets the device flow in auto
// mode, without opening a listener, and is asked for its metadata once.
func TestStartFlowAutoPicksDeviceForDeviceOnlyIssuer(t *testing.T) {
	t.Parallel()

	var discovered atomic.Int32
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/.well-known/oauth-authorization-server":
			discovered.Add(1)
			fmt.Fprintf(w, `{"token_endpoint":%q,"device_authorization_endpoint":%q}`, server.URL+"/token", server.URL+"/device")
		case "/device":
			fmt.Fprint(w, `{"device_code":"dc","user_code":"ABCD","verification_uri":"https://example.com/activate","expires_in":60}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	flow, prompt, err := StartFlow(context.Background(), &oauth.AuthSpec{ClientID: "crush", Issuer: server.URL}, "Example")
	require.NoError(t, err)
	defer flow.Close()
	require.IsType(t, &DeviceFlow{}, flow)
	require.Equal(t, Prompt{
		URL:        "https://example.com/activate",
		UserCode:   "ABCD",
		DeviceCode: "dc",
		ExpiresIn:  60,
	}, prompt)
	require.Equal(t, int32(1), discovered.Load(), "auto mode resolves once")
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

// A refresh names the declared refresh scopes rather than the authorization
// set. Anthropic's token endpoint answers the wider set with invalid_scope,
// which would end every subscription session when its first token expires.
func TestRefreshTokenSendsRefreshScopes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		spec oauth.AuthSpec
		want string
	}{
		{
			name: "json falls back to scopes",
			spec: oauth.AuthSpec{TokenEncoding: "json", Scopes: []string{"a", "b"}},
			want: "a b",
		},
		{
			name: "json prefers refresh scopes",
			spec: oauth.AuthSpec{TokenEncoding: "json", Scopes: []string{"org", "a", "b"}, RefreshScopes: []string{"a", "b"}},
			want: "a b",
		},
		{
			name: "form omits scopes by default",
			spec: oauth.AuthSpec{Scopes: []string{"a", "b"}},
			want: "",
		},
		{
			name: "form sends declared refresh scopes",
			spec: oauth.AuthSpec{Scopes: []string{"a", "b"}, RefreshScopes: []string{"a"}},
			want: "a",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var scope string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Content-Type") == "application/json" {
					var body map[string]string
					require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
					scope = body["scope"]
				} else {
					require.NoError(t, r.ParseForm())
					scope = r.PostForm.Get("scope")
				}
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{"access_token":"fresh","expires_in":3600}`)
			}))
			defer server.Close()

			spec := tt.spec
			spec.ClientID = "crush"
			spec.TokenURL = server.URL + "/token"
			_, err := RefreshToken(context.Background(), &spec, "old-refresh")
			require.NoError(t, err)
			require.Equal(t, tt.want, scope)
		})
	}
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

	flow, err := newBrowserFlow(spec, "Google AI Subscription", Endpoints{})
	require.NoError(t, err)
	defer flow.Close()

	_, err = flow.Start(context.Background())
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

	flow, err := newBrowserFlow(spec, "Example", Endpoints{})
	require.NoError(t, err)
	defer flow.Close()

	prompt, err := flow.Start(context.Background())
	require.NoError(t, err)
	require.Empty(t, prompt.UserCode, "the browser flow shows no code")
	require.Contains(t, prompt.URL, "/auth/start")

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

	flow, err := newBrowserFlow(&oauth.AuthSpec{
		ClientID:     "crush",
		AuthorizeURL: server.URL + "/authorize",
		TokenURL:     server.URL + "/token",
	}, "Example", Endpoints{})
	require.NoError(t, err)
	defer flow.Close()

	_, err = flow.Start(context.Background())
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

	flow, err := newBrowserFlow(&oauth.AuthSpec{
		ClientID:     "crush",
		AuthorizeURL: server.URL + "/authorize",
		TokenURL:     server.URL + "/token",
	}, "Example", Endpoints{})
	require.NoError(t, err)
	defer flow.Close()

	_, err = flow.Start(context.Background())
	require.NoError(t, err)

	// A bare code has no state to check, matching the paste fallback the
	// built-in providers offer.
	token, err := flow.CompleteWithCode(context.Background(), "pasted-code")
	require.NoError(t, err)
	require.Equal(t, "fresh", token.AccessToken)
}

// The device tests run in parallel, so the poll interval is shortened once,
// before any of them read it.
func init() { pollInterval = 10 * time.Millisecond }

func TestDeviceFlow(t *testing.T) {
	t.Parallel()

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

	flow := &DeviceFlow{spec: &oauth.AuthSpec{
		ClientID:      "crush",
		AuthorizeURL:  server.URL + "/authorize",
		TokenURL:      server.URL + "/token",
		DeviceAuthURL: server.URL + "/device",
	}}

	prompt, err := flow.Start(context.Background())
	require.NoError(t, err)
	require.Equal(t, "https://example.com/activate", prompt.URL)
	require.Equal(t, "ABCD-1234", prompt.UserCode)
	require.Equal(t, "dc", prompt.DeviceCode)
	require.Equal(t, 600, prompt.ExpiresIn)
	require.Equal(t, 1, prompt.Interval)

	token, err := flow.Wait(context.Background())
	require.NoError(t, err)
	require.Equal(t, "fresh", token.AccessToken)
	require.Equal(t, 2, polls, "the pending response must be retried")
}

func TestDeviceFlowDenied(t *testing.T) {
	t.Parallel()

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

	flow := &DeviceFlow{spec: &oauth.AuthSpec{
		ClientID:      "crush",
		TokenURL:      server.URL + "/token",
		DeviceAuthURL: server.URL + "/device",
	}}
	_, err := flow.Start(context.Background())
	require.NoError(t, err)

	_, err = flow.Wait(context.Background())
	require.ErrorContains(t, err, "denied")
}

func TestStartFlowSelectsDeclaredMode(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"device_code":"dc","user_code":"ABCD","verification_uri":"https://example.com/activate"}`)
	}))
	defer server.Close()

	device, _, err := StartFlow(context.Background(), &oauth.AuthSpec{
		Flow:          oauth.AuthFlowDevice,
		ClientID:      "crush",
		TokenURL:      server.URL + "/token",
		DeviceAuthURL: server.URL + "/device",
	}, "Example")
	require.NoError(t, err)
	defer device.Close()
	require.IsType(t, &DeviceFlow{}, device, "an explicit device spec must not start a listener")

	_, _, err = StartFlow(context.Background(), nil, "Example")
	require.ErrorIs(t, err, errNoSpec)
}

// startBrowserFlow starts a browser flow against fixed endpoints.
func startBrowserFlow(t *testing.T) *BrowserFlow {
	t.Helper()
	flow, err := newBrowserFlow(&oauth.AuthSpec{
		ClientID:     "crush",
		AuthorizeURL: "https://auth.example.com/authorize",
		TokenURL:     "https://auth.example.com/token",
	}, "Example", Endpoints{})
	require.NoError(t, err)
	t.Cleanup(flow.Close)
	return flow
}

func TestBrowserFlowCloseWakesWait(t *testing.T) {
	t.Parallel()

	for _, started := range []bool{true, false} {
		t.Run(fmt.Sprintf("started=%v", started), func(t *testing.T) {
			t.Parallel()

			flow := startBrowserFlow(t)
			if started {
				_, err := flow.Start(context.Background())
				require.NoError(t, err)
			}
			done := make(chan error, 1)
			go func() {
				_, err := flow.Wait(context.Background())
				done <- err
			}()

			flow.Close()
			flow.Close()
			select {
			case err := <-done:
				require.ErrorIs(t, err, ErrFlowClosed)
			case <-time.After(5 * time.Second):
				t.Fatal("Wait did not return after Close")
			}
		})
	}
}

// The handoff page continues to the authorization URL, so it must not be
// served before Start has built one.
func TestBrowserFlowServesOnlyAfterStart(t *testing.T) {
	t.Parallel()

	flow := startBrowserFlow(t)
	ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, flow.startURL, nil)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req) //nolint:gosec // test reaches its own loopback listener
	if err == nil {
		resp.Body.Close()
	}
	require.Error(t, err, "the handoff page must not answer before Start")

	_, err = flow.Start(context.Background())
	require.NoError(t, err)
	resp, err = get(t, flow.startURL)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), "auth.example.com/authorize")
}

func TestCallbackTargetRequiresLoopback(t *testing.T) {
	t.Parallel()

	tests := []struct {
		redirect string
		host     string
		ok       bool
	}{
		{redirect: "http://127.0.0.1:8979/callback", host: "127.0.0.1", ok: true},
		{redirect: "http://[::1]:8979/callback", host: "::1", ok: true},
		{redirect: "http://localhost/oauth2callback", host: "localhost", ok: true},
		{redirect: "http://example.com/callback"},
		{redirect: "http://0.0.0.0:8979/callback"},
		{redirect: "http://192.168.1.2:8979/callback"},
		{redirect: "https://localhost/callback"},
	}
	for _, tt := range tests {
		t.Run(tt.redirect, func(t *testing.T) {
			t.Parallel()

			host, _, _, err := callbackTarget(&oauth.AuthSpec{RedirectURI: tt.redirect})
			if !tt.ok {
				require.ErrorContains(t, err, "must be an http loopback URL")
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.host, host)
		})
	}
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

	flow, err := newBrowserFlow(&oauth.AuthSpec{
		ClientID:     "crush",
		AuthorizeURL: server.URL + "/authorize",
		TokenURL:     server.URL + "/token",
	}, "Claude", Endpoints{})
	require.NoError(t, err)
	defer flow.Close()

	_, err = flow.Start(context.Background())
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

// busyPort holds a loopback port so a browser flow cannot bind it.
func busyPort(t *testing.T) int {
	t.Helper()
	l, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = l.Close() })
	return l.Addr().(*net.TCPAddr).Port
}

// In auto mode a callback that cannot bind falls back to the device grant
// with the already resolved endpoints, and a device grant that is missing or
// fails reports the bind error alongside its own.
func TestStartFlowAutoFallbackReportsBothErrors(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name       string
		deviceURL  func(serverURL string) string
		wantDevice bool
		wantErrs   []string
	}{
		{
			name:       "device grant available",
			deviceURL:  func(s string) string { return s + "/device" },
			wantDevice: true,
		},
		{
			name:      "device grant unpublished",
			deviceURL: func(string) string { return "" },
			wantErrs:  []string{"listen OAuth callback", "no device_auth_url"},
		},
		{
			name:      "device grant rejected",
			deviceURL: func(s string) string { return s + "/broken" },
			wantErrs:  []string{"listen OAuth callback", "request device code"},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var discovered atomic.Int32
			var server *httptest.Server
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/.well-known/oauth-authorization-server":
					discovered.Add(1)
					fmt.Fprintf(w, `{"authorization_endpoint":%q,"token_endpoint":%q,"device_authorization_endpoint":%q}`,
						server.URL+"/authorize", server.URL+"/token", tt.deviceURL(server.URL))
				case "/device":
					fmt.Fprint(w, `{"device_code":"dc","user_code":"ABCD","verification_uri":"https://example.com/activate"}`)
				default:
					w.WriteHeader(http.StatusInternalServerError)
				}
			}))
			defer server.Close()

			flow, prompt, err := StartFlow(context.Background(), &oauth.AuthSpec{
				ClientID:     "crush",
				Issuer:       server.URL,
				CallbackPort: busyPort(t),
			}, "Example")
			require.Equal(t, int32(1), discovered.Load(), "the fallback reuses the resolved endpoints")
			if tt.wantDevice {
				require.NoError(t, err)
				defer flow.Close()
				require.IsType(t, &DeviceFlow{}, flow)
				require.Equal(t, "ABCD", prompt.UserCode)
				return
			}
			require.Error(t, err)
			for _, want := range tt.wantErrs {
				require.ErrorContains(t, err, want)
			}
		})
	}
}

// A denied consent comes back from Wait as the real error: the browser flow
// that started is never swapped for a device prompt.
func TestStartFlowDeniedConsentIsAnError(t *testing.T) {
	t.Parallel()

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/.well-known/oauth-authorization-server" {
			fmt.Fprintf(w, `{"authorization_endpoint":%q,"token_endpoint":%q,"device_authorization_endpoint":%q}`,
				server.URL+"/authorize", server.URL+"/token", server.URL+"/device")
			return
		}
		t.Errorf("unexpected request to %s", r.URL.Path)
	}))
	defer server.Close()

	flow, prompt, err := StartFlow(context.Background(), &oauth.AuthSpec{ClientID: "crush", Issuer: server.URL}, "Example")
	require.NoError(t, err)
	defer flow.Close()
	browser, ok := flow.(*BrowserFlow)
	require.True(t, ok, "got %T", flow)
	require.Empty(t, prompt.UserCode)

	resp, err := get(t, browser.RedirectURI()+"?error=access_denied&error_description=nope")
	require.NoError(t, err)
	resp.Body.Close()

	_, err = flow.Wait(context.Background())
	require.ErrorContains(t, err, "access_denied")
}
