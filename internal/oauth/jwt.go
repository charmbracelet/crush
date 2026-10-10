package oauth

import (
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"strings"
)

// EmailFromIDToken reads the account address out of an ID token's payload.
// The signature is not verified: the token arrives over TLS straight from the
// provider, and the value only labels which account is signed in, so it
// authorizes nothing and verification would add nothing.
//
// An empty result means the token carries no email claim, which is normal for
// providers that are not OpenID Connect.
func EmailFromIDToken(idToken string) string {
	if idToken == "" {
		return ""
	}
	parts := strings.Split(idToken, ".")
	if len(parts) < 2 {
		return ""
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		payload, err = base64.URLEncoding.DecodeString(parts[1])
		if err != nil {
			slog.Debug("Could not decode ID token payload")
			return ""
		}
	}
	var claims struct {
		Email string `json:"email"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return ""
	}
	return claims.Email
}
