package agent

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// snapshotTimeout bounds all git calls made for one worktree snapshot or
// comparison.
const snapshotTimeout = 5 * time.Second

// fileState is the cheap fingerprint of one dirty or untracked file.
type fileState struct {
	size    int64
	mtime   int64
	missing bool
}

// worktreeSnapshot records the git HEAD and the state of every modified or
// untracked, non-ignored file at one moment.
type worktreeSnapshot struct {
	top   string
	head  string
	files map[string]fileState
}

// turnWindow is the worktree state at the start of a background turn.
type turnWindow struct {
	before    *worktreeSnapshot
	startedAt time.Time
}

// gitOutput runs git in dir and returns its stdout.
func gitOutput(ctx context.Context, dir string, args ...string) (string, error) {
	out, err := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...).Output()
	return string(out), err
}

// takeWorktreeSnapshot snapshots the git worktree containing dir. It returns
// false when dir is not in a git repo or any git call fails.
func takeWorktreeSnapshot(ctx context.Context, dir string) (*worktreeSnapshot, bool) {
	ctx, cancel := context.WithTimeout(ctx, snapshotTimeout)
	defer cancel()

	top, err := gitOutput(ctx, dir, "rev-parse", "--show-toplevel")
	if err != nil {
		slog.Debug("Skipping worktree snapshot", "dir", dir, "error", err)
		return nil, false
	}
	top = strings.TrimSpace(top)
	if resolved, err := filepath.EvalSymlinks(top); err == nil {
		top = resolved
	}
	// A repo with no commits has no HEAD, which is fine.
	head, _ := gitOutput(ctx, top, "rev-parse", "HEAD")
	listing, err := gitOutput(ctx, top, "ls-files", "-m", "-o", "--exclude-standard", "-z", "--full-name", ":/")
	if err != nil {
		slog.Debug("Skipping worktree snapshot", "dir", dir, "error", err)
		return nil, false
	}

	snap := &worktreeSnapshot{top: top, head: strings.TrimSpace(head), files: map[string]fileState{}}
	for _, rel := range strings.Split(listing, "\x00") {
		if rel == "" {
			continue
		}
		info, err := os.Lstat(filepath.Join(top, rel))
		if err != nil {
			snap.files[rel] = fileState{missing: true}
			continue
		}
		snap.files[rel] = fileState{size: info.Size(), mtime: info.ModTime().UnixNano()}
	}
	return snap, true
}

// changedBetween returns the sorted absolute paths whose state differs
// between before and after, or nil when either snapshot is nil.
func changedBetween(ctx context.Context, before, after *worktreeSnapshot) []string {
	if before == nil || after == nil {
		return nil
	}
	var rels []string
	for rel, state := range before.files {
		if now, ok := after.files[rel]; !ok || now != state {
			rels = append(rels, rel)
		}
	}
	for rel := range after.files {
		if _, ok := before.files[rel]; !ok {
			rels = append(rels, rel)
		}
	}
	if before.head != after.head {
		ctx, cancel := context.WithTimeout(ctx, snapshotTimeout)
		defer cancel()
		out, err := gitOutput(ctx, after.top, "diff", "--name-only", "-z", before.head, after.head)
		if err != nil {
			slog.Debug("Failed to diff worktree heads", "error", err)
		}
		rels = append(rels, strings.Split(out, "\x00")...)
	}
	paths := make([]string, 0, len(rels))
	for _, rel := range rels {
		if rel != "" {
			paths = append(paths, filepath.Join(after.top, rel))
		}
	}
	slices.Sort(paths)
	return slices.Compact(paths)
}
