package oauth

import (
	"log/slog"
	"time"
)

// minRefreshBuffer is the minimum number of seconds before actual
// expiry at which IsExpired returns true. Prevents very short-lived
// tokens from having a meaningless refresh window.
const minRefreshBuffer = 30

// Token represents an OAuth2 token.
type Token struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	ExpiresAt    int64  `json:"expires_at"`
}

// SetExpiresAt calculates and sets the ExpiresAt field based on the
// current time and ExpiresIn. If ExpiresIn is zero or negative, it
// defaults to 3600 seconds and logs a warning.
func (t *Token) SetExpiresAt() {
	if t.ExpiresIn <= 0 {
		slog.Warn("OAuth token has invalid expires_in, defaulting to 3600s", "expires_in", t.ExpiresIn)
		t.ExpiresIn = 3600
	}
	t.ExpiresAt = time.Now().Add(time.Duration(t.ExpiresIn) * time.Second).Unix()
}

// IsExpired checks if the token is expired or about to expire. It
// uses a buffer of max(expires_in/10, minRefreshBuffer) seconds to
// trigger proactive refresh before the token actually expires.
func (t *Token) IsExpired() bool {
	buffer := int64(t.ExpiresIn) / 10
	if buffer < minRefreshBuffer {
		buffer = minRefreshBuffer
	}
	return time.Now().Unix() >= (t.ExpiresAt - buffer)
}

// SetExpiresIn calculates and sets the ExpiresIn field based on the ExpiresAt field.
func (t *Token) SetExpiresIn() {
	t.ExpiresIn = int(time.Until(time.Unix(t.ExpiresAt, 0)).Seconds())
}
