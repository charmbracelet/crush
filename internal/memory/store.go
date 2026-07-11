package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

var memoryPragmas = map[string]string{
	"foreign_keys":  "ON",
	"journal_mode":  "WAL",
	"temp_store":    "MEMORY",
	"synchronous":   "NORMAL",
	"secure_delete": "ON",
	"busy_timeout":  "30000",
}

type Store struct {
	db     *sql.DB
	dir    string
	dbPath string
	opts   Options
	mu     sync.Mutex
}

func Open(ctx context.Context, opts Options) (*Store, error) {
	opts = opts.withDefaults()
	if strings.TrimSpace(opts.Directory) == "" {
		return nil, fmt.Errorf("memory directory is not set")
	}
	dir, err := filepath.Abs(opts.Directory)
	if err != nil {
		return nil, fmt.Errorf("resolve memory directory: %w", err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create memory directory: %w", err)
	}
	dbPath := filepath.Join(dir, "memory.db")
	db, err := openHealthyDatabase(ctx, dir, dbPath)
	if err != nil {
		return nil, fmt.Errorf("open memory database: %w", err)
	}
	store := &Store{db: db, dir: dir, dbPath: dbPath, opts: opts}
	if err := store.migrate(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) Directory() string {
	if s == nil {
		return ""
	}
	return s.dir
}

func (s *Store) Options() Options {
	if s == nil {
		return Options{}.withDefaults()
	}
	return s.opts
}

func (s *Store) migrate(ctx context.Context) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS memory_records (
			id TEXT PRIMARY KEY,
			scope TEXT NOT NULL,
			project_id TEXT NOT NULL DEFAULT '',
			kind TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT NOT NULL,
			content TEXT NOT NULL,
			status TEXT NOT NULL,
			confidence REAL NOT NULL,
			pinned INTEGER NOT NULL DEFAULT 0,
			explicit INTEGER NOT NULL DEFAULT 0,
			derivable INTEGER NOT NULL DEFAULT 0,
			fingerprint TEXT NOT NULL,
			replaces_id TEXT NOT NULL DEFAULT '',
			file_path TEXT NOT NULL DEFAULT '',
			source_session_id TEXT NOT NULL DEFAULT '',
			source_message_id TEXT NOT NULL DEFAULT '',
			observed_at INTEGER NOT NULL,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL,
			last_recalled_at INTEGER NOT NULL DEFAULT 0,
			recall_count INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS memory_records_live_fingerprint
			ON memory_records(fingerprint)
			WHERE status IN ('active', 'pending')`,
		`CREATE INDEX IF NOT EXISTS memory_records_recall
			ON memory_records(status, scope, project_id, pinned, updated_at DESC)`,
		`CREATE TABLE IF NOT EXISTS memory_sources (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			memory_id TEXT NOT NULL REFERENCES memory_records(id) ON DELETE CASCADE,
			session_id TEXT NOT NULL DEFAULT '',
			message_id TEXT NOT NULL DEFAULT '',
			source_kind TEXT NOT NULL DEFAULT '',
			observed_at INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS memory_session_state (
			session_id TEXT PRIMARY KEY,
			last_message_created_at INTEGER NOT NULL DEFAULT 0,
			last_message_id TEXT NOT NULL DEFAULT '',
			updated_at INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS memory_session_controls (
			session_id TEXT PRIMARY KEY,
			recording_mode TEXT NOT NULL DEFAULT 'enabled',
			updated_at INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS memory_retrievals (
			id TEXT PRIMARY KEY,
			session_id TEXT NOT NULL DEFAULT '',
			project_id TEXT NOT NULL DEFAULT '',
			query TEXT NOT NULL DEFAULT '',
			selected_ids TEXT NOT NULL DEFAULT '[]',
			available INTEGER NOT NULL DEFAULT 0,
			fallback INTEGER NOT NULL DEFAULT 0,
			created_at INTEGER NOT NULL
		)`,
	}
	for _, statement := range statements {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("migrate memory database: %w", err)
		}
	}
	return nil
}

func (s *Store) SaveObservation(ctx context.Context, project Project, input Observation) (Record, error) {
	observation, err := NormalizeObservation(input, project, s.opts.AutoApproveConfidence)
	if err != nil {
		return Record{}, err
	}
	if observation.ObservedAt.IsZero() {
		observation.ObservedAt = time.Now()
	}
	fingerprint := Fingerprint(observation)
	now := time.Now().UnixMilli()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Record{}, fmt.Errorf("begin memory write: %w", err)
	}
	defer tx.Rollback()

	record, found, err := findLiveFingerprint(ctx, tx, fingerprint)
	if err != nil {
		return Record{}, err
	}
	if found {
		status := record.Status
		if observation.Status == StatusActive {
			status = StatusActive
		}
		confidence := max(record.Confidence, observation.Confidence)
		_, err = tx.ExecContext(ctx, `UPDATE memory_records SET
			name = ?, description = ?, content = ?, status = ?, confidence = ?,
			pinned = ?, explicit = ?, source_session_id = ?, source_message_id = ?,
			observed_at = ?, updated_at = ? WHERE id = ?`,
			observation.Name, observation.Description, observation.Content, status, confidence,
			boolInt(record.Pinned || observation.Pinned), boolInt(record.Explicit || observation.Explicit),
			observation.SourceSessionID, observation.SourceMessageID,
			observation.ObservedAt.UnixMilli(), now, record.ID)
		if err != nil {
			return Record{}, fmt.Errorf("update memory: %w", err)
		}
		record.ID = record.ID
	} else {
		record = Record{ID: uuid.NewString()}
		_, err = tx.ExecContext(ctx, `INSERT INTO memory_records (
			id, scope, project_id, kind, name, description, content, status,
			confidence, pinned, explicit, derivable, fingerprint, replaces_id,
			source_session_id, source_message_id, observed_at, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			record.ID, observation.Scope, observation.ProjectID, observation.Kind,
			observation.Name, observation.Description, observation.Content, observation.Status,
			observation.Confidence, boolInt(observation.Pinned), boolInt(observation.Explicit),
			boolInt(observation.Derivable), fingerprint, observation.ReplacesID,
			observation.SourceSessionID, observation.SourceMessageID,
			observation.ObservedAt.UnixMilli(), now, now)
		if err != nil {
			return Record{}, fmt.Errorf("insert memory: %w", err)
		}
	}

	if observation.ReplacesID != "" && observation.ReplacesID != record.ID {
		if _, err := tx.ExecContext(ctx, `UPDATE memory_records SET status = 'superseded', updated_at = ?
			WHERE id = ? AND status IN ('active', 'pending')`, now, observation.ReplacesID); err != nil {
			return Record{}, fmt.Errorf("supersede memory: %w", err)
		}
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO memory_sources
		(memory_id, session_id, message_id, source_kind, observed_at) VALUES (?, ?, ?, ?, ?)`,
		record.ID, observation.SourceSessionID, observation.SourceMessageID,
		observation.SourceKind, observation.ObservedAt.UnixMilli()); err != nil {
		return Record{}, fmt.Errorf("insert memory source: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Record{}, fmt.Errorf("commit memory: %w", err)
	}

	record, err = s.Get(ctx, record.ID)
	if err != nil {
		return Record{}, err
	}
	var projectionErrors []error
	if err := s.syncProjection(ctx, record); err != nil {
		projectionErrors = append(projectionErrors, err)
	}
	if observation.ReplacesID != "" && observation.ReplacesID != record.ID {
		if replaced, replacedErr := s.Get(ctx, observation.ReplacesID); replacedErr == nil {
			if syncErr := s.syncProjection(ctx, replaced); syncErr != nil {
				projectionErrors = append(projectionErrors, syncErr)
			}
		}
	}
	if err := errors.Join(projectionErrors...); err != nil {
		return Record{}, err
	}
	return s.Get(ctx, record.ID)
}

func (s *Store) Get(ctx context.Context, id string) (Record, error) {
	row := s.db.QueryRowContext(ctx, recordSelect+` WHERE id = ?`, id)
	record, err := scanRecord(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Record{}, fmt.Errorf("memory %q not found", id)
		}
		return Record{}, err
	}
	return record, nil
}

func (s *Store) GetMany(ctx context.Context, ids []string) ([]Record, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, len(ids))
	for i := range ids {
		args[i] = ids[i]
	}
	rows, err := s.db.QueryContext(ctx, recordSelect+` WHERE id IN (`+placeholders+`)`, args...)
	if err != nil {
		return nil, fmt.Errorf("load memories: %w", err)
	}
	defer rows.Close()
	byID := make(map[string]Record, len(ids))
	for rows.Next() {
		record, scanErr := scanRecord(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		byID[record.ID] = record
	}
	ordered := make([]Record, 0, len(byID))
	for _, id := range ids {
		if record, ok := byID[id]; ok {
			ordered = append(ordered, record)
		}
	}
	return ordered, rows.Err()
}

func (s *Store) Manifest(ctx context.Context, projectID string) ([]Record, error) {
	rows, err := s.db.QueryContext(ctx, recordSelect+` WHERE status = 'active'
		AND (scope = 'global' OR (scope = 'project' AND project_id = ?))
		ORDER BY pinned DESC, updated_at DESC LIMIT ?`, projectID, s.opts.MaxIndexEntries)
	if err != nil {
		return nil, fmt.Errorf("list memory manifest: %w", err)
	}
	defer rows.Close()
	var records []Record
	for rows.Next() {
		record, scanErr := scanRecord(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		record.Content = ""
		records = append(records, record)
	}
	return records, rows.Err()
}

func (s *Store) List(ctx context.Context, projectID string, status Status, limit int) ([]Record, error) {
	if limit <= 0 {
		limit = 100
	}
	query := recordSelect + ` WHERE (scope = 'global' OR project_id = ?)`
	args := []any{projectID}
	if status != "" {
		query += ` AND status = ?`
		args = append(args, status)
	}
	query += ` ORDER BY pinned DESC, updated_at DESC LIMIT ?`
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list memories: %w", err)
	}
	defer rows.Close()
	var records []Record
	for rows.Next() {
		record, scanErr := scanRecord(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

func (s *Store) SetStatus(ctx context.Context, id string, status Status) error {
	if status != StatusActive && status != StatusRejected && status != StatusDeleted {
		return fmt.Errorf("unsupported memory status %q", status)
	}
	now := time.Now().UnixMilli()
	result, err := s.db.ExecContext(ctx, `UPDATE memory_records SET status = ?, updated_at = ? WHERE id = ?`, status, now, id)
	if err != nil {
		return fmt.Errorf("update memory status: %w", err)
	}
	if count, _ := result.RowsAffected(); count == 0 {
		return fmt.Errorf("memory %q not found", id)
	}
	record, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	return s.syncProjection(ctx, record)
}

func (s *Store) SetPinned(ctx context.Context, id string, pinned bool) error {
	result, err := s.db.ExecContext(ctx, `UPDATE memory_records SET pinned = ?, updated_at = ? WHERE id = ?`, boolInt(pinned), time.Now().UnixMilli(), id)
	if err != nil {
		return fmt.Errorf("update memory pin: %w", err)
	}
	if count, _ := result.RowsAffected(); count == 0 {
		return fmt.Errorf("memory %q not found", id)
	}
	record, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	return s.syncProjection(ctx, record)
}

func (s *Store) RecorderCursor(ctx context.Context, sessionID string) (Cursor, error) {
	var cursor Cursor
	err := s.db.QueryRowContext(ctx, `SELECT last_message_created_at, last_message_id
		FROM memory_session_state WHERE session_id = ?`, sessionID).Scan(&cursor.CreatedAt, &cursor.MessageID)
	if errors.Is(err, sql.ErrNoRows) {
		return Cursor{}, nil
	}
	if err != nil {
		return Cursor{}, fmt.Errorf("load memory recorder cursor: %w", err)
	}
	return cursor, nil
}

func (s *Store) AdvanceRecorderCursor(ctx context.Context, sessionID string, cursor Cursor) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO memory_session_state
		(session_id, last_message_created_at, last_message_id, updated_at) VALUES (?, ?, ?, ?)
		ON CONFLICT(session_id) DO UPDATE SET
		last_message_created_at = excluded.last_message_created_at,
		last_message_id = excluded.last_message_id,
		updated_at = excluded.updated_at`, sessionID, cursor.CreatedAt, cursor.MessageID, time.Now().UnixMilli())
	if err != nil {
		return fmt.Errorf("advance memory recorder cursor: %w", err)
	}
	return nil
}

func (s *Store) SessionRecordingMode(ctx context.Context, sessionID string) (SessionRecordingMode, error) {
	if sessionID == "" {
		return SessionRecordingEnabled, nil
	}
	var mode SessionRecordingMode
	err := s.db.QueryRowContext(ctx, `SELECT recording_mode
		FROM memory_session_controls WHERE session_id = ?`, sessionID).Scan(&mode)
	if errors.Is(err, sql.ErrNoRows) {
		return SessionRecordingEnabled, nil
	}
	if err != nil {
		return "", fmt.Errorf("load session memory mode: %w", err)
	}
	if !validSessionRecordingMode(mode) {
		return "", fmt.Errorf("invalid session memory mode %q", mode)
	}
	return mode, nil
}

func (s *Store) SetSessionRecordingMode(ctx context.Context, sessionID string, mode SessionRecordingMode) error {
	if sessionID == "" {
		return errors.New("session ID is required")
	}
	if !validSessionRecordingMode(mode) {
		return fmt.Errorf("invalid session memory mode %q", mode)
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO memory_session_controls
		(session_id, recording_mode, updated_at) VALUES (?, ?, ?)
		ON CONFLICT(session_id) DO UPDATE SET
		recording_mode = excluded.recording_mode,
		updated_at = excluded.updated_at`, sessionID, mode, time.Now().UnixMilli())
	if err != nil {
		return fmt.Errorf("set session memory mode: %w", err)
	}
	return nil
}

func (s *Store) MarkSessionExternalContext(ctx context.Context, sessionID string) error {
	if sessionID == "" {
		return errors.New("session ID is required")
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO memory_session_controls
		(session_id, recording_mode, updated_at) VALUES (?, ?, ?)
		ON CONFLICT(session_id) DO UPDATE SET
		recording_mode = CASE
			WHEN memory_session_controls.recording_mode = 'disabled' THEN 'disabled'
			ELSE 'polluted'
		END,
		updated_at = excluded.updated_at`, sessionID, SessionRecordingPolluted, time.Now().UnixMilli())
	if err != nil {
		return fmt.Errorf("mark session memory as externally sourced: %w", err)
	}
	return nil
}

func validSessionRecordingMode(mode SessionRecordingMode) bool {
	switch mode {
	case SessionRecordingEnabled, SessionRecordingDisabled, SessionRecordingPolluted:
		return true
	default:
		return false
	}
}

func (s *Store) RecordRetrieval(ctx context.Context, retrieval Retrieval) error {
	selected, err := json.Marshal(retrieval.Selected)
	if err != nil {
		return err
	}
	now := time.Now().UnixMilli()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT INTO memory_retrievals
		(id, session_id, project_id, query, selected_ids, available, fallback, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, uuid.NewString(), retrieval.SessionID,
		retrieval.ProjectID, cleanLine(retrieval.Query, 500), string(selected), retrieval.Available,
		boolInt(retrieval.Fallback), now); err != nil {
		return fmt.Errorf("record memory retrieval: %w", err)
	}
	for _, id := range retrieval.Selected {
		if _, err := tx.ExecContext(ctx, `UPDATE memory_records SET
			last_recalled_at = ?, recall_count = recall_count + 1 WHERE id = ?`, now, id); err != nil {
			return fmt.Errorf("update memory recall: %w", err)
		}
	}
	return tx.Commit()
}

func (s *Store) Stats(ctx context.Context) (Stats, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT status, COUNT(*) FROM memory_records GROUP BY status`)
	if err != nil {
		return Stats{}, err
	}
	defer rows.Close()
	var stats Stats
	for rows.Next() {
		var status Status
		var count int64
		if err := rows.Scan(&status, &count); err != nil {
			return Stats{}, err
		}
		switch status {
		case StatusActive:
			stats.Active = count
		case StatusPending:
			stats.Pending = count
		case StatusSuperseded:
			stats.Superseded = count
		case StatusRejected:
			stats.Rejected = count
		case StatusDeleted:
			stats.Deleted = count
		}
	}
	return stats, rows.Err()
}

const recordSelect = `SELECT id, scope, project_id, kind, name, description, content,
	status, confidence, pinned, explicit, derivable, fingerprint, replaces_id,
	file_path, source_session_id, source_message_id, observed_at, created_at,
	updated_at, last_recalled_at, recall_count FROM memory_records`

type rowScanner interface {
	Scan(...any) error
}

func scanRecord(row rowScanner) (Record, error) {
	var record Record
	var pinned, explicit, derivable int
	var observedAt, createdAt, updatedAt, recalledAt int64
	err := row.Scan(
		&record.ID, &record.Scope, &record.ProjectID, &record.Kind, &record.Name,
		&record.Description, &record.Content, &record.Status, &record.Confidence,
		&pinned, &explicit, &derivable, &record.Fingerprint, &record.ReplacesID,
		&record.FilePath, &record.SourceSessionID, &record.SourceMessageID,
		&observedAt, &createdAt, &updatedAt, &recalledAt, &record.RecallCount,
	)
	if err != nil {
		return Record{}, err
	}
	record.Pinned = pinned != 0
	record.Explicit = explicit != 0
	record.Derivable = derivable != 0
	record.ObservedAt = millisTime(observedAt)
	record.CreatedAt = millisTime(createdAt)
	record.UpdatedAt = millisTime(updatedAt)
	record.LastRecalledAt = millisTime(recalledAt)
	return record, nil
}

func findLiveFingerprint(ctx context.Context, tx *sql.Tx, fingerprint string) (Record, bool, error) {
	record, err := scanRecord(tx.QueryRowContext(ctx, recordSelect+`
		WHERE fingerprint = ? AND status IN ('active', 'pending') LIMIT 1`, fingerprint))
	if errors.Is(err, sql.ErrNoRows) {
		return Record{}, false, nil
	}
	if err != nil {
		return Record{}, false, fmt.Errorf("find duplicate memory: %w", err)
	}
	return record, true, nil
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func millisTime(value int64) time.Time {
	if value <= 0 {
		return time.Time{}
	}
	return time.UnixMilli(value)
}
