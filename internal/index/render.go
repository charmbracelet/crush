package index

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

// skeletonTopFiles caps how many high-centrality files the skeleton
// expands with their symbol lists.
const skeletonTopFiles = 15

// symDef is a symbol's definition site: file plus the extracted tag.
type symDef struct {
	path string
	tag  tag
}

// Skeleton renders the ranked project overview: directory layout with
// file counts, then the most-referenced files with their exported
// symbols. maxTokens bounds the output at ~4 bytes/token.
func (s *Service) Skeleton(ctx context.Context, maxTokens int) (string, error) {
	if err := s.init(); err != nil {
		return "", err
	}
	s.ensureStarted(ctx)
	var b strings.Builder
	// Report the last completed WALK, not MAX(indexed_at) — lazy
	// re-tags bump indexed_at per write, which would claim the whole
	// layout is fresher than it is.
	stamp := "never"
	if lb := s.lastBuild.Load(); lb != 0 {
		stamp = time.Unix(0, lb).Format(time.RFC3339)
	} else if ts := s.indexedAt(ctx); !ts.IsZero() {
		stamp = ts.Format(time.RFC3339) // Process restart fallback.
	}
	switch {
	case s.indexing.Load():
		fmt.Fprintf(&b, "Project map — indexing in progress, %d files so far (partial; call again shortly for the full map)\n", s.fileCount(ctx))
	case s.err() != nil:
		fmt.Fprintf(&b, "Project map — index build failed: %v. Fall back to grep/glob.\n", s.err())
	default:
		fmt.Fprintf(&b, "Project map — %d files indexed (built %s)\n", s.fileCount(ctx), stamp)
	}

	if err := s.renderDirTree(ctx, &b, maxChars(maxTokens)/2); err != nil {
		return "", err
	}
	if err := s.renderTopFiles(ctx, &b, skeletonTopFiles); err != nil {
		return "", err
	}
	return capOutput(b.String(), maxTokens), nil
}

// Subtree renders every indexed file under relPath with its symbols.
// "." and "/" address the project root — every indexed file.
func (s *Service) Subtree(ctx context.Context, relPath string, maxTokens int) (string, error) {
	if err := s.init(); err != nil {
		return "", err
	}
	s.ensureStarted(ctx)
	if filepath.IsAbs(relPath) && relPath != "/" {
		return fmt.Sprintf("Path %q is absolute — pass a project-relative directory (e.g. \"internal/agent\"), or \".\" for the root.", relPath), nil
	}
	relPath = strings.Trim(filepath.ToSlash(filepath.Clean(relPath)), "/")
	if relPath == ".." || strings.HasPrefix(relPath, "../") {
		return fmt.Sprintf("Path %q escapes the project root — the index covers project-relative paths only.", relPath), nil
	}

	// LIKE metacharacters in the path are escaped so the input stays a
	// literal prefix.
	where := `path = ? OR path LIKE ? ESCAPE '\'`
	args := []any{relPath, escapeLike(relPath) + "/%"}
	if relPath == "" || relPath == "." {
		where, args, relPath = "1 = 1", nil, "."
	}

	// Refresh candidate paths BEFORE reading their symbols — re-tagging
	// after the rows are materialized would render pre-refresh data.
	pathRows, err := s.db.QueryContext(ctx,
		`SELECT path FROM files WHERE `+where+` ORDER BY path`, args...)
	if err != nil {
		return "", err
	}
	var paths []string
	for pathRows.Next() {
		var p string
		if err := pathRows.Scan(&p); err != nil {
			pathRows.Close()
			return "", err
		}
		paths = append(paths, p)
	}
	pathRows.Close()
	if len(paths) == 0 {
		return fmt.Sprintf("No indexed files under %q.%s", relPath, s.indexingSuffix()), nil
	}
	s.refreshPaths(ctx, paths)

	rows, err := s.db.QueryContext(ctx, `
		SELECT s.path, s.name, s.kind, s.line, s.exported
		FROM symbols s
		WHERE `+where+`
		ORDER BY s.path, s.line`, args...)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	byFile := map[string][]tag{}
	for rows.Next() {
		var p, name, kind string
		var line, exported int
		if err := rows.Scan(&p, &name, &kind, &line, &exported); err != nil {
			return "", err
		}
		byFile[p] = append(byFile[p], tag{name: name, kind: kind, line: line, exported: exported == 1})
	}
	if err := rows.Err(); err != nil {
		return "", err
	}

	var b strings.Builder
	if len(paths) == 1 && paths[0] == relPath {
		fmt.Fprintf(&b, "File %s:\n", relPath)
	} else {
		fmt.Fprintf(&b, "Contents of %s/:\n", relPath)
	}
	for _, p := range paths {
		fmt.Fprintf(&b, "\n%s\n", p)
		writeTags(&b, byFile[p])
	}
	return capOutput(b.String(), maxTokens), nil
}

// Symbol finds definitions of name plus the files that reference the
// files defining it.
func (s *Service) Symbol(ctx context.Context, name string, maxTokens int) (string, error) {
	if err := s.init(); err != nil {
		return "", err
	}
	s.ensureStarted(ctx)
	defs, err := s.symbolDefs(ctx, name)
	if err != nil {
		return "", err
	}
	if len(defs) == 0 {
		// A miss can mean the symbol was added to an already-indexed
		// file after the last walk — stat-scan for dirty files once
		// before reporting the miss. Brand-new files still require a
		// re-walk; that is the as-of-build boundary.
		s.refreshDirty(ctx)
		defs, err = s.symbolDefs(ctx, name)
		if err != nil {
			return "", err
		}
	}
	if len(defs) == 0 {
		return fmt.Sprintf("No symbol named %q in the index.%s", name, s.indexingSuffix()), nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Symbol %q — %d definition(s):\n", name, len(defs))
	for _, d := range defs {
		vis := "unexported"
		if d.tag.exported {
			vis = "exported"
		}
		fmt.Fprintf(&b, "  %s:%d  %s (%s)\n", d.path, d.tag.line, d.tag.kind, vis)
	}

	// Referrers: files whose refs point at a defining file or its dir.
	queryRefs := func() (map[string]bool, error) {
		seen := map[string]bool{}
		for _, d := range defs {
			dir := filepath.ToSlash(filepath.Dir(d.path))
			rows, err := s.db.QueryContext(ctx, `
				SELECT DISTINCT src_path FROM refs
				WHERE dst_path = ? OR dst_path = ?`, d.path, dir)
			if err != nil {
				return nil, err
			}
			for rows.Next() {
				var src string
				if err := rows.Scan(&src); err == nil && !seen[src] {
					seen[src] = true
				}
			}
			rows.Close()
		}
		return seen, nil
	}
	seen, err := queryRefs()
	if err != nil {
		return "", err
	}
	if len(seen) > 0 {
		// Refresh candidates so a file that dropped the import in an
		// external edit doesn't still render as a referrer — then
		// re-query against the post-refresh refs.
		cands := make([]string, 0, len(seen))
		for r := range seen {
			cands = append(cands, r)
		}
		s.refreshPaths(ctx, cands)
		if seen, err = queryRefs(); err != nil {
			return "", err
		}
	}
	if len(seen) > 0 {
		var refs []string
		for r := range seen {
			refs = append(refs, r)
		}
		sort.Strings(refs)
		fmt.Fprintf(&b, "\nReferenced by %d file(s):\n", len(refs))
		for _, r := range refs {
			fmt.Fprintf(&b, "  %s\n", r)
		}
	}
	return capOutput(b.String(), maxTokens), nil
}

// symbolDefs returns name's definition sites. Candidate paths are
// refreshed before the rows are materialized — refreshing after the
// select would render pre-refresh data.
func (s *Service) symbolDefs(ctx context.Context, name string) ([]symDef, error) {
	pathRows, err := s.db.QueryContext(ctx,
		`SELECT DISTINCT path FROM symbols WHERE name = ?`, name)
	if err != nil {
		return nil, err
	}
	var cand []string
	for pathRows.Next() {
		var p string
		if err := pathRows.Scan(&p); err != nil {
			pathRows.Close()
			return nil, err
		}
		cand = append(cand, p)
	}
	pathRows.Close()
	s.refreshPaths(ctx, cand)

	rows, err := s.db.QueryContext(ctx, `
		SELECT path, name, kind, line, exported FROM symbols
		WHERE name = ? ORDER BY exported DESC, path`, name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var defs []symDef
	for rows.Next() {
		var p, n, k string
		var line, exp int
		if err := rows.Scan(&p, &n, &k, &line, &exp); err != nil {
			return nil, err
		}
		defs = append(defs, symDef{p, tag{name: n, kind: k, line: line, exported: exp == 1}})
	}
	return defs, rows.Err()
}

// renderDirTree writes the two-level directory overview with file
// counts, capped at maxOut bytes.
func (s *Service) renderDirTree(ctx context.Context, b *strings.Builder, maxOut int) error {
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			CASE WHEN instr(path, '/') = 0 THEN '.'
			     WHEN instr(substr(path, instr(path, '/') + 1), '/') = 0
			     THEN substr(path, 1, instr(path, '/') - 1)
			     ELSE substr(path, 1, instr(path, '/') - 1) ||
			          '/' || substr(substr(path, instr(path, '/') + 1), 1,
			          instr(substr(path, instr(path, '/') + 1), '/') - 1)
			END AS dir,
			COUNT(*)
		FROM files GROUP BY dir ORDER BY dir`)
	if err != nil {
		return err
	}
	defer rows.Close()

	b.WriteString("\nLayout:\n")
	start := b.Len()
	for rows.Next() {
		var dir string
		var n int
		if err := rows.Scan(&dir, &n); err != nil {
			return err
		}
		if b.Len()-start > maxOut {
			b.WriteString("  …\n")
			break
		}
		fmt.Fprintf(b, "  %s/ (%d)\n", dir, n)
	}
	return rows.Err()
}

// renderTopFiles writes the most-referenced files with their exported
// symbols. In-degree is computed in Go — a file inherits refs to
// itself plus every ancestor directory (Go package-dir imports) —
// because expressing that join in SQL degenerates to a nested loop
// of files × distinct ref targets with no usable index.
func (s *Service) renderTopFiles(ctx context.Context, b *strings.Builder, limit int) error {
	degRows, err := s.db.QueryContext(ctx,
		`SELECT dst_path, COUNT(*) FROM refs GROUP BY dst_path`)
	if err != nil {
		return err
	}
	refDeg := map[string]int{}
	for degRows.Next() {
		var dst string
		var n int
		if err := degRows.Scan(&dst, &n); err != nil {
			degRows.Close()
			return err
		}
		refDeg[dst] = n
	}
	if err := degRows.Err(); err != nil {
		degRows.Close()
		return err
	}
	degRows.Close()

	fileRows, err := s.db.QueryContext(ctx, `SELECT path FROM files`)
	if err != nil {
		return err
	}
	var paths []string
	for fileRows.Next() {
		var p string
		if err := fileRows.Scan(&p); err != nil {
			fileRows.Close()
			return err
		}
		paths = append(paths, p)
	}
	if err := fileRows.Err(); err != nil {
		fileRows.Close()
		return err
	}
	fileRows.Close()
	if len(paths) == 0 {
		return nil
	}

	// O(F × depth): a file's degree sums ref counts over itself and
	// each ancestor dir.
	deg := make(map[string]int, len(paths))
	maxDeg := 0
	for _, p := range paths {
		deg[p] = refDeg[p]
		for d := path.Dir(p); d != "." && d != "/" && d != ""; d = path.Dir(d) {
			deg[p] += refDeg[d]
		}
		if deg[p] > maxDeg {
			maxDeg = deg[p]
		}
	}

	// Blend git churn with ref-degree so recently hot files rank with
	// well-referenced ones. Each signal is normalized to its own max
	// — raw addition would let whichever count runs larger dominate.
	// Empty churn (non-git project) reduces the score to pure degree.
	churn := s.churnCounts(ctx)
	maxChurn := 0
	for _, p := range paths {
		if churn[p] > maxChurn {
			maxChurn = churn[p]
		}
	}
	score := func(p string) float64 {
		var v float64
		if maxDeg > 0 {
			v = float64(deg[p]) / float64(maxDeg)
		}
		if maxChurn > 0 {
			v += float64(churn[p]) / float64(maxChurn)
		}
		return v
	}
	sort.Slice(paths, func(i, j int) bool {
		if si, sj := score(paths[i]), score(paths[j]); si != sj {
			return si > sj
		}
		return paths[i] < paths[j]
	})
	if len(paths) > limit {
		paths = paths[:limit]
	}
	s.refreshPaths(ctx, paths)

	b.WriteString("\nMost-referenced files:\n")
	for _, p := range paths {
		syms, err := s.fileSymbols(ctx, p, true)
		if err != nil {
			return err
		}
		// deg includes ancestor-dir (package) refs — label it so a
		// zero-direct-ref file in a hot package doesn't read as a
		// lie. The commits count is the churn half of the blend.
		fmt.Fprintf(b, "  %s (%d refs incl. pkg", p, deg[p])
		if n := churn[p]; n == 1 {
			b.WriteString(", 1 commit")
		} else if n > 0 {
			fmt.Fprintf(b, ", %d commits", n)
		}
		b.WriteString(")")
		if len(syms) > 0 {
			names := make([]string, 0, len(syms))
			for _, t := range syms {
				names = append(names, t.name)
			}
			fmt.Fprintf(b, " — %s", strings.Join(names, ", "))
		}
		b.WriteString("\n")
	}
	return nil
}

// fileSymbols returns a file's tags, exported-first.
func (s *Service) fileSymbols(ctx context.Context, path string, exportedOnly bool) ([]tag, error) {
	q := `SELECT name, kind, line, exported FROM symbols WHERE path = ?`
	if exportedOnly {
		q += ` AND exported = 1`
	}
	q += ` ORDER BY line`
	rows, err := s.db.QueryContext(ctx, q, path)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []tag
	for rows.Next() {
		var name, kind string
		var line, exp int
		if err := rows.Scan(&name, &kind, &line, &exp); err != nil {
			return nil, err
		}
		out = append(out, tag{name: name, kind: kind, line: line, exported: exp == 1})
	}
	return out, rows.Err()
}

// refreshPaths lazily re-tags result paths whose mtime changed since
// indexing — bounded to the result set, never a full rescan.
func (s *Service) refreshPaths(ctx context.Context, paths []string) {
	for _, p := range paths {
		info, err := os.Stat(filepath.Join(s.root, filepath.FromSlash(p)))
		if errors.Is(err, fs.ErrNotExist) {
			s.dropFile(ctx, p) // Gone on disk — drop the ghost row.
			continue
		}
		if err != nil {
			continue // Transient stat failure — keep the stale row.
		}
		s.refreshIfStale(ctx, p, info.ModTime().UnixNano(), info.Size())
	}
}

func writeTags(b *strings.Builder, tags []tag) {
	for _, t := range tags {
		marker := " "
		if t.exported {
			marker = "+"
		}
		fmt.Fprintf(b, "  %s %4d  %-7s %s\n", marker, t.line, t.kind, t.name)
	}
}

// escapeLike escapes LIKE metacharacters (and the escape char itself)
// so a caller-supplied path can't act as a pattern.
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

// maxOutputTokens ceilings the caller-supplied budget — a model
// asking for max_tokens=50000 shouldn't get a 200KB response.
const maxOutputTokens = 2000

func maxChars(maxTokens int) int {
	if maxTokens <= 0 {
		maxTokens = 500
	}
	if maxTokens > maxOutputTokens {
		maxTokens = maxOutputTokens
	}
	return maxTokens * 4
}

func capOutput(s string, maxTokens int) string {
	max := maxChars(maxTokens)
	if len(s) <= max {
		return s
	}
	// Back the cut off a truncated tail rune only — validating the
	// whole prefix is O(cut²) and one bad byte anywhere would eat
	// the output.
	cut := max
	for cut > 0 {
		r, size := utf8.DecodeLastRuneInString(s[:cut])
		if r != utf8.RuneError || size > 1 {
			break
		}
		cut -= size
	}
	return s[:cut] + "\n\n[Map truncated to stay within token budget]"
}

// indexingSuffix marks misses produced while a build is in flight —
// a partial-build "not found" must not read as authoritative.
func (s *Service) indexingSuffix() string {
	if s.indexing.Load() {
		return " (index still building — result may be incomplete; retry shortly or fall back to grep/glob)"
	}
	return ""
}
