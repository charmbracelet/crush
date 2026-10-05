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

// TestShortLivedTokenKeepsAUsableWindow keeps the buffer from outliving a
// short-lived token: the windows scale with the stated lifetime, floored at
// the minimum.
func TestShortLivedTokenKeepsAUsableWindow(t *testing.T) {
	t.Parallel()

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
