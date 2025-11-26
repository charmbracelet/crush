package oauth

import (
	"time"
)

// Token represents an OAuth2 token from Claude Code Max.
type Token struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	ExpiresAt    int64  `json:"expires_at"`
}

// SetExpiresAt calculates and sets the ExpiresAt field based on the current time and ExpiresIn.
func (t *Token) SetExpiresAt() {
	t.ExpiresAt = time.Now().Add(time.Duration(t.ExpiresIn) * time.Second).Unix()
}

// IsExpired checks if the token is expired or about to expire (within 10% of its lifetime).
func (t *Token) IsExpired() bool {
	return time.Now().Unix() >= (t.ExpiresAt - int64(t.ExpiresIn)/10)
}
