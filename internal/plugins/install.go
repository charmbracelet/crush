package plugins

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io/fs"
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
	removeStaleStages(opts.Root, opts.Source)

	previous, err := LoadManifest(dir)
	switch {
	case errors.Is(err, ErrNoManifest):
		// Publishing replaces the directory whole, so an unmanaged one is refused, not deleted.
		if _, statErr := os.Stat(dir); statErr == nil {
			return Result{}, fmt.Errorf("%s exists but is not a managed install; move it aside first", dir)
		}
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

	if err := os.MkdirAll(opts.Root, dirPerm); err != nil {
		return Result{}, fmt.Errorf("failed to create %s: %w", opts.Root, err)
	}
	// The release is assembled in a hidden sibling the loader skips, so the
	// install directory only ever holds a complete, recorded release.
	stage, err := os.MkdirTemp(opts.Root, "."+opts.Source.DirName()+".stage-*")
	if err != nil {
		return Result{}, fmt.Errorf("failed to create a staging directory in %s: %w", opts.Root, err)
	}
	defer os.RemoveAll(stage)

	files, err := opts.GitHub.stageRelease(ctx, opts.Source, commit.SHA, stage)
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
		Files:       files,
	}
	if !previous.InstalledAt.IsZero() {
		manifest.InstalledAt = previous.InstalledAt
	}
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

	if err := manifest.Save(stage); err != nil {
		return Result{}, err
	}
	if err := publish(stage, dir); err != nil {
		return Result{}, err
	}
	return res, nil
}

// removeStaleStages deletes hidden staging siblings that failed runs left behind.
func removeStaleStages(root string, src Source) {
	entries, _ := os.ReadDir(root)
	prefix := "." + src.DirName() + ".stage-"
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), prefix) {
			_ = os.RemoveAll(filepath.Join(root, entry.Name()))
		}
	}
}

// renameDir is fsext.Rename, replaced by tests to fail a publish.
var renameDir = fsext.Rename

// publish swaps a complete release into dir. Renaming over a non-empty
// directory is not portable, so the old release is moved aside first.
func publish(stage, dir string) error {
	old := stage + ".old"
	switch err := renameDir(dir, old); {
	case errors.Is(err, fs.ErrNotExist):
		old = ""
	case err != nil:
		return fmt.Errorf("failed to move the previous release aside: %w", err)
	}
	if err := renameDir(stage, dir); err != nil {
		if old != "" {
			if restoreErr := renameDir(old, dir); restoreErr != nil {
				return fmt.Errorf("failed to publish %s: %w; previous release preserved at %s (restore failed: %v)", dir, err, old, restoreErr)
			}
		}
		return fmt.Errorf("failed to publish %s: %w", dir, err)
	}
	if old != "" {
		// A leftover is hidden, so the loader ignores it either way.
		_ = os.RemoveAll(old)
	}
	return nil
}

// stageRelease extracts one commit's archive into stage and prunes it to the
// plugin scripts at the repository root, returning their digests.
func (g GitHub) stageRelease(ctx context.Context, src Source, sha, stage string) ([]File, error) {
	body, err := g.Tarball(ctx, src, sha)
	if err != nil {
		return nil, err
	}
	defer body.Close()
	if _, err := ExtractTarGz(body, stage); err != nil {
		return nil, err
	}

	files, err := HashDir(stage)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no plugins found: %s has no %s files at the repository root", src, PluginExt)
	}
	entries, err := os.ReadDir(stage)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		path := filepath.Join(stage, entry.Name())
		if !slices.ContainsFunc(files, func(f File) bool { return f.Path == entry.Name() }) {
			err = os.RemoveAll(path)
		} else {
			err = os.Chmod(path, scriptPerm)
		}
		if err != nil {
			return nil, err
		}
	}
	return files, os.Chmod(stage, dirPerm)
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
