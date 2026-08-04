package projects

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/fsext"
	"github.com/charmbracelet/crush/internal/lock"
)

const projectsFileName = "projects.json"

// MaxEntries is the maximum number of projects kept in the JSON file.
// When Register would exceed this limit, the least recently accessed
// entries are evicted (LRU by LastAccessed).
const MaxEntries = 2000

// Project represents a tracked project directory.
type Project struct {
	Path         string    `json:"path"`
	DataDir      string    `json:"data_dir"`
	LastAccessed time.Time `json:"last_accessed"`
}

// ProjectList holds the list of tracked projects.
type ProjectList struct {
	Projects []Project `json:"projects"`
}

var mu sync.Mutex

// timeNow is overridable in tests so the equal-timestamp path can be
// exercised deterministically; time.Now() is too fine-grained on Linux to
// reproduce it, but coarse enough on Windows to hit it routinely.
var timeNow = func() time.Time { return time.Now().UTC() }

// projectsFilePath returns the path to the projects.json file.
func projectsFilePath() string {
	return filepath.Join(filepath.Dir(config.GlobalConfigData()), projectsFileName)
}

// loadLocked reads the projects list from disk. Callers must hold mu.
func loadLocked() (*ProjectList, error) {
	path := projectsFilePath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &ProjectList{Projects: []Project{}}, nil
		}
		return nil, err
	}

	var list ProjectList
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, err
	}

	return &list, nil
}

// Load reads the projects list from disk.
func Load() (*ProjectList, error) {
	mu.Lock()
	defer mu.Unlock()
	return loadLocked()
}

// saveLocked writes the projects list to disk. Callers must hold mu.
func saveLocked(list *ProjectList) error {
	path := projectsFilePath()

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}

	return fsext.AtomicWriteFile(path, data, 0o600)
}

// Save writes the projects list to disk.
func Save(list *ProjectList) error {
	mu.Lock()
	defer mu.Unlock()
	return saveLocked(list)
}

// lockProjects serialises projects.json across processes. Register runs on
// every crush startup (cmd/root.go), so concurrent launches would otherwise
// lose updates; the process-local mu is not enough. Short timeout: a stuck
// holder must not hang startup.
func lockProjects(ctx context.Context) (func(), error) {
	dir := filepath.Dir(projectsFilePath())
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	lockPath := filepath.Join(dir, "projects.lock")
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return lock.File(ctx, lockPath)
}

// Register adds or updates a project in the list.
func Register(workingDir, dataDir string) error {
	mu.Lock()
	defer mu.Unlock()

	// Serialise against other processes' Registers. Acquired inside mu so the
	// lock order is total (mu -> file lock). On timeout, log and continue:
	// losing an LRU entry is cosmetic, refusing to launch is not.
	release, err := lockProjects(context.Background())
	if err != nil {
		slog.Debug("Failed to acquire cross-process projects lock; registering without it", "err", err)
	} else {
		defer release()
	}

	list, err := loadLocked()
	if err != nil {
		return err
	}

	now := timeNow()

	// Move the project being registered to the front, so that when its
	// timestamp ties with an existing entry the just-accessed one still sorts
	// first. Clock granularity on some platforms (notably Windows) makes two
	// registrations in quick succession indistinguishable by LastAccessed
	// alone, and the sort below only preserves this order because it is stable.
	entry := Project{Path: workingDir, DataDir: dataDir, LastAccessed: now}
	if i := slices.IndexFunc(list.Projects, func(p Project) bool {
		return p.Path == workingDir
	}); i >= 0 {
		list.Projects = slices.Delete(list.Projects, i, i+1)
	}
	list.Projects = slices.Insert(list.Projects, 0, entry)

	// Sort by last accessed (most recent first). Must be stable: equal
	// timestamps otherwise order arbitrarily.
	slices.SortStableFunc(list.Projects, func(a, b Project) int {
		return b.LastAccessed.Compare(a.LastAccessed)
	})

	if len(list.Projects) > MaxEntries {
		// Evict the least recently accessed entries -- but never the one just
		// registered. A store dated ahead of the local clock (skew, an NTP step
		// back, a file copied from a machine whose clock ran fast) sorts the new
		// entry to the tail, and truncating blindly would drop it on every run,
		// leaving the current project permanently absent from `crush projects`.
		kept := list.Projects[:MaxEntries]
		if !slices.ContainsFunc(kept, func(p Project) bool { return p.Path == workingDir }) {
			kept[MaxEntries-1] = entry
		}
		list.Projects = kept
	}

	return saveLocked(list)
}

// List returns all tracked projects sorted by last accessed.
func List() ([]Project, error) {
	list, err := Load()
	if err != nil {
		return nil, err
	}
	return list.Projects, nil
}
