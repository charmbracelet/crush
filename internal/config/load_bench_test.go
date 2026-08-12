package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func BenchmarkLoadFromConfigPaths(b *testing.B) {
	// Create temp config files with realistic content.
	tmpDir := b.TempDir()

	globalConfig := filepath.Join(tmpDir, "global.json")
	localConfig := filepath.Join(tmpDir, "local.json")

	globalContent := []byte(`{
		"providers": {
			"openai": {
				"api_key": "$OPENAI_API_KEY",
				"base_url": "https://api.openai.com/v1"
			},
			"anthropic": {
				"api_key": "$ANTHROPIC_API_KEY",
				"base_url": "https://api.anthropic.com"
			}
		},
		"options": {
			"tui": {
				"theme": "dark"
			}
		}
	}`)

	localContent := []byte(`{
		"providers": {
			"openai": {
				"api_key": "sk-override-key"
			}
		},
		"options": {
			"context_paths": ["README.md", "AGENTS.md"]
		}
	}`)

	if err := os.WriteFile(globalConfig, globalContent, 0o644); err != nil {
		b.Fatal(err)
	}
	if err := os.WriteFile(localConfig, localContent, 0o644); err != nil {
		b.Fatal(err)
	}

	configPaths := []string{globalConfig, localConfig}

	b.ReportAllocs()
	for b.Loop() {
		_, _, err := loadFromConfigPaths(context.Background(), configPaths)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkLoadFromConfigPaths_MissingFiles(b *testing.B) {
	// Test with mix of existing and non-existing paths.
	tmpDir := b.TempDir()

	existingConfig := filepath.Join(tmpDir, "exists.json")
	content := []byte(`{"options": {"tui": {"theme": "dark"}}}`)
	if err := os.WriteFile(existingConfig, content, 0o644); err != nil {
		b.Fatal(err)
	}

	configPaths := []string{
		filepath.Join(tmpDir, "nonexistent1.json"),
		existingConfig,
		filepath.Join(tmpDir, "nonexistent2.json"),
	}

	b.ReportAllocs()
	for b.Loop() {
		_, _, err := loadFromConfigPaths(context.Background(), configPaths)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkLoadFromConfigPaths_Empty(b *testing.B) {
	// Test with no config files.
	tmpDir := b.TempDir()
	configPaths := []string{
		filepath.Join(tmpDir, "nonexistent1.json"),
		filepath.Join(tmpDir, "nonexistent2.json"),
	}

	b.ReportAllocs()
	for b.Loop() {
		_, _, err := loadFromConfigPaths(context.Background(), configPaths)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkLoadFromConfigPaths_ShellConfig measures the crushrc execution
// path (shell interpreter + config builtins + JSON marshal), which now sits on
// the startup and reload critical path alongside JSON parsing. Keeps a
// regression in shell config loading from going unnoticed.
func BenchmarkLoadFromConfigPaths_ShellConfig(b *testing.B) {
	tmpDir := b.TempDir()
	rcPath := filepath.Join(tmpDir, "crushrc")

	rcContent := []byte(`provider add openai --api-key "$OPENAI_API_KEY" --base-url "https://api.openai.com/v1"
provider add anthropic --api-key "$ANTHROPIC_API_KEY"
model large openai/gpt-4o --think
permissions allow bash view
option data-directory .crush
option metrics false`)

	if err := os.WriteFile(rcPath, rcContent, 0o644); err != nil {
		b.Fatal(err)
	}
	configPaths := []string{rcPath}

	b.ReportAllocs()
	for b.Loop() {
		_, _, err := loadFromConfigPaths(context.Background(), configPaths)
		if err != nil {
			b.Fatal(err)
		}
	}
}
