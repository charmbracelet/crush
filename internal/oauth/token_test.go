package oauth

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsRefreshTokenRevoked(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		err     TokenExchangeError
		revoked bool
	}{
		{"invalid grant", TokenExchangeError{StatusCode: 400, Body: `{"error":"invalid_grant","error_description":"refresh token revoked"}`}, true},
		{"unauthorized", TokenExchangeError{StatusCode: 401, Body: `{"error":"refresh token revoked"}`}, true},
		{"unauthorized without body", TokenExchangeError{StatusCode: 401}, true},
		// Hyper's /token/exchange answers 401 for revoked, expired and
		// unknown refresh tokens alike.
		{"hyper expired", TokenExchangeError{StatusCode: 401, Body: `{"error":"refresh token expired"}` + "\n"}, true},
		{"rate limited", TokenExchangeError{StatusCode: 429, Body: `{"error":"rate_limit_error"}`}, false},
		{"server error mentioning revoked", TokenExchangeError{StatusCode: 500, Body: `upstream revoked our cache`}, false},
		{"bad request for another reason", TokenExchangeError{StatusCode: 400, Body: `{"error":"invalid_request"}`}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.revoked, tc.err.IsRefreshTokenRevoked())
		})
	}
}
