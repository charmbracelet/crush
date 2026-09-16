package index

import (
	"context"
	"database/sql"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/charlievieth/fastwalk"
	"github.com/charmbracelet/crush/internal/fsext"
)

// walkedFile is one candidate source file seen during a walk.
type walkedFile struct {
	path  string
	mtime int64
	size  int64
}

// walk performs a full index pass: collects every candidate file,
// drops rows for vanished files, and re-tags new or mtime-changed
// files. Commits happen in batches bounded by walkBatchSize and
// walkBatchMax so queries can serve partial results during the build.
// Refs resolve against the complete post-walk path set — imports
// pointing at files already walked still resolve.
func (s *Service) walk(ctx context.Context) error {
	files, err := s.collect(ctx)
	if err != nil {
		return err
	}
	seen := make(map[string]walkedFile, len(files))
	knownDirs := map[string]bool{}
	for _, f := range files {
		seen[f.path] = f
		for d := filepath.ToSlash(filepath.Dir(f.path)); d != "." && d != "/" && d != ""; d = filepath.ToSlash(filepath.Dir(d)) {
			if knownDirs[d] {
				break
			}
			knownDirs[d] = true
		}
	}
	exists := func(p string) bool {
		if _, ok := seen[p]; ok {
			return true
		}
		return knownDirs[p]
	}
	modulePath := s.modulePath()

	// Reconcile: drop rows for files that vanished since last walk.
	// A NotifyWritten row can land after its directory was already
	// collected — absent from `seen` but live on disk — so stat
	// before dropping; a stale row self-heals via refreshPaths,
	// an erased live file doesn't.
	stored, err := s.storedFiles(ctx)
	if err != nil {
		return err
	}
	for p := range stored {
		if _, ok := seen[p]; ok {
			continue
		}
		if _, err := os.Stat(filepath.Join(s.root, filepath.FromSlash(p))); err == nil {
			continue
		}
		s.dropFile(ctx, p)
	}

	// Tag new and changed files in batches. The walk runs in the
	// background while queries serve the committed state — committing
	// every walkBatchSize files lets partial results accrue instead of
	// hiding everything behind one transaction.
	now := time.Now().Unix()
	n := 0
	var tx *sql.Tx
	var stmts *tagStmts
	var batchStart time.Time
	commit := func() error {
		if tx == nil {
			return nil
		}
		err := tx.Commit()
		stmts.close()
		tx, stmts = nil, nil
		return err
	}
	for path, f := range seen {
		if ctx.Err() != nil {
			commit()
			return ctx.Err()
		}
		if prev, ok := stored[path]; ok && prev.mtime == f.mtime && prev.size == f.size {
			continue // Unchanged.
		}
		tags, refs, tagErr := tagFile(s.root, path, modulePath, exists)
		if tagErr != nil {
			continue // Unreadable or vanished mid-walk; next pass catches it.
		}
		if tx == nil {
			tx, err = s.db.BeginTx(ctx, nil)
			if err != nil {
				return err
			}
			stmts, err = prepareTagStmts(ctx, tx)
			if err != nil {
				tx.Rollback()
				return err
			}
			batchStart = time.Now()
		}
		if err := stmts.tag(ctx, path, f, now, tags, refs); err != nil {
			tx.Rollback()
			return err
		}
		n++
		// Single-connection store: a commit is the only moment readers
		// can squeeze in. Bound the batch by count AND time so a slow
		// filesystem can't hold the write lock indefinitely.
		if n%walkBatchSize == 0 || time.Since(batchStart) > walkBatchMax {
			if err := commit(); err != nil {
				return err
			}
		}
	}
	if err := commit(); err != nil {
		return err
	}

	// Files lazily tagged while this build was in flight resolved
	// refs against a partial index — re-resolve them against the
	// complete `seen` set, or the undercount sticks until each file
	// next changes on disk.
	if pend := s.drainRefix(); len(pend) > 0 {
		if tx, err := s.db.BeginTx(ctx, nil); err == nil {
			if stmts, err := prepareTagStmts(ctx, tx); err == nil {
				for p := range pend {
					// Re-stat rather than trusting `seen` — a
					// concurrent touchFile may have moved the file
					// past collect's pair, and a stat-keep survivor
					// (live on disk, absent from seen) still
					// deserves its refs re-resolved.
					info, statErr := os.Stat(filepath.Join(s.root, filepath.FromSlash(p)))
					if statErr != nil || info.IsDir() || info.Size() == 0 {
						continue
					}
					f := walkedFile{path: p, mtime: info.ModTime().UnixNano(), size: info.Size()}
					tags, refs, tagErr := tagFile(s.root, p, modulePath, exists)
					if tagErr != nil || stmts.tag(ctx, p, f, now, tags, refs) != nil {
						continue
					}
				}
				stmts.close()
			}
			tx.Commit()
		}
	}

	// One git log pass refreshes the churn table — the skeleton's
	// hot-file ranking blends it with ref-degree. No-op off git.
	s.rebuildChurn(ctx, seen)
	return nil
}

// walkBatchSize is how many files a tagging transaction covers before
// committing — small enough that readers see progress during a cold
// build, large enough that per-commit cost stays amortized.
const walkBatchSize = 200

// walkBatchMax bounds how long one tagging transaction may hold the
// single connection before committing — slow files must not starve
// concurrent map queries.
const walkBatchMax = 500 * time.Millisecond

// tagStmts are the prepared statements one tagging transaction needs.
type tagStmts struct {
	upsertFile *sql.Stmt
	delSymbols *sql.Stmt
	delRefs    *sql.Stmt
	insSymbol  *sql.Stmt
	insRef     *sql.Stmt
}

func prepareTagStmts(ctx context.Context, tx *sql.Tx) (*tagStmts, error) {
	var s tagStmts
	var err error
	prepare := func(q string) *sql.Stmt {
		if err != nil {
			return nil
		}
		var st *sql.Stmt
		st, err = tx.PrepareContext(ctx, q)
		return st
	}
	s.upsertFile = prepare(`INSERT INTO files (path, mtime, size, indexed_at) VALUES (?, ?, ?, ?)
		ON CONFLICT(path) DO UPDATE SET mtime=excluded.mtime, size=excluded.size, indexed_at=excluded.indexed_at`)
	s.delSymbols = prepare(`DELETE FROM symbols WHERE path = ?`)
	s.delRefs = prepare(`DELETE FROM refs WHERE src_path = ?`)
	s.insSymbol = prepare(`INSERT INTO symbols (path, name, kind, line, exported) VALUES (?, ?, ?, ?, ?)`)
	s.insRef = prepare(`INSERT INTO refs (src_path, dst_path) VALUES (?, ?)`)
	if err != nil {
		s.close()
		return nil, err
	}
	return &s, nil
}

func (s *tagStmts) close() {
	for _, st := range []*sql.Stmt{s.upsertFile, s.delSymbols, s.delRefs, s.insSymbol, s.insRef} {
		if st != nil {
			st.Close()
		}
	}
}

// tag writes one file's rows inside the transaction.
func (s *tagStmts) tag(ctx context.Context, path string, f walkedFile, now int64, tags []tag, refs []string) error {
	if _, err := s.upsertFile.ExecContext(ctx, path, f.mtime, f.size, now); err != nil {
		return err
	}
	if _, err := s.delSymbols.ExecContext(ctx, path); err != nil {
		return err
	}
	if _, err := s.delRefs.ExecContext(ctx, path); err != nil {
		return err
	}
	for _, t := range tags {
		exported := 0
		if t.exported {
			exported = 1
		}
		if _, err := s.insSymbol.ExecContext(ctx, path, t.name, t.kind, t.line, exported); err != nil {
			return err
		}
	}
	for _, dst := range refs {
		if _, err := s.insRef.ExecContext(ctx, path, dst); err != nil {
			return err
		}
	}
	return nil
}

// collect walks the working tree with gitignore/crushignore rules
// plus the unconditional fastIgnoreDirs/commonIgnorePatterns set —
// dot-files like .github/ and .env ARE indexed (only dot-dirs in
// the built-in list are skipped).
func (s *Service) collect(ctx context.Context) ([]walkedFile, error) {
	walker := fsext.NewFastGlobWalker(s.root)
	var mu sync.Mutex // fastwalk invokes the callback from worker goroutines
	var files []walkedFile
	// A data-directory configured inside the project would otherwise
	// index its own scratch (index.db, crush-fetch-* dumps).
	var dataAbs string
	if filepath.IsAbs(s.dataDir) {
		if rel, err := filepath.Rel(s.root, s.dataDir); err == nil && rel != ".." &&
			!strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			dataAbs = filepath.Join(s.root, rel)
		}
	}
	conf := fastwalk.Config{
		Follow:  false,
		ToSlash: fastwalk.DefaultToSlash(),
		Sort:    fastwalk.SortFilesFirst,
	}
	err := fastwalk.Walk(&conf, s.root, func(path string, d os.DirEntry, err error) error {
		if ctx.Err() != nil {
			return filepath.SkipAll
		}
		if err != nil {
			return nil
		}
		if dataAbs != "" &&
			(path == dataAbs || strings.HasPrefix(path, dataAbs+string(filepath.Separator))) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			if walker.ShouldSkipDir(path) {
				return filepath.SkipDir
			}
			return nil
		}
		if walker.ShouldSkip(path) {
			return nil
		}
		info, err := d.Info()
		if err == nil && d.Type()&os.ModeSymlink != 0 {
			// tagFile reads the target — store the target's pair so a
			// target edit registers as staleness and the link's own
			// mtime doesn't trigger a re-tag on every refresh.
			info, err = os.Stat(path)
		}
		// IsDir: a symlink pointing at a directory reaches here (the
		// DirEntry check used lstat) — don't record it as a file.
		if err != nil || info.IsDir() || info.Size() == 0 {
			return nil
		}
		rel, err := filepath.Rel(s.root, path)
		if err != nil {
			return nil
		}
		mu.Lock()
		files = append(files, walkedFile{
			path:  filepath.ToSlash(rel),
			mtime: info.ModTime().UnixNano(),
			size:  info.Size(),
		})
		mu.Unlock()
		return nil
	})
	// fastwalk surfaces our ctx-cancel SkipAll as an error, not a
	// clean stop — translate it back to the context error so a
	// timed-out collect reports the real cause.
	if errors.Is(err, filepath.SkipAll) {
		return files, ctx.Err()
	}
	return files, err
}

// storedFiles returns the index's current path → {mtime, size} map.
func (s *Service) storedFiles(ctx context.Context) (map[string]walkedFile, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT path, mtime, size FROM files`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]walkedFile{}
	for rows.Next() {
		var f walkedFile
		if err := rows.Scan(&f.path, &f.mtime, &f.size); err != nil {
			return nil, err
		}
		out[f.path] = f
	}
	return out, rows.Err()
}

// readModulePath extracts the module path from go.mod when present —
// needed to map Go import paths back onto project directories.
func readModulePath(root string) string {
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return ""
	}
	for line := range strings.Lines(string(data)) {
		if m, ok := strings.CutPrefix(strings.TrimSpace(line), "module "); ok {
			return strings.TrimSpace(m)
		}
	}
	return ""
}

// refreshIfStale re-tags a single path when its on-disk {mtime, size}
// differs from the indexed pair — the lazy half of invalidation, used
// by renderers before they serve paths to the model.
func (s *Service) refreshIfStale(ctx context.Context, relPath string, currentMtime, size int64) {
	// Capture at entry: if a build is running, the exists probes
	// below may see committed-so-far rows — the commit-window check
	// at the end alone would miss probes that ran mid-build.
	midBuild := s.indexing.Load()
	if m, sz, found := s.indexedFile(ctx, relPath); found && m == currentMtime && sz == size {
		return
	}
	if prev, ok := s.tagFails.Load(relPath); ok {
		if f := prev.(walkedFile); f.mtime == currentMtime && f.size == size {
			return // Same file already failed tagging — retry on change.
		}
	}
	// exists covers directories too: Go imports resolve to package
	// dirs, and a probe limited to file rows would silently drop
	// every outgoing ref on each lazy re-tag.
	exists := func(p string) bool {
		if _, _, ok := s.indexedFile(ctx, p); ok {
			return true
		}
		return s.knownDir(ctx, p)
	}
	tags, refs, err := tagFile(s.root, relPath, s.modulePath(), exists)
	if err != nil {
		// Keep the stale row on a transient read failure, same as the
		// walk's tagErr path — drops happen only when the file stat
		// fails outright. Remember the pair so a permanently
		// unreadable file isn't re-attempted on every query.
		s.tagFails.Store(relPath, walkedFile{path: relPath, mtime: currentMtime, size: size})
		return
	}
	s.tagFails.Delete(relPath)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return
	}
	defer tx.Rollback()
	tx.ExecContext(ctx, `INSERT INTO files (path, mtime, size, indexed_at) VALUES (?, ?, ?, ?)
		ON CONFLICT(path) DO UPDATE SET mtime=excluded.mtime, size=excluded.size, indexed_at=excluded.indexed_at`,
		relPath, currentMtime, size, time.Now().Unix())
	tx.ExecContext(ctx, `DELETE FROM symbols WHERE path = ?`, relPath)
	tx.ExecContext(ctx, `DELETE FROM refs WHERE src_path = ?`, relPath)
	for _, t := range tags {
		exported := 0
		if t.exported {
			exported = 1
		}
		tx.ExecContext(ctx, `INSERT INTO symbols (path, name, kind, line, exported) VALUES (?, ?, ?, ?, ?)`,
			relPath, t.name, t.kind, t.line, exported)
	}
	for _, dst := range refs {
		tx.ExecContext(ctx, `INSERT INTO refs (src_path, dst_path) VALUES (?, ?)`, relPath, dst)
	}
	tx.Commit()
	// Refs just resolved against a possibly in-flight build — flag
	// for re-resolution at walk end so the undercount isn't sticky.
	if midBuild || s.indexing.Load() {
		s.markRefix(relPath)
	}
}

// refreshDirty stat-scans every indexed file and re-tags those whose
// {mtime, size} changed on disk — the fallback when a query misses
// (a symbol added to an already-indexed file mid-session). Bounded
// by file count, never a re-walk; files created since the last walk
// remain invisible until the next EnsureIndexed. Rate-limited to one
// scan per dirtyScanInterval — a model guessing wrong symbol names
// must not pay a full stat-scan per miss.
func (s *Service) refreshDirty(ctx context.Context) {
	now := time.Now().UnixNano()
	if last := s.lastDirtyScan.Load(); now-last < int64(dirtyScanInterval) ||
		!s.lastDirtyScan.CompareAndSwap(last, now) {
		return
	}

	stored, err := s.storedFiles(ctx)
	if err != nil {
		return
	}
	for p := range stored {
		info, err := os.Stat(filepath.Join(s.root, filepath.FromSlash(p)))
		if errors.Is(err, fs.ErrNotExist) {
			s.dropFile(ctx, p)
			continue
		}
		if err != nil {
			continue
		}
		s.refreshIfStale(ctx, p, info.ModTime().UnixNano(), info.Size())
	}
}

// dirtyScanInterval is the minimum spacing between full stat-scans
// triggered by symbol-miss fallback.
const dirtyScanInterval = 10 * time.Second
