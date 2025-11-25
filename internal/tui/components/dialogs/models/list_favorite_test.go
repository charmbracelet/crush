package models

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/log"
	"github.com/charmbracelet/crush/internal/tui/exp/list"
	"github.com/stretchr/testify/require"
)

func TestModelList_Favorites(t *testing.T) {
	// Pre-initialize logger to os.DevNull to prevent file lock on Windows.
	log.Setup(os.DevNull, false)

	// Isolate config/data paths
	cfgDir := t.TempDir()
	dataDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfgDir)
	t.Setenv("XDG_DATA_HOME", dataDir)

	// Pre-seed config with favorited models.
	// p1 will have all its models favorited.
	// p2 will have only one of its models favorited.
	confPath := filepath.Join(cfgDir, "crush", "crush.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(confPath), 0o755))
	initial := map[string]any{
		"options": map[string]any{
			"disable_provider_auto_update": true,
		},
		"models": map[string]any{
			"large": map[string]any{
				"model":    "p2-m2", // A default model that is not a favorite
				"provider": "p2",
			},
		},
		"favorited_models": map[string]any{
			"large": []any{
				map[string]any{"model": "p1-m1", "provider": "p1"}, // All of p1's models
				map[string]any{"model": "p1-m2", "provider": "p1"},
				map[string]any{"model": "p2-m1", "provider": "p2"}, // Only one of p2's models
			},
		},
	}
	bts, err := json.Marshal(initial)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(confPath, bts, 0o644))

	// Create empty providers.json to prevent loading real providers
	dataConfDir := filepath.Join(dataDir, "crush")
	require.NoError(t, os.MkdirAll(dataConfDir, 0o755))
	emptyProviders := []byte("[]")
	require.NoError(t, os.WriteFile(filepath.Join(dataConfDir, "providers.json"), emptyProviders, 0o644))

	// Initialize global config instance
	_, err = config.Init(cfgDir, dataDir, false)
	require.NoError(t, err)

	// Build a provider set for the list component
	providers := []catwalk.Provider{
		{
			ID:   catwalk.InferenceProvider("p1"),
			Name: "Provider One",
			Models: []catwalk.Model{
				{ID: "p1-m1", Name: "P1 Model One"},
				{ID: "p1-m2", Name: "P1 Model Two"},
			},
		},
		{
			ID:   catwalk.InferenceProvider("p2"),
			Name: "Provider Two",
			Models: []catwalk.Model{
				{ID: "p2-m1", Name: "P2 Model One"},
				{ID: "p2-m2", Name: "P2 Model Two"},
			},
		},
	}

	// Create and initialize the component with our provider set
	listKeyMap := list.DefaultKeyMap()
	cmp := NewModelListComponent(listKeyMap, "Find your fave", false)
	cmp.providers = providers
	execCmdML(t, cmp, cmp.Init())

	// --- ASSERTIONS ---
	groups := cmp.list.Groups()
	require.NotEmpty(t, groups)

	require.Len(t, groups, 2, "Expected 2 groups: Favorites and Provider Two")

	favGroup := groups[0]
	p2Group := groups[1]

	// Assert Favorites group is correct
	require.Len(t, favGroup.Items, 3, "Favorites group should have 3 items")
	favIDs := make([]string, len(favGroup.Items))
	for i, item := range favGroup.Items {
		favIDs[i] = item.ID()
	}
	require.Contains(t, favIDs, "p1:p1-m1")
	require.Contains(t, favIDs, "p1:p1-m2")
	require.Contains(t, favIDs, "p2:p2-m1")

	// Assert Provider One group is HIDDEN by checking the group count is 2.
	// Assert Provider Two group is VISIBLE and has the correct content
	require.Len(t, p2Group.Items, 1, "Provider Two group should have 1 non-favorite model")
	require.Equal(t, "p2:p2-m2", p2Group.Items[0].ID(), "The remaining model in Provider Two should be p2-m2")
}
