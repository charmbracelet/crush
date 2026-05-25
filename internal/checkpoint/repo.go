// Package checkpoint provides filesystem snapshot functionality using an
// embedded go-git repository. Snapshots are stored in .crush/git/ and never
// touch the user's .git/ directory.
package checkpoint

import (
	"cmp"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/filesystem"
)

// Common errors.
var (
	ErrRepoNotInitialized = errors.New("checkpoint repo not initialized")
	ErrSnapshotNotFound   = errors.New("snapshot not found")
	ErrWorktreeExists     = errors.New("worktree already exists")
	ErrWorktreeNotFound   = errors.New("worktree not found")
)

// blobCacheEntry stores the mtime and size of a file at the time it was
// last snapshotted, along with its git blob hash. If a file's mtime and
// size haven't changed, we can skip reading and compressing it.
type blobCacheEntry struct {
	ModTime time.Time
	Size    int64
	Hash    plumbing.Hash
}

// Repo manages Crush's private git repository for snapshots.
// It uses go-git with a custom GIT_DIR (.crush/git) while operating
// on the user's project directory as the work tree.
type Repo struct {
	repo       *git.Repository
	gitDir     string // .crush/git
	projectDir string // User's project root
	config     *Config

	// blobCache maps relative file paths to their last known state.
	// Used to skip re-reading and re-compressing unchanged files.
	blobCache map[string]blobCacheEntry
}

// Config holds snapshot configuration.
type Config struct {
	// Exclude is a list of glob patterns to exclude from snapshots.
	// Supports doublestar patterns (e.g., **/node_modules).
	Exclude []string
}

// DefaultConfig returns the default snapshot configuration.
func DefaultConfig() *Config {
	return &Config{
		Exclude: []string{
			"node_modules",
			"**/node_modules",
			"vendor",
			".venv",
			"venv",
			"__pycache__",
			"**/__pycache__",
			"*.pyc",
			"target",
			"dist",
			"build",
			".next",
			".nuxt",
			".output",
			".cache",
			"*.log",
			".DS_Store",
		},
	}
}

// InitRepo initializes or opens the Crush git repo at .crush/git/.
// If the repo doesn't exist, it creates a new one.
func InitRepo(projectDir string, cfg *Config) (*Repo, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	// Don't snapshot if the project dir is the user's home directory or
	// if there's no .git directory (not a project).
	homeDir, _ := os.UserHomeDir()
	if homeDir != "" && projectDir == homeDir {
		return nil, fmt.Errorf("refusing to snapshot home directory")
	}
	if _, err := os.Stat(filepath.Join(projectDir, ".git")); err != nil {
		return nil, fmt.Errorf("not a git repository: %w", err)
	}

	crushDir := filepath.Join(projectDir, ".crush")
	gitDir := filepath.Join(crushDir, "git")

	// Ensure .crush directory exists.
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		return nil, fmt.Errorf("create git dir: %w", err)
	}

	var repo *git.Repository
	var err error

	// Create storage with cache for object lookups.
	storage := filesystem.NewStorage(osfs.New(gitDir), cache.NewObjectLRUDefault())

	// Check if repo already exists.
	if _, statErr := os.Stat(filepath.Join(gitDir, "HEAD")); os.IsNotExist(statErr) {
		// Initialize new bare-ish repo.
		repo, err = git.Init(storage, nil)
		if err != nil {
			return nil, fmt.Errorf("init git repo: %w", err)
		}
	} else {
		// Open existing repo.
		repo, err = git.Open(storage, nil)
		if err != nil {
			return nil, fmt.Errorf("open git repo: %w", err)
		}
	}

	return &Repo{
		repo:       repo,
		gitDir:     gitDir,
		projectDir: projectDir,
		config:     cfg,
	}, nil
}

// CreateSnapshot creates a snapshot of the current filesystem state.
// Returns the git commit hash.
func (r *Repo) CreateSnapshot(description string) (string, error) {
	if r.repo == nil {
		return "", ErrRepoNotInitialized
	}

	// Build tree from project directory.
	treeHash, err := r.buildTree(r.projectDir, "")
	if err != nil {
		return "", fmt.Errorf("build tree: %w", err)
	}

	// Get parent commit if exists.
	var parents []plumbing.Hash
	headRef, err := r.repo.Head()
	if err == nil {
		parents = append(parents, headRef.Hash())
	}

	// Create commit.
	sig := &object.Signature{
		Name:  "Crush",
		Email: "crush@charm.sh",
		When:  time.Now(),
	}

	commit := &object.Commit{
		Author:    *sig,
		Committer: *sig,
		Message:   description,
		TreeHash:  treeHash,
	}

	if len(parents) > 0 {
		commit.ParentHashes = parents
	}

	// Encode and store commit.
	obj := &plumbing.MemoryObject{}
	if err := commit.Encode(obj); err != nil {
		return "", fmt.Errorf("encode commit: %w", err)
	}

	commitHash, err := r.repo.Storer.SetEncodedObject(obj)
	if err != nil {
		return "", fmt.Errorf("store commit: %w", err)
	}

	// Update HEAD.
	ref := plumbing.NewHashReference(plumbing.HEAD, commitHash)
	if err := r.repo.Storer.SetReference(ref); err != nil {
		return "", fmt.Errorf("update HEAD: %w", err)
	}

	return commitHash.String(), nil
}

// CreateSnapshotRef creates a snapshot and stores it at a named ref.
// The ref format is: refs/snapshots/{sessionID}/{messageID}
func (r *Repo) CreateSnapshotRef(sessionID, messageID, description string) (string, error) {
	commitHash, err := r.CreateSnapshot(description)
	if err != nil {
		return "", err
	}

	// Create ref for this snapshot.
	refName := plumbing.ReferenceName(fmt.Sprintf("refs/snapshots/%s/%s", sessionID, messageID))
	ref := plumbing.NewHashReference(refName, plumbing.NewHash(commitHash))
	if err := r.repo.Storer.SetReference(ref); err != nil {
		return "", fmt.Errorf("create snapshot ref: %w", err)
	}

	return commitHash, nil
}

// RestoreSnapshot restores the filesystem to a snapshot.
func (r *Repo) RestoreSnapshot(commitHash string, targetDir string) error {
	if r.repo == nil {
		return ErrRepoNotInitialized
	}

	if targetDir == "" {
		targetDir = r.projectDir
	}

	// Get commit.
	hash := plumbing.NewHash(commitHash)
	commit, err := r.repo.CommitObject(hash)
	if err != nil {
		return fmt.Errorf("get commit: %w", err)
	}

	// Get tree.
	tree, err := commit.Tree()
	if err != nil {
		return fmt.Errorf("get tree: %w", err)
	}

	// Restore files.
	return r.restoreTree(tree, targetDir, "")
}

// Diff returns the diff between two snapshots as a unified diff string.
func (r *Repo) Diff(fromHash, toHash string) (string, error) {
	if r.repo == nil {
		return "", ErrRepoNotInitialized
	}

	fromCommit, err := r.repo.CommitObject(plumbing.NewHash(fromHash))
	if err != nil {
		return "", fmt.Errorf("get from commit: %w", err)
	}

	toCommit, err := r.repo.CommitObject(plumbing.NewHash(toHash))
	if err != nil {
		return "", fmt.Errorf("get to commit: %w", err)
	}

	fromTree, err := fromCommit.Tree()
	if err != nil {
		return "", fmt.Errorf("get from tree: %w", err)
	}

	toTree, err := toCommit.Tree()
	if err != nil {
		return "", fmt.Errorf("get to tree: %w", err)
	}

	changes, err := fromTree.Diff(toTree)
	if err != nil {
		return "", fmt.Errorf("compute diff: %w", err)
	}

	patch, err := changes.Patch()
	if err != nil {
		return "", fmt.Errorf("generate patch: %w", err)
	}

	return patch.String(), nil
}

// GetSnapshotRef retrieves a commit hash by ref name.
func (r *Repo) GetSnapshotRef(sessionID, messageID string) (string, error) {
	refName := plumbing.ReferenceName(fmt.Sprintf("refs/snapshots/%s/%s", sessionID, messageID))
	ref, err := r.repo.Storer.Reference(refName)
	if err != nil {
		return "", ErrSnapshotNotFound
	}
	return ref.Hash().String(), nil
}

// DeleteSnapshotRef deletes a snapshot ref.
func (r *Repo) DeleteSnapshotRef(sessionID, messageID string) error {
	refName := plumbing.ReferenceName(fmt.Sprintf("refs/snapshots/%s/%s", sessionID, messageID))
	return r.repo.Storer.RemoveReference(refName)
}

// ListSessionSnapshots lists all snapshot refs for a session.
func (r *Repo) ListSessionSnapshots(sessionID string) ([]string, error) {
	prefix := fmt.Sprintf("refs/snapshots/%s/", sessionID)
	refs, err := r.repo.References()
	if err != nil {
		return nil, err
	}

	var messageIDs []string
	err = refs.ForEach(func(ref *plumbing.Reference) error {
		name := string(ref.Name())
		if messageID, ok := strings.CutPrefix(name, prefix); ok {
			messageIDs = append(messageIDs, messageID)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return messageIDs, nil
}

// GC runs garbage collection on the repository.
func (r *Repo) GC() error {
	// go-git doesn't have built-in GC, but we can prune unreachable objects
	// by repacking. For now, this is a no-op placeholder.
	// In the future, we could shell out to `git gc` or implement pruning.
	return nil
}

// buildTree recursively builds a git tree from a directory.
func (r *Repo) buildTree(dir string, relPath string) (plumbing.Hash, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return plumbing.ZeroHash, fmt.Errorf("read dir %s: %w", dir, err)
	}

	// Sort entries for deterministic tree hashes.
	slices.SortFunc(entries, func(a, b os.DirEntry) int {
		return cmp.Compare(a.Name(), b.Name())
	})

	var treeEntries []object.TreeEntry

	for _, entry := range entries {
		name := entry.Name()
		entryPath := filepath.Join(dir, name)
		entryRelPath := filepath.Join(relPath, name)

		// Skip .git and .crush directories.
		if name == ".git" || name == ".crush" {
			continue
		}

		// Check exclusions.
		if r.isExcluded(entryRelPath) {
			continue
		}

		if entry.IsDir() {
			// Recurse into directory.
			subTreeHash, err := r.buildTree(entryPath, entryRelPath)
			if err != nil {
				return plumbing.ZeroHash, err
			}

			// Skip empty directories.
			if subTreeHash == plumbing.ZeroHash {
				continue
			}

			treeEntries = append(treeEntries, object.TreeEntry{
				Name: name,
				Mode: filemode.Dir,
				Hash: subTreeHash,
			})
		} else if entry.Type().IsRegular() || entry.Type()&fs.ModeSymlink != 0 {
			// Add file or symlink.
			info, err := entry.Info()
			if err != nil {
				return plumbing.ZeroHash, fmt.Errorf("stat %s: %w", entryPath, err)
			}

			var mode filemode.FileMode
			if info.Mode()&fs.ModeSymlink != 0 {
				mode = filemode.Symlink
			} else if info.Mode()&0o111 != 0 {
				mode = filemode.Executable
			} else {
				mode = filemode.Regular
			}

			blobHash, err := r.addBlob(entryPath, info.Mode()&fs.ModeSymlink != 0)
			if err != nil {
				return plumbing.ZeroHash, err
			}

			treeEntries = append(treeEntries, object.TreeEntry{
				Name: name,
				Mode: mode,
				Hash: blobHash,
			})
		}
	}

	// Return zero hash for empty trees.
	if len(treeEntries) == 0 {
		return plumbing.ZeroHash, nil
	}

	// Sort tree entries using git's sort order (directories treated as name + "/").
	slices.SortFunc(treeEntries, func(a, b object.TreeEntry) int {
		nameA, nameB := a.Name, b.Name
		if a.Mode == filemode.Dir {
			nameA += "/"
		}
		if b.Mode == filemode.Dir {
			nameB += "/"
		}
		return cmp.Compare(nameA, nameB)
	})

	// Create tree object.
	tree := &object.Tree{Entries: treeEntries}

	obj := &plumbing.MemoryObject{}
	if err := tree.Encode(obj); err != nil {
		return plumbing.ZeroHash, fmt.Errorf("encode tree: %w", err)
	}

	hash, err := r.repo.Storer.SetEncodedObject(obj)
	if err != nil {
		return plumbing.ZeroHash, fmt.Errorf("store tree: %w", err)
	}

	return hash, nil
}

// addBlob adds a file's content as a blob and returns its hash.
// It uses the blob cache to skip reading/compressing files whose mtime
// and size haven't changed since the last snapshot.
func (r *Repo) addBlob(path string, isSymlink bool) (plumbing.Hash, error) {
	// For regular files, check the mtime cache.
	if !isSymlink {
		relPath, _ := filepath.Rel(r.projectDir, path)
		if relPath != "" {
			info, err := os.Stat(path)
			if err == nil {
				if cached, ok := r.blobCache[relPath]; ok {
					if info.ModTime().Equal(cached.ModTime) && info.Size() == cached.Size {
						return cached.Hash, nil
					}
				}
			}
		}
	}

	var content []byte
	var err error

	if isSymlink {
		// For symlinks, store the link target.
		target, err := os.Readlink(path)
		if err != nil {
			return plumbing.ZeroHash, fmt.Errorf("read symlink %s: %w", path, err)
		}
		content = []byte(target)
	} else {
		content, err = os.ReadFile(path)
		if err != nil {
			return plumbing.ZeroHash, fmt.Errorf("read file %s: %w", path, err)
		}
	}

	obj := r.repo.Storer.NewEncodedObject()
	obj.SetType(plumbing.BlobObject)
	obj.SetSize(int64(len(content)))
	writer, err := obj.Writer()
	if err != nil {
		return plumbing.ZeroHash, fmt.Errorf("create blob writer: %w", err)
	}
	if _, err := writer.Write(content); err != nil {
		writer.Close()
		return plumbing.ZeroHash, fmt.Errorf("write blob: %w", err)
	}
	writer.Close()

	hash, err := r.repo.Storer.SetEncodedObject(obj)
	if err != nil {
		return plumbing.ZeroHash, fmt.Errorf("store blob: %w", err)
	}

	// Update the cache.
	if !isSymlink {
		relPath, _ := filepath.Rel(r.projectDir, path)
		if relPath != "" {
			if info, err := os.Stat(path); err == nil {
				if r.blobCache == nil {
					r.blobCache = make(map[string]blobCacheEntry)
				}
				r.blobCache[relPath] = blobCacheEntry{
					ModTime: info.ModTime(),
					Size:    info.Size(),
					Hash:    hash,
				}
			}
		}
	}

	return hash, nil
}

// isExcluded checks if a path matches any exclusion pattern.
func (r *Repo) isExcluded(relPath string) bool {
	// Normalize path separators for matching.
	relPath = filepath.ToSlash(relPath)

	for _, pattern := range r.config.Exclude {
		pattern = filepath.ToSlash(pattern)

		// Try exact match first.
		if pattern == relPath {
			return true
		}

		// Try as base name match.
		if pattern == filepath.Base(relPath) {
			return true
		}

		// Try glob match.
		matched, err := doublestar.Match(pattern, relPath)
		if err == nil && matched {
			return true
		}
	}

	return false
}

// restoreTree recursively restores a tree to a directory.
func (r *Repo) restoreTree(tree *object.Tree, targetDir string, relPath string) error {
	// Create target directory if needed.
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("create dir %s: %w", targetDir, err)
	}

	// Track files we restore so we can clean up extras.
	restored := make(map[string]struct{})

	for _, entry := range tree.Entries {
		targetPath := filepath.Join(targetDir, entry.Name)
		entryRelPath := filepath.Join(relPath, entry.Name)
		restored[entry.Name] = struct{}{}

		switch entry.Mode {
		case filemode.Dir:
			// Get subtree and recurse.
			subTree, err := r.repo.TreeObject(entry.Hash)
			if err != nil {
				return fmt.Errorf("get subtree %s: %w", entry.Name, err)
			}
			if err := r.restoreTree(subTree, targetPath, entryRelPath); err != nil {
				return err
			}

		case filemode.Regular, filemode.Executable:
			// Restore file.
			blob, err := r.repo.BlobObject(entry.Hash)
			if err != nil {
				return fmt.Errorf("get blob %s: %w", entry.Name, err)
			}

			reader, err := blob.Reader()
			if err != nil {
				return fmt.Errorf("read blob %s: %w", entry.Name, err)
			}

			content, err := io.ReadAll(reader)
			reader.Close()
			if err != nil {
				return fmt.Errorf("read blob content %s: %w", entry.Name, err)
			}

			mode := os.FileMode(0o644)
			if entry.Mode == filemode.Executable {
				mode = 0o755
			}

			if err := os.WriteFile(targetPath, content, mode); err != nil {
				return fmt.Errorf("write file %s: %w", targetPath, err)
			}

		case filemode.Symlink:
			// Restore symlink.
			blob, err := r.repo.BlobObject(entry.Hash)
			if err != nil {
				return fmt.Errorf("get symlink blob %s: %w", entry.Name, err)
			}

			reader, err := blob.Reader()
			if err != nil {
				return fmt.Errorf("read symlink blob %s: %w", entry.Name, err)
			}

			target, err := io.ReadAll(reader)
			reader.Close()
			if err != nil {
				return fmt.Errorf("read symlink target %s: %w", entry.Name, err)
			}

			// Remove existing file/symlink if exists.
			os.Remove(targetPath)

			if err := os.Symlink(string(target), targetPath); err != nil {
				return fmt.Errorf("create symlink %s: %w", targetPath, err)
			}
		}
	}

	// Clean up files that exist on disk but not in the snapshot.
	// Skip excluded paths - they're managed separately (node_modules, etc.).
	entries, err := os.ReadDir(targetDir)
	if err != nil {
		return nil // Ignore read errors during cleanup.
	}

	for _, entry := range entries {
		name := entry.Name()
		if name == ".git" || name == ".crush" {
			continue
		}
		if _, ok := restored[name]; ok {
			continue
		}

		entryRelPath := filepath.Join(relPath, name)
		if r.isExcluded(entryRelPath) {
			continue // Don't delete excluded paths.
		}

		// Remove file/dir that's not in the snapshot.
		targetPath := filepath.Join(targetDir, name)
		if err := os.RemoveAll(targetPath); err != nil {
			// Log but don't fail on cleanup errors.
			continue
		}
	}

	return nil
}

// ProjectDir returns the project directory this repo operates on.
func (r *Repo) ProjectDir() string {
	return r.projectDir
}

// GitDir returns the git directory path.
func (r *Repo) GitDir() string {
	return r.gitDir
}

// DiskUsage returns the size of the git directory in bytes.
func (r *Repo) DiskUsage() int64 {
	var size int64
	_ = filepath.Walk(r.gitDir, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size
}
