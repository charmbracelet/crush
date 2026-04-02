package tools

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSearchRegistryEntries_DeferredAndExposedFilters(t *testing.T) {
	entries := []RegistryEntry{
		{
			Name:        "view",
			Description: "read file",
			Source:      "builtin",
			Metadata: ToolMetadata{
				ReadOnly: true,
				Exposure: ToolExposureDefault,
			},
			Exposed: true,
		},
		{
			Name:        "sourcegraph",
			Description: "search public repositories",
			Source:      "builtin",
			Metadata: ToolMetadata{
				ReadOnly: true,
				Exposure: ToolExposureDeferred,
			},
			Exposed: false,
		},
	}

	defaultResults := SearchRegistryEntries(entries, "", RegistrySearchOptions{Limit: 10})
	require.Len(t, defaultResults, 1)
	require.Equal(t, "view", defaultResults[0].Name)

	withDeferred := SearchRegistryEntries(entries, "", RegistrySearchOptions{Limit: 10, IncludeDeferred: true})
	require.Len(t, withDeferred, 2)
	require.Equal(t, "sourcegraph", withDeferred[1].Name)

	exposedOnly := SearchRegistryEntries(entries, "", RegistrySearchOptions{Limit: 10, IncludeDeferred: true, ExposedOnly: true})
	require.Len(t, exposedOnly, 1)
	require.Equal(t, "view", exposedOnly[0].Name)
}
