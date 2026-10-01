package shell

import (
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
)

// Limits on how much bang-mode output is kept in the session and sent to
// the model. Commands like `git log` or `cat` on a large file can emit
// megabytes; keeping all of it would both blow up the context window and
// inflate the session forever.
const (
	maxPersistedLines = 2000
	maxPersistedBytes = 50 << 10
)

// spillSubdir is where oversized command output is parked, relative to the
// caller-supplied data directory.
const spillSubdir = "shell-output"

// spillPattern matches the files SpillOutput creates. Pruning is limited to
// this shape so a misconfigured directory cannot lose unrelated files.
const spillPattern = "output-*.log"

// spillRetention is how long a spill file outlives the command that wrote
// it. Long enough to resume yesterday's session and still read what it
// referenced, short enough that stale output does not pile up.
const spillRetention = 7 * 24 * time.Hour

// spillDirLimit caps the spill directory. Age alone does not bound disk:
// a busy afternoon of large tool output can fill it well inside the
// retention window, so the oldest files go early to stay under this.
const spillDirLimit = 256 << 20

// prunedDirs records the spill directories this process has already swept,
// so the sweep costs one ReadDir per directory rather than one per command.
var prunedDirs sync.Map

// TruncateForPersist bounds shell output before it is stored in the session.
// Output inside the limits is returned unchanged. Anything larger is written
// in full to a file under spillDir and replaced by its head and tail plus a
// pointer to that file, so the agent can read the omitted middle on demand.
//
// Truncated output is stripped of ANSI sequences: the cut points land in the
// middle of the stream, where an unterminated escape would bleed styling into
// the rest of the chat.
func TruncateForPersist(output, spillDir string) string {
	if withinBudget(output) {
		return output
	}

	// Colour codes are not content. Output that only crosses the limit
	// because of escape sequences is kept whole.
	plain := ansi.Strip(output)
	if withinBudget(plain) {
		return output
	}

	head, tail := headAndTail(plain)
	omitted := strings.Count(plain, "\n") -
		strings.Count(head, "\n") - strings.Count(tail, "\n")

	note := "output truncated"
	if omitted > 0 {
		note = fmt.Sprintf("%d lines omitted", omitted)
	}
	if path, err := SpillOutput(plain, spillDir); err == nil {
		note += ", full output: " + path
	} else {
		slog.Debug("Could not spill command output", "dir", spillDir, "error", err)
	}
	return head + "\n\n" + note + "\n\n" + tail
}

// withinBudget reports whether output is small enough to keep whole.
func withinBudget(output string) bool {
	return len(output) <= maxPersistedBytes &&
		strings.Count(output, "\n")+1 <= maxPersistedLines
}

// headAndTail splits output into a leading and a trailing part, each
// bounded by both the line and the byte budget.
//
// Lines are claimed from the front until the head budget runs out, then
// from the back until the tail budget does. Spending the budgets from both
// ends is what keeps the end of the output alive when it is only a few
// lines long but very wide: a plain lines[:n] / lines[len-n:] split hands
// every line to the head and leaves the tail empty.
func headAndTail(output string) (head, tail string) {
	lines := strings.Split(output, "\n")
	halfLines := maxPersistedLines / 2
	halfBytes := maxPersistedBytes / 2

	first, last := 0, len(lines)
	var headBytes, tailBytes int
	for first < last {
		if first < halfLines && headBytes+len(lines[first])+1 <= halfBytes {
			headBytes += len(lines[first]) + 1
			first++
			continue
		}
		if len(lines)-last < halfLines && tailBytes+len(lines[last-1])+1 <= halfBytes {
			tailBytes += len(lines[last-1]) + 1
			last--
			continue
		}
		break
	}

	head = strings.Join(lines[:first], "\n")
	tail = strings.Join(lines[last:], "\n")

	// A line wider than the budget fits in neither end. Keep as much of it
	// as the budget allows rather than returning nothing.
	if head == "" {
		head = clipHead(lines[0], halfBytes)
	}
	if tail == "" {
		tail = clipTail(lines[len(lines)-1], halfBytes)
	}
	return head, tail
}

// SpillOutput writes command output that was too large to keep in context to
// a file the agent can read back, and returns that file's path.
//
// Files land in a subdirectory of dir, which callers set to the project data
// directory so the agent's view tool can read them without a
// outside-the-working-directory permission prompt. An empty dir falls back to
// the system temp directory.
//
// Spill files outlive the command that wrote them, since the agent may want
// to read one later in the session. They are swept on a retention window
// instead; see [pruneSpills].
func SpillOutput(output, dir string) (string, error) {
	if dir == "" {
		// No data directory to own: fall back to the system temp
		// directory and leave pruning to the OS. Sweeping a shared
		// directory for a name we do not exclusively own would risk
		// deleting someone else's files.
		dir = os.TempDir()
	} else {
		dir = filepath.Join(dir, spillSubdir)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return "", err
		}
		pruneSpills(dir)
	}

	f, err := os.CreateTemp(dir, spillPattern)
	if err != nil {
		return "", err
	}
	defer f.Close() //nolint:errcheck
	if _, err := f.WriteString(output); err != nil {
		return "", err
	}
	return f.Name(), nil
}

// pruneSpills deletes spill files older than [spillRetention], then the
// oldest of what remains until the directory fits [spillDirLimit]. It runs
// once per directory per process. Failures are not worth surfacing: the
// spill this sweep precedes is still perfectly good.
func pruneSpills(dir string) {
	if _, swept := prunedDirs.LoadOrStore(dir, true); swept {
		return
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		slog.Debug("Could not read spill directory", "dir", dir, "error", err)
		return
	}

	cutoff := time.Now().Add(-spillRetention)
	var kept []fs.FileInfo
	var keptBytes int64
	for _, entry := range entries {
		if match, _ := filepath.Match(spillPattern, entry.Name()); !match {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			removeSpill(dir, info.Name())
			continue
		}
		kept = append(kept, info)
		keptBytes += info.Size()
	}

	// Still over the cap once the stale files are gone: drop the oldest
	// survivors until it fits.
	slices.SortFunc(kept, func(a, b fs.FileInfo) int {
		return a.ModTime().Compare(b.ModTime())
	})
	for _, info := range kept {
		if keptBytes <= spillDirLimit {
			break
		}
		removeSpill(dir, info.Name())
		keptBytes -= info.Size()
	}
}

func removeSpill(dir, name string) {
	if err := os.Remove(filepath.Join(dir, name)); err != nil {
		slog.Debug("Could not prune spill file", "name", name, "error", err)
	}
}

// clipHead returns at most n bytes from the start of s, cut on a rune
// boundary.
func clipHead(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
}

// clipTail returns at most n bytes from the end of s, cut on a rune
// boundary.
func clipTail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	i := len(s) - n
	for i < len(s) && !utf8.RuneStart(s[i]) {
		i++
	}
	return s[i:]
}
