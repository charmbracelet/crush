package projects

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/charmbracelet/crush/internal/config"
)

const projectsFileName = "projects.json"

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

// Load reads the projects list from disk.
func Load() (*ProjectList, error) {
	mu.Lock()
	defer mu.Unlock()

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

// Save writes the projects list to disk.
func Save(list *ProjectList) error {
	mu.Lock()
	defer mu.Unlock()

	path := projectsFilePath()

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o600)
}

// Register adds or updates a project in the list.
func Register(workingDir, dataDir string) error {
	list, err := Load()
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

	return Save(list)
}

// List returns all tracked projects, most recently registered first.
func List() ([]Project, error) {
	list, err := Load()
	if err != nil {
		return nil, err
	}
	return list.Projects, nil
}
