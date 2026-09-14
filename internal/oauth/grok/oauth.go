// Package grok implements OAuth authentication against xAI's
// authorization server, letting users sign in with the Grok account their
// subscription is attached to and use it through the xAI API.
//
// The flow is the same PKCE-based browser authorization the Grok CLI
// uses: a loopback HTTP server on a random port receives the
// authorization code, which is then exchanged for access, refresh, and
// ID tokens. The access token is a valid Bearer credential for
// api.x.ai/v1.
package grok

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
	// ClientID is the public OAuth client ID xAI's own clients (the
	// Grok CLI) register with the authorization server. xAI does not
	// offer self-service client registration, so agent CLIs share it.
	ClientID = "b1a00492-073a-47ea-816f-4c329264a828"

	// Issuer is xAI's authorization server.
	Issuer = "https://auth.x.ai"

	// Scope requests the claims needed to identify the account, a
	// refresh token via offline_access, and access to the xAI API. The
	// conversation and workspace scopes are part of the shared
	// client's registered set, so the server expects all of them.
	Scope = "openid profile email offline_access grok-cli:access " +
		"api:access conversations:read conversations:write " +
		"workspaces:read workspaces:write"
)

// authorizeEndpoint and tokenEndpoint are variables so tests can point
// them at a stub server.
var (
	authorizeEndpoint = Issuer + "/oauth2/authorize"
	tokenEndpoint     = Issuer + "/oauth2/token"
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

// Nonce returns a fresh random nonce binding the ID token to this
// authorization.
func Nonce() (string, error) {
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
// redirect URI, PKCE pair, state, and nonce.
func AuthorizeURL(redirectURI string, pkce PKCE, state, nonce string) string {
	vals := url.Values{
		"response_type":         {"code"},
		"client_id":             {ClientID},
		"redirect_uri":          {redirectURI},
		"scope":                 {Scope},
		"code_challenge":        {pkce.Challenge},
		"code_challenge_method": {"S256"},
		"state":                 {state},
		"nonce":                 {nonce},
		"referrer":              {"crush"},
	}
	return authorizeEndpoint + "?" + vals.Encode()
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

	return token, nil
}
