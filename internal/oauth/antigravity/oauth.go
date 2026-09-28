// Package antigravity implements OAuth against Google's subscription backend,
// letting users sign in with a Google AI plan (the one behind the Antigravity
// and Gemini CLIs) instead of a metered Gemini API key.
//
// Unlike a normal third-party integration, this reuses Google's own
// first-party desktop client so that the resulting token is accepted by the
// Cloud Code Assist gateway (`v1internal`), which serves the subscription.
// That gateway is not the public Gemini API: the request/response envelope
// differences are handled by the Transport in this package.
//
// The login itself is an ordinary PKCE authorization-code flow. Google treats
// this client as a desktop app, so any loopback redirect port is allowed and
// no hosted callback or manual code pasting is needed.
package antigravity

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/crush/internal/oauth"
)

const (
	// ClientID is Google's first-party client for the Antigravity CLI. It
	// is public and ships inside that binary.
	ClientID = "1071006060591-tmhssin2h21lcre235vtolojh4g403ep.apps.googleusercontent.com"

	// ClientSecretEnv names the environment variable that can supply the
	// OAuth client secret for ClientID, for people who would rather not put
	// it in their configuration file. Configuration wins when both exist.
	ClientSecretEnv = "CRUSH_ANTIGRAVITY_CLIENT_SECRET"

	// AuthorizeEndpoint is Google's OAuth authorization endpoint.
	AuthorizeEndpoint = "https://accounts.google.com/o/oauth2/auth"

	// Scope is the exact set Antigravity asks for. `aicode` and
	// `cloud-platform` are what grant access to the subscription backend.
	Scope = "https://www.googleapis.com/auth/cloud-platform " +
		"https://www.googleapis.com/auth/userinfo.email " +
		"https://www.googleapis.com/auth/userinfo.profile " +
		"https://www.googleapis.com/auth/cclog " +
		"https://www.googleapis.com/auth/experimentsandconfigs " +
		"https://www.googleapis.com/auth/aicode openid"

	// APIBase is the Cloud Code Assist gateway that serves subscription
	// models. Antigravity uses this "daily" host; the plain
	// cloudcode-pa.googleapis.com host is the production equivalent.
	APIBase = "https://daily-cloudcode-pa.googleapis.com/v1internal"

	// UserAgent identifies the client to the subscription backend, which
	// rejects requests that do not look like a supported first-party CLI.
	UserAgent = "antigravity/cli/1.2.4 (aidev_client; os_type=linux; arch=amd64; auth_method=consumer)"
)

// ErrClientSecretRequired means the OAuth client secret Google's token
// endpoint demands was not supplied. It is a sentinel so the CLI and TUI can
// recognise the case and ask for the value instead of failing.
var ErrClientSecretRequired = errors.New(
	"the Google AI subscription needs its OAuth client secret: enter it when " +
		"prompted by `crush login gemini`, set providers." + ProviderID +
		".oauth_client_secret in the configuration, or export " + ClientSecretEnv,
)

// tokenEndpoint is a variable so tests can point it at a stub server.
var tokenEndpoint = "https://oauth2.googleapis.com/token"

// normalizeClientSecret validates the client secret supplied for ClientID.
//
// Google's token endpoint requires it on both grants, so neither sign-in nor
// refresh can work without it. It is supplied through configuration rather
// than being committed here: the value is a shared installed-app secret
// published inside Google's own CLIs, not a user credential, but GitHub push
// protection rejects the literal on every push of a fork.
func normalizeClientSecret(secret string) (string, error) {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return "", ErrClientSecretRequired
	}
	return secret, nil
}

// HTTPClient allows tests to stub the token endpoint.
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

// mustToken is randomToken for paths that cannot fail meaningfully, such as
// labelling a tool call whose backend omitted the id.
func mustToken(n int) string {
	token, err := randomToken(n)
	if err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	return token
}

// AuthorizeURL builds the browser authorization URL for the given redirect
// URI, PKCE pair, and state.
//
// `access_type=offline` is what makes Google return a refresh token, and
// `prompt=consent` forces it even when the account has consented before.
func AuthorizeURL(redirectURI string, pkce PKCE, state string) string {
	vals := url.Values{
		"response_type":         {"code"},
		"client_id":             {ClientID},
		"redirect_uri":          {redirectURI},
		"scope":                 {Scope},
		"code_challenge":        {pkce.Challenge},
		"code_challenge_method": {"S256"},
		"access_type":           {"offline"},
		"prompt":                {"consent"},
		"state":                 {state},
	}
	return AuthorizeEndpoint + "?" + vals.Encode()
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"`
	ExpiresIn    int    `json:"expires_in"`
}

// ExchangeCode trades the authorization code captured by the callback server
// for an OAuth token.
func ExchangeCode(ctx context.Context, code, redirectURI string, pkce PKCE, clientSecret string) (*oauth.Token, error) {
	secret, err := normalizeClientSecret(clientSecret)
	if err != nil {
		return nil, err
	}
	vals := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"client_id":     {ClientID},
		"client_secret": {secret},
		"code_verifier": {pkce.Verifier},
	}
	token, err := requestToken(ctx, vals)
	if err != nil {
		return nil, fmt.Errorf("exchange authorization code: %w", err)
	}
	return token, nil
}

// RefreshToken exchanges a refresh token for a fresh access token. Google
// does not always rotate the refresh token, so the previous one is kept when
// the response omits it.
func RefreshToken(ctx context.Context, refreshToken, clientSecret string) (*oauth.Token, error) {
	secret, err := normalizeClientSecret(clientSecret)
	if err != nil {
		return nil, err
	}
	vals := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
		"client_id":     {ClientID},
		"client_secret": {secret},
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

	// AccountID carries the signed-in address, which the TUI shows next to
	// the provider and is the only stable identity these tokens expose.
	token.AccountID = EmailFromIDToken(tr.IDToken)

	return token, nil
}

// EmailFromIDToken reads the account address out of an (unverified) ID token.
// The value is used for display only, never to authorize anything.
func EmailFromIDToken(idToken string) string {
	claims, err := ParseJWTClaims(idToken)
	if err != nil || claims == nil {
		return ""
	}
	return claims.Email
}

// Claims holds the untrusted metadata extracted from a token's JWT payload.
type Claims struct {
	Email string `json:"email"`
	Scope string `json:"scope"`
}

// ParseJWTClaims decodes a JWT payload without verifying its signature. The
// token comes straight from Google over TLS, and the fields are only used to
// label the account, so verification adds nothing here.
func ParseJWTClaims(raw string) (*Claims, error) {
	if raw == "" {
		return nil, nil
	}
	parts := strings.Split(raw, ".")
	if len(parts) < 2 {
		return nil, fmt.Errorf("malformed token: expected 3 parts, got %d", len(parts))
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		payload, err = base64.URLEncoding.DecodeString(parts[1])
		if err != nil {
			return nil, fmt.Errorf("decode token payload: %w", err)
		}
	}
	claims := &Claims{}
	if err := json.Unmarshal(payload, claims); err != nil {
		return nil, fmt.Errorf("decode token claims: %w", err)
	}
	return claims, nil
}
