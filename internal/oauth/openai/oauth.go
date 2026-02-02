package openai

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/charmbracelet/crush/internal/oauth"
)

const (
	oauthClientID   = "app_EMoamEEZ73f0CkXaXp7hrann"
	authorizeURL    = "https://auth.openai.com/oauth/authorize"
	defaultTokenURL = "https://auth.openai.com/oauth/token"
)

var tokenURL = defaultTokenURL

const (
	Scope       = "openid profile email offline_access"
	RedirectURL = "http://localhost:1455/auth/callback"
)

const (
	HeaderAccountID     = "chatgpt-account-id"
	HeaderOpenAIBeta    = "OpenAI-Beta"
	HeaderOriginator    = "originator"
	HeaderOpenAIBetaVal = "responses=experimental"
	HeaderOriginatorVal = "codex_cli_rs"
)

const jwtClaimPath = "https://api.openai.com/auth"

// AuthFlow holds the state for the OAuth2 authorization flow.
type AuthFlow struct {
	URL      string
	State    string
	Verifier string
}

// CreateAuthorizationFlow creates a new OAuth2 authorization flow.
func CreateAuthorizationFlow() (AuthFlow, error) {
	verifier, err := newCodeVerifier()
	if err != nil {
		return AuthFlow{}, err
	}

	state, err := newState()
	if err != nil {
		return AuthFlow{}, err
	}

	challenge := codeChallenge(verifier)
	u, err := url.Parse(authorizeURL)
	if err != nil {
		return AuthFlow{}, fmt.Errorf("failed to parse authorize URL: %w", err)
	}

	q := u.Query()
	q.Set("response_type", "code")
	q.Set("client_id", oauthClientID)
	q.Set("redirect_uri", RedirectURL)
	q.Set("scope", Scope)
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")
	q.Set("state", state)
	q.Set("id_token_add_organizations", "true")
	q.Set("codex_cli_simplified_flow", "true")
	q.Set("originator", HeaderOriginatorVal)
	ud := *u
	ud.RawQuery = q.Encode()

	return AuthFlow{
		URL:      ud.String(),
		State:    state,
		Verifier: verifier,
	}, nil
}

func ParseAuthorizationInput(input string) (code, state string) {
	value := strings.TrimSpace(input)
	if value == "" {
		return "", ""
	}

	if parsed, err := url.Parse(value); err == nil {
		return parsed.Query().Get("code"), parsed.Query().Get("state")
	}

	if strings.Contains(value, "#") {
		parts := strings.SplitN(value, "#", 2)
		if len(parts) == 2 {
			return parts[0], parts[1]
		}
	}

	if strings.Contains(value, "code=") {
		params, err := url.ParseQuery(value)
		if err == nil {
			return params.Get("code"), params.Get("state")
		}
	}

	return value, ""
}

func ExchangeAuthorizationCode(ctx context.Context, code, verifier string) (*oauth.Token, error) {
	if code == "" {
		return nil, errors.New("authorization code is required")
	}
	values := url.Values{}
	values.Set("grant_type", "authorization_code")
	values.Set("client_id", oauthClientID)
	values.Set("code", code)
	values.Set("code_verifier", verifier)
	values.Set("redirect_uri", RedirectURL)
	return requestToken(ctx, values)
}

func RefreshToken(ctx context.Context, refreshToken string) (*oauth.Token, error) {
	if refreshToken == "" {
		return nil, errors.New("refresh token is required")
	}
	values := url.Values{}
	values.Set("grant_type", "refresh_token")
	values.Set("refresh_token", refreshToken)
	values.Set("client_id", oauthClientID)
	return requestToken(ctx, values)
}

func ExtractAccountID(accessToken string) (string, error) {
	parts := strings.Split(accessToken, ".")
	if len(parts) != 3 {
		return "", errors.New("invalid JWT format")
	}

	decoded, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", fmt.Errorf("failed to decode JWT payload: %w", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(decoded, &payload); err != nil {
		return "", fmt.Errorf("failed to parse JWT payload: %w", err)
	}

	claim, ok := payload[jwtClaimPath].(map[string]any)
	if !ok {
		return "", errors.New("JWT claim missing chatgpt account data")
	}
	accountID, _ := claim["chatgpt_account_id"].(string)
	if accountID == "" {
		return "", errors.New("chatgpt account id missing in JWT")
	}
	return accountID, nil
}

func requestToken(ctx context.Context, values url.Values) (*oauth.Token, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(values.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to request token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token request failed: %s", resp.Status)
	}

	var payload struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("failed to decode token response: %w", err)
	}
	if payload.AccessToken == "" || payload.RefreshToken == "" || payload.ExpiresIn == 0 {
		return nil, errors.New("token response missing required fields")
	}

	token := &oauth.Token{
		AccessToken:  payload.AccessToken,
		RefreshToken: payload.RefreshToken,
		ExpiresIn:    payload.ExpiresIn,
	}
	token.SetExpiresAt()
	return token, nil
}

func newCodeVerifier() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate verifier: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func codeChallenge(verifier string) string {
	hash := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(hash[:])
}

func newState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate state: %w", err)
	}
	return hex.EncodeToString(b), nil
}
