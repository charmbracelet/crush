// Package gitutil provides utility functions for interacting with Git
// repositories.
package gitutil

import (
	"sync"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

const refreshInterval = 5 * time.Second

// cachedBranch stores the last read branch name per directory. Keying the
// cache by directory keeps callers that operate on different directories from
// overwriting each other's entries.
type cachedBranch struct {
	mu      sync.RWMutex
	entries map[string]cacheEntry
}

type cacheEntry struct {
	value    string
	lastRead time.Time
}

var cache = &cachedBranch{entries: make(map[string]cacheEntry)}

// CurrentBranch returns the current Git branch name for the given directory.
// The result is cached per directory and refreshed at most once every 5
// seconds. Returns an empty string if the directory is not in a Git
// repository, the repository is in a detached HEAD state, or any error occurs.
func CurrentBranch(dir string) string {
	cache.mu.RLock()
	entry, ok := cache.entries[dir]
	cache.mu.RUnlock()
	if ok && time.Since(entry.lastRead) < refreshInterval {
		return entry.value
	}

	branch := readBranch(dir)

	cache.mu.Lock()
	cache.entries[dir] = cacheEntry{value: branch, lastRead: time.Now()}
	cache.mu.Unlock()

	return branch
}

func readBranch(dir string) string {
	repo, err := git.PlainOpenWithOptions(dir, &git.PlainOpenOptions{
		DetectDotGit: true,
	})
	if err != nil {
		return ""
	}

	head, err := repo.Head()
	if err != nil {
		return ""
	}

	if head.Type() != plumbing.HashReference {
		return ""
	}

	name := head.Name().Short()
	if name == "HEAD" {
		return ""
	}
	return name
}
