package critic

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/crush/internal/diff"
)

// ComputeDiff produces a unified diff for the given paths. It prefers git diff
// for tracked files and falls back to the internal diff library for non-git
// environments or untracked files. If maxSize > 0, the output is truncated.
func ComputeDiff(changedPaths []string, snapshot *SnapshotStore, after map[string][]byte, maxSize int) (string, bool, error) {
	if len(changedPaths) == 0 {
		return "", false, nil
	}

	// Filter out binary files — they produce unreadable diffs.
	var textPaths []string
	for _, p := range changedPaths {
		content, ok := after[p]
		if !ok || content == nil {
			// File was deleted — no content to check.
			textPaths = append(textPaths, p)
			continue
		}
		if isBinary(content) {
			slog.Warn("Skipping binary file in critic diff", "path", p)
			continue
		}
		textPaths = append(textPaths, p)
	}
	if len(textPaths) == 0 {
		return "", false, nil
	}

	// For large change sets, skip git subprocess and use bounded library diff.
	useGit := len(textPaths) <= 10

	if useGit {
		if d, err := gitDiff(textPaths); err == nil && d != "" {
			if maxSize > 0 && len(d) > maxSize {
				return d[:maxSize] + "\n... (diff truncated)\n", true, nil
			}
			return d, false, nil
		}
	}

	// Fallback to internal diff library using snapshot before/after.
	return libraryDiff(textPaths, snapshot, after, maxSize)
}

func gitDiff(paths []string) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// Check if we're in a git repo.
	if _, err := exec.LookPath("git"); err != nil {
		return "", err
	}
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = cwd
	if err := cmd.Run(); err != nil {
		return "", err
	}

	args := append([]string{"diff", "--no-color"}, paths...)
	cmd = exec.Command("git", args...)
	cmd.Dir = cwd
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}

	// Also diff untracked files.
	var untracked []string
	for _, p := range paths {
		cmd := exec.Command("git", "ls-files", "--error-unmatch", p)
		cmd.Dir = cwd
		if err := cmd.Run(); err != nil {
			untracked = append(untracked, p)
		}
	}

	var sb strings.Builder
	sb.Write(out)

	for _, p := range untracked {
		cmd := exec.Command("git", "diff", "--no-color", "--no-index", os.DevNull, p)
		cmd.Dir = cwd
		out, err := cmd.Output()
		if err != nil {
			// git diff --no-index exits 1 when differences are found.
			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) {
				continue
			}
			// out already contains stdout even on error.
		}
		// Strip the a//dev/null header lines.
		lines := strings.Split(string(out), "\n")
		for i, line := range lines {
			if strings.HasPrefix(line, "--- "+os.DevNull) || strings.HasPrefix(line, "+++ "+os.DevNull) {
				continue
			}
			if i > 0 || line != "" {
				sb.WriteString(line)
				sb.WriteByte('\n')
			}
		}
	}

	return sb.String(), nil
}

func libraryDiff(paths []string, snapshot *SnapshotStore, after map[string][]byte, maxSize int) (string, bool, error) {
	var sb strings.Builder
	truncated := false
	for _, p := range paths {
		if maxSize > 0 && sb.Len() > maxSize {
			truncated = true
			break
		}

		before := ""
		if snap, ok := snapshot.stash[p]; ok && snap.Exists {
			before = string(snap.Content)
		}
		afterStr := ""
		if b, ok := after[p]; ok {
			afterStr = string(b)
		}

		rel, _ := filepath.Rel(".", p)
		if rel == "" {
			rel = p
		}

		// Detect binary files and skip diff.
		if isBinary([]byte(before)) || isBinary([]byte(afterStr)) {
			fmt.Fprintf(&sb, "Binary file %s differs\n", rel)
			continue
		}

		unified, _, _ := diff.GenerateDiff(before, afterStr, rel)
		sb.WriteString(unified)
		sb.WriteByte('\n')
	}

	result := sb.String()
	if maxSize > 0 && len(result) > maxSize {
		result = result[:maxSize] + "\n... (diff truncated)\n"
		truncated = true
	}
	return result, truncated, nil
}

// isBinary reports whether data appears to be binary (contains null bytes).
func isBinary(data []byte) bool {
	const sniffLen = 8192
	if len(data) > sniffLen {
		data = data[:sniffLen]
	}
	return bytes.Contains(data, []byte{0})
}
