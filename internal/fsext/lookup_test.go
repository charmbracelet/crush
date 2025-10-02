package fsext

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/crush/internal/home"
	"github.com/stretchr/testify/require"
)

func TestLookupClosest(t *testing.T) {
	tempDir := t.TempDir()
	t.Chdir(tempDir)

	t.Run("target found in starting directory", func(t *testing.T) {
		testDir := t.TempDir()

		// Create target file in current directory
		targetFile := filepath.Join(testDir, "target.txt")
		err := os.WriteFile(targetFile, []byte("test"), 0o644)
		require.NoError(t, err)

		foundPath, found := LookupClosest(testDir, "target.txt")
		require.True(t, found)
		require.Equal(t, targetFile, foundPath)
	})

	t.Run("target found in parent directory", func(t *testing.T) {
		testDir := t.TempDir()

		// Create subdirectory
		subDir := filepath.Join(testDir, "subdir")
		err := os.Mkdir(subDir, 0o755)
		require.NoError(t, err)

		// Create target file in parent directory
		targetFile := filepath.Join(testDir, "target.txt")
		err = os.WriteFile(targetFile, []byte("test"), 0o644)
		require.NoError(t, err)

		foundPath, found := LookupClosest(subDir, "target.txt")
		require.True(t, found)
		require.Equal(t, targetFile, foundPath)
	})

	t.Run("target found in grandparent directory", func(t *testing.T) {
		testDir := t.TempDir()

		// Create nested subdirectories
		subDir := filepath.Join(testDir, "subdir")
		err := os.Mkdir(subDir, 0o755)
		require.NoError(t, err)

		subSubDir := filepath.Join(subDir, "subsubdir")
		err = os.Mkdir(subSubDir, 0o755)
		require.NoError(t, err)

		// Create target file in grandparent directory
		targetFile := filepath.Join(testDir, "target.txt")
		err = os.WriteFile(targetFile, []byte("test"), 0o644)
		require.NoError(t, err)

		foundPath, found := LookupClosest(subSubDir, "target.txt")
		require.True(t, found)
		require.Equal(t, targetFile, foundPath)
	})

	t.Run("target not found", func(t *testing.T) {
		testDir := t.TempDir()

		foundPath, found := LookupClosest(testDir, "nonexistent.txt")
		require.False(t, found)
		require.Empty(t, foundPath)
	})

	t.Run("target directory found", func(t *testing.T) {
		testDir := t.TempDir()

		// Create target directory in current directory
		targetDir := filepath.Join(testDir, "targetdir")
		err := os.Mkdir(targetDir, 0o755)
		require.NoError(t, err)

		foundPath, found := LookupClosest(testDir, "targetdir")
		require.True(t, found)
		require.Equal(t, targetDir, foundPath)
	})

	t.Run("stops at home directory", func(t *testing.T) {
		// This test is limited as we can't easily create files above home directory
		// but we can test the behavior by searching from home directory itself
		homeDir := home.Dir()

		// Search for a file that doesn't exist from home directory
		foundPath, found := LookupClosest(homeDir, "nonexistent_file_12345.txt")
		require.False(t, found)
		require.Empty(t, foundPath)
	})

	t.Run("invalid starting directory", func(t *testing.T) {
		foundPath, found := LookupClosest("/invalid/path/that/does/not/exist", "target.txt")
		require.False(t, found)
		require.Empty(t, foundPath)
	})

	t.Run("relative path handling", func(t *testing.T) {
		// Create target file in current directory
		require.NoError(t, os.WriteFile("target.txt", []byte("test"), 0o644))

		// Search using relative path
		foundPath, found := LookupClosest(".", "target.txt")
		require.True(t, found)

		// Resolve symlinks to handle macOS /private/var vs /var discrepancy
		expectedPath, err := filepath.EvalSymlinks(filepath.Join(tempDir, "target.txt"))
		require.NoError(t, err)
		actualPath, err := filepath.EvalSymlinks(foundPath)
		require.NoError(t, err)
		require.Equal(t, expectedPath, actualPath)
	})
}

func TestLookupClosestWithOwnership(t *testing.T) {
	// Note: Testing ownership boundaries is difficult in a cross-platform way
	// without creating complex directory structures with different owners.
	// This test focuses on the basic functionality when ownership checks pass.

	tempDir := t.TempDir()
	t.Chdir(tempDir)

	t.Run("search respects same ownership", func(t *testing.T) {
		testDir := t.TempDir()

		// Create subdirectory structure
		subDir := filepath.Join(testDir, "subdir")
		err := os.Mkdir(subDir, 0o755)
		require.NoError(t, err)

		// Create target file in parent directory
		targetFile := filepath.Join(testDir, "target.txt")
		err = os.WriteFile(targetFile, []byte("test"), 0o644)
		require.NoError(t, err)

		// Search should find the target assuming same ownership
		foundPath, found := LookupClosest(subDir, "target.txt")
		require.True(t, found)
		require.Equal(t, targetFile, foundPath)
	})
}

func TestLookup(t *testing.T) {
	tempDir := t.TempDir()
	t.Chdir(tempDir)

	t.Run("no targets returns empty slice", func(t *testing.T) {
		testDir := t.TempDir()

		found, err := Lookup(testDir)
		require.NoError(t, err)
		require.Empty(t, found)
	})

	t.Run("single target found in starting directory", func(t *testing.T) {
		testDir := t.TempDir()

		// Create target file in current directory
		targetFile := filepath.Join(testDir, "target.txt")
		err := os.WriteFile(targetFile, []byte("test"), 0o644)
		require.NoError(t, err)

		found, err := Lookup(testDir, "target.txt")
		require.NoError(t, err)
		require.Len(t, found, 1)
		require.Equal(t, targetFile, found[0])
	})

	t.Run("multiple targets found in starting directory", func(t *testing.T) {
		testDir := t.TempDir()

		// Create multiple target files in current directory
		targetFile1 := filepath.Join(testDir, "target1.txt")
		targetFile2 := filepath.Join(testDir, "target2.txt")
		targetFile3 := filepath.Join(testDir, "target3.txt")

		err := os.WriteFile(targetFile1, []byte("test1"), 0o644)
		require.NoError(t, err)
		err = os.WriteFile(targetFile2, []byte("test2"), 0o644)
		require.NoError(t, err)
		err = os.WriteFile(targetFile3, []byte("test3"), 0o644)
		require.NoError(t, err)

		found, err := Lookup(testDir, "target1.txt", "target2.txt", "target3.txt")
		require.NoError(t, err)
		require.Len(t, found, 3)
		require.Contains(t, found, targetFile1)
		require.Contains(t, found, targetFile2)
		require.Contains(t, found, targetFile3)
	})

	t.Run("targets found in parent directories", func(t *testing.T) {
		testDir := t.TempDir()

		// Create subdirectory
		subDir := filepath.Join(testDir, "subdir")
		err := os.Mkdir(subDir, 0o755)
		require.NoError(t, err)

		// Create target files in parent directory
		targetFile1 := filepath.Join(testDir, "target1.txt")
		targetFile2 := filepath.Join(testDir, "target2.txt")
		err = os.WriteFile(targetFile1, []byte("test1"), 0o644)
		require.NoError(t, err)
		err = os.WriteFile(targetFile2, []byte("test2"), 0o644)
		require.NoError(t, err)

		found, err := Lookup(subDir, "target1.txt", "target2.txt")
		require.NoError(t, err)
		require.Len(t, found, 2)
		require.Contains(t, found, targetFile1)
		require.Contains(t, found, targetFile2)
	})

	t.Run("targets found across multiple directory levels", func(t *testing.T) {
		testDir := t.TempDir()

		// Create nested subdirectories
		subDir := filepath.Join(testDir, "subdir")
		err := os.Mkdir(subDir, 0o755)
		require.NoError(t, err)

		subSubDir := filepath.Join(subDir, "subsubdir")
		err = os.Mkdir(subSubDir, 0o755)
		require.NoError(t, err)

		// Create target files at different levels
		targetFile1 := filepath.Join(testDir, "target1.txt")
		targetFile2 := filepath.Join(subDir, "target2.txt")
		targetFile3 := filepath.Join(subSubDir, "target3.txt")

		err = os.WriteFile(targetFile1, []byte("test1"), 0o644)
		require.NoError(t, err)
		err = os.WriteFile(targetFile2, []byte("test2"), 0o644)
		require.NoError(t, err)
		err = os.WriteFile(targetFile3, []byte("test3"), 0o644)
		require.NoError(t, err)

		found, err := Lookup(subSubDir, "target1.txt", "target2.txt", "target3.txt")
		require.NoError(t, err)
		require.Len(t, found, 3)
		require.Contains(t, found, targetFile1)
		require.Contains(t, found, targetFile2)
		require.Contains(t, found, targetFile3)
	})

	t.Run("some targets not found", func(t *testing.T) {
		testDir := t.TempDir()

		// Create only some target files
		targetFile1 := filepath.Join(testDir, "target1.txt")
		targetFile2 := filepath.Join(testDir, "target2.txt")

		err := os.WriteFile(targetFile1, []byte("test1"), 0o644)
		require.NoError(t, err)
		err = os.WriteFile(targetFile2, []byte("test2"), 0o644)
		require.NoError(t, err)

		// Search for existing and non-existing targets
		found, err := Lookup(testDir, "target1.txt", "nonexistent.txt", "target2.txt", "another_nonexistent.txt")
		require.NoError(t, err)
		require.Len(t, found, 2)
		require.Contains(t, found, targetFile1)
		require.Contains(t, found, targetFile2)
	})

	t.Run("no targets found", func(t *testing.T) {
		testDir := t.TempDir()

		found, err := Lookup(testDir, "nonexistent1.txt", "nonexistent2.txt", "nonexistent3.txt")
		require.NoError(t, err)
		require.Empty(t, found)
	})

	t.Run("target directories found", func(t *testing.T) {
		testDir := t.TempDir()

		// Create target directories
		targetDir1 := filepath.Join(testDir, "targetdir1")
		targetDir2 := filepath.Join(testDir, "targetdir2")
		err := os.Mkdir(targetDir1, 0o755)
		require.NoError(t, err)
		err = os.Mkdir(targetDir2, 0o755)
		require.NoError(t, err)

		found, err := Lookup(testDir, "targetdir1", "targetdir2")
		require.NoError(t, err)
		require.Len(t, found, 2)
		require.Contains(t, found, targetDir1)
		require.Contains(t, found, targetDir2)
	})

	t.Run("mixed files and directories", func(t *testing.T) {
		testDir := t.TempDir()

		// Create target files and directories
		targetFile := filepath.Join(testDir, "target.txt")
		targetDir := filepath.Join(testDir, "targetdir")
		err := os.WriteFile(targetFile, []byte("test"), 0o644)
		require.NoError(t, err)
		err = os.Mkdir(targetDir, 0o755)
		require.NoError(t, err)

		found, err := Lookup(testDir, "target.txt", "targetdir")
		require.NoError(t, err)
		require.Len(t, found, 2)
		require.Contains(t, found, targetFile)
		require.Contains(t, found, targetDir)
	})

	t.Run("invalid starting directory", func(t *testing.T) {
		found, err := Lookup("/invalid/path/that/does/not/exist", "target.txt")
		require.Error(t, err)
		require.Empty(t, found)
	})

	t.Run("relative path handling", func(t *testing.T) {
		// Create target files in current directory
		require.NoError(t, os.WriteFile("target1.txt", []byte("test1"), 0o644))
		require.NoError(t, os.WriteFile("target2.txt", []byte("test2"), 0o644))

		// Search using relative path
		found, err := Lookup(".", "target1.txt", "target2.txt")
		require.NoError(t, err)
		require.Len(t, found, 2)

		// Resolve symlinks to handle macOS /private/var vs /var discrepancy
		expectedPath1, err := filepath.EvalSymlinks(filepath.Join(tempDir, "target1.txt"))
		require.NoError(t, err)
		expectedPath2, err := filepath.EvalSymlinks(filepath.Join(tempDir, "target2.txt"))
		require.NoError(t, err)

		// Check that found paths match expected paths (order may vary)
		foundEvalSymlinks := make([]string, len(found))
		for i, path := range found {
			evalPath, err := filepath.EvalSymlinks(path)
			require.NoError(t, err)
			foundEvalSymlinks[i] = evalPath
		}

		require.Contains(t, foundEvalSymlinks, expectedPath1)
		require.Contains(t, foundEvalSymlinks, expectedPath2)
	})
}

func TestProbeEnt(t *testing.T) {
	t.Run("existing file with correct owner", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create test file
		testFile := filepath.Join(tempDir, "test.txt")
		err := os.WriteFile(testFile, []byte("test"), 0o644)
		require.NoError(t, err)

		// Get owner of temp directory
		owner, err := Owner(tempDir)
		require.NoError(t, err)

		// Test probeEnt with correct owner
		err = probeEnt(testFile, owner)
		require.NoError(t, err)
	})

	t.Run("existing directory with correct owner", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create test directory
		testDir := filepath.Join(tempDir, "testdir")
		err := os.Mkdir(testDir, 0o755)
		require.NoError(t, err)

		// Get owner of temp directory
		owner, err := Owner(tempDir)
		require.NoError(t, err)

		// Test probeEnt with correct owner
		err = probeEnt(testDir, owner)
		require.NoError(t, err)
	})

	t.Run("nonexistent file", func(t *testing.T) {
		tempDir := t.TempDir()

		nonexistentFile := filepath.Join(tempDir, "nonexistent.txt")
		owner, err := Owner(tempDir)
		require.NoError(t, err)

		err = probeEnt(nonexistentFile, owner)
		require.Error(t, err)
		require.True(t, errors.Is(err, os.ErrNotExist))
	})

	t.Run("nonexistent file in nonexistent directory", func(t *testing.T) {
		nonexistentFile := "/this/directory/does/not/exists/nonexistent.txt"

		err := probeEnt(nonexistentFile, -1)
		require.Error(t, err)
		require.True(t, errors.Is(err, os.ErrNotExist))
	})

	t.Run("ownership bypass with -1", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create test file
		testFile := filepath.Join(tempDir, "test.txt")
		err := os.WriteFile(testFile, []byte("test"), 0o644)
		require.NoError(t, err)

		// Test probeEnt with -1 (bypass ownership check)
		err = probeEnt(testFile, -1)
		require.NoError(t, err)
	})

	t.Run("ownership mismatch returns permission error", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create test file
		testFile := filepath.Join(tempDir, "test.txt")
		err := os.WriteFile(testFile, []byte("test"), 0o644)
		require.NoError(t, err)

		// Test probeEnt with different owner (use 9999 which is unlikely to be the actual owner)
		err = probeEnt(testFile, 9999)
		require.Error(t, err)
		require.True(t, errors.Is(err, os.ErrPermission))
	})
}
