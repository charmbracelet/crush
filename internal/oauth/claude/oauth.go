package claude

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/charmbracelet/crush/internal/oauth"
)

const clientId = "9d1c250a-e61b-44d9-88ed-5944d1962f5e"

// AuthorizeURL returns the Claude Code Max OAuth2 authorization URL.
func AuthorizeURL(verifier, challenge string) (string, error) {
	u, err := url.Parse("https://claude.ai/oauth/authorize")
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("response_type", "code")
	q.Set("client_id", clientId)
	q.Set("redirect_uri", "https://console.anthropic.com/oauth/code/callback")
	q.Set("scope", "org:create_api_key user:profile user:inference")
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")
	q.Set("state", verifier)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// ExchangeToken exchanges the authorization code for an OAuth2 token.
func ExchangeToken(ctx context.Context, code, verifier string) (*oauth.Token, error) {
	code = strings.TrimSpace(code)
	parts := strings.SplitN(code, "#", 2)
	pure := parts[0]
	state := ""
	if len(parts) > 1 {
		state = parts[1]
	}

	reqBody := map[string]string{
		"code":          pure,
		"state":         state,
		"grant_type":    "authorization_code",
		"client_id":     clientId,
		"redirect_uri":  "https://console.anthropic.com/oauth/code/callback",
		"code_verifier": verifier,
	}

	resp, err := request(ctx, "POST", "https://console.anthropic.com/v1/oauth/token", reqBody)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("claude code max: failed to exchange token: status %d body %q", resp.StatusCode, string(body))
	}

	var token oauth.Token
	if err := json.Unmarshal(body, &token); err != nil {
		return nil, err
	}
	token.SetExpiresAt()
	return &token, nil
}

// RefreshToken refreshes the OAuth2 token using the provided refresh token.
func RefreshToken(ctx context.Context, refreshToken string) (*oauth.Token, error) {
	reqBody := map[string]string{
		"grant_type":    "refresh_token",
		"refresh_token": refreshToken,
		"client_id":     clientId,
	}

	resp, err := request(ctx, "POST", "https://console.anthropic.com/v1/oauth/token", reqBody)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("claude code max: failed to refresh token: status %d body %q", resp.StatusCode, string(body))
	}

	var token oauth.Token
	if err := json.Unmarshal(body, &token); err != nil {
		return nil, err
	}
	token.SetExpiresAt()
	return &token, nil
}

func request(ctx context.Context, method, url string, body any) (*http.Response, error) {
	date, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(date))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "anthropic")

	client := &http.Client{Timeout: 30 * time.Second}
	return client.Do(req)
}
