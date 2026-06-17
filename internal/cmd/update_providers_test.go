package cmd

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUpdateProvidersCmd_SourceFlagIncludesLiveProviders(t *testing.T) {
	t.Parallel()

	flag := updateProvidersCmd.Flags().Lookup("source")
	require.NotNil(t, flag)
	require.Equal(t, "catwalk", flag.DefValue)
	require.Contains(t, flag.Usage, "venice")
	require.Contains(t, flag.Usage, "copilot")
}

func TestUpdateProvidersCmd_ExamplesIncludeLiveProviders(t *testing.T) {
	t.Parallel()

	example := strings.ToLower(updateProvidersCmd.Example)
	require.Contains(t, example, "--source=venice")
	require.Contains(t, example, "--source=copilot")
	require.Contains(t, example, "authenticated live providers")
}
