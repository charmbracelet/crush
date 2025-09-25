package fsext

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGlobWithDoubleStar(t *testing.T) {
	t.Run("finds files matching pattern", func(t *testing.T) {
		testDir := t.TempDir()

		// Create test files
		mainGo := filepath.Join(testDir, "src", "main.go")
		utilsGo := filepath.Join(testDir, "src", "utils.go")
		helperGo := filepath.Join(testDir, "pkg", "helper.go")
		readmeMd := filepath.Join(testDir, "README.md")

		for _, file := range []string{mainGo, utilsGo, helperGo, readmeMd} {
			require.NoError(t, os.MkdirAll(filepath.Dir(file), 0o755))
			require.NoError(t, os.WriteFile(file, []byte("test content"), 0o644))
		}

		// Test finding a specific .go file pattern
		matches, truncated, err := GlobWithDoubleStar("**/main.go", testDir, 0)
		require.NoError(t, err)
		require.False(t, truncated)

		// Should find exactly main.go file
		require.Equal(t, matches, []string{mainGo})
	})

	t.Run("finds directories matching pattern", func(t *testing.T) {
		testDir := t.TempDir()

		// Create test directories and files
		srcDir := filepath.Join(testDir, "src")
		pkgDir := filepath.Join(testDir, "pkg")
		internalDir := filepath.Join(testDir, "internal")
		cmdDir := filepath.Join(testDir, "cmd")
		pkgFile := filepath.Join(testDir, "pkg.txt")

		for _, dir := range []string{srcDir, pkgDir, internalDir, cmdDir} {
			require.NoError(t, os.MkdirAll(dir, 0o755))
		}

		// Create files in directories and a similarly named file
		require.NoError(t, os.WriteFile(filepath.Join(srcDir, "main.go"), []byte("package main"), 0o644))
		require.NoError(t, os.WriteFile(pkgFile, []byte("test"), 0o644))

		// Test finding a specific directory pattern (this tests the fix for the bug)
		// Look specifically for "pkg" directory, not others and not similar files
		matches, truncated, err := GlobWithDoubleStar("pkg", testDir, 0)
		require.NoError(t, err)
		require.False(t, truncated)

		// Should find exactly the pkg directory
		require.Equal(t, matches, []string{pkgDir})
	})

	t.Run("finds nested directories with wildcard patterns", func(t *testing.T) {
		testDir := t.TempDir()

		// Create nested directory structure
		srcPkgDir := filepath.Join(testDir, "src", "pkg")
		libPkgDir := filepath.Join(testDir, "lib", "pkg")
		mainPkgDir := filepath.Join(testDir, "pkg")
		otherDir := filepath.Join(testDir, "other")

		for _, dir := range []string{srcPkgDir, libPkgDir, mainPkgDir, otherDir} {
			require.NoError(t, os.MkdirAll(dir, 0o755))
		}

		// Test **/pkg pattern - should find all pkg directories at any level
		matches, truncated, err := GlobWithDoubleStar("**/pkg", testDir, 0)
		require.NoError(t, err)
		require.False(t, truncated)

		// Convert to relative paths for easier comparison
		var relativeMatches []string
		for _, match := range matches {
			rel, err := filepath.Rel(testDir, match)
			require.NoError(t, err)
			relativeMatches = append(relativeMatches, filepath.ToSlash(rel))
		}

		// Should find all three pkg directories
		require.ElementsMatch(t, relativeMatches, []string{"pkg", "src/pkg", "lib/pkg"})
	})

	t.Run("finds directory contents with recursive patterns", func(t *testing.T) {
		testDir := t.TempDir()

		// Create directory with contents
		pkgDir := filepath.Join(testDir, "pkg")
		pkgFile1 := filepath.Join(pkgDir, "main.go")
		pkgFile2 := filepath.Join(pkgDir, "utils.go")
		pkgSubdir := filepath.Join(pkgDir, "internal")
		pkgSubfile := filepath.Join(pkgSubdir, "helper.go")

		require.NoError(t, os.MkdirAll(pkgSubdir, 0o755))

		for _, file := range []string{pkgFile1, pkgFile2, pkgSubfile} {
			require.NoError(t, os.WriteFile(file, []byte("package main"), 0o644))
		}

		// Test pkg/** pattern - should find directory and all its contents
		matches, truncated, err := GlobWithDoubleStar("pkg/**", testDir, 0)
		require.NoError(t, err)
		require.False(t, truncated)

		var relativeMatches []string
		for _, match := range matches {
			rel, err := filepath.Rel(testDir, match)
			require.NoError(t, err)
			relativeMatches = append(relativeMatches, filepath.ToSlash(rel))
		}

		// Should find the directory itself and all contents
		require.ElementsMatch(t, relativeMatches, []string{
			"pkg",
			"pkg/main.go",
			"pkg/utils.go",
			"pkg/internal",
			"pkg/internal/helper.go",
		})
	})

	t.Run("respects limit parameter", func(t *testing.T) {
		testDir := t.TempDir()

		// Create many test files
		for i := range 10 {
			file := filepath.Join(testDir, "file", fmt.Sprintf("test%d.txt", i))
			require.NoError(t, os.MkdirAll(filepath.Dir(file), 0o755))
			require.NoError(t, os.WriteFile(file, []byte("test"), 0o644))
		}

		// Test with limit
		matches, truncated, err := GlobWithDoubleStar("**/*.txt", testDir, 5)
		require.NoError(t, err)
		require.True(t, truncated, "Expected truncation with limit")
		require.Len(t, matches, 5, "Expected exactly 5 matches with limit")
	})

	t.Run("handles nested directory patterns", func(t *testing.T) {
		testDir := t.TempDir()

		// Create nested structure
		file1 := filepath.Join(testDir, "a", "b", "c", "file1.txt")
		file2 := filepath.Join(testDir, "a", "b", "file2.txt")
		file3 := filepath.Join(testDir, "a", "file3.txt")
		file4 := filepath.Join(testDir, "file4.txt")

		for _, file := range []string{file1, file2, file3, file4} {
			require.NoError(t, os.MkdirAll(filepath.Dir(file), 0o755))
			require.NoError(t, os.WriteFile(file, []byte("test"), 0o644))
		}

		// Test specific nested pattern - look for file1.txt specifically
		matches, truncated, err := GlobWithDoubleStar("a/b/c/file1.txt", testDir, 0)
		require.NoError(t, err)
		require.False(t, truncated)

		// Should find exactly file1.txt
		require.Equal(t, matches, []string{file1})
	})

	t.Run("returns results sorted by modification time (newest first)", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			testDir := t.TempDir()

			// Create files
			file1 := filepath.Join(testDir, "file1.txt")
			file2 := filepath.Join(testDir, "file2.txt")
			file3 := filepath.Join(testDir, "file3.txt")

			require.NoError(t, os.WriteFile(file1, []byte("first"), 0o644))
			require.NoError(t, os.WriteFile(file2, []byte("second"), 0o644))
			require.NoError(t, os.WriteFile(file3, []byte("third"), 0o644))

			// Set deterministic mtimes using the fake clock
			base := time.Now()
			m1 := base
			m2 := base.Add(1 * time.Millisecond)
			m3 := base.Add(2 * time.Millisecond)

			// Set atime and mtime; only mtime matters for your sort
			require.NoError(t, os.Chtimes(file1, m1, m1))
			require.NoError(t, os.Chtimes(file2, m2, m2))
			require.NoError(t, os.Chtimes(file3, m3, m3))

			matches, truncated, err := GlobWithDoubleStar("*.txt", testDir, 0)
			require.NoError(t, err)
			require.False(t, truncated)
			// Files should be sorted by modification time (newest first)
			// So file3 should be first, file2 second, file1 last
			require.Equal(t, matches, []string{file3, file2, file1})
		})
	})

	t.Run("handles empty directory", func(t *testing.T) {
		testDir := t.TempDir()

		matches, truncated, err := GlobWithDoubleStar("**", testDir, 0)
		require.NoError(t, err)
		require.False(t, truncated)
		// Even empty directories should return the directory itself
		require.Equal(t, matches, []string{testDir})
	})

	t.Run("handles non-existent search path", func(t *testing.T) {
		nonExistentDir := filepath.Join(t.TempDir(), "does", "not", "exist")

		matches, truncated, err := GlobWithDoubleStar("**", nonExistentDir, 0)
		require.Error(t, err, "Should return error for non-existent search path")
		require.False(t, truncated)
		require.Empty(t, matches)
	})

	t.Run("respects basic ignore patterns", func(t *testing.T) {
		testDir := t.TempDir()

		// Create basic ignore structure
		rootIgnore := filepath.Join(testDir, ".crushignore")

		// Root .crushignore ignores *.tmp files and backup directories
		require.NoError(t, os.WriteFile(rootIgnore, []byte("*.tmp\nbackup/\n"), 0o644))

		// Create test files and directories
		goodFile := filepath.Join(testDir, "good.txt")
		badFile := filepath.Join(testDir, "bad.tmp")
		goodDir := filepath.Join(testDir, "src")
		ignoredDir := filepath.Join(testDir, "backup")
		ignoredFileInDir := filepath.Join(testDir, "backup", "old.txt")

		// Create all files and directories
		require.NoError(t, os.WriteFile(goodFile, []byte("content"), 0o644))
		require.NoError(t, os.WriteFile(badFile, []byte("temp content"), 0o644))
		require.NoError(t, os.MkdirAll(goodDir, 0o755))
		require.NoError(t, os.MkdirAll(ignoredDir, 0o755))
		require.NoError(t, os.WriteFile(ignoredFileInDir, []byte("old content"), 0o644))

		// Test that ignore patterns work for files
		matches, truncated, err := GlobWithDoubleStar("*.tmp", testDir, 0)
		require.NoError(t, err)
		require.False(t, truncated)

		// Should find no .tmp files (ignored by .crushignore)
		require.Empty(t, matches, "Expected no matches for '*.tmp' pattern (should be ignored)")

		// Test that ignore patterns work for directories
		matches, truncated, err = GlobWithDoubleStar("backup", testDir, 0)
		require.NoError(t, err)
		require.False(t, truncated)

		// Should find no backup directory (ignored by .crushignore)
		require.Empty(t, matches, "Expected no matches for 'backup' pattern (should be ignored)")

		// Test that non-ignored files are found
		matches, truncated, err = GlobWithDoubleStar("*.txt", testDir, 0)
		require.NoError(t, err)
		require.False(t, truncated)

		// Should find only the good file, not the one in ignored directory
		require.Equal(t, matches, []string{goodFile})
	})

	t.Run("handles mixed file and directory matching with sorting", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			testDir := t.TempDir()

			oldFile := filepath.Join(testDir, "old.test")
			newDir := filepath.Join(testDir, "new.test")
			midFile := filepath.Join(testDir, "mid.test")

			require.NoError(t, os.WriteFile(oldFile, []byte("old"), 0o644))
			require.NoError(t, os.MkdirAll(newDir, 0o755))
			require.NoError(t, os.WriteFile(midFile, []byte("mid"), 0o644))

			// Deterministic ordering via fake time
			base := time.Now()
			tOld := base
			tDir := base.Add(1 * time.Millisecond)
			tMid := base.Add(2 * time.Millisecond)

			require.NoError(t, os.Chtimes(oldFile, tOld, tOld))
			require.NoError(t, os.Chtimes(newDir, tDir, tDir))
			require.NoError(t, os.Chtimes(midFile, tMid, tMid))

			// Test pattern that matches both files and directories
			matches, truncated, err := GlobWithDoubleStar("*.test", testDir, 0)
			require.NoError(t, err)
			require.False(t, truncated)
			// Results should be sorted by modification time (newest first)
			require.Equal(t, matches, []string{midFile, newDir, oldFile})
		})
	})
}
