// Package xai provides OAuth2 device-code authentication for xAI SuperGrok.
package xai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/charmbracelet/crush/internal/oauth"
)

const (
	clientID = "b1a00492-073a-47ea-816f-4c329264a828"
	scope    = "openid profile email offline_access grok-cli:access api:access"

	defaultIssuer       = "https://auth.x.ai"
	defaultDiscoveryURL = defaultIssuer + "/.well-known/openid-configuration"
	deviceGrantType     = "urn:ietf:params:oauth:grant-type:device_code"

	userAgent = "crush"
)

// Overridable for tests.
var (
	discoveryURL = defaultDiscoveryURL
	httpClient   = &http.Client{Timeout: 30 * time.Second}
)

// DeviceCode is the response from the device authorization endpoint.
type DeviceCode struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete,omitempty"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`

	// TokenEndpoint is resolved via OIDC discovery and used while polling.
	TokenEndpoint string `json:"-"`
}

type oidcDiscovery struct {
	DeviceAuthorizationEndpoint string `json:"device_authorization_endpoint"`
	TokenEndpoint               string `json:"token_endpoint"`
}

type tokenJSON struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	Error        string `json:"error"`
	ErrorDesc    string `json:"error_description"`
}

// discoverEndpoints fetches and validates OIDC discovery document endpoints.
func discoverEndpoints(ctx context.Context) (deviceAuthURL, tokenURL string, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, discoveryURL, nil)
	if err != nil {
		return "", "", fmt.Errorf("create discovery request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("discovery request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", "", fmt.Errorf("read discovery response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("discovery failed: status %d body %q", resp.StatusCode, string(body))
	}

	var doc oidcDiscovery
	if err := json.Unmarshal(body, &doc); err != nil {
		return "", "", fmt.Errorf("unmarshal discovery: %w", err)
	}
	if doc.DeviceAuthorizationEndpoint == "" || doc.TokenEndpoint == "" {
		return "", "", errors.New("discovery missing device_authorization_endpoint or token_endpoint")
	}
	if err := requireTrustedXAIEndpoint(doc.DeviceAuthorizationEndpoint); err != nil {
		return "", "", err
	}
	if err := requireTrustedXAIEndpoint(doc.TokenEndpoint); err != nil {
		return "", "", err
	}
	return doc.DeviceAuthorizationEndpoint, doc.TokenEndpoint, nil
}

func requireTrustedXAIEndpoint(endpoint string) error {
	u, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("invalid endpoint %q: %w", endpoint, err)
	}
	if u.Scheme != "https" {
		return fmt.Errorf("untrusted endpoint (need https): %s", endpoint)
	}
	host := strings.ToLower(u.Hostname())
	if host != "x.ai" && !strings.HasSuffix(host, ".x.ai") {
		return fmt.Errorf("untrusted endpoint host: %s", host)
	}
	return nil
}

// RequestDeviceCode starts the xAI device authorization flow.
func RequestDeviceCode(ctx context.Context) (*DeviceCode, error) {
	deviceAuthURL, tokenURL, err := discoverEndpoints(ctx)
	if err != nil {
		return nil, err
	}

	form := url.Values{}
	form.Set("client_id", clientID)
	form.Set("scope", scope)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, deviceAuthURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create device auth request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("device auth request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read device auth response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("device auth failed: status %d body %q", resp.StatusCode, string(body))
	}

	var dc DeviceCode
	if err := json.Unmarshal(body, &dc); err != nil {
		return nil, fmt.Errorf("unmarshal device auth response: %w", err)
	}
	if dc.DeviceCode == "" || dc.UserCode == "" || dc.VerificationURI == "" {
		return nil, errors.New("device auth response missing device_code, user_code, or verification_uri")
	}
	if err := requireTrustedXAIEndpoint(dc.VerificationURI); err != nil {
		return nil, err
	}
	if dc.ExpiresIn <= 0 {
		dc.ExpiresIn = 900
	}
	if dc.Interval <= 0 {
		dc.Interval = 5
	}
	dc.TokenEndpoint = tokenURL
	return &dc, nil
}

// PollForToken polls the token endpoint until the user authorizes or the code expires.
func PollForToken(ctx context.Context, dc *DeviceCode) (*oauth.Token, error) {
	if dc == nil {
		return nil, errors.New("device code is nil")
	}
	if dc.TokenEndpoint == "" {
		return nil, errors.New("token endpoint not set on device code")
	}

	interval := max(dc.Interval, 1)
	deadline := time.Now().Add(time.Duration(dc.ExpiresIn) * time.Second)
	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	// First poll after one interval (standard device flow).
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}

		if time.Now().After(deadline) {
			return nil, errors.New("authorization timed out")
		}

		token, err := tryDeviceToken(ctx, dc.TokenEndpoint, dc.DeviceCode)
		if err == errPending {
			continue
		}
		if err == errSlowDown {
			interval += 5
			ticker.Reset(time.Duration(interval) * time.Second)
			continue
		}
		if err != nil {
			return nil, err
		}
		return token, nil
	}
}

var (
	errPending  = errors.New("authorization_pending")
	errSlowDown = errors.New("slow_down")
)

func tryDeviceToken(ctx context.Context, tokenURL, deviceCode string) (*oauth.Token, error) {
	form := url.Values{}
	form.Set("grant_type", deviceGrantType)
	form.Set("device_code", deviceCode)
	form.Set("client_id", clientID)

	tok, err := postTokenForm(ctx, tokenURL, form)
	if err != nil {
		return nil, err
	}
	switch tok.Error {
	case "":
		if tok.AccessToken == "" {
			return nil, errPending
		}
		return toOAuthToken(tok), nil
	case "authorization_pending":
		return nil, errPending
	case "slow_down":
		return nil, errSlowDown
	case "expired_token", "access_denied":
		desc := tok.ErrorDesc
		if desc == "" {
			desc = tok.Error
		}
		return nil, errors.New(desc)
	default:
		desc := tok.ErrorDesc
		if desc == "" {
			desc = tok.Error
		}
		return nil, fmt.Errorf("authorization failed: %s", desc)
	}
}

// RefreshToken exchanges a refresh token for a new access token.
func RefreshToken(ctx context.Context, refreshToken string) (*oauth.Token, error) {
	if refreshToken == "" {
		return nil, errors.New("refresh token is empty")
	}
	_, tokenURL, err := discoverEndpoints(ctx)
	if err != nil {
		// Fall back to the well-known token path if discovery is unavailable.
		tokenURL = defaultIssuer + "/oauth/token"
		if err2 := requireTrustedXAIEndpoint(tokenURL); err2 != nil {
			return nil, err
		}
	}

	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", refreshToken)
	form.Set("client_id", clientID)

	tok, err := postTokenForm(ctx, tokenURL, form)
	if err != nil {
		return nil, err
	}
	if tok.Error != "" {
		desc := tok.ErrorDesc
		if desc == "" {
			desc = tok.Error
		}
		return nil, fmt.Errorf("token refresh failed: %s", desc)
	}
	if tok.AccessToken == "" {
		return nil, errors.New("token refresh response missing access_token")
	}
	// Preserve refresh token if the provider omits a rotated one.
	if tok.RefreshToken == "" {
		tok.RefreshToken = refreshToken
	}
	return toOAuthToken(tok), nil
}

func postTokenForm(ctx context.Context, tokenURL string, form url.Values) (tokenJSON, error) {
	var result tokenJSON
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return result, fmt.Errorf("create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)

	resp, err := httpClient.Do(req)
	if err != nil {
		return result, fmt.Errorf("token request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return result, fmt.Errorf("read token response: %w", err)
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return result, fmt.Errorf("unmarshal token response: %w: %s", err, string(body))
	}
	// HTTP errors without structured OAuth error fields.
	if resp.StatusCode != http.StatusOK && result.Error == "" {
		return result, fmt.Errorf("token request failed: status %d body %q", resp.StatusCode, string(body))
	}
	return result, nil
}

func toOAuthToken(t tokenJSON) *oauth.Token {
	tok := &oauth.Token{
		AccessToken:  t.AccessToken,
		RefreshToken: t.RefreshToken,
		ExpiresIn:    t.ExpiresIn,
	}
	tok.SetExpiresAt()
	return tok
}
