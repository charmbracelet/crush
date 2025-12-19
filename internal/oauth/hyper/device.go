// Package hyper provides functions to handle Hyper device flow authentication.
package hyper

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/crush/internal/agent/hyper"
	"github.com/charmbracelet/crush/internal/event"
	"github.com/charmbracelet/crush/internal/oauth"
)

// DeviceAuthResponse contains the response from the device authorization endpoint.
type DeviceAuthResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURL string `json:"verification_url"`
	ExpiresIn       int    `json:"expires_in"`
}

// TokenResponse contains the response from the polling endpoint.
type TokenResponse struct {
	RefreshToken     string `json:"refresh_token,omitempty"`
	UserID           string `json:"user_id"`
	OrganizationID   string `json:"organization_id"`
	OrganizationName string `json:"organization_name"`
	Error            string `json:"error,omitempty"`
	ErrorDescription string `json:"error_description,omitempty"`
}

// InitiateDeviceAuth calls the /device/auth endpoint to start the device flow.
func InitiateDeviceAuth(ctx context.Context) (*DeviceAuthResponse, error) {
	url := hyper.BaseURL() + "/device/auth"

	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost, url,
		strings.NewReader(fmt.Sprintf(`{"device_name":%q}`, deviceName())),
	)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "crush")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("device auth failed: status %d, body %q", resp.StatusCode, string(body))
	}

	var authResp DeviceAuthResponse
	if err := json.Unmarshal(body, &authResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &authResp, nil
}

func deviceName() string {
	if hostname, err := os.Hostname(); err == nil && hostname != "" {
		return "Crush (" + hostname + ")"
	}
	return "Crush"
}

// PollForToken polls the /device/token endpoint until authorization is complete.
// It respects the polling interval and handles various error states.
func PollForToken(ctx context.Context, deviceCode string, expiresIn int) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Duration(expiresIn)*time.Second)
	defer cancel()

	d := 5 * time.Second
	ticker := time.NewTicker(d)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-ticker.C:
			result, err := pollOnce(ctx, deviceCode)
			if err != nil {
				return "", err
			}
			if result.RefreshToken != "" {
				event.Alias(result.UserID)
				return result.RefreshToken, nil
			}
			switch result.Error {
			case "authorization_pending":
				continue
			default:
				return "", errors.New(result.ErrorDescription)
			}
		}
	}
}

func pollOnce(ctx context.Context, deviceCode string) (TokenResponse, error) {
	var result TokenResponse
	url := fmt.Sprintf("%s/device/auth/%s", hyper.BaseURL(), deviceCode)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return result, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "crush")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return result, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return result, fmt.Errorf("read response: %w", err)
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return result, fmt.Errorf("unmarshal response: %w: %s", err, string(body))
	}

	if resp.StatusCode != http.StatusOK {
		return result, fmt.Errorf("token request failed: status %d body %q", resp.StatusCode, string(body))
	}

	return result, nil
}

// ExchangeToken exchanges a refresh token for an access token.
func ExchangeToken(ctx context.Context, refreshToken string) (*oauth.Token, error) {
	reqBody := map[string]string{
		"refresh_token": refreshToken,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := hyper.BaseURL() + "/token/exchange"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "crush")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token exchange failed: status %d body %q", resp.StatusCode, string(body))
	}

	var token oauth.Token
	if err := json.Unmarshal(body, &token); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	token.SetExpiresAt()
	return &token, nil
}

// IntrospectTokenResponse contains the response from the token introspection endpoint.
type IntrospectTokenResponse struct {
	Active bool   `json:"active"`
	Sub    string `json:"sub,omitempty"`
	OrgID  string `json:"org_id,omitempty"`
	Exp    int64  `json:"exp,omitempty"`
	Iat    int64  `json:"iat,omitempty"`
	Iss    string `json:"iss,omitempty"`
	Jti    string `json:"jti,omitempty"`
}

// IntrospectToken validates an access token using the introspection endpoint.
// Implements OAuth2 Token Introspection (RFC 7662).
func IntrospectToken(ctx context.Context, accessToken string) (*IntrospectTokenResponse, error) {
	reqBody := map[string]string{
		"token": accessToken,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := hyper.BaseURL() + "/token/introspect"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "crush")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token introspection failed: status %d body %q", resp.StatusCode, string(body))
	}

	var result IntrospectTokenResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &result, nil
}
