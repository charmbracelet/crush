// Package openai implements OAuth authentication against OpenAI's
// authorization server, letting users sign in with a ChatGPT account and
// use their subscription through the Codex backend.
//
// The flow is the same PKCE-based browser authorization the Codex CLI
// uses: a loopback HTTP server on a fixed port receives the authorization
// code, which is then exchanged for access, refresh, and ID tokens.
package openai

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/charmbracelet/crush/internal/oauth"
)

const (
	// ClientID is the public OAuth client ID the Codex CLI desktop flow
	// registers with OpenAI's authorization server.
	ClientID = "app_EMoamEEZ73f0CkXaXp7hrann"

	// Issuer is OpenAI's authorization server.
	Issuer = "https://auth.openai.com"

	// CodexBaseURL is the ChatGPT backend that serves Codex requests
	// for ChatGPT-plan accounts. It speaks the Responses API.
	CodexBaseURL = "https://chatgpt.com/backend-api/codex"

	// Scope requests the claims needed to identify the account plus a
	// refresh token via offline_access.
	Scope = "openid profile email offline_access"
)

// tokenEndpoint and modelsEndpoint are variables so tests can point them
// at a stub server.
var (
	tokenEndpoint  = Issuer + "/oauth/token"
	modelsEndpoint = CodexBaseURL + "/models"
)

// HTTPClient allows tests to stub out the token endpoint. Production
// leaves it as a plain client with a generous timeout.
var HTTPClient = &http.Client{Timeout: 30 * time.Second}

// PKCE holds the proof key for the code exchange.
type PKCE struct {
	Verifier  string
	Challenge string
}

// NewPKCE generates a PKCE pair using the S256 method.
func NewPKCE() (PKCE, error) {
	verifier, err := randomToken(64)
	if err != nil {
		return PKCE{}, fmt.Errorf("generate code verifier: %w", err)
	}
	sum := sha256.Sum256([]byte(verifier))
	return PKCE{
		Verifier:  verifier,
		Challenge: base64.RawURLEncoding.EncodeToString(sum[:]),
	}, nil
}

// State returns a fresh random state parameter for CSRF protection.
func State() (string, error) {
	return randomToken(32)
}

func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// AuthorizeURL builds the browser authorization URL for the given
// redirect URI, PKCE pair, and state.
func AuthorizeURL(redirectURI string, pkce PKCE, state string) string {
	vals := url.Values{
		"response_type":              {"code"},
		"client_id":                  {ClientID},
		"redirect_uri":               {redirectURI},
		"scope":                      {Scope},
		"code_challenge":             {pkce.Challenge},
		"code_challenge_method":      {"S256"},
		"id_token_add_organizations": {"true"},
		"codex_cli_simplified_flow":  {"true"},
		"state":                      {state},
		"originator":                 {"crush"},
	}
	return Issuer + "/oauth/authorize?" + vals.Encode()
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"`
	ExpiresIn    int    `json:"expires_in"`
}

// ExchangeCode trades the authorization code captured by the callback
// server for an OAuth token.
func ExchangeCode(ctx context.Context, code, redirectURI string, pkce PKCE) (*oauth.Token, error) {
	vals := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"client_id":     {ClientID},
		"code_verifier": {pkce.Verifier},
	}
	token, err := requestToken(ctx, vals)
	if err != nil {
		return nil, fmt.Errorf("exchange authorization code: %w", err)
	}
	return token, nil
}

// RefreshToken exchanges a refresh token for a fresh token pair. The
// authorization server does not always rotate refresh tokens, so the
// previous one is kept when the response omits it.
func RefreshToken(ctx context.Context, refreshToken string) (*oauth.Token, error) {
	vals := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
		"client_id":     {ClientID},
	}
	token, err := requestToken(ctx, vals)
	if err != nil {
		return nil, fmt.Errorf("refresh token: %w", err)
	}
	if token.RefreshToken == "" {
		token.RefreshToken = refreshToken
	}
	return token, nil
}

func requestToken(ctx context.Context, vals url.Values) (*oauth.Token, error) {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		tokenEndpoint,
		strings.NewReader(vals.Encode()),
	)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, &oauth.TokenExchangeError{
			StatusCode: resp.StatusCode,
			Body:       string(body),
		}
	}

	var tr tokenResponse
	if err := json.Unmarshal(body, &tr); err != nil {
		return nil, fmt.Errorf("decode token response: %w", err)
	}
	if tr.AccessToken == "" {
		return nil, fmt.Errorf("token response contained no access token")
	}

	token := &oauth.Token{
		AccessToken:  tr.AccessToken,
		RefreshToken: tr.RefreshToken,
		IDToken:      tr.IDToken,
		ExpiresIn:    tr.ExpiresIn,
	}
	token.SetExpiresAt()

	// The account ID travels inside the ID token (falling back to the
	// access token for servers that omit one). The ChatGPT backend
	// requires it on every request to attribute usage to the right
	// account.
	claims, err := ParseJWTClaims(token.IDToken)
	if err != nil || claims == nil {
		claims, _ = ParseJWTClaims(token.AccessToken)
	}
	token.AccountID = ExtractAccountID(claims)

	return token, nil
}

// Claims holds the untrusted metadata extracted from a token's JWT
// payload. Values are used only for routing requests, never for
// authorization decisions.
type Claims struct {
	ChatGPTAccountID string `json:"chatgpt_account_id"`
	Auth             *struct {
		ChatGPTAccountID string `json:"chatgpt_account_id"`
	} `json:"https://api.openai.com/auth"`
}

// ParseJWTClaims decodes the payload segment of a JWT without verifying
// its signature. The claims are advisory metadata, so signature checking
// is the server's job, not ours.
func ParseJWTClaims(token string) (*Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 || parts[1] == "" {
		return nil, fmt.Errorf("invalid JWT structure")
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		payload, err = base64.URLEncoding.DecodeString(parts[1])
		if err != nil {
			return nil, fmt.Errorf("decode JWT payload: %w", err)
		}
	}

	var claims Claims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, fmt.Errorf("decode JWT claims: %w", err)
	}
	return &claims, nil
}

// ExtractAccountID resolves the ChatGPT account ID from token claims.
func ExtractAccountID(claims *Claims) string {
	if claims == nil {
		return ""
	}
	if claims.ChatGPTAccountID != "" {
		return claims.ChatGPTAccountID
	}
	if claims.Auth != nil {
		return claims.Auth.ChatGPTAccountID
	}
	return ""
}
