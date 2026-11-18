package pinentry

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPromptMessage(t *testing.T) {
	t.Parallel()

	require.Equal(t, "Please enter the passphrase for your GPG key:", promptMessage(KindPassphrase))
	require.Equal(t, "Please enter the PIN for your security key:", promptMessage(KindPIN))
}
