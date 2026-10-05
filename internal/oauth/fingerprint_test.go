package oauth

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// A fingerprint goes into log files, so it must name a token without
// carrying any part of it.
func TestFingerprintDisclosesNothing(t *testing.T) {
	t.Parallel()

	const secret = "sk-ant-ort01-not-a-real-refresh-token"
	tok := &Token{RefreshToken: secret}
	fp := tok.Fingerprint()

	require.NotContains(t, secret, fp)
	require.NotContains(t, fp, secret)
	require.Len(t, fp, 12)
	require.Equal(t, fp, (&Token{RefreshToken: secret}).Fingerprint(), "stable across values")
	require.NotEqual(t, fp, (&Token{RefreshToken: secret + "x"}).Fingerprint(), "distinguishes rotations")
	require.False(t, strings.ContainsAny(fp, " \t"), "safe as a log field")
}

// A missing token still has to log as something readable rather than an
// empty field that reads like a bug.
func TestFingerprintWithoutAToken(t *testing.T) {
	t.Parallel()

	var nilToken *Token
	require.Equal(t, "none", nilToken.Fingerprint())
	require.Equal(t, "none", (&Token{}).Fingerprint())
	require.Equal(t, "none", FingerprintSecret(""))
}
