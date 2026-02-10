package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/crush/internal/oauth"
)

type tokens struct {
	IDToken      string `json:"id_token"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	AccountID    string `json:"account_id"`
}

type tokenData struct {
	AuthMode    string    `json:"auth_mode"`
	Tokens      tokens    `json:"tokens"`
	LastRefresh time.Time `json:"last_refresh"`
}

const defaultRefreshTokenTTL = 3600

func refreshTokenFromDisk() (string, bool) {
	data, err := os.ReadFile(tokenFilePath())
	if err != nil {
		return "", false
	}

	var tokenData tokenData
	if err := json.Unmarshal(data, &tokenData); err != nil {
		return "", false
	}

	if tokenData.AuthMode != "openai" {
		return "", false
	}

	if tokenData.LastRefresh.Before(time.Now().Add(-1 * time.Hour)) {
		return tokenData.Tokens.AccessToken, true
	}

	return "", false
}

// RefreshToken refreshes an OpenAI OAuth token with a refresh token.
func RefreshToken(ctx context.Context, refreshToken string) (*oauth.Token, error) {
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return nil, fmt.Errorf("refresh token is empty")
	}

	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", refreshToken)
	data.Set("client_id", defaultClientID)

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		defaultIssuer+tokenEndpoint,
		strings.NewReader(data.Encode()),
	)
	if err != nil {
		return nil, fmt.Errorf("create refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "crush")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute refresh request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read refresh response: %w", err)
	}

	var payload struct {
		AccessToken      string `json:"access_token"`
		RefreshToken     string `json:"refresh_token"`
		ExpiresIn        int    `json:"expires_in"`
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("decode refresh response: %w: %s", err, string(body))
	}

	if resp.StatusCode != http.StatusOK {
		detail := strings.TrimSpace(payload.ErrorDescription)
		if detail == "" {
			detail = strings.TrimSpace(payload.Error)
		}
		if detail == "" {
			detail = strings.TrimSpace(string(body))
		}
		return nil, fmt.Errorf("token refresh failed: status %d: %s", resp.StatusCode, detail)
	}

	if payload.AccessToken == "" {
		return nil, fmt.Errorf("token refresh response did not contain access_token")
	}

	newRefreshToken := payload.RefreshToken
	if newRefreshToken == "" {
		newRefreshToken = refreshToken
	}

	newToken := &oauth.Token{
		AccessToken:  payload.AccessToken,
		RefreshToken: newRefreshToken,
		ExpiresIn:    payload.ExpiresIn,
	}
	NormalizeToken(newToken)

	return newToken, nil
}

func (t TokenData) ToOAuthToken(providerAPIKey string) *oauth.Token {
	accessToken := strings.TrimSpace(providerAPIKey)
	if accessToken == "" {
		accessToken = strings.TrimSpace(t.AccessToken)
	}

	token := &oauth.Token{
		AccessToken:  accessToken,
		RefreshToken: strings.TrimSpace(t.RefreshToken),
		ExpiresIn:    t.ExpiresIn,
		ExpiresAt:    t.ExpiresAt,
	}
	NormalizeToken(token)

	return token
}

// NormalizeToken fills token expiry fields when missing.
// It prefers explicit fields, then JWT exp claim, then a sane fallback.
func NormalizeToken(token *oauth.Token) bool {
	if token == nil {
		return false
	}

	originalExpiresIn := token.ExpiresIn
	originalExpiresAt := token.ExpiresAt

	switch {
	case token.ExpiresAt > 0 && token.ExpiresIn <= 0:
		token.SetExpiresIn()
	case token.ExpiresIn > 0 && token.ExpiresAt <= 0:
		token.SetExpiresAt()
	}

	if token.ExpiresIn <= 0 || token.ExpiresAt <= 0 {
		if claims, err := parseJWTClaims(strings.TrimSpace(token.AccessToken)); err == nil {
			if exp := extractUnixClaim(claims, "exp"); exp > 0 {
				token.ExpiresAt = exp
				token.SetExpiresIn()
			}
		}
	}

	if token.ExpiresIn <= 0 || token.ExpiresAt <= 0 {
		token.ExpiresIn = defaultRefreshTokenTTL
		token.SetExpiresAt()
	}

	return token.ExpiresIn != originalExpiresIn || token.ExpiresAt != originalExpiresAt
}

func tokenFilePath() string {
	switch runtime.GOOS {
	case "windows":
		return filepath.Join(os.Getenv("LOCALAPPDATA"), "crush/oauth.json")
	default:
		return filepath.Join(os.Getenv("HOME"), ".crush/oauth.json")
	}
}
