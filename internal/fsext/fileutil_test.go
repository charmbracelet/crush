package fsext

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGlobWithDoubleStar(t *testing.T) {
	t.Run("finds files matching pattern", func(t *testing.T) {
		testDir := t.TempDir()

		mainGo := filepath.Join(testDir, "src", "main.go")
		utilsGo := filepath.Join(testDir, "src", "utils.go")
		helperGo := filepath.Join(testDir, "pkg", "helper.go")
		readmeMd := filepath.Join(testDir, "README.md")

		for _, file := range []string{mainGo, utilsGo, helperGo, readmeMd} {
			require.NoError(t, os.MkdirAll(filepath.Dir(file), 0o755))
			require.NoError(t, os.WriteFile(file, []byte("test content"), 0o644))
		}

		matches, truncated, err := GlobWithDoubleStar("**/main.go", testDir, 0)
		require.NoError(t, err)
		require.False(t, truncated)

		require.Equal(t, matches, []string{mainGo})
	})

	t.Run("finds directories matching pattern", func(t *testing.T) {
		testDir := t.TempDir()

		srcDir := filepath.Join(testDir, "src")
		pkgDir := filepath.Join(testDir, "pkg")
		internalDir := filepath.Join(testDir, "internal")
		cmdDir := filepath.Join(testDir, "cmd")
		pkgFile := filepath.Join(testDir, "pkg.txt")

		for _, dir := range []string{srcDir, pkgDir, internalDir, cmdDir} {
			require.NoError(t, os.MkdirAll(dir, 0o755))
		}

		require.NoError(t, os.WriteFile(filepath.Join(srcDir, "main.go"), []byte("package main"), 0o644))
		require.NoError(t, os.WriteFile(pkgFile, []byte("test"), 0o644))

		matches, truncated, err := GlobWithDoubleStar("pkg", testDir, 0)
		require.NoError(t, err)
		require.False(t, truncated)

		require.Equal(t, matches, []string{pkgDir})
	})

	t.Run("finds nested directories with wildcard patterns", func(t *testing.T) {
		testDir := t.TempDir()

		srcPkgDir := filepath.Join(testDir, "src", "pkg")
		libPkgDir := filepath.Join(testDir, "lib", "pkg")
		mainPkgDir := filepath.Join(testDir, "pkg")
		otherDir := filepath.Join(testDir, "other")

		for _, dir := range []string{srcPkgDir, libPkgDir, mainPkgDir, otherDir} {
			require.NoError(t, os.MkdirAll(dir, 0o755))
		}

		matches, truncated, err := GlobWithDoubleStar("**/pkg", testDir, 0)
		require.NoError(t, err)
		require.False(t, truncated)

		var relativeMatches []string
		for _, match := range matches {
			rel, err := filepath.Rel(testDir, match)
			require.NoError(t, err)
			relativeMatches = append(relativeMatches, filepath.ToSlash(rel))
		}

		require.ElementsMatch(t, relativeMatches, []string{"pkg", "src/pkg", "lib/pkg"})
	})

	t.Run("finds directory contents with recursive patterns", func(t *testing.T) {
		testDir := t.TempDir()

		pkgDir := filepath.Join(testDir, "pkg")
		pkgFile1 := filepath.Join(pkgDir, "main.go")
		pkgFile2 := filepath.Join(pkgDir, "utils.go")
		pkgSubdir := filepath.Join(pkgDir, "internal")
		pkgSubfile := filepath.Join(pkgSubdir, "helper.go")

		require.NoError(t, os.MkdirAll(pkgSubdir, 0o755))

		for _, file := range []string{pkgFile1, pkgFile2, pkgSubfile} {
			require.NoError(t, os.WriteFile(file, []byte("package main"), 0o644))
		}

		matches, truncated, err := GlobWithDoubleStar("pkg/**", testDir, 0)
		require.NoError(t, err)
		require.False(t, truncated)

		var relativeMatches []string
		for _, match := range matches {
			rel, err := filepath.Rel(testDir, match)
			require.NoError(t, err)
			relativeMatches = append(relativeMatches, filepath.ToSlash(rel))
		}

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

		for i := range 10 {
			file := filepath.Join(testDir, "file", fmt.Sprintf("test%d.txt", i))
			require.NoError(t, os.MkdirAll(filepath.Dir(file), 0o755))
			require.NoError(t, os.WriteFile(file, []byte("test"), 0o644))
		}

		matches, truncated, err := GlobWithDoubleStar("**/*.txt", testDir, 5)
		require.NoError(t, err)
		require.True(t, truncated, "Expected truncation with limit")
		require.Len(t, matches, 5, "Expected exactly 5 matches with limit")
	})

	t.Run("handles nested directory patterns", func(t *testing.T) {
		testDir := t.TempDir()

		file1 := filepath.Join(testDir, "a", "b", "c", "file1.txt")
		file2 := filepath.Join(testDir, "a", "b", "file2.txt")
		file3 := filepath.Join(testDir, "a", "file3.txt")
		file4 := filepath.Join(testDir, "file4.txt")

		for _, file := range []string{file1, file2, file3, file4} {
			require.NoError(t, os.MkdirAll(filepath.Dir(file), 0o755))
			require.NoError(t, os.WriteFile(file, []byte("test"), 0o644))
		}

		matches, truncated, err := GlobWithDoubleStar("a/b/c/file1.txt", testDir, 0)
		require.NoError(t, err)
		require.False(t, truncated)

		require.Equal(t, []string{file1}, matches)
	})

	t.Run("returns results sorted by modification time (newest first)", func(t *testing.T) {
		testDir := t.TempDir()

		file1 := filepath.Join(testDir, "file1.txt")
		require.NoError(t, os.WriteFile(file1, []byte("first"), 0o644))

		file2 := filepath.Join(testDir, "file2.txt")
		require.NoError(t, os.WriteFile(file2, []byte("second"), 0o644))

		file3 := filepath.Join(testDir, "file3.txt")
		require.NoError(t, os.WriteFile(file3, []byte("third"), 0o644))

		base := time.Now()
		m1 := base
		m2 := base.Add(10 * time.Hour)
		m3 := base.Add(20 * time.Hour)

		require.NoError(t, os.Chtimes(file1, m1, m1))
		require.NoError(t, os.Chtimes(file2, m2, m2))
		require.NoError(t, os.Chtimes(file3, m3, m3))

		matches, truncated, err := GlobWithDoubleStar("*.txt", testDir, 0)
		require.NoError(t, err)
		require.False(t, truncated)

		require.Equal(t, []string{file3, file2, file1}, matches)
	})

	t.Run("handles empty directory", func(t *testing.T) {
		testDir := t.TempDir()

		matches, truncated, err := GlobWithDoubleStar("**", testDir, 0)
		require.NoError(t, err)
		require.False(t, truncated)
		// Even empty directories should return the directory itself
		require.Equal(t, []string{testDir}, matches)
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

		rootIgnore := filepath.Join(testDir, ".crushignore")

		require.NoError(t, os.WriteFile(rootIgnore, []byte("*.tmp\nbackup/\n"), 0o644))

		goodFile := filepath.Join(testDir, "good.txt")
		require.NoError(t, os.WriteFile(goodFile, []byte("content"), 0o644))

		badFile := filepath.Join(testDir, "bad.tmp")
		require.NoError(t, os.WriteFile(badFile, []byte("temp content"), 0o644))

		goodDir := filepath.Join(testDir, "src")
		require.NoError(t, os.MkdirAll(goodDir, 0o755))

		ignoredDir := filepath.Join(testDir, "backup")
		require.NoError(t, os.MkdirAll(ignoredDir, 0o755))

		ignoredFileInDir := filepath.Join(testDir, "backup", "old.txt")
		require.NoError(t, os.WriteFile(ignoredFileInDir, []byte("old content"), 0o644))

		matches, truncated, err := GlobWithDoubleStar("*.tmp", testDir, 0)
		require.NoError(t, err)
		require.False(t, truncated)
		require.Empty(t, matches, "Expected no matches for '*.tmp' pattern (should be ignored)")

		matches, truncated, err = GlobWithDoubleStar("backup", testDir, 0)
		require.NoError(t, err)
		require.False(t, truncated)
		require.Empty(t, matches, "Expected no matches for 'backup' pattern (should be ignored)")

		matches, truncated, err = GlobWithDoubleStar("*.txt", testDir, 0)
		require.NoError(t, err)
		require.False(t, truncated)
		require.Equal(t, []string{goodFile}, matches)
	})

	t.Run("handles mixed file and directory matching with sorting", func(t *testing.T) {
		testDir := t.TempDir()

		oldestFile := filepath.Join(testDir, "old.rs")
		require.NoError(t, os.WriteFile(oldestFile, []byte("old"), 0o644))

		middleDir := filepath.Join(testDir, "mid.rs")
		require.NoError(t, os.MkdirAll(middleDir, 0o755))

		newestFile := filepath.Join(testDir, "new.rs")
		require.NoError(t, os.WriteFile(newestFile, []byte("new"), 0o644))

		base := time.Now()
		tOldest := base
		tMiddle := base.Add(10 * time.Hour)
		tNewest := base.Add(20 * time.Hour)

		// Reverse the expected order
		require.NoError(t, os.Chtimes(newestFile, tOldest, tOldest))
		require.NoError(t, os.Chtimes(middleDir, tMiddle, tMiddle))
		require.NoError(t, os.Chtimes(oldestFile, tNewest, tNewest))

		matches, truncated, err := GlobWithDoubleStar("*.rs", testDir, 0)
		require.NoError(t, err)
		require.False(t, truncated)
		require.Len(t, matches, 3)

		// Results should be sorted by mod time, but we set the oldestFile
		// to have the most recent mod time
		require.Equal(t, []string{oldestFile, middleDir, newestFile}, matches)
	})
}
