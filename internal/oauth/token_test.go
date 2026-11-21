package oauth

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// tokenExpiringIn builds a token of the given lifetime with the given time
// left on it, so tests can describe a position in the refresh window
// instead of computing timestamps.
func tokenExpiringIn(lifetime, remaining time.Duration) *Token {
	return &Token{
		AccessToken: "at",
		ExpiresIn:   int(lifetime.Seconds()),
		ExpiresAt:   time.Now().Add(remaining).Unix(),
	}
}

// TestRefreshWindowsDoNotOverlap pins the shape of the two windows: a token
// is renewed ahead of time, then inline, and never both at once. An overlap
// would put an exchange on the critical path that the ahead-of-time pass
// was supposed to have already taken care of.
func TestRefreshWindowsDoNotOverlap(t *testing.T) {
	t.Parallel()

	// An 8h token: renewal is due with 48m left, and the ahead-of-time
	// window opens an equal span earlier, at 1h36m.
	const lifetime = 8 * time.Hour

	for _, tc := range []struct {
		name      string
		remaining time.Duration
		expired   bool
		ahead     bool
	}{
		{"fresh", 7 * time.Hour, false, false},
		{"inside the ahead-of-time window", 70 * time.Minute, false, true},
		{"past the renewal deadline", 30 * time.Minute, true, false},
		{"lapsed", -time.Minute, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			token := tokenExpiringIn(lifetime, tc.remaining)
			require.Equal(t, tc.expired, token.IsExpired(), "IsExpired")
			require.Equal(t, tc.ahead, token.ShouldRefreshAhead(), "ShouldRefreshAhead")
			require.False(t, token.IsExpired() && token.ShouldRefreshAhead(),
				"a token cannot be due for both renewals at once")
		})
	}
}

// TestShortLivedTokenKeepsAUsableWindow covers tokens whose lifetime is too
// short for the proportional buffer to mean anything. The floor has to
// apply to both windows, or a short token is born already past its
// ahead-of-time pass and every turn pays for an inline exchange.
func TestShortLivedTokenKeepsAUsableWindow(t *testing.T) {
	t.Parallel()

	token := tokenExpiringIn(2*time.Minute, 2*time.Minute)
	require.False(t, token.IsExpired())
	require.False(t, token.ShouldRefreshAhead(), "a freshly issued token has nothing to do yet")

	// minRefreshBuffer is 30s, so renewal is due at 30s left and the
	// ahead-of-time window opens at 60s.
	require.True(t, tokenExpiringIn(2*time.Minute, 45*time.Second).ShouldRefreshAhead())
	require.True(t, tokenExpiringIn(2*time.Minute, 20*time.Second).IsExpired())
}

// TestJustIssuedNamesTheCredentialsWorthLeavingAlone pins the window that
// keeps unrelated triggers from each rotating the same credential in turn.
func TestJustIssuedNamesTheCredentialsWorthLeavingAlone(t *testing.T) {
	t.Parallel()

	const window = time.Minute
	lifetime := time.Hour

	// Remaining time is what dates a token: an hour-long token with 59
	// minutes left was minted a minute ago.
	require.True(t, tokenExpiringIn(lifetime, lifetime).JustIssued(window),
		"a credential that arrived this instant")
	require.True(t, tokenExpiringIn(lifetime, lifetime-30*time.Second).JustIssued(window))
	require.False(t, tokenExpiringIn(lifetime, lifetime-2*time.Minute).JustIssued(window),
		"past the window, a refresh request is honoured")
	require.False(t, tokenExpiringIn(lifetime, 30*time.Second).JustIssued(window),
		"an aging credential is never fresh")

	// A token that states no lifetime cannot say when it was issued, so it
	// gets no protection from a claim it never made.
	require.False(t, (&Token{AccessToken: "at", ExpiresAt: time.Now().Add(time.Hour).Unix()}).JustIssued(window))
	require.False(t, (*Token)(nil).JustIssued(window))
}

func TestIsRefreshTokenRevoked(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		err     *TokenExchangeError
		revoked bool
	}{
		{"invalid grant", NewTokenExchangeError(400, `{"error":"invalid_grant","error_description":"refresh token revoked"}`), true},
		{"unauthorized", NewTokenExchangeError(401, `{"error":"refresh token revoked"}`), true},
		{"unauthorized without body", NewTokenExchangeError(401, ""), true},
		// Hyper's /token/exchange answers 401 for revoked, expired and
		// unknown refresh tokens alike.
		{"hyper expired", NewTokenExchangeError(401, `{"error":"refresh token expired"}`+"\n"), true},
		{"rate limited", NewTokenExchangeError(429, `{"error":"rate_limit_error"}`), false},
		{"server error mentioning revoked", NewTokenExchangeError(500, `upstream revoked our cache`), false},
		{"bad request for another reason", NewTokenExchangeError(400, `{"error":"invalid_request"}`), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.revoked, tc.err.IsRefreshTokenRevoked())
		})
	}
}

// The body a token endpoint answers with may echo a credential back, so the
// error string that reaches logs and the screen must never carry it. Only a
// standard OAuth error code is allowed through.
func TestTokenExchangeErrorKeepsTheBodyOutOfItsMessage(t *testing.T) {
	t.Parallel()

	const secret = "rt-secret-value"
	for _, tc := range []struct {
		name string
		body string
		want string
	}{
		{"standard code", `{"error":"invalid_grant","error_description":"` + secret + ` revoked"}`, "token exchange failed: status 400 (invalid_grant)"},
		{"free-form error", `{"error":"refresh token ` + secret + ` expired"}`, "token exchange failed: status 400"},
		{"not json", "bad token " + secret, "token exchange failed: status 400"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := NewTokenExchangeError(400, tc.body)
			require.Equal(t, tc.want, err.Error())
			require.NotContains(t, err.Error(), secret)
			require.Equal(t, tc.body, err.Body(), "the body is kept for classification")
		})
	}
}
