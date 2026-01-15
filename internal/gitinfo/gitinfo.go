// Package gitinfo provides utilities for extracting git repository information.
package gitinfo

import (
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Info contains git repository information.
type Info struct {
	IsRepo     bool   // Whether the path is inside a git repo
	RepoRoot   string // Absolute path to repo root
	RepoName   string // Name of the repo (directory name)
	PathInRepo string // Path relative to repo root
	Branch     string // Current branch name
	IsDirty    bool   // Whether there are uncommitted changes
}

var (
	cache     *Info
	cachePath string
	cacheMu   sync.RWMutex
	cacheTime time.Time
	cacheTTL  = 2 * time.Second
)

// Get returns git info for the given path, with caching.
func Get(path string) Info {
	cacheMu.RLock()
	if cache != nil && cachePath == path && time.Since(cacheTime) < cacheTTL {
		info := *cache
		cacheMu.RUnlock()
		return info
	}
	cacheMu.RUnlock()

	info := fetch(path)

	cacheMu.Lock()
	cache = &info
	cachePath = path
	cacheTime = time.Now()
	cacheMu.Unlock()

	return info
}

// Invalidate clears the cache, forcing a refresh on next Get.
func Invalidate() {
	cacheMu.Lock()
	cache = nil
	cacheMu.Unlock()
}

func fetch(path string) Info {
	info := Info{}

	// Get repo root.
	cmd := exec.Command("git", "-C", path, "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		return info
	}

	info.IsRepo = true
	info.RepoRoot = strings.TrimSpace(string(out))
	info.RepoName = filepath.Base(info.RepoRoot)

	// Calculate path inside repo.
	if path != info.RepoRoot {
		relPath, err := filepath.Rel(info.RepoRoot, path)
		if err == nil && relPath != "." {
			info.PathInRepo = relPath
		}
	}

	// Get current branch.
	cmd = exec.Command("git", "-C", path, "branch", "--show-current")
	out, err = cmd.Output()
	if err == nil {
		info.Branch = strings.TrimSpace(string(out))
	}
	// Fallback for detached HEAD.
	if info.Branch == "" {
		cmd = exec.Command("git", "-C", path, "rev-parse", "--short", "HEAD")
		out, err = cmd.Output()
		if err == nil {
			info.Branch = strings.TrimSpace(string(out))
		}
	}

	// Check if dirty (uncommitted changes).
	cmd = exec.Command("git", "-C", path, "status", "--porcelain")
	out, err = cmd.Output()
	if err == nil {
		info.IsDirty = len(strings.TrimSpace(string(out))) > 0
	}

	return info
}
