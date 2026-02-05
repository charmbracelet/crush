package config

import (
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
		_, err := loadFromConfigPaths(configPaths)
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
		_, err := loadFromConfigPaths(configPaths)
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
		_, err := loadFromConfigPaths(configPaths)
		if err != nil {
			b.Fatal(err)
		}
	}
}
