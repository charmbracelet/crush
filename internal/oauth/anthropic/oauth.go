// Package anthropic provides functions to handle Anthropic OAuth authentication.
package anthropic

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/charmbracelet/crush/internal/oauth"
)

const (
	clientID     = "9d1c250a-e61b-44d9-88ed-5944d1962f5e"
	authorizeURL = "https://claude.ai/oauth/authorize"
	tokenURL     = "https://console.anthropic.com/v1/oauth/token"
	redirectURI  = "https://console.anthropic.com/oauth/code/callback"
	scope        = "org:create_api_key user:profile user:inference"
)

// AuthParams contains the parameters needed to complete the OAuth flow.
type AuthParams struct {
	AuthURL       string
	CodeVerifier  string
	State         string
	CodeChallenge string
}

// TokenResponse represents the response from the token endpoint.
type TokenResponse struct {
	TokenType    string         `json:"token_type"`
	AccessToken  string         `json:"access_token"`
	ExpiresIn    int            `json:"expires_in"`
	RefreshToken string         `json:"refresh_token"`
	Scope        string         `json:"scope"`
	Organization map[string]any `json:"organization"`
	Account      map[string]any `json:"account"`
}

// InitiateAuth initiates the OAuth authorization flow by generating PKCE parameters
// and building the authorization URL.
func InitiateAuth() (*AuthParams, error) {
	verifier, err := generateCodeVerifier()
	if err != nil {
		return nil, fmt.Errorf("generate code verifier: %w", err)
	}

	challenge := generateCodeChallenge(verifier)

	state, err := generateState()
	if err != nil {
		return nil, fmt.Errorf("generate state: %w", err)
	}

	authURL := buildAuthURL(challenge, state)

	return &AuthParams{
		AuthURL:       authURL,
		CodeVerifier:  verifier,
		State:         state,
		CodeChallenge: challenge,
	}, nil
}

// ExchangeCode exchanges the authorization code for access and refresh tokens.
func ExchangeCode(ctx context.Context, code, codeVerifier, state string) (*oauth.Token, error) {
	requestBody := map[string]string{
		"code":          code,
		"state":         state,
		"grant_type":    "authorization_code",
		"client_id":     clientID,
		"redirect_uri":  redirectURI,
		"code_verifier": codeVerifier,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, bytes.NewBuffer(jsonData))
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

	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	token := &oauth.Token{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		ExpiresIn:    tokenResp.ExpiresIn,
	}
	token.SetExpiresAt()

	return token, nil
}

// RefreshToken refreshes an access token using the refresh token.
func RefreshToken(ctx context.Context, refreshToken string) (*oauth.Token, error) {
	requestBody := map[string]string{
		"grant_type":    "refresh_token",
		"client_id":     clientID,
		"refresh_token": refreshToken,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, bytes.NewBuffer(jsonData))
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
		return nil, fmt.Errorf("token refresh failed: status %d body %q", resp.StatusCode, string(body))
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	token := &oauth.Token{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		ExpiresIn:    tokenResp.ExpiresIn,
	}
	token.SetExpiresAt()

	return token, nil
}

// buildAuthURL constructs the authorization URL with all required parameters.
func buildAuthURL(challenge, state string) string {
	params := url.Values{
		"code":                  {"true"},
		"client_id":             {clientID},
		"redirect_uri":          {redirectURI},
		"response_type":         {"code"},
		"scope":                 {scope},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
		"state":                 {state},
	}
	return authorizeURL + "?" + params.Encode()
}

// generateCodeVerifier generates a cryptographically random code verifier for PKCE.
func generateCodeVerifier() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64URLEncode(b), nil
}

// generateCodeChallenge generates the code challenge from the verifier using SHA256.
func generateCodeChallenge(verifier string) string {
	hash := sha256.Sum256([]byte(verifier))
	return base64URLEncode(hash[:])
}

// generateState generates a cryptographically random state parameter for CSRF protection.
func generateState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64URLEncode(b), nil
}

// base64URLEncode encodes data using base64 URL encoding without padding.
func base64URLEncode(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(data)
}
