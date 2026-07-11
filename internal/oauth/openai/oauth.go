// Package openai provides ChatGPT subscription OAuth (Codex device-code flow).
package openai

import (
	"bytes"
	"context"
	"encoding/base64"
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
	// ClientID is the public Codex CLI OAuth client id (shared by many tools).
	ClientID = "app_EMoamEEZ73f0CkXaXp7hrann"

	defaultIssuer        = "https://auth.openai.com"
	defaultUserCodeURL   = defaultIssuer + "/api/accounts/deviceauth/usercode"
	defaultTokenPollURL  = defaultIssuer + "/api/accounts/deviceauth/token"
	defaultOAuthTokenURL = defaultIssuer + "/oauth/token"
	// VerificationURL is the page where the user enters the device code.
	VerificationURL   = defaultIssuer + "/codex/device"
	deviceRedirectURI = defaultIssuer + "/deviceauth/callback"

	userAgent = "crush"
)

// Overridable for tests.
var (
	userCodeURL   = defaultUserCodeURL
	tokenPollURL  = defaultTokenPollURL
	oauthTokenURL = defaultOAuthTokenURL
	httpClient    = &http.Client{Timeout: 30 * time.Second}
)

// DeviceCode holds the user-facing codes from the usercode endpoint.
type DeviceCode struct {
	DeviceAuthID string
	UserCode     string
	Interval     int
	// ExpiresIn is the max wait for authorization (seconds).
	ExpiresIn int
}

type userCodeResp struct {
	DeviceAuthID string `json:"device_auth_id"`
	UserCode     string `json:"user_code"`
	// Alternate field name used by some responses.
	UserCodeAlt string `json:"usercode"`
	Interval    any    `json:"interval"`
}

type tokenPollReq struct {
	DeviceAuthID string `json:"device_auth_id"`
	UserCode     string `json:"user_code"`
}

type codeSuccessResp struct {
	AuthorizationCode string `json:"authorization_code"`
	CodeChallenge     string `json:"code_challenge"`
	CodeVerifier      string `json:"code_verifier"`
}

type tokenResp struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	IDToken      string `json:"id_token"`
	Error        string `json:"error"`
	ErrorDesc    string `json:"error_description"`
}

// RequestDeviceCode starts the Codex device-code login flow.
func RequestDeviceCode(ctx context.Context) (*DeviceCode, error) {
	body, err := json.Marshal(map[string]string{"client_id": ClientID})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, userCodeURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create usercode request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("usercode request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read usercode response: %w", err)
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, errors.New("device code login is not enabled for this account; enable it in ChatGPT Codex security settings or use an API key")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("usercode request failed: status %d body %q", resp.StatusCode, string(raw))
	}

	var uc userCodeResp
	if err := json.Unmarshal(raw, &uc); err != nil {
		return nil, fmt.Errorf("unmarshal usercode response: %w", err)
	}
	userCode := uc.UserCode
	if userCode == "" {
		userCode = uc.UserCodeAlt
	}
	if uc.DeviceAuthID == "" || userCode == "" {
		return nil, errors.New("usercode response missing device_auth_id or user_code")
	}

	interval := parseInterval(uc.Interval)
	if interval <= 0 {
		interval = 5
	}

	return &DeviceCode{
		DeviceAuthID: uc.DeviceAuthID,
		UserCode:     userCode,
		Interval:     interval,
		ExpiresIn:    15 * 60,
	}, nil
}

func parseInterval(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case string:
		var i int
		_, _ = fmt.Sscanf(strings.TrimSpace(n), "%d", &i)
		return i
	default:
		return 0
	}
}

// PollForToken waits for the user to authorize and returns an OAuth token.
func PollForToken(ctx context.Context, dc *DeviceCode) (*oauth.Token, error) {
	if dc == nil {
		return nil, errors.New("device code is nil")
	}

	code, err := pollForAuthCode(ctx, dc)
	if err != nil {
		return nil, err
	}
	tok, err := exchangeCode(ctx, code.AuthorizationCode, code.CodeVerifier, deviceRedirectURI)
	if err != nil {
		return nil, err
	}
	if tok.RefreshToken == "" {
		return nil, errors.New("token response missing refresh_token")
	}
	return tok, nil
}

func pollForAuthCode(ctx context.Context, dc *DeviceCode) (*codeSuccessResp, error) {
	interval := max(dc.Interval, 1)
	deadline := time.Now().Add(time.Duration(dc.ExpiresIn) * time.Second)
	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
		if time.Now().After(deadline) {
			return nil, errors.New("device auth timed out after 15 minutes")
		}

		payload, err := json.Marshal(tokenPollReq{
			DeviceAuthID: dc.DeviceAuthID,
			UserCode:     dc.UserCode,
		})
		if err != nil {
			return nil, err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenPollURL, bytes.NewReader(payload))
		if err != nil {
			return nil, fmt.Errorf("create token poll request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", userAgent)

		resp, err := httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("token poll request: %w", err)
		}
		raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("read token poll response: %w", err)
		}

		if resp.StatusCode == http.StatusOK {
			var success codeSuccessResp
			if err := json.Unmarshal(raw, &success); err != nil {
				return nil, fmt.Errorf("unmarshal auth code response: %w", err)
			}
			if success.AuthorizationCode == "" || success.CodeVerifier == "" {
				return nil, errors.New("auth code response missing authorization_code or code_verifier")
			}
			return &success, nil
		}

		// 403/404 mean still pending in Codex device flow.
		if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusNotFound {
			continue
		}
		return nil, fmt.Errorf("device auth failed: status %d body %q", resp.StatusCode, string(raw))
	}
}

func exchangeCode(ctx context.Context, code, verifier, redirectURI string) (*oauth.Token, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", ClientID)
	form.Set("code", code)
	form.Set("code_verifier", verifier)
	form.Set("redirect_uri", redirectURI)

	return postOAuthToken(ctx, form)
}

// RefreshToken refreshes a ChatGPT OAuth access token.
func RefreshToken(ctx context.Context, refreshToken string) (*oauth.Token, error) {
	if refreshToken == "" {
		return nil, errors.New("refresh token is empty")
	}
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("client_id", ClientID)
	form.Set("refresh_token", refreshToken)

	tok, err := postOAuthToken(ctx, form)
	if err != nil {
		return nil, err
	}
	// Preserve refresh token if the response omits a rotated one.
	if tok.RefreshToken == "" {
		tok.RefreshToken = refreshToken
	}
	return tok, nil
}

func postOAuthToken(ctx context.Context, form url.Values) (*oauth.Token, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, oauthTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read token response: %w", err)
	}

	var tr tokenResp
	if err := json.Unmarshal(raw, &tr); err != nil {
		return nil, fmt.Errorf("unmarshal token response: %w: %s", err, string(raw))
	}
	if resp.StatusCode != http.StatusOK || tr.Error != "" {
		desc := tr.ErrorDesc
		if desc == "" {
			desc = tr.Error
		}
		if desc == "" {
			desc = string(raw)
		}
		return nil, fmt.Errorf("token request failed (status %d): %s", resp.StatusCode, desc)
	}
	if tr.AccessToken == "" {
		return nil, errors.New("token response missing access_token")
	}

	tok := &oauth.Token{
		AccessToken:  tr.AccessToken,
		RefreshToken: tr.RefreshToken,
		ExpiresIn:    tr.ExpiresIn,
	}
	tok.SetExpiresAt()
	return tok, nil
}

// ChatGPTAccountID extracts the ChatGPT account id claim from a JWT access or id token, if present.
func ChatGPTAccountID(jwt string) string {
	parts := strings.Split(jwt, ".")
	if len(parts) < 2 {
		return ""
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		// Some tokens use standard base64 with padding.
		payload, err = base64.URLEncoding.DecodeString(parts[1])
		if err != nil {
			return ""
		}
	}
	var claims struct {
		Auth struct {
			AccountID string `json:"chatgpt_account_id"`
		} `json:"https://api.openai.com/auth"`
		// Some tokens put account id at the top level.
		AccountID string `json:"chatgpt_account_id"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return ""
	}
	if claims.Auth.AccountID != "" {
		return claims.Auth.AccountID
	}
	return claims.AccountID
}
