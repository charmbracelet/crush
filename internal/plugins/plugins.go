// Package plugins installs and updates provider plugins from GitHub.
//
// A plugin is a Bash script that declares providers with the same builtins a
// crushrc uses, and the config loader executes every non-hidden *.sh it finds
// in a plugins directory. This package is the installer half: it fetches a
// repository, records the exact commit each installed file came from, and
// keeps that record in a hidden lock file beside the scripts.
package plugins

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	// DefaultRef tracks whatever a repository calls its default branch.
	DefaultRef = "HEAD"

	// DirSeparator joins author and repo into a single plugin directory.
	DirSeparator = "__"

	// ManifestName is the hidden lock file inside an installed plugin
	// directory. It is hidden because the loader executes every *.sh it
	// finds, and a lock file is not a plugin.
	ManifestName = ".plugin.json"

	// PluginExt is the only extension the loader is willing to run.
	PluginExt = ".sh"

	// ProjectConfigDirName is the working directory's Crush config folder and
	// PluginDirName the folder inside it that the loader scans. Both mirror
	// internal/config, which owns the loading half.
	ProjectConfigDirName = ".crush"
	PluginDirName        = "plugins"

	// ManifestVersion is the lock file format this package reads and writes.
	ManifestVersion = 1
)

// File modes for installed content, matching how a hand-dropped plugin is
// written by convention.
const (
	scriptPerm   = 0o755
	manifestPerm = 0o644
	dirPerm      = 0o755
)

// namePattern is the shape GitHub allows for an owner or repository name.
// Validating it keeps a crafted spec like "../../etc" from escaping the
// plugins directory, since both parts become path components.
var namePattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// refPattern admits branch, tag, and commit names. The ref is placed into an
// API URL path, so anything that could inject a query, fragment, or traversal
// segment is refused instead of escaped.
var refPattern = regexp.MustCompile(`^[A-Za-z0-9._/-]+$`)

// Scope says which plugins directory an install lives in.
type Scope string

const (
	ScopeGlobal  Scope = "global"
	ScopeProject Scope = "project"
)

// ScopedRoot pairs a plugins directory with the scope it represents.
type ScopedRoot struct {
	Scope Scope
	Path  string
}

// Source is a plugin repository on GitHub.
type Source struct {
	Author string
	Repo   string
}

func (s Source) String() string { return s.Author + "/" + s.Repo }

// DirName is the plugin subdirectory a source installs into. GitHub treats
// owner and repository names case-insensitively, so they are lowercased to
// keep two spellings of one repository from colliding on case-insensitive
// filesystems.
func (s Source) DirName() string {
	return strings.ToLower(s.Author + DirSeparator + s.Repo)
}

// Valid reports whether both name parts can safely become path components.
func (s Source) Valid() bool {
	return safeName(s.Author) && safeName(s.Repo)
}

func safeName(name string) bool {
	if !namePattern.MatchString(name) {
		return false
	}
	// A name of only dots would still escape when joined into a path.
	return strings.Trim(name, ".") != ""
}

// ParseSource turns an author/repo[@ref] spec into a source and the ref to
// install from. The ref is empty when the spec does not name one, which the
// caller resolves to [DefaultRef].
func ParseSource(spec string) (Source, string, error) {
	name, ref, _ := strings.Cut(strings.TrimSpace(spec), "@")
	author, repo, ok := strings.Cut(name, "/")
	if !ok {
		return Source{}, "", fmt.Errorf("expected <author>/<repo>[@ref], got %q", spec)
	}
	src := Source{Author: author, Repo: repo}
	if !src.Valid() {
		return Source{}, "", fmt.Errorf("invalid repository %q: GitHub names may contain only letters, digits, dots, hyphens, and underscores", name)
	}
	if ref != "" && !ValidRef(ref) {
		return Source{}, "", fmt.Errorf("invalid ref %q", ref)
	}
	return src, ref, nil
}

// ValidRef reports whether ref can be placed into an API URL path. Dot-only
// segments are rejected: they survive the pattern but traverse the path.
func ValidRef(ref string) bool {
	if !refPattern.MatchString(ref) {
		return false
	}
	for part := range strings.SplitSeq(ref, "/") {
		if part == "" || strings.Trim(part, ".") == "" {
			return false
		}
	}
	return true
}

// Dir is the directory under root that a source installs into.
func Dir(root string, src Source) string {
	return filepath.Join(root, src.DirName())
}

// GlobalRoot is the user-wide plugins directory that sits beside a global
// config file, which is where an install without --project writes.
func GlobalRoot(globalConfigPath string) string {
	return filepath.Join(filepath.Dir(globalConfigPath), PluginDirName)
}

// Installed is one managed plugin directory found on disk.
type Installed struct {
	Scope    Scope
	Root     string
	Dir      string
	Manifest Manifest
	// Modified reports local edits made after the install.
	Modified bool
	// Err is set when the directory holds a lock file that cannot be read,
	// so a damaged install is reported instead of silently skipped.
	Err error
}

// Discover lists every managed install in the given roots, in root order and
// then directory name order. Directories without a lock file are hand-dropped
// plugin folders and are not reported.
func Discover(roots []ScopedRoot) ([]Installed, error) {
	var found []Installed
	for _, root := range roots {
		entries, err := os.ReadDir(root.Path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("failed to read %s: %w", root.Path, err)
		}
		for _, entry := range entries {
			if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
				continue
			}
			dir := filepath.Join(root.Path, entry.Name())
			m, err := LoadManifest(dir)
			if errors.Is(err, ErrNoManifest) {
				continue
			}
			install := Installed{Scope: root.Scope, Root: root.Path, Dir: dir, Manifest: m}
			if err != nil {
				install.Err = err
			} else if diff, err := m.Diff(dir); err == nil {
				install.Modified = !diff.Empty()
			} else {
				install.Err = err
			}
			found = append(found, install)
		}
	}
	return found, nil
}

// Find returns the installs for one source across the roots, which is how a
// command given only author/repo can act on a project and a global install of
// the same repository.
func Find(roots []ScopedRoot, src Source) ([]Installed, error) {
	all, err := Discover(roots)
	if err != nil {
		return nil, err
	}
	var matches []Installed
	for _, install := range all {
		if strings.EqualFold(install.Manifest.Source, src.String()) {
			matches = append(matches, install)
		}
	}
	return matches, nil
}

// Roots returns the plugins directories for a scope selection. Both scopes are
// included unless global and project say otherwise, and a caller that gets two
// false values is asking for nothing, so it gets both.
func Roots(globalRoot, cwd string, global, project bool) []ScopedRoot {
	if !global && !project {
		global, project = true, true
	}
	var roots []ScopedRoot
	if global {
		roots = append(roots, ScopedRoot{Scope: ScopeGlobal, Path: globalRoot})
	}
	if project {
		roots = append(roots, ScopedRoot{
			Scope: ScopeProject,
			Path:  ProjectRoot(cwd),
		})
	}
	return roots
}

// ProjectRoot is the plugins directory of a working directory's project config.
func ProjectRoot(cwd string) string {
	return filepath.Join(cwd, ProjectConfigDirName, PluginDirName)
}

// TrustNote is the standing warning for every installed plugin: a plugin is
// Bash that runs at config load with the user's shell privileges, so the
// review is the only check it gets.
const TrustNote = `Plugins are trusted code: each one is a Bash script that runs at
config load with your shell privileges. Review them before you use them.`
