package agent

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsExplicitCancelPrompt(t *testing.T) {
	t.Parallel()

	for _, prompt := range []string{"stop", "STOP!", "stop please", "cancel current run", "abort"} {
		require.True(t, IsExplicitCancelPrompt(prompt), prompt)
	}
	for _, prompt := range []string{"stop the PM2 service", "do not stop", "cancel the deployment after verification", ""} {
		require.False(t, IsExplicitCancelPrompt(prompt), prompt)
	}
}
