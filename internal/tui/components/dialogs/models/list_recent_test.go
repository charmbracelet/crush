package models

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/log"
	"github.com/charmbracelet/crush/internal/tui/exp/list"
	"github.com/stretchr/testify/require"
)

// execCmdML runs a tea.Cmd through the ModelListComponent's Update loop.
func execCmdML(t *testing.T, m *ModelListComponent, cmd tea.Cmd) {
	t.Helper()
	for cmd != nil {
		msg := cmd()
		var next tea.Cmd
		_, next = m.Update(msg)
		cmd = next
	}
}

// readConfigJSON reads and unmarshals the JSON config file at path.
func readConfigJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	baseDir := filepath.Dir(path)
	fileName := filepath.Base(path)
	b, err := fs.ReadFile(os.DirFS(baseDir), fileName)
	require.NoError(t, err)
	var out map[string]any
	require.NoError(t, json.Unmarshal(b, &out))
	return out
}

// readRecentModels reads the recent_models section from the config file.
func readRecentModels(t *testing.T, path string) map[string]any {
	t.Helper()
	out := readConfigJSON(t, path)
	rm, ok := out["recent_models"].(map[string]any)
	require.True(t, ok)
	return rm
}

func TestModelList_RecentlyUsedSectionAndPrunesInvalid(t *testing.T) {
	// Pre-initialize logger to os.DevNull to prevent file lock on Windows.
	log.Setup(os.DevNull, false)

	// Isolate config/data paths
	cfgDir := t.TempDir()
	dataDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfgDir)
	t.Setenv("XDG_DATA_HOME", dataDir)

	// Pre-seed config so provider auto-update is disabled and we have recents
	confPath := filepath.Join(cfgDir, "crush", "crush.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(confPath), 0o755))
	initial := map[string]any{
		"options": map[string]any{
			"disable_provider_auto_update": true,
		},
		"models": map[string]any{
			"large": map[string]any{
				"model":    "m1",
				"provider": "p1",
			},
		},
		"recent_models": map[string]any{
			"large": []any{
				map[string]any{"model": "m2", "provider": "p1"},              // valid
				map[string]any{"model": "x", "provider": "unknown-provider"}, // invalid -> pruned
			},
		},
	}
	bts, err := json.Marshal(initial)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(confPath, bts, 0o644))

	// Also create empty providers.json to prevent loading real providers
	dataConfDir := filepath.Join(dataDir, "crush")
	require.NoError(t, os.MkdirAll(dataConfDir, 0o755))
	emptyProviders := []byte("[]")
	require.NoError(t, os.WriteFile(filepath.Join(dataConfDir, "providers.json"), emptyProviders, 0o644))

	// Initialize global config instance (no network due to auto-update disabled)
	_, err = config.Init(cfgDir, dataDir, false)
	require.NoError(t, err)

	// Build a small provider set for the list component
	provider := catwalk.Provider{
		ID:   catwalk.InferenceProvider("p1"),
		Name: "Provider One",
		Models: []catwalk.Model{
			{ID: "m1", Name: "Model One", DefaultMaxTokens: 100},
			{ID: "m2", Name: "Model Two", DefaultMaxTokens: 100}, // recent
		},
	}

	// Create and initialize the component with our provider set
	listKeyMap := list.DefaultKeyMap()
	cmp := NewModelListComponent(listKeyMap, "Find your fave", false)
	cmp.providers = []catwalk.Provider{provider}
	execCmdML(t, cmp, cmp.Init())

	// Find all recent items (IDs prefixed with "recent::") and verify pruning
	groups := cmp.list.Groups()
	require.NotEmpty(t, groups)
	var recentItems []list.CompletionItem[ModelOption]
	for _, g := range groups {
		for _, it := range g.Items {
			if strings.HasPrefix(it.ID(), "recent::") {
				recentItems = append(recentItems, it)
			}
		}
	}
	require.NotEmpty(t, recentItems, "no recent items found")
	// Ensure the valid recent (p1:m2) is present and the invalid one is not
	foundValid := false
	for _, it := range recentItems {
		if it.ID() == "recent::p1:m2" {
			foundValid = true
		}
		require.NotEqual(t, "recent::unknown-provider:x", it.ID(), "invalid recent should be pruned")
	}
	require.True(t, foundValid, "expected valid recent not found")

	// Verify original config in cfgDir remains unchanged
	origConfPath := filepath.Join(cfgDir, "crush", "crush.json")
	afterOrig, err := fs.ReadFile(os.DirFS(filepath.Dir(origConfPath)), filepath.Base(origConfPath))
	require.NoError(t, err)
	var origParsed map[string]any
	require.NoError(t, json.Unmarshal(afterOrig, &origParsed))
	origRM := origParsed["recent_models"].(map[string]any)
	origLarge := origRM["large"].([]any)
	require.Len(t, origLarge, 2, "original config should be unchanged")

	// Config should be rewritten with pruned recents in dataDir
	dataConf := filepath.Join(dataDir, "crush", "crush.json")
	rm := readRecentModels(t, dataConf)
	largeAny, ok := rm["large"].([]any)
	require.True(t, ok)
	// Ensure that only valid recent(s) remain and the invalid one is removed
	found := false
	for _, v := range largeAny {
		m := v.(map[string]any)
		require.NotEqual(t, "unknown-provider", m["provider"], "invalid provider should be pruned")
		if m["provider"] == "p1" && m["model"] == "m2" {
			found = true
		}
	}
	require.True(t, found, "persisted recents should include p1:m2")
}

func TestModelList_PrunesInvalidModelWithinValidProvider(t *testing.T) {
	// Pre-initialize logger to os.DevNull to prevent file lock on Windows.
	log.Setup(os.DevNull, false)

	// Isolate config/data paths
	cfgDir := t.TempDir()
	dataDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfgDir)
	t.Setenv("XDG_DATA_HOME", dataDir)

	// Pre-seed config with valid provider but one invalid model
	confPath := filepath.Join(cfgDir, "crush", "crush.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(confPath), 0o755))
	initial := map[string]any{
		"options": map[string]any{
			"disable_provider_auto_update": true,
		},
		"models": map[string]any{
			"large": map[string]any{
				"model":    "m1",
				"provider": "p1",
			},
		},
		"recent_models": map[string]any{
			"large": []any{
				map[string]any{"model": "m1", "provider": "p1"},      // valid
				map[string]any{"model": "missing", "provider": "p1"}, // invalid model
			},
		},
	}
	bts, err := json.Marshal(initial)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(confPath, bts, 0o644))

	// Create empty providers.json
	dataConfDir := filepath.Join(dataDir, "crush")
	require.NoError(t, os.MkdirAll(dataConfDir, 0o755))
	emptyProviders := []byte("[]")
	require.NoError(t, os.WriteFile(filepath.Join(dataConfDir, "providers.json"), emptyProviders, 0o644))

	// Initialize global config instance
	_, err = config.Init(cfgDir, dataDir, false)
	require.NoError(t, err)

	// Build provider set that only includes m1, not "missing"
	provider := catwalk.Provider{
		ID:   catwalk.InferenceProvider("p1"),
		Name: "Provider One",
		Models: []catwalk.Model{
			{ID: "m1", Name: "Model One", DefaultMaxTokens: 100},
		},
	}

	// Create and initialize component
	listKeyMap := list.DefaultKeyMap()
	cmp := NewModelListComponent(listKeyMap, "Find your fave", false)
	cmp.providers = []catwalk.Provider{provider}
	execCmdML(t, cmp, cmp.Init())

	// Find all recent items
	groups := cmp.list.Groups()
	require.NotEmpty(t, groups)
	var recentItems []list.CompletionItem[ModelOption]
	for _, g := range groups {
		for _, it := range g.Items {
			if strings.HasPrefix(it.ID(), "recent::") {
				recentItems = append(recentItems, it)
			}
		}
	}
	require.NotEmpty(t, recentItems, "valid recent should exist")

	// Verify the valid recent is present and invalid model is not
	foundValid := false
	for _, it := range recentItems {
		if it.ID() == "recent::p1:m1" {
			foundValid = true
		}
		require.NotEqual(t, "recent::p1:missing", it.ID(), "invalid model should be pruned")
	}
	require.True(t, foundValid, "valid recent p1:m1 should be present")

	// Verify original config in cfgDir remains unchanged
	origConfPath := filepath.Join(cfgDir, "crush", "crush.json")
	afterOrig, err := fs.ReadFile(os.DirFS(filepath.Dir(origConfPath)), filepath.Base(origConfPath))
	require.NoError(t, err)
	var origParsed map[string]any
	require.NoError(t, json.Unmarshal(afterOrig, &origParsed))
	origRM := origParsed["recent_models"].(map[string]any)
	origLarge := origRM["large"].([]any)
	require.Len(t, origLarge, 2, "original config should be unchanged")

	// Config should be rewritten with pruned recents in dataDir
	dataConf := filepath.Join(dataDir, "crush", "crush.json")
	rm := readRecentModels(t, dataConf)
	largeAny, ok := rm["large"].([]any)
	require.True(t, ok)
	require.Len(t, largeAny, 1, "should only have one valid model")
	// Verify only p1:m1 remains
	m := largeAny[0].(map[string]any)
	require.Equal(t, "p1", m["provider"])
	require.Equal(t, "m1", m["model"])
}

func TestModelKey_EmptyInputs(t *testing.T) {
	// Empty provider
	require.Equal(t, "", modelKey("", "model"))
	// Empty model
	require.Equal(t, "", modelKey("provider", ""))
	// Both empty
	require.Equal(t, "", modelKey("", ""))
	// Valid inputs
	require.Equal(t, "p:m", modelKey("p", "m"))
}

func TestModelList_AllRecentsInvalid(t *testing.T) {
	// Pre-initialize logger to os.DevNull to prevent file lock on Windows.
	log.Setup(os.DevNull, false)

	// Isolate config/data paths
	cfgDir := t.TempDir()
	dataDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfgDir)
	t.Setenv("XDG_DATA_HOME", dataDir)

	// Pre-seed config with only invalid recents
	confPath := filepath.Join(cfgDir, "crush", "crush.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(confPath), 0o755))
	initial := map[string]any{
		"options": map[string]any{
			"disable_provider_auto_update": true,
		},
		"models": map[string]any{
			"large": map[string]any{
				"model":    "m1",
				"provider": "p1",
			},
		},
		"recent_models": map[string]any{
			"large": []any{
				map[string]any{"model": "x", "provider": "unknown1"},
				map[string]any{"model": "y", "provider": "unknown2"},
			},
		},
	}
	bts, err := json.Marshal(initial)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(confPath, bts, 0o644))

	// Also create empty providers.json and data config
	dataConfDir := filepath.Join(dataDir, "crush")
	require.NoError(t, os.MkdirAll(dataConfDir, 0o755))
	emptyProviders := []byte("[]")
	require.NoError(t, os.WriteFile(filepath.Join(dataConfDir, "providers.json"), emptyProviders, 0o644))

	// Initialize global config instance with isolated dataDir
	_, err = config.Init(cfgDir, dataDir, false)
	require.NoError(t, err)

	// Build provider set (doesn't include unknown1 or unknown2)
	provider := catwalk.Provider{
		ID:   catwalk.InferenceProvider("p1"),
		Name: "Provider One",
		Models: []catwalk.Model{
			{ID: "m1", Name: "Model One", DefaultMaxTokens: 100},
		},
	}

	// Create and initialize component
	listKeyMap := list.DefaultKeyMap()
	cmp := NewModelListComponent(listKeyMap, "Find your fave", false)
	cmp.providers = []catwalk.Provider{provider}
	execCmdML(t, cmp, cmp.Init())

	// Verify no recent items exist in UI
	groups := cmp.list.Groups()
	require.NotEmpty(t, groups)
	var recentItems []list.CompletionItem[ModelOption]
	for _, g := range groups {
		for _, it := range g.Items {
			if strings.HasPrefix(it.ID(), "recent::") {
				recentItems = append(recentItems, it)
			}
		}
	}
	require.Empty(t, recentItems, "all invalid recents should be pruned, resulting in no recent section")

	// Verify original config in cfgDir remains unchanged
	origConfPath := filepath.Join(cfgDir, "crush", "crush.json")
	afterOrig, err := fs.ReadFile(os.DirFS(filepath.Dir(origConfPath)), filepath.Base(origConfPath))
	require.NoError(t, err)
	var origParsed map[string]any
	require.NoError(t, json.Unmarshal(afterOrig, &origParsed))
	origRM := origParsed["recent_models"].(map[string]any)
	origLarge := origRM["large"].([]any)
	require.Len(t, origLarge, 2, "original config should be unchanged")

	// Config should be rewritten with empty recents in dataDir
	dataConf := filepath.Join(dataDir, "crush", "crush.json")
	rm := readRecentModels(t, dataConf)
	// When all recents are pruned, the value may be nil or an empty array
	largeVal := rm["large"]
	if largeVal == nil {
		// nil is acceptable - means empty
		return
	}
	largeAny, ok := largeVal.([]any)
	require.True(t, ok, "large key should be nil or array")
	require.Empty(t, largeAny, "persisted recents should be empty after pruning all invalid entries")
}
