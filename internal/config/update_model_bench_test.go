package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// BenchmarkUpdatePreferredModel measures the full write cost of a single
// model selection, which is what the user experiences when picking a model
// in the TUI. It guards against regressing back to a full config reload on
// every selection.
func BenchmarkUpdatePreferredModel(b *testing.B) {
	dir := b.TempDir()
	configPath := filepath.Join(dir, "crush.json")

	b.Setenv("CRUSH_GLOBAL_CONFIG", dir)
	b.Setenv("CRUSH_GLOBAL_DATA", dir)
	resetProviderState()
	b.Cleanup(resetProviderState)

	cfg := `{
		"models": {
			"large": {"provider": "openai", "model": "gpt-4"},
			"small": {"provider": "openai", "model": "gpt-4o-mini"}
		},
		"providers": {
			"openai": {
				"api_key": "test-key",
				"models": [
					{"id": "gpt-4", "name": "GPT-4"},
					{"id": "gpt-4o", "name": "GPT-4o"},
					{"id": "gpt-4o-mini", "name": "GPT-4o mini"}
				]
			},
			"anthropic": {
				"api_key": "test-key-2",
				"models": [
					{"id": "claude-3", "name": "Claude 3"},
					{"id": "claude-3-5", "name": "Claude 3.5"}
				]
			}
		}
	}`
	if err := os.WriteFile(configPath, []byte(cfg), 0o600); err != nil {
		b.Fatal(err)
	}

	store, err := Load(b.Context(), dir, dir, false)
	if err != nil {
		b.Fatal(err)
	}
	store.globalDataPath = configPath

	models := []SelectedModel{
		{Provider: "openai", Model: "gpt-4"},
		{Provider: "anthropic", Model: "claude-3"},
		{Provider: "openai", Model: "gpt-4o"},
		{Provider: "anthropic", Model: "claude-3-5"},
	}

	b.ReportAllocs()
	i := 0
	for b.Loop() {
		m := models[i%len(models)]
		if err := store.UpdatePreferredModel(ScopeGlobal, SelectedModelTypeLarge, m); err != nil {
			b.Fatal(err)
		}
		i++
	}
}

// BenchmarkReloadFromDisk isolates the full reload cost for comparison with
// BenchmarkUpdatePreferredModel.
func BenchmarkReloadFromDisk(b *testing.B) {
	dir := b.TempDir()
	configPath := filepath.Join(dir, "crush.json")

	b.Setenv("CRUSH_GLOBAL_CONFIG", dir)
	b.Setenv("CRUSH_GLOBAL_DATA", dir)
	resetProviderState()
	b.Cleanup(resetProviderState)

	cfg := `{
		"models": {"large": {"provider": "openai", "model": "gpt-4"}},
		"providers": {
			"openai": {"api_key": "test-key", "models": [{"id": "gpt-4", "name": "GPT-4"}]}
		}
	}`
	if err := os.WriteFile(configPath, []byte(cfg), 0o600); err != nil {
		b.Fatal(err)
	}

	store, err := Load(b.Context(), dir, dir, false)
	if err != nil {
		b.Fatal(err)
	}
	store.globalDataPath = configPath

	ctx := context.Background()
	b.ReportAllocs()
	for b.Loop() {
		if err := store.ReloadFromDisk(ctx); err != nil {
			b.Fatal(err)
		}
	}
}
