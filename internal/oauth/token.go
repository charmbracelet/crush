package oauth

import (
	"crypto/sha256"
	"encoding/hex"
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
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	IDToken      string `json:"id_token,omitempty"`
	// AccountID is the provider account the token belongs to, extracted
	// from the ID token when the provider embeds one. The ChatGPT
	// backend requires it as a header on every request.
	AccountID string       `json:"account_id,omitempty"`
	ExpiresIn int          `json:"expires_in"`
	ExpiresAt int64        `json:"expires_at"`
	Client    *OAuthClient `json:"client,omitempty"`
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
	return time.Now().Unix() >= (t.ExpiresAt - t.refreshBuffer())
}

// aheadRefreshMultiplier widens the expiry buffer to open a window in
// which a credential is renewed early, before anything is waiting on it.
// Inside that window the current token is still perfectly good, so the
// renewal runs off the critical path; only once the token crosses the
// IsExpired line does a turn have to stop and wait for one.
//
// Renewing early also keeps sessions from converging on the same moment.
// Providers that rotate refresh tokens retire the old one on every
// exchange, so two sessions that both wait for expiry and then exchange
// race to present the same credential, and the loser's is already dead.
const aheadRefreshMultiplier = 2

// ShouldRefreshAhead reports whether the token is near enough to its
// refresh deadline to be worth renewing now, while it is still usable.
// Callers renew in the background and carry on with the current token,
// so a turn only ever waits on an exchange when this window was missed.
func (t *Token) ShouldRefreshAhead() bool {
	if t.IsExpired() {
		return false
	}
	return time.Now().Unix() >= (t.ExpiresAt - t.refreshBuffer()*aheadRefreshMultiplier)
}

// Fingerprint identifies a refresh token in logs without disclosing it: the
// first six bytes of its SHA-256, hex encoded. Enough to tell one credential
// from another, and to follow one across processes and restarts, which is
// how a rotation lost between the exchange and the write to disk becomes
// visible. Returns "none" when there is no refresh token to name.
func (t *Token) Fingerprint() string {
	if t == nil || t.RefreshToken == "" {
		return "none"
	}
	return FingerprintSecret(t.RefreshToken)
}

// FingerprintSecret names a secret for logs without disclosing it.
func FingerprintSecret(secret string) string {
	if secret == "" {
		return "none"
	}
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:6])
}

// ExpiresInSeconds reports how long the access token has left, negative once
// it has lapsed. Logged at decision points so a session that refuses to
// refresh can be told apart from one that never looked.
func (t *Token) ExpiresInSeconds() int64 {
	if t == nil {
		return 0
	}
	return t.ExpiresAt - time.Now().Unix()
}

// refreshBuffer is how long before actual expiry a token is treated as
// needing renewal.
func (t *Token) refreshBuffer() int64 {
	return max(int64(t.ExpiresIn)/10, minRefreshBuffer)
}

// JustIssued reports whether the provider minted this token within the
// given window, derived from its stated lifetime.
//
// It answers "has an exchange already happened here a moment ago", which
// is what separates a credential worth renewing from one that several
// independent triggers are all reacting to at once. A provider that
// rotates refresh tokens retires the previous one on every exchange, so
// renewing a credential that is seconds old gains nothing and costs the
// peers still carrying its predecessor.
//
// A token that does not state a lifetime has no issue time to reason
// about, so it is never considered fresh.
func (t *Token) JustIssued(window time.Duration) bool {
	if t == nil || t.ExpiresIn <= 0 || t.ExpiresAt <= 0 {
		return false
	}
	age := time.Since(time.Unix(t.ExpiresAt-int64(t.ExpiresIn), 0))
	return age >= 0 && age < window
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
