package projects

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/crush/internal/config"
)

const (
	projectsFileName = "projects.json"
	lockFileName     = projectsFileName + ".lock"
)

// Project represents a tracked workspace. Identity is Host + Path: the same
// path on two machines is two workspaces.
type Project struct {
	Path         string    `json:"path"`
	DataDir      string    `json:"data_dir"`
	LastAccessed time.Time `json:"last_accessed"`
	Host         string    `json:"host,omitempty"`
	Pinned       bool      `json:"pinned,omitempty"`
	// Locations lists every {cwd, data_dir} pair this workspace was launched
	// with (newest first, at most maxLocations), because a subdirectory shares
	// its repository's entry but may keep its own data directory. DataDir is
	// always the most recent one. Files from before this field have none; use
	// DataDirs rather than reading the field directly.
	Locations []Location `json:"locations,omitempty"`
}

// Location is one {cwd, data_dir} pair a workspace was launched with.
type Location struct {
	Cwd          string    `json:"cwd"`
	DataDir      string    `json:"data_dir"`
	LastAccessed time.Time `json:"last_accessed"`
}

// maxLocations caps Project.Locations; the oldest are dropped first.
const maxLocations = 8

// DataDirs returns every distinct data directory of the workspace, most
// recently used first. Callers that look for sessions (Home, stats --all)
// must scan all of them.
func (p Project) DataDirs() []string {
	var dirs []string
	add := func(d string) {
		if d != "" && !slices.Contains(dirs, d) {
			dirs = append(dirs, d)
		}
	}
	add(p.DataDir)
	for _, l := range p.Locations {
		add(l.DataDir)
	}
	return dirs
}

// ProjectList holds the list of tracked projects. The full history is kept;
// any display limit is the caller's business.
type ProjectList struct {
	Projects []Project `json:"projects"`
}

// mu serializes goroutines of this process; the flock on lockFileName
// serializes processes.
var mu sync.Mutex

// projectsFilePath returns the path to the projects.json file.
func projectsFilePath() string {
	return filepath.Join(filepath.Dir(config.GlobalConfigData()), projectsFileName)
}

func currentHost() string {
	h, _ := os.Hostname()
	return h
}

// NormalizeWorkspace returns the canonical workspace root of dir: its resolved
// path, then the git toplevel when inside a repository. This must match the
// root the launcher (scripts/install-agent-cli.sh) derives, so Home and the
// per-project server agree. A linked git worktree is its own workspace (its
// toplevel is itself); a directory outside git is its own root.
func NormalizeWorkspace(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--show-toplevel")
	cmd.Dir = resolved
	out, err := cmd.Output()
	if err != nil {
		// Not a repo, or git is missing: same fallback as the launcher.
		return resolved, nil
	}
	if top := strings.TrimSpace(string(out)); top != "" {
		return top, nil
	}
	return resolved, nil
}

// canonical normalizes dir, falling back to the absolute path when it cannot
// be resolved (e.g. the directory no longer exists).
func canonical(dir string) string {
	if p, err := NormalizeWorkspace(dir); err == nil {
		return p
	}
	if abs, err := filepath.Abs(dir); err == nil {
		return abs
	}
	return filepath.Clean(dir)
}

// read parses the registry file. Entries from older files get Host = this
// machine. Callers must hold mu.
func read() (*ProjectList, error) {
	data, err := os.ReadFile(projectsFilePath())
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
	host := currentHost()
	for i := range list.Projects {
		if list.Projects[i].Host == "" {
			list.Projects[i].Host = host
		}
	}
	return &list, nil
}

// resolveLink follows path while it is a symlink (dangling ones included), so
// a registry kept in a dotfiles repo stays a symlink after a write. At most 8
// hops, then the last path is used as is.
func resolveLink(path string) string {
	for range 8 {
		target, err := os.Readlink(path)
		if err != nil {
			return path
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(path), target)
		}
		path = target
	}
	return path
}

// write replaces the registry atomically (temp file next to the real file,
// then rename), so a reader never sees a half-written file. Callers must hold
// mu.
func write(list *ProjectList) error {
	path := resolveLink(projectsFilePath())
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), projectsFileName+".tmp-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // no-op after a successful rename
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return err
	}
	// Make the rename durable. Best effort: not every platform can fsync a
	// directory.
	if d, err := os.Open(filepath.Dir(path)); err == nil {
		_ = d.Sync()
		d.Close()
	}
	return nil
}

// update runs a read-modify-write cycle under both locks.
func update(fn func(*ProjectList) error) error {
	mu.Lock()
	defer mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(projectsFilePath()), 0o700); err != nil {
		return err
	}
	unlock, err := lockFile(filepath.Join(filepath.Dir(projectsFilePath()), lockFileName))
	if err != nil {
		return err
	}
	defer unlock()
	list, err := read()
	if err != nil {
		return err
	}
	if err := fn(list); err != nil {
		return err
	}
	return write(list)
}

// Load reads the projects list from disk.
func Load() (*ProjectList, error) {
	mu.Lock()
	defer mu.Unlock()
	return read()
}

// Save writes the projects list to disk, replacing it wholesale.
func Save(list *ProjectList) error {
	return update(func(cur *ProjectList) error {
		*cur = *list
		return nil
	})
}

// Register adds or updates a workspace in the list. workingDir is normalized
// (see NormalizeWorkspace), so a subdirectory refreshes its repository's entry
// instead of creating a new one.
func Register(workingDir, dataDir string) error {
	path := canonical(workingDir)
	cwd, err := filepath.Abs(workingDir)
	if err != nil {
		cwd = filepath.Clean(workingDir)
	}
	host := currentHost()
	return update(func(list *ProjectList) error {
		// The list is kept most-recent-first by moving each registration to
		// the front, not by sorting on timestamps, which tie on coarse clocks.
		var pinned bool
		var locs []Location
		list.Projects = slices.DeleteFunc(list.Projects, func(p Project) bool {
			if p.Path != path || p.Host != host {
				return false
			}
			pinned = p.Pinned
			locs = p.Locations
			if len(locs) == 0 && p.DataDir != "" { // Old-format entry.
				locs = []Location{{Cwd: p.Path, DataDir: p.DataDir, LastAccessed: p.LastAccessed}}
			}
			return true
		})
		now := time.Now().UTC()
		locs = slices.DeleteFunc(slices.Clone(locs), func(l Location) bool { return l.DataDir == dataDir })
		locs = slices.Insert(locs, 0, Location{Cwd: cwd, DataDir: dataDir, LastAccessed: now})
		slices.SortStableFunc(locs, func(a, b Location) int { return b.LastAccessed.Compare(a.LastAccessed) })
		if len(locs) > maxLocations {
			locs = locs[:maxLocations]
		}
		list.Projects = slices.Insert(list.Projects, 0, Project{
			Path:         path,
			DataDir:      dataDir,
			LastAccessed: now,
			Host:         host,
			Pinned:       pinned,
			Locations:    locs,
		})
		return nil
	})
}

// find returns the index of this host's entry for path: an exact match on the
// stored path first (so stale entries whose directory is gone can still be
// removed), then the normalized path. normalized is computed by the caller
// before taking the locks because it may run git.
func find(list *ProjectList, path, normalized string) int {
	host := currentHost()
	match := func(want string) int {
		return slices.IndexFunc(list.Projects, func(p Project) bool {
			return p.Path == want && p.Host == host
		})
	}
	if i := match(path); i >= 0 {
		return i
	}
	return match(normalized)
}

// Pin sets or clears the pinned flag of a workspace.
func Pin(path string, pinned bool) error {
	normalized := canonical(path)
	return update(func(list *ProjectList) error {
		i := find(list, path, normalized)
		if i < 0 {
			return fmt.Errorf("project not found: %s", path)
		}
		list.Projects[i].Pinned = pinned
		return nil
	})
}

// Remove drops a workspace from the registry. It never touches the project's
// files or data directory.
func Remove(path string) error {
	normalized := canonical(path)
	return update(func(list *ProjectList) error {
		i := find(list, path, normalized)
		if i < 0 {
			return fmt.Errorf("project not found: %s", path)
		}
		list.Projects = slices.Delete(list.Projects, i, i+1)
		return nil
	})
}

// List returns all tracked projects, most recently registered first.
func List() ([]Project, error) {
	list, err := Load()
	if err != nil {
		return nil, err
	}
	return list.Projects, nil
}
