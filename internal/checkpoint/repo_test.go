package checkpoint_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/crush/internal/checkpoint"
	"github.com/stretchr/testify/require"
)

func TestInitRepo(t *testing.T) {
	t.Parallel()

	t.Run("creates new repo", func(t *testing.T) {
		t.Parallel()
		projectDir := t.TempDir()

		repo, err := checkpoint.InitRepo(projectDir, nil)
		require.NoError(t, err)
		require.NotNil(t, repo)

		// Verify .crush/git was created.
		gitDir := filepath.Join(projectDir, ".crush", "git")
		info, err := os.Stat(gitDir)
		require.NoError(t, err)
		require.True(t, info.IsDir())

		// Verify HEAD exists.
		headPath := filepath.Join(gitDir, "HEAD")
		_, err = os.Stat(headPath)
		require.NoError(t, err)
	})

	t.Run("reopens existing repo", func(t *testing.T) {
		t.Parallel()
		projectDir := t.TempDir()

		// Create repo.
		repo1, err := checkpoint.InitRepo(projectDir, nil)
		require.NoError(t, err)

		// Create a file and snapshot.
		err = os.WriteFile(filepath.Join(projectDir, "test.txt"), []byte("hello"), 0o644)
		require.NoError(t, err)

		hash, err := repo1.CreateSnapshot("initial")
		require.NoError(t, err)

		// Reopen repo.
		repo2, err := checkpoint.InitRepo(projectDir, nil)
		require.NoError(t, err)
		require.NotNil(t, repo2)

		// Verify we can read the snapshot.
		diff, err := repo2.Diff(hash, hash)
		require.NoError(t, err)
		require.Empty(t, diff)
	})
}

func TestCreateSnapshot(t *testing.T) {
	t.Parallel()

	t.Run("creates snapshot of files", func(t *testing.T) {
		t.Parallel()
		projectDir := t.TempDir()

		// Create some files.
		err := os.WriteFile(filepath.Join(projectDir, "main.go"), []byte("package main"), 0o644)
		require.NoError(t, err)

		subDir := filepath.Join(projectDir, "pkg")
		err = os.MkdirAll(subDir, 0o755)
		require.NoError(t, err)

		err = os.WriteFile(filepath.Join(subDir, "util.go"), []byte("package pkg"), 0o644)
		require.NoError(t, err)

		repo, err := checkpoint.InitRepo(projectDir, nil)
		require.NoError(t, err)

		hash, err := repo.CreateSnapshot("test snapshot")
		require.NoError(t, err)
		require.NotEmpty(t, hash)
		require.Len(t, hash, 40) // SHA-1 hex string.
	})

	t.Run("excludes node_modules by default", func(t *testing.T) {
		t.Parallel()
		projectDir := t.TempDir()

		// Create a file and node_modules.
		err := os.WriteFile(filepath.Join(projectDir, "index.js"), []byte("// code"), 0o644)
		require.NoError(t, err)

		nmDir := filepath.Join(projectDir, "node_modules")
		err = os.MkdirAll(nmDir, 0o755)
		require.NoError(t, err)

		err = os.WriteFile(filepath.Join(nmDir, "something.js"), []byte("// deps"), 0o644)
		require.NoError(t, err)

		repo, err := checkpoint.InitRepo(projectDir, nil)
		require.NoError(t, err)

		hash, err := repo.CreateSnapshot("test")
		require.NoError(t, err)

		// Restore to a new directory and verify node_modules is not there.
		restoreDir := t.TempDir()
		err = repo.RestoreSnapshot(hash, restoreDir)
		require.NoError(t, err)

		_, err = os.Stat(filepath.Join(restoreDir, "index.js"))
		require.NoError(t, err)

		_, err = os.Stat(filepath.Join(restoreDir, "node_modules"))
		require.True(t, os.IsNotExist(err))
	})

	t.Run("excludes .git directory", func(t *testing.T) {
		t.Parallel()
		projectDir := t.TempDir()

		// Create file and .git dir.
		err := os.WriteFile(filepath.Join(projectDir, "file.txt"), []byte("content"), 0o644)
		require.NoError(t, err)

		gitDir := filepath.Join(projectDir, ".git")
		err = os.MkdirAll(gitDir, 0o755)
		require.NoError(t, err)

		err = os.WriteFile(filepath.Join(gitDir, "config"), []byte("[core]"), 0o644)
		require.NoError(t, err)

		repo, err := checkpoint.InitRepo(projectDir, nil)
		require.NoError(t, err)

		hash, err := repo.CreateSnapshot("test")
		require.NoError(t, err)

		// Restore and verify .git is not restored.
		restoreDir := t.TempDir()
		err = repo.RestoreSnapshot(hash, restoreDir)
		require.NoError(t, err)

		_, err = os.Stat(filepath.Join(restoreDir, ".git"))
		require.True(t, os.IsNotExist(err))
	})

	t.Run("preserves executable permissions", func(t *testing.T) {
		t.Parallel()
		projectDir := t.TempDir()

		// Create executable file.
		scriptPath := filepath.Join(projectDir, "run.sh")
		err := os.WriteFile(scriptPath, []byte("#!/bin/bash\necho hello"), 0o755)
		require.NoError(t, err)

		repo, err := checkpoint.InitRepo(projectDir, nil)
		require.NoError(t, err)

		hash, err := repo.CreateSnapshot("test")
		require.NoError(t, err)

		// Restore and verify permissions.
		restoreDir := t.TempDir()
		err = repo.RestoreSnapshot(hash, restoreDir)
		require.NoError(t, err)

		restoredPath := filepath.Join(restoreDir, "run.sh")
		info, err := os.Stat(restoredPath)
		require.NoError(t, err)
		require.True(t, info.Mode()&0o111 != 0, "expected executable permissions")
	})

	t.Run("handles empty directories", func(t *testing.T) {
		t.Parallel()
		projectDir := t.TempDir()

		// Create file and empty dir.
		err := os.WriteFile(filepath.Join(projectDir, "file.txt"), []byte("content"), 0o644)
		require.NoError(t, err)

		emptyDir := filepath.Join(projectDir, "empty")
		err = os.MkdirAll(emptyDir, 0o755)
		require.NoError(t, err)

		repo, err := checkpoint.InitRepo(projectDir, nil)
		require.NoError(t, err)

		hash, err := repo.CreateSnapshot("test")
		require.NoError(t, err)

		// Empty directories are not tracked (same as git behavior).
		restoreDir := t.TempDir()
		err = repo.RestoreSnapshot(hash, restoreDir)
		require.NoError(t, err)

		_, err = os.Stat(filepath.Join(restoreDir, "file.txt"))
		require.NoError(t, err)
	})
}

func TestRestoreSnapshot(t *testing.T) {
	t.Parallel()

	t.Run("restores files correctly", func(t *testing.T) {
		t.Parallel()
		projectDir := t.TempDir()

		// Create initial files.
		err := os.WriteFile(filepath.Join(projectDir, "main.go"), []byte("package main\n\nfunc main() {}"), 0o644)
		require.NoError(t, err)

		repo, err := checkpoint.InitRepo(projectDir, nil)
		require.NoError(t, err)

		hash, err := repo.CreateSnapshot("initial")
		require.NoError(t, err)

		// Restore to new directory.
		restoreDir := t.TempDir()
		err = repo.RestoreSnapshot(hash, restoreDir)
		require.NoError(t, err)

		content, err := os.ReadFile(filepath.Join(restoreDir, "main.go"))
		require.NoError(t, err)
		require.Equal(t, "package main\n\nfunc main() {}", string(content))
	})

	t.Run("removes files not in snapshot", func(t *testing.T) {
		t.Parallel()
		projectDir := t.TempDir()

		// Create file.
		err := os.WriteFile(filepath.Join(projectDir, "keep.txt"), []byte("keep"), 0o644)
		require.NoError(t, err)

		repo, err := checkpoint.InitRepo(projectDir, nil)
		require.NoError(t, err)

		hash, err := repo.CreateSnapshot("first")
		require.NoError(t, err)

		// Add another file.
		err = os.WriteFile(filepath.Join(projectDir, "extra.txt"), []byte("extra"), 0o644)
		require.NoError(t, err)

		// Restore first snapshot - extra.txt should be removed.
		err = repo.RestoreSnapshot(hash, projectDir)
		require.NoError(t, err)

		_, err = os.Stat(filepath.Join(projectDir, "keep.txt"))
		require.NoError(t, err)

		_, err = os.Stat(filepath.Join(projectDir, "extra.txt"))
		require.True(t, os.IsNotExist(err))
	})

	t.Run("preserves excluded directories during restore", func(t *testing.T) {
		t.Parallel()
		projectDir := t.TempDir()

		// Create file and node_modules.
		err := os.WriteFile(filepath.Join(projectDir, "index.js"), []byte("// v1"), 0o644)
		require.NoError(t, err)

		nmDir := filepath.Join(projectDir, "node_modules")
		err = os.MkdirAll(nmDir, 0o755)
		require.NoError(t, err)

		err = os.WriteFile(filepath.Join(nmDir, "pkg.js"), []byte("// deps"), 0o644)
		require.NoError(t, err)

		repo, err := checkpoint.InitRepo(projectDir, nil)
		require.NoError(t, err)

		hash, err := repo.CreateSnapshot("first")
		require.NoError(t, err)

		// Modify index.js.
		err = os.WriteFile(filepath.Join(projectDir, "index.js"), []byte("// v2"), 0o644)
		require.NoError(t, err)

		// Restore - node_modules should still exist (excluded from deletion).
		err = repo.RestoreSnapshot(hash, projectDir)
		require.NoError(t, err)

		content, err := os.ReadFile(filepath.Join(projectDir, "index.js"))
		require.NoError(t, err)
		require.Equal(t, "// v1", string(content))

		// node_modules should still be there.
		_, err = os.Stat(filepath.Join(nmDir, "pkg.js"))
		require.NoError(t, err)
	})
}

func TestSnapshotRefs(t *testing.T) {
	t.Parallel()

	t.Run("creates and retrieves snapshot ref", func(t *testing.T) {
		t.Parallel()
		projectDir := t.TempDir()

		err := os.WriteFile(filepath.Join(projectDir, "file.txt"), []byte("content"), 0o644)
		require.NoError(t, err)

		repo, err := checkpoint.InitRepo(projectDir, nil)
		require.NoError(t, err)

		sessionID := "session-123"
		messageID := "msg-456"

		hash, err := repo.CreateSnapshotRef(sessionID, messageID, "test snapshot")
		require.NoError(t, err)
		require.NotEmpty(t, hash)

		// Retrieve by ref.
		retrieved, err := repo.GetSnapshotRef(sessionID, messageID)
		require.NoError(t, err)
		require.Equal(t, hash, retrieved)
	})

	t.Run("lists session snapshots", func(t *testing.T) {
		t.Parallel()
		projectDir := t.TempDir()

		err := os.WriteFile(filepath.Join(projectDir, "file.txt"), []byte("content"), 0o644)
		require.NoError(t, err)

		repo, err := checkpoint.InitRepo(projectDir, nil)
		require.NoError(t, err)

		sessionID := "session-abc"

		// Create multiple snapshots.
		_, err = repo.CreateSnapshotRef(sessionID, "msg-1", "first")
		require.NoError(t, err)

		_, err = repo.CreateSnapshotRef(sessionID, "msg-2", "second")
		require.NoError(t, err)

		// List snapshots.
		messageIDs, err := repo.ListSessionSnapshots(sessionID)
		require.NoError(t, err)
		require.Len(t, messageIDs, 2)
		require.Contains(t, messageIDs, "msg-1")
		require.Contains(t, messageIDs, "msg-2")
	})

	t.Run("deletes snapshot ref", func(t *testing.T) {
		t.Parallel()
		projectDir := t.TempDir()

		err := os.WriteFile(filepath.Join(projectDir, "file.txt"), []byte("content"), 0o644)
		require.NoError(t, err)

		repo, err := checkpoint.InitRepo(projectDir, nil)
		require.NoError(t, err)

		sessionID := "session-del"
		messageID := "msg-del"

		_, err = repo.CreateSnapshotRef(sessionID, messageID, "to delete")
		require.NoError(t, err)

		err = repo.DeleteSnapshotRef(sessionID, messageID)
		require.NoError(t, err)

		_, err = repo.GetSnapshotRef(sessionID, messageID)
		require.Error(t, err)
		require.Equal(t, checkpoint.ErrSnapshotNotFound, err)
	})
}

func TestDiff(t *testing.T) {
	t.Parallel()

	t.Run("shows diff between snapshots", func(t *testing.T) {
		t.Parallel()
		projectDir := t.TempDir()

		// Create initial file.
		err := os.WriteFile(filepath.Join(projectDir, "main.go"), []byte("package main\n"), 0o644)
		require.NoError(t, err)

		repo, err := checkpoint.InitRepo(projectDir, nil)
		require.NoError(t, err)

		hash1, err := repo.CreateSnapshot("first")
		require.NoError(t, err)

		// Modify file.
		err = os.WriteFile(filepath.Join(projectDir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644)
		require.NoError(t, err)

		hash2, err := repo.CreateSnapshot("second")
		require.NoError(t, err)

		// Get diff.
		diff, err := repo.Diff(hash1, hash2)
		require.NoError(t, err)
		require.Contains(t, diff, "main.go")
		require.Contains(t, diff, "func main()")
	})

	t.Run("empty diff for same commit", func(t *testing.T) {
		t.Parallel()
		projectDir := t.TempDir()

		err := os.WriteFile(filepath.Join(projectDir, "file.txt"), []byte("content"), 0o644)
		require.NoError(t, err)

		repo, err := checkpoint.InitRepo(projectDir, nil)
		require.NoError(t, err)

		hash, err := repo.CreateSnapshot("test")
		require.NoError(t, err)

		diff, err := repo.Diff(hash, hash)
		require.NoError(t, err)
		require.Empty(t, diff)
	})
}

func TestCustomExclusions(t *testing.T) {
	t.Parallel()

	t.Run("respects custom exclusions", func(t *testing.T) {
		t.Parallel()
		projectDir := t.TempDir()

		// Create files.
		err := os.WriteFile(filepath.Join(projectDir, "keep.txt"), []byte("keep"), 0o644)
		require.NoError(t, err)

		err = os.WriteFile(filepath.Join(projectDir, "exclude.tmp"), []byte("temp"), 0o644)
		require.NoError(t, err)

		// Custom config excluding .tmp files.
		cfg := &checkpoint.Config{
			Exclude: []string{"*.tmp"},
		}

		repo, err := checkpoint.InitRepo(projectDir, cfg)
		require.NoError(t, err)

		hash, err := repo.CreateSnapshot("test")
		require.NoError(t, err)

		// Restore and verify.
		restoreDir := t.TempDir()
		err = repo.RestoreSnapshot(hash, restoreDir)
		require.NoError(t, err)

		_, err = os.Stat(filepath.Join(restoreDir, "keep.txt"))
		require.NoError(t, err)

		_, err = os.Stat(filepath.Join(restoreDir, "exclude.tmp"))
		require.True(t, os.IsNotExist(err))
	})

	t.Run("supports doublestar patterns", func(t *testing.T) {
		t.Parallel()
		projectDir := t.TempDir()

		// Create nested structure.
		subDir := filepath.Join(projectDir, "src", "cache")
		err := os.MkdirAll(subDir, 0o755)
		require.NoError(t, err)

		err = os.WriteFile(filepath.Join(projectDir, "main.go"), []byte("package main"), 0o644)
		require.NoError(t, err)

		err = os.WriteFile(filepath.Join(subDir, "data.tmp"), []byte("cached"), 0o644)
		require.NoError(t, err)

		// Exclude **/cache.
		cfg := &checkpoint.Config{
			Exclude: []string{"**/cache"},
		}

		repo, err := checkpoint.InitRepo(projectDir, cfg)
		require.NoError(t, err)

		hash, err := repo.CreateSnapshot("test")
		require.NoError(t, err)

		restoreDir := t.TempDir()
		err = repo.RestoreSnapshot(hash, restoreDir)
		require.NoError(t, err)

		_, err = os.Stat(filepath.Join(restoreDir, "main.go"))
		require.NoError(t, err)

		_, err = os.Stat(filepath.Join(restoreDir, "src", "cache"))
		require.True(t, os.IsNotExist(err))
	})
}

func TestSymlinks(t *testing.T) {
	t.Parallel()

	t.Run("handles symlinks", func(t *testing.T) {
		t.Parallel()
		projectDir := t.TempDir()

		// Create file and symlink.
		targetPath := filepath.Join(projectDir, "target.txt")
		err := os.WriteFile(targetPath, []byte("target content"), 0o644)
		require.NoError(t, err)

		linkPath := filepath.Join(projectDir, "link.txt")
		err = os.Symlink("target.txt", linkPath)
		require.NoError(t, err)

		repo, err := checkpoint.InitRepo(projectDir, nil)
		require.NoError(t, err)

		hash, err := repo.CreateSnapshot("test")
		require.NoError(t, err)

		// Restore.
		restoreDir := t.TempDir()
		err = repo.RestoreSnapshot(hash, restoreDir)
		require.NoError(t, err)

		// Check symlink was restored.
		restoredLink := filepath.Join(restoreDir, "link.txt")
		linkTarget, err := os.Readlink(restoredLink)
		require.NoError(t, err)
		require.Equal(t, "target.txt", linkTarget)
	})
}

func TestRepoAccessors(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	repo, err := checkpoint.InitRepo(projectDir, nil)
	require.NoError(t, err)

	require.Equal(t, projectDir, repo.ProjectDir())
	require.Equal(t, filepath.Join(projectDir, ".crush", "git"), repo.GitDir())
}

func TestGC(t *testing.T) {
	t.Parallel()

	// GC is currently a no-op, but should not error.
	projectDir := t.TempDir()
	repo, err := checkpoint.InitRepo(projectDir, nil)
	require.NoError(t, err)

	err = repo.GC()
	require.NoError(t, err)
}
