package config

import (
	"net/http"
	"slices"
	"testing"

	"charm.land/catwalk/v3/pkg/catwalk"
	"github.com/stretchr/testify/require"
)

func catalogWith(id string) []catwalk.Provider {
	return []catwalk.Provider{{
		Name: id, ID: catwalk.InferenceProvider(id), Type: catwalk.TypeCompletions,
		DefaultLargeModelID: "m", DefaultSmallModelID: "m",
		Models: []catwalk.Model{{ID: "m", Name: "M"}},
	}}
}

func knows(s *ConfigStore, id string) bool {
	return slices.ContainsFunc(s.KnownProviders(), func(p catwalk.Provider) bool { return string(p.ID) == id })
}

// TestRefreshCatalogsReachesEveryWorkspace covers a server hosting several
// workspaces in one process: a catalog update fetched by one workspace's
// refresh must still reach a workspace loaded before it.
func TestRefreshCatalogsReachesEveryWorkspace(t *testing.T) {
	// Not parallel: swaps package-level provider state and sets env vars.
	workDir, dataDir := isolateLoadEnv(t)
	t.Cleanup(resetProviderState)

	newCatwalkStub(t, http.StatusOK, catalogWith("old"))
	resetProviderState()
	_, err := Load(t.Context(), workDir, dataDir, false)
	require.NoError(t, err)

	// A new process starting from that cache, with a newer catalog upstream.
	newCatwalkStub(t, http.StatusOK, catalogWith("new"))
	resetProviderState()
	first, err := Load(t.Context(), workDir, dataDir, false)
	require.NoError(t, err)
	second, err := Load(t.Context(), workDir, dataDir, false)
	require.NoError(t, err)
	require.True(t, knows(second, "old"))

	reloaded, err := first.RefreshCatalogs(t.Context())
	require.NoError(t, err)
	require.True(t, reloaded)
	require.True(t, knows(first, "new"))

	reloaded, err = second.RefreshCatalogs(t.Context())
	require.NoError(t, err)
	require.True(t, reloaded, "the second workspace still has the old catalog")
	require.True(t, knows(second, "new"))

	reloaded, err = second.RefreshCatalogs(t.Context())
	require.NoError(t, err)
	require.False(t, reloaded, "a current workspace does not reload")
}
