package oauth

import "time"

// DefaultExpiryBuffer is the default buffer time (in seconds) before actual expiry
// when a token is considered "expired" for safety purposes.
const DefaultExpiryBuffer = 30

// DeviceCodeResponse represents the response from a device code OAuth flow.
type DeviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

// Token represents an OAuth2 token.
type Token struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresIn    int    `json:"expires_in,omitempty"`
	ExpiresAt    int64  `json:"expires_at,omitempty"`
}

// SetExpiryFromNow sets ExpiresAt based on the current time plus ExpiresIn seconds.
func (t *Token) SetExpiryFromNow() {
	if t.ExpiresIn > 0 {
		t.ExpiresAt = time.Now().Add(time.Duration(t.ExpiresIn) * time.Second).Unix()
	}
}

// UpdateExpiryFromTimestamp recalculates ExpiresIn based on the current time and ExpiresAt.
// This is useful when you have an expires_at timestamp and need to sync the ExpiresIn field.
func (t *Token) UpdateExpiryFromTimestamp() {
	if t.ExpiresAt > 0 {
		t.ExpiresIn = int(time.Until(time.Unix(t.ExpiresAt, 0)).Seconds())
	}
}

// IsExpired checks if the token is expired or will expire within DefaultExpiryBuffer seconds.
func (t *Token) IsExpired() bool {
	if t.ExpiresAt == 0 {
		return true
	}
	return time.Now().Unix() >= (t.ExpiresAt - DefaultExpiryBuffer)
}

// NewToken creates a new token with the given access token and optional refresh token.
func NewToken(accessToken, refreshToken string) *Token {
	return &Token{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}
}
