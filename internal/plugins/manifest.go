package plugins

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/charmbracelet/crush/internal/fsext"
)

// ErrNoManifest means a directory is not a managed install, which is the
// normal case for a hand-dropped plugin folder.
var ErrNoManifest = errors.New("no plugin manifest")

// File is one installed plugin script and the content it was installed with.
type File struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

// Manifest is the lock record for one installed plugin repository. It pins the
// commit the files came from, so an install is reproducible and an update can
// say exactly which commit it moved from and to.
type Manifest struct {
	Version     int       `json:"version"`
	Source      string    `json:"source"`
	Ref         string    `json:"ref"`
	Commit      string    `json:"commit"`
	CommitURL   string    `json:"commit_url,omitempty"`
	InstalledAt time.Time `json:"installed_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Files       []File    `json:"files"`
}

// ManifestPath is the lock file inside an installed plugin directory.
func ManifestPath(dir string) string { return filepath.Join(dir, ManifestName) }

// LoadManifest reads the lock file in dir. It returns [ErrNoManifest] when the
// directory holds no lock file, and a decode error when one is unreadable.
func LoadManifest(dir string) (Manifest, error) {
	data, err := os.ReadFile(ManifestPath(dir))
	if err != nil {
		if os.IsNotExist(err) {
			return Manifest{}, fmt.Errorf("%w: %s", ErrNoManifest, dir)
		}
		return Manifest{}, fmt.Errorf("failed to read plugin manifest %s: %w", ManifestPath(dir), err)
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return Manifest{}, fmt.Errorf("failed to parse plugin manifest %s: %w", ManifestPath(dir), err)
	}
	if m.Version > ManifestVersion {
		return Manifest{}, fmt.Errorf("plugin manifest %s has version %d, this Crush reads at most %d", ManifestPath(dir), m.Version, ManifestVersion)
	}
	if m.Source == "" || m.Commit == "" {
		return Manifest{}, fmt.Errorf("plugin manifest %s is missing its source or commit", ManifestPath(dir))
	}
	return m, nil
}

// Save writes the lock file atomically, so a crash mid-install cannot leave a
// half-written record that the next run trusts.
func (m Manifest) Save(dir string) error {
	m.Version = ManifestVersion
	if err := os.MkdirAll(dir, dirPerm); err != nil {
		return fmt.Errorf("failed to create %s: %w", dir, err)
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode plugin manifest: %w", err)
	}
	data = append(data, '\n')
	if err := fsext.AtomicWriteFile(ManifestPath(dir), data, manifestPerm); err != nil {
		return fmt.Errorf("failed to write plugin manifest: %w", err)
	}
	return nil
}

// ShortCommit is the commit in the form a person recognizes in a log.
func (m Manifest) ShortCommit() string {
	if len(m.Commit) <= 8 {
		return m.Commit
	}
	return m.Commit[:8]
}

// FileNames lists the recorded plugin paths in name order.
func (m Manifest) FileNames() []string {
	names := make([]string, 0, len(m.Files))
	for _, f := range m.Files {
		names = append(names, f.Path)
	}
	slices.Sort(names)
	return names
}

// Diff is the difference between a lock record and the directory on disk.
type Diff struct {
	Added   []string
	Removed []string
	Changed []string
}

// Empty reports whether the directory still matches what was installed.
func (d Diff) Empty() bool {
	return len(d.Added) == 0 && len(d.Removed) == 0 && len(d.Changed) == 0
}

// Names summarizes a diff for a one-line message.
func (d Diff) Names() []string {
	var all []string
	all = append(all, d.Added...)
	all = append(all, d.Removed...)
	all = append(all, d.Changed...)
	slices.Sort(all)
	return all
}

// Diff compares the recorded content with the files now in dir, which is how
// an update knows it would overwrite a local edit.
func (m Manifest) Diff(dir string) (Diff, error) {
	now, err := HashDir(dir)
	if err != nil {
		return Diff{}, err
	}
	want := make(map[string]string, len(m.Files))
	for _, f := range m.Files {
		want[f.Path] = f.SHA256
	}
	have := make(map[string]string, len(now))
	for _, f := range now {
		have[f.Path] = f.SHA256
	}
	var d Diff
	for _, f := range now {
		old, ok := want[f.Path]
		switch {
		case !ok:
			d.Added = append(d.Added, f.Path)
		case old != f.SHA256:
			d.Changed = append(d.Changed, f.Path)
		}
	}
	for _, f := range m.Files {
		if _, ok := have[f.Path]; !ok {
			d.Removed = append(d.Removed, f.Path)
		}
	}
	slices.Sort(d.Added)
	slices.Sort(d.Removed)
	slices.Sort(d.Changed)
	return d, nil
}

// PluginFiles lists the plugin scripts directly inside dir in name order,
// skipping hidden entries and anything nested deeper.
func PluginFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read %s: %w", dir, err)
	}
	var names []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, PluginExt) || strings.HasPrefix(name, ".") {
			continue
		}
		names = append(names, name)
	}
	slices.Sort(names)
	return names, nil
}

// HashDir fingerprints the plugin scripts currently in dir.
func HashDir(dir string) ([]File, error) {
	names, err := PluginFiles(dir)
	if err != nil {
		return nil, err
	}
	files := make([]File, 0, len(names))
	for _, name := range names {
		f, err := HashFile(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		f.Path = name
		files = append(files, f)
	}
	return files, nil
}

// HashFile returns the digest and size of one file.
func HashFile(path string) (File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return File{}, fmt.Errorf("failed to read %s: %w", path, err)
	}
	sum := sha256.Sum256(data)
	return File{SHA256: hex.EncodeToString(sum[:]), Size: int64(len(data))}, nil
}

// Digest returns the digest of bytes, used for content that has not been
// written yet.
func Digest(data []byte) File {
	sum := sha256.Sum256(data)
	return File{SHA256: hex.EncodeToString(sum[:]), Size: int64(len(data))}
}

// Trust re-pins the lock record to the files as they are on disk. A plugin that
// is edited after install stays exactly as dangerous as it was before, so the
// only honest repair is to make the record describe it.
func Trust(root string, src Source) (Manifest, Diff, error) {
	dir := Dir(root, src)
	m, err := LoadManifest(dir)
	if err != nil {
		return Manifest{}, Diff{}, err
	}
	d, err := m.Diff(dir)
	if err != nil {
		return Manifest{}, Diff{}, err
	}
	files, err := HashDir(dir)
	if err != nil {
		return Manifest{}, Diff{}, err
	}
	if len(files) == 0 {
		return Manifest{}, Diff{}, fmt.Errorf("%s has no %s files to trust", dir, PluginExt)
	}
	m.Files = files
	if err := m.Save(dir); err != nil {
		return Manifest{}, Diff{}, err
	}
	return m, d, nil
}
