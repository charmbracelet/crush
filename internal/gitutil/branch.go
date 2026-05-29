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

type cachedBranch struct {
	mu       sync.RWMutex
	value    string
	dir      string
	lastRead time.Time
}

var cache = &cachedBranch{}

// CurrentBranch returns the current Git branch name for the given directory.
// The result is cached and refreshed at most once every 5 seconds. Returns an
// empty string if the directory is not in a Git repository, the repository is
// in a detached HEAD state, or any error occurs.
func CurrentBranch(dir string) string {
	cache.mu.RLock()
	if cache.dir == dir && time.Since(cache.lastRead) < refreshInterval {
		v := cache.value
		cache.mu.RUnlock()
		return v
	}
	cache.mu.RUnlock()

	branch := readBranch(dir)

	cache.mu.Lock()
	cache.dir = dir
	cache.value = branch
	cache.lastRead = time.Now()
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
