package projects

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/lock"
)

const projectsFileName = "projects.json"

// lockDeadline bounds how long an access to projects.json waits for
// another process to finish with it. A few seconds is plenty for honest
// contention; longer suggests something is wedged.
const lockDeadline = 5 * time.Second

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

// projectsFilePath returns the path to the projects.json file.
func projectsFilePath() string {
	return filepath.Join(filepath.Dir(config.GlobalConfigData()), projectsFileName)
}

// lockProjects takes mu and the cross-process lock that guards
// projects.json, so that crush processes cannot interleave their reads and
// writes of the file. It returns the path of projects.json and a function
// that releases both locks.
func lockProjects() (string, func(), error) {
	mu.Lock()
	path := projectsFilePath()

	// The lock file lives next to projects.json, so ensure the directory
	// exists.
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		mu.Unlock()
		return "", nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), lockDeadline)
	defer cancel()
	release, err := lock.File(ctx, path+".lock")
	if err != nil {
		mu.Unlock()
		return "", nil, fmt.Errorf("acquire projects lock: %w", err)
	}
	return path, func() {
		release()
		mu.Unlock()
	}, nil
}

// Load reads the projects list from disk.
func Load() (*ProjectList, error) {
	path, unlock, err := lockProjects()
	if err != nil {
		return nil, err
	}
	defer unlock()
	return load(path)
}

// load reads the projects list at path. The caller must hold lockProjects.
func load(path string) (*ProjectList, error) {
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

// Save writes the projects list to disk.
func Save(list *ProjectList) error {
	path, unlock, err := lockProjects()
	if err != nil {
		return err
	}
	defer unlock()
	return save(path, list)
}

// save writes the projects list to path. The caller must hold lockProjects.
func save(path string, list *ProjectList) error {
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}

	// Replace the file in one step so that a crash, or a reader that does
	// not take the lock, never sees a partly written list.
	return config.AtomicWriteFile(path, data, 0o600)
}

// Register adds or updates a project in the list.
func Register(workingDir, dataDir string) error {
	// Hold the lock from load to save so that a registration another
	// process makes in between is not lost.
	path, unlock, err := lockProjects()
	if err != nil {
		return err
	}
	defer unlock()

	list, err := load(path)
	if err != nil {
		return err
	}

	// The list is kept most-recent-first by moving each registration to the
	// front, not by sorting on timestamps, which tie on coarse clocks.
	list.Projects = slices.DeleteFunc(list.Projects, func(p Project) bool {
		return p.Path == workingDir
	})
	list.Projects = slices.Insert(list.Projects, 0, Project{
		Path:         workingDir,
		DataDir:      dataDir,
		LastAccessed: time.Now().UTC(),
	})

	return save(path, list)
}

// List returns all tracked projects, most recently registered first.
func List() ([]Project, error) {
	list, err := Load()
	if err != nil {
		return nil, err
	}
	return list.Projects, nil
}
