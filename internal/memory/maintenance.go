package memory

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	defaultMaintenanceInterval = 24 * time.Hour
	retrievalRetention         = 30 * 24 * time.Hour
	rejectedRetention          = 30 * 24 * time.Hour
	supersededRetention        = 90 * 24 * time.Hour
	sessionStateRetention      = 90 * 24 * time.Hour
)

// MaintenanceResult describes deterministic cleanup performed on the memory
// store. Semantic merging remains the recorder's responsibility.
type MaintenanceResult struct {
	BackupPath          string
	RetrievalsPurged    int64
	RecordsPurged       int64
	SessionStatesPurged int64
}

func openHealthyDatabase(ctx context.Context, dir, dbPath string) (*sql.DB, error) {
	db, err := connectHealthyDatabase(ctx, dbPath)
	if err == nil {
		return db, nil
	}
	initialErr := err
	if restoreErr := restoreLatestBackup(dir, dbPath); restoreErr != nil {
		return nil, errors.Join(initialErr, fmt.Errorf("restore memory backup: %w", restoreErr))
	}
	db, err = connectHealthyDatabase(ctx, dbPath)
	if err != nil {
		return nil, errors.Join(initialErr, fmt.Errorf("open restored memory database: %w", err))
	}
	return db, nil
}

func connectHealthyDatabase(ctx context.Context, path string) (*sql.DB, error) {
	db, err := openDatabase(path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("connect memory database: %w", err)
	}
	var result string
	if err := db.QueryRowContext(ctx, "PRAGMA quick_check").Scan(&result); err != nil {
		db.Close()
		return nil, fmt.Errorf("check memory database: %w", err)
	}
	if !strings.EqualFold(strings.TrimSpace(result), "ok") {
		db.Close()
		return nil, fmt.Errorf("memory database integrity check failed: %s", result)
	}
	return db, nil
}

// Backup creates a consistent SQLite snapshot and applies backup retention.
func (s *Store) Backup(ctx context.Context) (string, error) {
	release, err := s.lockFiles(ctx)
	if err != nil {
		return "", err
	}
	defer release()
	return s.backupLocked(ctx)
}

func (s *Store) backupLocked(ctx context.Context) (string, error) {
	dir := filepath.Join(s.dir, "backups")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create memory backup directory: %w", err)
	}
	name := time.Now().UTC().Format("20060102T150405.000000000Z") + "-memory.db"
	path := filepath.Join(dir, name)
	tmp := filepath.Join(dir, "."+uuid.NewString()+".tmp")
	defer os.Remove(tmp)
	if _, err := s.db.ExecContext(ctx, "VACUUM INTO ?", tmp); err != nil {
		return "", fmt.Errorf("create memory backup: %w", err)
	}
	if err := os.Chmod(tmp, 0o600); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, path); err != nil {
		return "", fmt.Errorf("publish memory backup: %w", err)
	}
	if err := pruneBackups(dir, s.opts.MaxBackups); err != nil {
		return "", err
	}
	return path, nil
}

// MaybeMaintain runs bounded cleanup when the previous successful pass is old
// enough. It is safe to call during startup.
func (s *Store) MaybeMaintain(ctx context.Context, project Project, interval time.Duration) (MaintenanceResult, bool, error) {
	if interval <= 0 {
		interval = defaultMaintenanceInterval
	}
	statePath := filepath.Join(s.dir, ".maintenance")
	if info, err := os.Stat(statePath); err == nil && time.Since(info.ModTime()) < interval {
		return MaintenanceResult{}, false, nil
	}
	release, err := s.lockFiles(ctx)
	if err != nil {
		return MaintenanceResult{}, false, err
	}
	defer release()
	if info, err := os.Stat(statePath); err == nil && time.Since(info.ModTime()) < interval {
		return MaintenanceResult{}, false, nil
	}
	result, err := s.maintainLocked(ctx, project)
	if err != nil {
		return MaintenanceResult{}, false, err
	}
	if err := atomicWrite(statePath, []byte(time.Now().UTC().Format(time.RFC3339)+"\n"), 0o600); err != nil {
		return MaintenanceResult{}, false, err
	}
	return result, true, nil
}

// Maintain creates a backup, prunes bounded telemetry and tombstones, and
// refreshes indexes. It never removes active or pending memories.
func (s *Store) Maintain(ctx context.Context, project Project) (MaintenanceResult, error) {
	release, err := s.lockFiles(ctx)
	if err != nil {
		return MaintenanceResult{}, err
	}
	defer release()
	return s.maintainLocked(ctx, project)
}

func (s *Store) maintainLocked(ctx context.Context, project Project) (MaintenanceResult, error) {
	backupPath, err := s.backupLocked(ctx)
	if err != nil {
		return MaintenanceResult{}, err
	}
	now := time.Now()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return MaintenanceResult{}, err
	}
	defer tx.Rollback()

	retrievalResult, err := tx.ExecContext(ctx, "DELETE FROM memory_retrievals WHERE created_at < ?", now.Add(-retrievalRetention).UnixMilli())
	if err != nil {
		return MaintenanceResult{}, fmt.Errorf("prune memory retrievals: %w", err)
	}
	sessionResult, err := tx.ExecContext(ctx, "DELETE FROM memory_session_state WHERE updated_at < ?", now.Add(-sessionStateRetention).UnixMilli())
	if err != nil {
		return MaintenanceResult{}, fmt.Errorf("prune memory session state: %w", err)
	}
	controlResult, err := tx.ExecContext(ctx, "DELETE FROM memory_session_controls WHERE updated_at < ?", now.Add(-sessionStateRetention).UnixMilli())
	if err != nil {
		return MaintenanceResult{}, fmt.Errorf("prune memory session controls: %w", err)
	}
	recordResult, err := tx.ExecContext(ctx,
		"DELETE FROM memory_records WHERE (status IN ('rejected', 'deleted') AND updated_at < ?) OR (status = 'superseded' AND updated_at < ?)",
		now.Add(-rejectedRetention).UnixMilli(), now.Add(-supersededRetention).UnixMilli())
	if err != nil {
		return MaintenanceResult{}, fmt.Errorf("prune memory records: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return MaintenanceResult{}, fmt.Errorf("commit memory maintenance: %w", err)
	}

	if _, err := s.db.ExecContext(ctx, "PRAGMA optimize"); err != nil {
		return MaintenanceResult{}, fmt.Errorf("optimize memory database: %w", err)
	}
	if err := s.syncIndexLocked(ctx, ScopeGlobal, ""); err != nil {
		return MaintenanceResult{}, err
	}
	if project.ID != "" {
		if err := s.syncIndexLocked(ctx, ScopeProject, project.ID); err != nil {
			return MaintenanceResult{}, err
		}
	}
	retrievalsPurged, _ := retrievalResult.RowsAffected()
	sessionStatesPurged, _ := sessionResult.RowsAffected()
	controlsPurged, _ := controlResult.RowsAffected()
	recordsPurged, _ := recordResult.RowsAffected()
	return MaintenanceResult{
		BackupPath:          backupPath,
		RetrievalsPurged:    retrievalsPurged,
		RecordsPurged:       recordsPurged,
		SessionStatesPurged: sessionStatesPurged + controlsPurged,
	}, nil
}

func restoreLatestBackup(dir, dbPath string) error {
	backups, err := listBackups(filepath.Join(dir, "backups"))
	if err != nil {
		return err
	}
	if len(backups) == 0 {
		return fmt.Errorf("no memory backup is available")
	}
	corruptDir := filepath.Join(dir, "corrupt")
	if err := os.MkdirAll(corruptDir, 0o700); err != nil {
		return err
	}
	if _, err := os.Stat(dbPath); err == nil {
		corruptPath := filepath.Join(corruptDir, time.Now().UTC().Format("20060102T150405.000000000Z")+"-memory.db")
		if err := os.Rename(dbPath, corruptPath); err != nil {
			return fmt.Errorf("preserve corrupt memory database: %w", err)
		}
	}
	for _, suffix := range []string{"-wal", "-shm"} {
		if err := os.Remove(dbPath + suffix); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return copyAtomic(backups[0], dbPath, 0o600)
}

func pruneBackups(dir string, maxBackups int) error {
	backups, err := listBackups(dir)
	if err != nil {
		return err
	}
	if len(backups) <= maxBackups {
		return nil
	}
	for _, path := range backups[maxBackups:] {
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("remove old memory backup: %w", err)
		}
	}
	return nil
}

func listBackups(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), "-memory.db") {
			continue
		}
		paths = append(paths, filepath.Join(dir, entry.Name()))
	}
	sort.Sort(sort.Reverse(sort.StringSlice(paths)))
	return paths, nil
}

func copyAtomic(source, destination string, mode os.FileMode) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	tmp, err := os.CreateTemp(filepath.Dir(destination), ".memory-restore-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if _, err := io.Copy(tmp, input); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, destination)
}
