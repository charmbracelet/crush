package oauth

import (
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// minRefreshBuffer is the minimum number of seconds before actual
// expiry at which IsExpired returns true. Prevents very short-lived
// tokens from having a meaningless refresh window.
const minRefreshBuffer = 30

// OAuthClient stores the client registration and authorization-server
// endpoints captured on the first successful authorization. Persisting
// them lets a later start rebuild the oauth2 config and refresh a saved
// token without re-running discovery or the browser flow.
type OAuthClient struct {
	ClientID     string `json:"client_id,omitempty"`
	ClientSecret string `json:"client_secret,omitempty"`
	AuthURL      string `json:"auth_url,omitempty"`
	TokenURL     string `json:"token_url,omitempty"`
	AuthStyle    int    `json:"auth_style,omitempty"`
}

// Token represents an OAuth2 token.
type Token struct {
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token,omitempty"`
	ExpiresIn    int          `json:"expires_in"`
	ExpiresAt    int64        `json:"expires_at"`
	Client       *OAuthClient `json:"client,omitempty"`
}

// SetExpiresAt calculates and sets the ExpiresAt field based on the
// current time and ExpiresIn. If ExpiresIn is zero or negative and
// ExpiresAt is already set (e.g. from the provider response), it is
// left unchanged. If neither is usable, ExpiresAt is set to zero so
// IsExpired treats the token as immediately expired, forcing a refresh
// rather than guessing a lifetime.
func (t *Token) SetExpiresAt() {
	if t.ExpiresIn > 0 {
		t.ExpiresAt = time.Now().Add(time.Duration(t.ExpiresIn) * time.Second).Unix()
		return
	}
	// ExpiresIn is missing or invalid. If ExpiresAt was already
	// populated by the provider (some return exp directly), trust it.
	if t.ExpiresAt > 0 {
		slog.Warn("OAuth token has invalid expires_in but valid expires_at, using expires_at",
			"expires_in", t.ExpiresIn, "expires_at", t.ExpiresAt)
		return
	}
	// Neither field is usable. Mark as expired so the caller is forced
	// to refresh rather than operating with a fabricated lifetime.
	slog.Warn("OAuth token has no valid expiry information, marking as expired",
		"expires_in", t.ExpiresIn, "expires_at", t.ExpiresAt)
	t.ExpiresAt = 0
}

// IsExpired checks if the token is expired or about to expire. It
// uses a buffer of max(expires_in/10, minRefreshBuffer) seconds to
// trigger proactive refresh before the token actually expires.
func (t *Token) IsExpired() bool {
	buffer := max(int64(t.ExpiresIn)/10, minRefreshBuffer)
	return time.Now().Unix() >= (t.ExpiresAt - buffer)
}

// SetExpiresIn calculates and sets the ExpiresIn field based on the ExpiresAt field.
func (t *Token) SetExpiresIn() {
	t.ExpiresIn = int(time.Until(time.Unix(t.ExpiresAt, 0)).Seconds())
}

// TokenExchangeError represents a failed OAuth token exchange. It carries
// the HTTP status code and response body so callers can distinguish between
// recoverable failures (e.g. temporary server error) and terminal ones
// (e.g. revoked refresh token).
type TokenExchangeError struct {
	StatusCode int
	Body       string
}

func (e *TokenExchangeError) Error() string {
	return fmt.Sprintf("token exchange failed: status %d body %q", e.StatusCode, e.Body)
}

// IsRefreshTokenRevoked reports whether the exchange failed because the
// refresh token was revoked or invalidated by the provider. This indicates
// that interactive re-authentication is required.
func (e *TokenExchangeError) IsRefreshTokenRevoked() bool {
	return strings.Contains(e.Body, "revoked") ||
		strings.Contains(e.Body, "invalid_grant")
}
