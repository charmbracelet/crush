package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestShouldCancelSubmittedPrompt(t *testing.T) {
	t.Parallel()

	require.True(t, shouldCancelSubmittedPrompt("stop", 0, true, 0))
	require.True(t, shouldCancelSubmittedPrompt("stop please", 0, false, 1))
	require.False(t, shouldCancelSubmittedPrompt("stop the service", 0, true, 0))
	require.False(t, shouldCancelSubmittedPrompt("stop", 1, true, 0))
	require.False(t, shouldCancelSubmittedPrompt("stop", 0, false, 0))
}
