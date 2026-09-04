package openai

import (
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
	// ClientID is the public OAuth client ID registered for Codex CLI desktop flow.
	ClientID = "app_EMoamEEZ73f0CkXaXp7hrann"
	// Issuer is the OpenAI authorization server.
	Issuer = "https://auth.openai.com"
	// CodexEndpoint is the ChatGPT backend responses endpoint for Codex.
	CodexEndpoint = "https://chatgpt.com/backend-api/codex/responses"
	// CallbackPort is the loopback port registered for the desktop client.
	CallbackPort = 1455
)

// HTTPClient allows tests to mock outgoing OAuth requests.
var HTTPClient = &http.Client{Timeout: 30 * time.Second}

// Claims contains untrusted metadata extracted from the JWT token.
type Claims struct {
	ChatGPTAccountID        string `json:"chatgpt_account_id"`
	ChatGPTComputeResidency string `json:"chatgpt_compute_residency"`
	Organizations           []struct {
		ID string `json:"id"`
	} `json:"organizations"`
	Auth *struct {
		ChatGPTAccountID        string `json:"chatgpt_account_id"`
		ChatGPTComputeResidency string `json:"chatgpt_compute_residency"`
	} `json:"https://api.openai.com/auth"`
}

// ParseJWTClaims extracts metadata from an untrusted JWT payload without signature verification.
func ParseJWTClaims(token string) (*Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 || parts[1] == "" {
		return nil, errors.New("invalid JWT structure")
	}

	payload := parts[1]
	// Handle unpadded and padded base64url.
	decoded, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		decoded, err = base64.URLEncoding.DecodeString(payload)
		if err != nil {
			return nil, fmt.Errorf("decode JWT payload: %w", err)
		}
	}

	var claims Claims
	if err := json.Unmarshal(decoded, &claims); err != nil {
		return nil, fmt.Errorf("unmarshal JWT claims: %w", err)
	}

	return &claims, nil
}

// ExtractAccountID resolves the effective ChatGPT account ID from token claims.
func ExtractAccountID(claims *Claims) string {
	if claims == nil {
		return ""
	}
	if claims.ChatGPTAccountID != "" {
		return claims.ChatGPTAccountID
	}
	if claims.Auth != nil && claims.Auth.ChatGPTAccountID != "" {
		return claims.Auth.ChatGPTAccountID
	}
	if len(claims.Organizations) > 0 {
		return claims.Organizations[0].ID
	}
	return ""
}

// ExtractResidency resolves the compute residency constraint from token claims.
func ExtractResidency(claims *Claims) string {
	if claims == nil {
		return ""
	}
	residency := claims.ChatGPTComputeResidency
	if claims.Auth != nil && claims.Auth.ChatGPTComputeResidency != "" {
		residency = claims.Auth.ChatGPTComputeResidency
	}
	if residency == "no_constraint" {
		return ""
	}
	return residency
}

// AuthorizeURL builds the browser authorization URL with PKCE and state.
func AuthorizeURL(redirectURI string, pkce oauth.PKCE, state string) string {
	vals := url.Values{
		"response_type":              {"code"},
		"client_id":                  {ClientID},
		"redirect_uri":               {redirectURI},
		"scope":                      {"openid profile email offline_access"},
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

func exchange(ctx context.Context, vals url.Values) (*oauth.Token, error) {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		Issuer+"/oauth/token",
		strings.NewReader(vals.Encode()),
	)
	if err != nil {
		return nil, fmt.Errorf("create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute token request: %w", err)
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
		return nil, fmt.Errorf("unmarshal token response: %w", err)
	}

	tok := &oauth.Token{
		AccessToken:  tr.AccessToken,
		RefreshToken: tr.RefreshToken,
		IDToken:      tr.IDToken,
		ExpiresIn:    tr.ExpiresIn,
		Extra:        make(map[string]string),
	}
	tok.SetExpiresAt()

	// Extract claims from ID token first, falling back to access token.
	claims, err := ParseJWTClaims(tr.IDToken)
	if err != nil || claims == nil {
		claims, _ = ParseJWTClaims(tr.AccessToken)
	}
	tok.AccountID = ExtractAccountID(claims)
	if res := ExtractResidency(claims); res != "" {
		tok.Extra["residency"] = res
	}

	return tok, nil
}

// ExchangeCode exchanges an authorization code and PKCE verifier for tokens.
func ExchangeCode(ctx context.Context, code, redirectURI string, pkce oauth.PKCE) (*oauth.Token, error) {
	vals := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"client_id":     {ClientID},
		"code_verifier": {pkce.Verifier},
	}
	return exchange(ctx, vals)
}

// RefreshToken exchanges a refresh token for a fresh token pair.
func RefreshToken(ctx context.Context, refreshToken string) (*oauth.Token, error) {
	vals := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
		"client_id":     {ClientID},
	}
	token, err := exchange(ctx, vals)
	if err != nil {
		return nil, err
	}
	if token.RefreshToken == "" {
		token.RefreshToken = refreshToken
	}
	return token, nil
}
