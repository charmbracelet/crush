package lsp

import (
	"os"
	"path/filepath"
	"testing"

	powernapconfig "github.com/charmbracelet/x/powernap/pkg/config"
	"github.com/stretchr/testify/require"
)

func TestFilterMatching(t *testing.T) {
	t.Parallel()

	t.Run("matches servers with existing root markers", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()

		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module test"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "Cargo.toml"), []byte("[package]"), 0o644))

		servers := map[string]*powernapconfig.ServerConfig{
			"gopls":          {RootMarkers: []string{"go.mod", "go.work"}},
			"rust-analyzer":  {RootMarkers: []string{"Cargo.toml"}},
			"typescript-lsp": {RootMarkers: []string{"package.json", "tsconfig.json"}},
		}

		result := FilterMatching(tmpDir, servers)

		require.Contains(t, result, "gopls")
		require.Contains(t, result, "rust-analyzer")
		require.NotContains(t, result, "typescript-lsp")
	})

	t.Run("returns empty for empty servers", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()

		result := FilterMatching(tmpDir, map[string]*powernapconfig.ServerConfig{})

		require.Empty(t, result)
	})

	t.Run("returns empty when no markers match", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()

		servers := map[string]*powernapconfig.ServerConfig{
			"gopls":  {RootMarkers: []string{"go.mod"}},
			"python": {RootMarkers: []string{"pyproject.toml"}},
		}

		result := FilterMatching(tmpDir, servers)

		require.Empty(t, result)
	})

	t.Run("glob patterns work", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()

		require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "src"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "src", "main.go"), []byte("package main"), 0o644))

		servers := map[string]*powernapconfig.ServerConfig{
			"gopls":  {RootMarkers: []string{"**/*.go"}},
			"python": {RootMarkers: []string{"**/*.py"}},
		}

		result := FilterMatching(tmpDir, servers)

		require.Contains(t, result, "gopls")
		require.NotContains(t, result, "python")
	})

	t.Run("servers with empty root markers are not included", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()

		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module test"), 0o644))

		servers := map[string]*powernapconfig.ServerConfig{
			"gopls":   {RootMarkers: []string{"go.mod"}},
			"generic": {RootMarkers: []string{}},
		}

		result := FilterMatching(tmpDir, servers)

		require.Contains(t, result, "gopls")
		require.NotContains(t, result, "generic")
	})

	t.Run("stops early when all servers match", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()

		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module test"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "Cargo.toml"), []byte("[package]"), 0o644))

		servers := map[string]*powernapconfig.ServerConfig{
			"gopls":         {RootMarkers: []string{"go.mod"}},
			"rust-analyzer": {RootMarkers: []string{"Cargo.toml"}},
		}

		result := FilterMatching(tmpDir, servers)

		require.Len(t, result, 2)
		require.Contains(t, result, "gopls")
		require.Contains(t, result, "rust-analyzer")
	})
}
