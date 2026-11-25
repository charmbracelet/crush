package plugins

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
)

// UpdateOptions refreshes one managed install.
type UpdateOptions struct {
	Root   string
	Source Source
	// Ref changes which ref the install follows. Empty keeps the recorded one.
	Ref    string
	Force  bool
	GitHub GitHub
}

// ErrNotInstalled means the plugins directory holds no managed install for a
// source, so there is nothing to update, trust, or remove.
var ErrNotInstalled = errors.New("plugin is not installed")

// Update moves an install to the current commit of the ref it follows.
//
// Local edits are refused unless [UpdateOptions.Force] says otherwise: an
// update rewrites every recorded file, so silently overwriting them would lose
// work the lock file cannot distinguish from an install.
func Update(ctx context.Context, opts UpdateOptions) (Result, error) {
	dir := Dir(opts.Root, opts.Source)
	recorded, err := LoadManifest(dir)
	if err != nil {
		if errors.Is(err, ErrNoManifest) {
			return Result{}, fmt.Errorf("%w: %s in %s", ErrNotInstalled, opts.Source, opts.Root)
		}
		return Result{}, err
	}

	diff, err := recorded.Diff(dir)
	if err != nil {
		return Result{}, err
	}
	if !diff.Empty() && !opts.Force {
		return Result{}, fmt.Errorf(
			"%s has local changes (%s); run crush plugin trust %s to accept them, or pass --force to overwrite",
			opts.Source, strings.Join(diff.Names(), ", "), opts.Source,
		)
	}

	res, err := Install(ctx, Options{
		Root:   opts.Root,
		Source: opts.Source,
		Ref:    cmp.Or(opts.Ref, recorded.Ref, DefaultRef),
		GitHub: opts.GitHub,
	})
	if err != nil {
		return Result{}, err
	}
	res.Previous = recorded.Commit

	// Pointing the same commit at a new ref is still a change worth
	// recording, even when no file had to be rewritten.
	if res.UpToDate && opts.Ref != "" && opts.Ref != recorded.Ref {
		updated := recorded
		updated.Ref = opts.Ref
		if err := updated.Save(dir); err != nil {
			return Result{}, err
		}
		res.Manifest = updated
	}
	return res, nil
}

// Remove deletes an installed plugin repository and returns the record it had.
func Remove(root string, src Source) (Manifest, error) {
	dir := Dir(root, src)
	m, err := LoadManifest(dir)
	if err != nil {
		if errors.Is(err, ErrNoManifest) {
			return Manifest{}, fmt.Errorf("%w: %s in %s", ErrNotInstalled, src, root)
		}
		return Manifest{}, err
	}
	if err := os.RemoveAll(dir); err != nil {
		return Manifest{}, fmt.Errorf("failed to remove %s: %w", dir, err)
	}
	return m, nil
}
