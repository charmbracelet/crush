package index

import (
	"context"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// churnCommits caps how much history feeds the churn signal. Recent
// churn is the useful ranking signal anyway, and the bound keeps a
// pathological history (hundreds of MB of --name-only output) from
// dominating build time.
const churnCommits = 2000

// rebuildChurn rewrites the churn table from one `git log` pass:
// touches-per-file over recent history. A failed refresh (git missing,
// not a work tree, no commits) keeps the previous build's rows; a
// successful pass that produced nothing still rewrites — the same
// staleness by another route.
func (s *Service) rebuildChurn(ctx context.Context, seen map[string]walkedFile) {
	top, err := gitOutput(ctx, s.root, "rev-parse", "--show-toplevel")
	if err != nil {
		return
	}
	// Git reports paths relative to the repository root — when the
	// indexed root is a subdirectory, strip that prefix so paths line
	// up with the index's project-relative keys. EvalSymlinks first:
	// macOS /tmp-style aliasing makes the two spellings diverge.
	prefix := ""
	if rootResolved, err1 := filepath.EvalSymlinks(s.root); err1 == nil {
		if topResolved, err2 := filepath.EvalSymlinks(top); err2 == nil && topResolved != rootResolved {
			if rel, err3 := filepath.Rel(topResolved, rootResolved); err3 == nil && rel != "." &&
				rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				prefix = filepath.ToSlash(rel) + "/"
			}
		}
	}
	out, err := gitOutput(ctx, s.root, "log", "--no-renames", "--name-only", "-z",
		"--pretty=format:", "-n", strconv.Itoa(churnCommits))
	if err != nil {
		return
	}
	touches := map[string]int{}
	// -z output is NUL-terminated paths with a format newline between
	// commits — splitting on either keeps quoted/special names intact.
	for _, name := range strings.FieldsFunc(out, func(r rune) bool { return r == 0 || r == '\n' }) {
		if prefix != "" {
			if !strings.HasPrefix(name, prefix) {
				continue // Outside the indexed subtree.
			}
			name = strings.TrimPrefix(name, prefix)
		}
		if _, ok := seen[name]; ok {
			touches[name]++
		}
	}
	// The log pass succeeded — rewrite unconditionally so an empty
	// result clears rows that no longer reflect history.
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM churn`); err != nil {
		return
	}
	st, err := tx.PrepareContext(ctx, `INSERT INTO churn (path, touches) VALUES (?, ?)`)
	if err != nil {
		return
	}
	for p, n := range touches {
		if _, err := st.ExecContext(ctx, p, n); err != nil {
			st.Close()
			return
		}
	}
	if err := st.Close(); err != nil {
		return
	}
	tx.Commit()
}

// churnCounts loads the churn table — one row per tracked path.
func (s *Service) churnCounts(ctx context.Context) map[string]int {
	rows, err := s.db.QueryContext(ctx, `SELECT path, touches FROM churn`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var p string
		var n int
		if err := rows.Scan(&p, &n); err != nil {
			return nil
		}
		out[p] = n
	}
	return out
}

func gitOutput(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
