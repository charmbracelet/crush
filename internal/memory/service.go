package memory

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	defaultLimit = 20
	maxLimit     = 100
)

var ErrNotFound = errors.New("memory key not found")

type Entry struct {
	Key       string `json:"key"`
	Value     string `json:"value"`
	UpdatedAt int64  `json:"updated_at"`
}

type Service interface {
	Store(context.Context, string, string) error
	Get(context.Context, string) (Entry, error)
	Delete(context.Context, string) error
	Search(context.Context, string, int) ([]Entry, error)
	List(context.Context, int) ([]Entry, error)
}

type service struct {
	storePath string
	auditPath string
	mu        sync.Mutex
}

type persistedData struct {
	Entries map[string]Entry `json:"entries"`
}

type auditRecord struct {
	Action    string `json:"action"`
	Key       string `json:"key"`
	Timestamp int64  `json:"timestamp"`
}

func NewService(dataDir string) (Service, error) {
	root := strings.TrimSpace(dataDir)
	if root == "" {
		return nil, fmt.Errorf("data directory is required")
	}

	memoryDir := filepath.Join(root, "memory")
	if err := os.MkdirAll(memoryDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating memory directory: %w", err)
	}

	s := &service{
		storePath: filepath.Join(memoryDir, "entries.json"),
		auditPath: filepath.Join(memoryDir, "audit.log"),
	}
	if err := s.ensureStoreFile(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *service) Store(ctx context.Context, key, value string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	normalizedKey := strings.TrimSpace(key)
	if normalizedKey == "" {
		return fmt.Errorf("key is required")
	}
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("value is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := s.readDataLocked()
	if err != nil {
		return err
	}

	data.Entries[normalizedKey] = Entry{
		Key:       normalizedKey,
		Value:     value,
		UpdatedAt: time.Now().UnixNano(),
	}
	if err := s.writeDataLocked(data); err != nil {
		return err
	}
	return s.appendAuditLocked("store", normalizedKey)
}

func (s *service) Get(ctx context.Context, key string) (Entry, error) {
	if err := ctx.Err(); err != nil {
		return Entry{}, err
	}

	normalizedKey := strings.TrimSpace(key)
	if normalizedKey == "" {
		return Entry{}, fmt.Errorf("key is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := s.readDataLocked()
	if err != nil {
		return Entry{}, err
	}

	entry, ok := data.Entries[normalizedKey]
	if !ok {
		return Entry{}, ErrNotFound
	}
	return entry, nil
}

func (s *service) Delete(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	normalizedKey := strings.TrimSpace(key)
	if normalizedKey == "" {
		return fmt.Errorf("key is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := s.readDataLocked()
	if err != nil {
		return err
	}

	if _, ok := data.Entries[normalizedKey]; !ok {
		return ErrNotFound
	}
	delete(data.Entries, normalizedKey)
	if err := s.writeDataLocked(data); err != nil {
		return err
	}
	return s.appendAuditLocked("delete", normalizedKey)
}

func (s *service) Search(ctx context.Context, query string, limit int) ([]Entry, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	normalizedQuery := strings.ToLower(strings.TrimSpace(query))
	if normalizedQuery == "" {
		return nil, fmt.Errorf("query is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := s.readDataLocked()
	if err != nil {
		return nil, err
	}

	results := make([]Entry, 0, len(data.Entries))
	for _, entry := range data.Entries {
		if strings.Contains(strings.ToLower(entry.Key), normalizedQuery) || strings.Contains(strings.ToLower(entry.Value), normalizedQuery) {
			results = append(results, entry)
		}
	}
	sortEntries(results)
	return applyLimit(results, limit), nil
}

func (s *service) List(ctx context.Context, limit int) ([]Entry, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := s.readDataLocked()
	if err != nil {
		return nil, err
	}

	results := make([]Entry, 0, len(data.Entries))
	for _, entry := range data.Entries {
		results = append(results, entry)
	}
	sortEntries(results)
	return applyLimit(results, limit), nil
}

func (s *service) ensureStoreFile() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := os.Stat(s.storePath); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("checking memory store: %w", err)
	}

	return s.writeDataLocked(persistedData{Entries: map[string]Entry{}})
}

func (s *service) readDataLocked() (persistedData, error) {
	content, err := os.ReadFile(s.storePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return persistedData{Entries: map[string]Entry{}}, nil
		}
		return persistedData{}, fmt.Errorf("reading memory store: %w", err)
	}

	if len(content) == 0 {
		return persistedData{Entries: map[string]Entry{}}, nil
	}

	var data persistedData
	if err := json.Unmarshal(content, &data); err != nil {
		return persistedData{}, fmt.Errorf("parsing memory store: %w", err)
	}
	if data.Entries == nil {
		data.Entries = map[string]Entry{}
	}
	return data, nil
}

func (s *service) writeDataLocked(data persistedData) error {
	if data.Entries == nil {
		data.Entries = map[string]Entry{}
	}

	payload, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding memory store: %w", err)
	}
	if err := os.WriteFile(s.storePath, payload, 0o644); err != nil {
		return fmt.Errorf("writing memory store: %w", err)
	}
	return nil
}

func (s *service) appendAuditLocked(action, key string) error {
	record := auditRecord{Action: action, Key: key, Timestamp: time.Now().Unix()}
	payload, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("encoding memory audit record: %w", err)
	}

	file, err := os.OpenFile(s.auditPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("opening memory audit log: %w", err)
	}
	defer file.Close()

	if _, err := file.Write(append(payload, '\n')); err != nil {
		return fmt.Errorf("writing memory audit log: %w", err)
	}
	return nil
}

func sortEntries(entries []Entry) {
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].UpdatedAt == entries[j].UpdatedAt {
			return entries[i].Key < entries[j].Key
		}
		return entries[i].UpdatedAt > entries[j].UpdatedAt
	})
}

func applyLimit(entries []Entry, limit int) []Entry {
	normalized := limit
	if normalized <= 0 {
		normalized = defaultLimit
	}
	if normalized > maxLimit {
		normalized = maxLimit
	}
	if normalized >= len(entries) {
		return entries
	}
	return entries[:normalized]
}
