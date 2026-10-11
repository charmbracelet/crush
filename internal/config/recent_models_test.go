package config

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

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

// testStoreWithPath creates a ConfigStore backed by a Config for recent model tests.
func testStoreWithPath(cfg *Config, dir string) *ConfigStore {
	return &ConfigStore{
		config:         cfg,
		globalDataPath: filepath.Join(dir, "config.json"),
	}
}

// configWithRecents builds a Config seeded with the given recent models for
// the large type, for exercising the pure nextRecentModels helper.
func configWithRecents(recents ...SelectedModel) *Config {
	return &Config{
		RecentModels: map[SelectedModelType][]SelectedModel{
			SelectedModelTypeLarge: recents,
		},
	}
}

func TestNextRecentModels_AddsToFront(t *testing.T) {
	t.Parallel()

	cfg := configWithRecents()
	updated, changed := nextRecentModels(cfg, SelectedModelTypeLarge, SelectedModel{Provider: "openai", Model: "gpt-4o"})
	require.True(t, changed)
	require.Equal(t, []SelectedModel{{Provider: "openai", Model: "gpt-4o"}}, updated)
}

func TestNextRecentModels_DedupeAndMoveToFront(t *testing.T) {
	t.Parallel()

	cfg := configWithRecents(
		SelectedModel{Provider: "anthropic", Model: "claude"},
		SelectedModel{Provider: "openai", Model: "gpt-4o"},
	)
	updated, changed := nextRecentModels(cfg, SelectedModelTypeLarge, SelectedModel{Provider: "openai", Model: "gpt-4o"})
	require.True(t, changed)
	require.Equal(t, []SelectedModel{
		{Provider: "openai", Model: "gpt-4o"},
		{Provider: "anthropic", Model: "claude"},
	}, updated)
}

func TestNextRecentModels_TrimsToMax(t *testing.T) {
	t.Parallel()

	var seed []SelectedModel
	for _, id := range []string{"m5", "m4", "m3", "m2", "m1"} {
		seed = append(seed, SelectedModel{Provider: "p", Model: id})
	}
	cfg := configWithRecents(seed...)

	updated, changed := nextRecentModels(cfg, SelectedModelTypeLarge, SelectedModel{Provider: "p", Model: "m6"})
	require.True(t, changed)
	require.Len(t, updated, maxRecentModelsPerType)
	require.Equal(t, SelectedModel{Provider: "p", Model: "m6"}, updated[0])
	require.Equal(t, SelectedModel{Provider: "p", Model: "m2"}, updated[maxRecentModelsPerType-1])
}

func TestNextRecentModels_SkipsEmptyValues(t *testing.T) {
	t.Parallel()

	cfg := configWithRecents()
	_, changed := nextRecentModels(cfg, SelectedModelTypeLarge, SelectedModel{Provider: "", Model: "m"})
	require.False(t, changed)
	_, changed = nextRecentModels(cfg, SelectedModelTypeLarge, SelectedModel{Provider: "p", Model: ""})
	require.False(t, changed)
}

func TestNextRecentModels_NoChangeWhenAlreadyFront(t *testing.T) {
	t.Parallel()

	entry := SelectedModel{Provider: "openai", Model: "gpt-4o"}
	cfg := configWithRecents(entry)
	_, changed := nextRecentModels(cfg, SelectedModelTypeLarge, entry)
	require.False(t, changed)
}

func TestUpdatePreferredModel_PersistsModelAndRecents(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfg := &Config{}
	cfg.setDefaults(dir, "")
	store := testStoreWithPath(cfg, dir)

	sel := SelectedModel{Provider: "openai", Model: "gpt-4o"}
	require.NoError(t, store.UpdatePreferredModel(ScopeGlobal, SelectedModelTypeLarge, sel))

	// in-memory state (read through the store; copy-on-write publishes a
	// new Config, so the seed cfg pointer is intentionally unchanged).
	require.Equal(t, sel, store.Config().Models[SelectedModelTypeLarge])
	require.Len(t, store.Config().RecentModels[SelectedModelTypeLarge], 1)

	// persisted state
	rm := readRecentModels(t, store.globalDataPath)
	large, ok := rm[string(SelectedModelTypeLarge)].([]any)
	require.True(t, ok)
	require.Len(t, large, 1)
	item := large[0].(map[string]any)
	require.Equal(t, "openai", item["provider"])
	require.Equal(t, "gpt-4o", item["model"])
}

func TestUpdatePreferredModel_TypeIsolation(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfg := &Config{}
	cfg.setDefaults(dir, "")
	store := testStoreWithPath(cfg, dir)

	largeModel := SelectedModel{Provider: "openai", Model: "gpt-4o"}
	smallModel := SelectedModel{Provider: "anthropic", Model: "claude"}
	require.NoError(t, store.UpdatePreferredModel(ScopeGlobal, SelectedModelTypeLarge, largeModel))
	require.NoError(t, store.UpdatePreferredModel(ScopeGlobal, SelectedModelTypeSmall, smallModel))

	// Adding to large leaves small untouched.
	anotherLarge := SelectedModel{Provider: "google", Model: "gemini"}
	require.NoError(t, store.UpdatePreferredModel(ScopeGlobal, SelectedModelTypeLarge, anotherLarge))

	require.Len(t, store.Config().RecentModels[SelectedModelTypeLarge], 2)
	require.Equal(t, anotherLarge, store.Config().RecentModels[SelectedModelTypeLarge][0])
	require.Len(t, store.Config().RecentModels[SelectedModelTypeSmall], 1)
	require.Equal(t, smallModel, store.Config().RecentModels[SelectedModelTypeSmall][0])
}

func TestUpdatePreferredModel_AutoModePersistsClassifierAndRecents(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfg := &Config{}
	cfg.setDefaults(dir, "")
	store := testStoreWithPath(cfg, dir)

	sel := SelectedModel{Provider: "local-llama", Model: "gemma4-e4b"}
	require.NoError(t, store.UpdatePreferredModel(ScopeGlobal, SelectedModelTypeAutoMode, sel))

	// in-memory state: the classifier is set, the models map untouched.
	require.NotNil(t, store.Config().AutoMode)
	require.NotNil(t, store.Config().AutoMode.Classifier)
	require.Equal(t, "local-llama", store.Config().AutoMode.Classifier.Provider)
	require.Equal(t, "gemma4-e4b", store.Config().AutoMode.Classifier.Model)
	require.Empty(t, store.Config().Models)
	require.Len(t, store.Config().RecentModels[SelectedModelTypeAutoMode], 1)
	require.Equal(t, sel, store.Config().RecentModels[SelectedModelTypeAutoMode][0])

	// persisted state
	out := readConfigJSON(t, store.globalDataPath)
	autoMode, ok := out["auto_mode"].(map[string]any)
	require.True(t, ok)
	classifier, ok := autoMode["classifier"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "local-llama", classifier["provider"])
	require.Equal(t, "gemma4-e4b", classifier["model"])

	rm := readRecentModels(t, store.globalDataPath)
	autoRecents, ok := rm[string(SelectedModelTypeAutoMode)].([]any)
	require.True(t, ok)
	require.Len(t, autoRecents, 1)
	item := autoRecents[0].(map[string]any)
	require.Equal(t, "local-llama", item["provider"])
	require.Equal(t, "gemma4-e4b", item["model"])

	// Selecting another classifier moves it to the front of the recents.
	another := SelectedModel{Provider: "openai", Model: "gpt-4o"}
	require.NoError(t, store.UpdatePreferredModel(ScopeGlobal, SelectedModelTypeAutoMode, another))
	require.Len(t, store.Config().RecentModels[SelectedModelTypeAutoMode], 2)
	require.Equal(t, another, store.Config().RecentModels[SelectedModelTypeAutoMode][0])
	require.Equal(t, sel, store.Config().RecentModels[SelectedModelTypeAutoMode][1])
	require.Equal(t, "openai", store.Config().AutoMode.Classifier.Provider)
}

func TestUpdatePreferredModel_AutoModePreservesOtherFields(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfg := &Config{
		AutoMode: &AutoModeConfig{
			Enabled:    true,
			MaxTokens:  2048,
			Classifier: &AutoModeClassifier{Provider: "old", Model: "old-model"},
		},
	}
	cfg.setDefaults(dir, "")
	store := testStoreWithPath(cfg, dir)

	sel := SelectedModel{Provider: "local-llama", Model: "gemma4-e4b"}
	require.NoError(t, store.UpdatePreferredModel(ScopeGlobal, SelectedModelTypeAutoMode, sel))

	am := store.Config().AutoMode
	require.True(t, am.Enabled)
	require.Equal(t, 2048, am.MaxTokens)
	require.Equal(t, "local-llama", am.Classifier.Provider)
	require.Equal(t, "gemma4-e4b", am.Classifier.Model)
}
