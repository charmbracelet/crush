package memory

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/crush/internal/lock"
	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

const (
	indexMaxLines = 200
	indexMaxBytes = 25_000
)

var slugPattern = regexp.MustCompile(`[^a-z0-9]+`)

type projectionHeader struct {
	ID          string  `yaml:"id"`
	Name        string  `yaml:"name"`
	Description string  `yaml:"description"`
	Type        Kind    `yaml:"type"`
	Scope       Scope   `yaml:"scope"`
	ProjectID   string  `yaml:"project_id,omitempty"`
	Confidence  float64 `yaml:"confidence"`
	Pinned      bool    `yaml:"pinned"`
	UpdatedAt   string  `yaml:"updated_at"`
}

func (s *Store) SyncFromDisk(ctx context.Context, project Project) error {
	release, err := s.lockFiles(ctx)
	if err != nil {
		return err
	}
	defer release()

	locations := []struct {
		scope     Scope
		projectID string
		dir       string
	}{
		{scope: ScopeGlobal, dir: s.projectionDir(ScopeGlobal, "")},
	}
	if project.ID != "" {
		locations = append(locations, struct {
			scope     Scope
			projectID string
			dir       string
		}{scope: ScopeProject, projectID: project.ID, dir: s.projectionDir(ScopeProject, project.ID)})
	}

	var syncErrors []error
	for _, location := range locations {
		if err := os.MkdirAll(location.dir, 0o700); err != nil {
			syncErrors = append(syncErrors, err)
			continue
		}
		seen := make(map[string]bool)
		entries, err := os.ReadDir(location.dir)
		if err != nil {
			syncErrors = append(syncErrors, err)
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() || entry.Name() == "MEMORY.md" || !strings.HasSuffix(strings.ToLower(entry.Name()), ".md") {
				continue
			}
			path := filepath.Join(location.dir, entry.Name())
			if !safeProjectionPath(location.dir, path) {
				syncErrors = append(syncErrors, fmt.Errorf("unsafe memory projection path %q", path))
				continue
			}
			header, content, readErr := readProjection(path)
			if readErr != nil {
				syncErrors = append(syncErrors, readErr)
				continue
			}
			if header.Scope != location.scope || (location.scope == ScopeProject && header.ProjectID != location.projectID) {
				syncErrors = append(syncErrors, fmt.Errorf("memory projection scope mismatch in %q", path))
				continue
			}
			observation, normalizeErr := NormalizeObservation(Observation{
				Scope:       header.Scope,
				ProjectID:   header.ProjectID,
				Kind:        header.Type,
				Name:        header.Name,
				Description: header.Description,
				Content:     content,
				Confidence:  header.Confidence,
				Pinned:      header.Pinned,
				Explicit:    true,
				Status:      StatusActive,
			}, project, s.opts.AutoApproveConfidence)
			if normalizeErr != nil {
				if header.ID != "" {
					_, _ = s.db.ExecContext(ctx, `UPDATE memory_records SET status = 'rejected', updated_at = ? WHERE id = ?`, time.Now().UnixMilli(), header.ID)
				}
				syncErrors = append(syncErrors, fmt.Errorf("reject memory projection %q: %w", path, normalizeErr))
				continue
			}
			if header.ID == "" {
				if _, saveErr := s.saveProjectionObservationLocked(ctx, observation, "", path); saveErr != nil {
					syncErrors = append(syncErrors, saveErr)
				}
				continue
			}
			seen[header.ID] = true
			result, updateErr := s.db.ExecContext(ctx, `UPDATE memory_records SET
				scope = ?, project_id = ?, kind = ?, name = ?, description = ?, content = ?,
				confidence = ?, pinned = ?, explicit = 1, fingerprint = ?, file_path = ?,
				status = 'active', updated_at = ? WHERE id = ?`,
				observation.Scope, observation.ProjectID, observation.Kind, observation.Name,
				observation.Description, observation.Content, observation.Confidence,
				boolInt(observation.Pinned), Fingerprint(observation), path, time.Now().UnixMilli(), header.ID)
			if updateErr != nil {
				syncErrors = append(syncErrors, fmt.Errorf("reconcile memory projection %q: %w", path, updateErr))
				continue
			}
			if count, _ := result.RowsAffected(); count == 0 {
				if _, saveErr := s.saveProjectionObservationLocked(ctx, observation, header.ID, path); saveErr != nil {
					syncErrors = append(syncErrors, saveErr)
				}
			}
		}

		rows, queryErr := s.db.QueryContext(ctx, `SELECT id, file_path FROM memory_records
			WHERE status = 'active' AND scope = ? AND project_id = ? AND file_path != ''`, location.scope, location.projectID)
		if queryErr != nil {
			syncErrors = append(syncErrors, queryErr)
			continue
		}
		var missingIDs []string
		for rows.Next() {
			var id, path string
			if scanErr := rows.Scan(&id, &path); scanErr != nil {
				syncErrors = append(syncErrors, scanErr)
				continue
			}
			if seen[id] {
				continue
			}
			if _, statErr := os.Stat(path); errors.Is(statErr, fs.ErrNotExist) {
				missingIDs = append(missingIDs, id)
			}
		}
		rows.Close()
		for _, id := range missingIDs {
			_, _ = s.db.ExecContext(ctx, `UPDATE memory_records SET status = 'deleted', updated_at = ? WHERE id = ?`, time.Now().UnixMilli(), id)
		}
		if indexErr := s.syncIndexLocked(ctx, location.scope, location.projectID); indexErr != nil {
			syncErrors = append(syncErrors, indexErr)
		}
	}
	return errors.Join(syncErrors...)
}

func (s *Store) saveProjectionObservationLocked(ctx context.Context, observation Observation, id, path string) (Record, error) {
	fingerprint := Fingerprint(observation)
	now := time.Now().UnixMilli()
	if id == "" {
		id = uuid.NewString()
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO memory_records (
		id, scope, project_id, kind, name, description, content, status, confidence,
		pinned, explicit, derivable, fingerprint, file_path, observed_at, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, 'active', ?, ?, 1, 0, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET name = excluded.name, description = excluded.description,
		content = excluded.content, confidence = excluded.confidence, pinned = excluded.pinned,
		fingerprint = excluded.fingerprint, file_path = excluded.file_path,
		status = 'active', updated_at = excluded.updated_at`,
		id, observation.Scope, observation.ProjectID, observation.Kind, observation.Name,
		observation.Description, observation.Content, observation.Confidence,
		boolInt(observation.Pinned), fingerprint, path, now, now, now)
	if err != nil {
		return Record{}, fmt.Errorf("import memory projection %q: %w", path, err)
	}
	record, err := s.Get(ctx, id)
	if err != nil {
		return Record{}, err
	}
	if err := s.syncProjectionLocked(ctx, record); err != nil {
		return Record{}, err
	}
	return s.Get(ctx, id)
}

func readProjection(path string) (projectionHeader, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return projectionHeader{}, "", err
	}
	if !bytes.HasPrefix(data, []byte("---\n")) {
		return projectionHeader{}, "", fmt.Errorf("memory projection %q has no frontmatter", path)
	}
	end := bytes.Index(data[4:], []byte("\n---\n"))
	if end < 0 {
		return projectionHeader{}, "", fmt.Errorf("memory projection %q has invalid frontmatter", path)
	}
	end += 4
	var header projectionHeader
	if err := yaml.Unmarshal(data[4:end], &header); err != nil {
		return projectionHeader{}, "", fmt.Errorf("parse memory projection %q: %w", path, err)
	}
	content := strings.TrimSpace(string(data[end+5:]))
	return header, content, nil
}

func safeProjectionPath(root, path string) bool {
	root = canonicalPath(root)
	resolved := canonicalPath(path)
	relative, err := filepath.Rel(root, resolved)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func (s *Store) syncProjection(ctx context.Context, record Record) error {
	release, err := s.lockFiles(ctx)
	if err != nil {
		return err
	}
	defer release()
	return s.syncProjectionLocked(ctx, record)
}

func (s *Store) syncProjectionLocked(ctx context.Context, record Record) error {
	dir := s.projectionDir(record.Scope, record.ProjectID)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create memory projection directory: %w", err)
	}

	path := record.FilePath
	if path == "" {
		path = filepath.Join(dir, projectionFilename(record))
	}
	if !safeProjectionPath(dir, path) {
		return fmt.Errorf("unsafe memory projection path %q", path)
	}
	if record.Status == StatusActive {
		data, err := renderProjection(record)
		if err != nil {
			return err
		}
		if err := atomicWrite(path, data, 0o600); err != nil {
			return err
		}
		if record.FilePath != path {
			if _, err := s.db.ExecContext(ctx, `UPDATE memory_records SET file_path = ? WHERE id = ?`, path, record.ID); err != nil {
				return fmt.Errorf("save memory projection path: %w", err)
			}
		}
	} else if path != "" {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove memory projection: %w", err)
		}
	}

	return s.syncIndexLocked(ctx, record.Scope, record.ProjectID)
}

func (s *Store) syncIndexLocked(ctx context.Context, scope Scope, projectID string) error {
	query := recordSelect + ` WHERE status = 'active' AND scope = ?`
	args := []any{scope}
	if scope == ScopeProject {
		query += ` AND project_id = ?`
		args = append(args, projectID)
	}
	query += ` ORDER BY pinned DESC, updated_at DESC LIMIT ?`
	args = append(args, s.opts.MaxIndexEntries)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("load memory index: %w", err)
	}
	defer rows.Close()
	var records []Record
	for rows.Next() {
		record, scanErr := scanRecord(rows)
		if scanErr != nil {
			return scanErr
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	sort.SliceStable(records, func(i, j int) bool {
		if records[i].Pinned != records[j].Pinned {
			return records[i].Pinned
		}
		return records[i].UpdatedAt.After(records[j].UpdatedAt)
	})
	var b strings.Builder
	b.WriteString("# Memory Index\n\n")
	b.WriteString("Memory entries are fallible observations. Verify project facts against current source.\n\n")
	lineCount := 4
	for _, record := range records {
		filename := filepath.Base(record.FilePath)
		if filename == "." || filename == "" {
			filename = projectionFilename(record)
		}
		line := fmt.Sprintf("- [%s](%s) [%s] %s", record.Name, filepath.ToSlash(filename), record.Kind, record.Description)
		if record.Pinned {
			line += " (pinned)"
		}
		line = cleanLine(line, 200)
		if lineCount+1 >= indexMaxLines || b.Len()+len(line)+1 > indexMaxBytes {
			b.WriteString("- Index truncated; use memory list for remaining entries.\n")
			break
		}
		b.WriteString(line)
		b.WriteByte('\n')
		lineCount++
	}
	return atomicWrite(filepath.Join(s.projectionDir(scope, projectID), "MEMORY.md"), []byte(b.String()), 0o600)
}

func (s *Store) lockFiles(ctx context.Context) (func(), error) {
	s.mu.Lock()
	release, err := lock.File(ctx, filepath.Join(s.dir, "memory.lock"))
	if err != nil {
		s.mu.Unlock()
		return nil, fmt.Errorf("acquire memory file lock: %w", err)
	}
	return func() {
		release()
		s.mu.Unlock()
	}, nil
}

func (s *Store) projectionDir(scope Scope, projectID string) string {
	if scope == ScopeProject {
		return filepath.Join(s.dir, "projects", projectID)
	}
	return filepath.Join(s.dir, "global")
}

func projectionFilename(record Record) string {
	slug := strings.Trim(slugPattern.ReplaceAllString(strings.ToLower(record.Name), "-"), "-")
	if slug == "" {
		slug = "memory"
	}
	if len(slug) > 48 {
		slug = strings.Trim(slug[:48], "-")
	}
	id := record.ID
	if len(id) > 8 {
		id = id[:8]
	}
	return fmt.Sprintf("%s_%s_%s.md", record.Kind, slug, id)
}

func renderProjection(record Record) ([]byte, error) {
	header := projectionHeader{
		ID:          record.ID,
		Name:        record.Name,
		Description: record.Description,
		Type:        record.Kind,
		Scope:       record.Scope,
		ProjectID:   record.ProjectID,
		Confidence:  record.Confidence,
		Pinned:      record.Pinned,
		UpdatedAt:   record.UpdatedAt.UTC().Format(time.RFC3339),
	}
	frontmatter, err := yaml.Marshal(header)
	if err != nil {
		return nil, fmt.Errorf("render memory frontmatter: %w", err)
	}
	var b bytes.Buffer
	b.WriteString("---\n")
	b.Write(frontmatter)
	b.WriteString("---\n\n")
	b.WriteString(strings.TrimSpace(record.Content))
	b.WriteByte('\n')
	return b.Bytes(), nil
}

func atomicWrite(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create memory directory: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".memory-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary memory file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write temporary memory file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("sync temporary memory file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("replace memory file: %w", err)
	}
	return nil
}
