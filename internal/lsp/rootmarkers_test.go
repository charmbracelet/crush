package lsp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHasRootMarkers(t *testing.T) {
	t.Parallel()

	// Create a temporary directory for testing
	tmpDir := t.TempDir()

	// Test with empty root markers (should return true)
	require.True(t, HasRootMarkers(tmpDir, []string{}))

	// Test with non-existent markers
	require.False(t, HasRootMarkers(tmpDir, []string{"go.mod", "package.json"}))

	// Create a go.mod file
	goModPath := filepath.Join(tmpDir, "go.mod")
	err := os.WriteFile(goModPath, []byte("module test"), 0o644)
	require.NoError(t, err)

	// Test with existing marker
	require.True(t, HasRootMarkers(tmpDir, []string{"go.mod", "package.json"}))

	// Test with only non-existent markers
	require.False(t, HasRootMarkers(tmpDir, []string{"package.json", "Cargo.toml"}))

	// Test with glob patterns
	require.True(t, HasRootMarkers(tmpDir, []string{"*.mod"}))
	require.False(t, HasRootMarkers(tmpDir, []string{"*.json"}))
}
