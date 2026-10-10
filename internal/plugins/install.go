package plugins

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/charmbracelet/crush/internal/fsext"
)

// Options describes one install: which repository, which plugins directory it
// lands in, and which ref to follow.
type Options struct {
	Root   string
	Source Source
	// Ref is the branch, tag, or commit to install. Empty follows the
	// repository's default branch, and an existing install keeps the ref it
	// was installed from.
	Ref    string
	GitHub GitHub
}

// Result reports what an install or update changed, which is everything the
// command needs to print without going back to disk.
type Result struct {
	Dir      string
	Manifest Manifest
	// Previous is the commit replaced by this install, empty on a first one.
	Previous string
	Added    []string
	Removed  []string
	Changed  []string
	// UpToDate means the tracked ref still points at the installed commit, so
	// nothing was written.
	UpToDate bool
}

// Install fetches a repository and writes its plugins, recording the commit
// every file came from. Installing a repository that is already installed
// refreshes it, so the operation is idempotent.
func Install(ctx context.Context, opts Options) (Result, error) {
	if !opts.Source.Valid() {
		return Result{}, fmt.Errorf("invalid repository %q", opts.Source)
	}
	dir := Dir(opts.Root, opts.Source)

	previous, err := LoadManifest(dir)
	switch {
	case errors.Is(err, ErrNoManifest):
	case err != nil:
		return Result{}, err
	case !strings.EqualFold(previous.Source, opts.Source.String()):
		// Two spellings can land in one directory: a__b/c and a/b__c share a
		// name. Refuse rather than mix two repositories' files together.
		return Result{}, fmt.Errorf("%s already holds plugins from %s", dir, previous.Source)
	}

	ref := cmp.Or(opts.Ref, previous.Ref, DefaultRef)
	commit, err := opts.GitHub.ResolveRef(ctx, opts.Source, ref)
	if err != nil {
		return Result{}, err
	}
	if previous.Commit != "" && previous.Commit == commit.SHA {
		return Result{Dir: dir, Manifest: previous, Previous: previous.Commit, UpToDate: true}, nil
	}

	fetched, err := opts.GitHub.FetchPlugins(ctx, opts.Source, commit.SHA, opts.Root)
	if err != nil {
		return Result{}, err
	}

	now := time.Now().UTC()
	manifest := Manifest{
		Version:     ManifestVersion,
		Source:      opts.Source.String(),
		Ref:         ref,
		Commit:      commit.SHA,
		CommitURL:   commit.HTMLURL,
		InstalledAt: now,
		UpdatedAt:   now,
	}
	if !previous.InstalledAt.IsZero() {
		manifest.InstalledAt = previous.InstalledAt
	}
	for _, f := range fetched {
		manifest.Files = append(manifest.Files, f.entry())
	}
	slices.SortFunc(manifest.Files, func(a, b File) int { return strings.Compare(a.Path, b.Path) })

	before := previous.FileNames()
	after := manifest.FileNames()
	res := Result{
		Dir:      dir,
		Manifest: manifest,
		Previous: previous.Commit,
		Added:    subtract(after, before),
		Removed:  subtract(before, after),
		Changed:  intersect(before, after),
	}

	if err := os.MkdirAll(dir, dirPerm); err != nil {
		return Result{}, fmt.Errorf("failed to create %s: %w", dir, err)
	}
	for _, f := range fetched {
		if err := fsext.AtomicWriteFile(filepath.Join(dir, f.Path), f.Content, scriptPerm); err != nil {
			return Result{}, fmt.Errorf("failed to install %s: %w", f.Path, err)
		}
	}
	// A plugin deleted upstream has to go, or the load keeps running a file
	// the recorded commit no longer contains.
	for _, name := range res.Removed {
		if err := os.Remove(filepath.Join(dir, name)); err != nil && !os.IsNotExist(err) {
			return Result{}, fmt.Errorf("failed to remove %s: %w", name, err)
		}
	}
	// The lock is written last: until it exists the directory describes
	// nothing that was fetched.
	if err := manifest.Save(dir); err != nil {
		return Result{}, err
	}
	return res, nil
}

// fetched is one plugin script pulled from an archive. The recorded digest is
// computed from these bytes, so the lock describes what actually landed.
type fetched struct {
	Path    string
	Content []byte
}

func (f fetched) entry() File {
	d := Digest(f.Content)
	d.Path = f.Path
	return d
}

// FetchPlugins downloads the archive for one commit and returns the plugin
// scripts at its root. Staging happens inside root so writing a file into the
// install directory cannot cross a filesystem boundary.
func (g GitHub) FetchPlugins(ctx context.Context, src Source, sha, root string) ([]fetched, error) {
	if err := os.MkdirAll(root, dirPerm); err != nil {
		return nil, fmt.Errorf("failed to create %s: %w", root, err)
	}
	stage, err := os.MkdirTemp(root, ".fetch-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create a staging directory in %s: %w", root, err)
	}
	defer os.RemoveAll(stage)

	body, err := g.Tarball(ctx, src, sha)
	if err != nil {
		return nil, err
	}
	defer body.Close()

	if _, err := ExtractTarGz(body, stage); err != nil {
		return nil, err
	}
	names, err := PluginFiles(stage)
	if err != nil {
		return nil, err
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("no plugins found: %s has no %s files at the repository root", src, PluginExt)
	}

	files := make([]fetched, 0, len(names))
	for _, name := range names {
		content, err := os.ReadFile(filepath.Join(stage, name))
		if err != nil {
			return nil, fmt.Errorf("failed to read %s: %w", name, err)
		}
		files = append(files, fetched{Path: name, Content: content})
	}
	return files, nil
}

func subtract(a, b []string) []string {
	var out []string
	for _, s := range a {
		if !slices.Contains(b, s) {
			out = append(out, s)
		}
	}
	return out
}

func intersect(a, b []string) []string {
	var out []string
	for _, s := range a {
		if slices.Contains(b, s) {
			out = append(out, s)
		}
	}
	return out
}
