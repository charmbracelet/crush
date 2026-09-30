package pinentry

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPromptMessage(t *testing.T) {
	t.Parallel()

	const passphraseBase = "Your GPG key wants its passphrase. "
	msg := promptMessage(KindPassphrase)
	require.True(t, strings.HasPrefix(msg, passphraseBase), "message: %q", msg)
	require.Contains(t, passphraseAsides, strings.TrimPrefix(msg, passphraseBase))

	const pinBase = "Your security key wants its PIN. "
	pin := promptMessage(KindPIN)
	require.True(t, strings.HasPrefix(pin, pinBase), "message: %q", pin)
	require.Contains(t, pinAsides, strings.TrimPrefix(pin, pinBase))
}
