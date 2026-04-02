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
	Key       string   `json:"key"`
	Value     string   `json:"value"`
	Scope     string   `json:"scope,omitempty"`
	Category  string   `json:"category,omitempty"`
	Type      string   `json:"type,omitempty"`
	Tags      []string `json:"tags,omitempty"`
	UpdatedAt int64    `json:"updated_at"`
}

type StoreParams struct {
	Key      string
	Value    string
	Scope    string
	Category string
	Type     string
	Tags     []string
}

type SearchParams struct {
	Query    string
	Scope    string
	Category string
	Type     string
	Tags     []string
	Limit    int
}

type ListParams struct {
	Scope    string
	Category string
	Type     string
	Tags     []string
	Limit    int
}

type Service interface {
	Store(context.Context, StoreParams) error
	Get(context.Context, string) (Entry, error)
	Delete(context.Context, string) error
	Search(context.Context, SearchParams) ([]Entry, error)
	List(context.Context, ListParams) ([]Entry, error)
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

type entryFilters struct {
	Scope    string
	Category string
	Type     string
	Tags     []string
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

func (s *service) Store(ctx context.Context, params StoreParams) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	normalizedKey := strings.TrimSpace(params.Key)
	if normalizedKey == "" {
		return fmt.Errorf("key is required")
	}
	if strings.TrimSpace(params.Value) == "" {
		return fmt.Errorf("value is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := s.readDataLocked()
	if err != nil {
		return err
	}

	data.Entries[normalizedKey] = normalizeEntry(Entry{
		Key:       normalizedKey,
		Value:     params.Value,
		Scope:     params.Scope,
		Category:  params.Category,
		Type:      params.Type,
		Tags:      params.Tags,
		UpdatedAt: time.Now().UnixNano(),
	}, normalizedKey)
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

func (s *service) Search(ctx context.Context, params SearchParams) ([]Entry, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	normalizedQuery := strings.ToLower(strings.TrimSpace(params.Query))
	if normalizedQuery == "" {
		return nil, fmt.Errorf("query is required")
	}
	filters := entryFilters{
		Scope:    strings.TrimSpace(params.Scope),
		Category: strings.TrimSpace(params.Category),
		Type:     strings.TrimSpace(params.Type),
		Tags:     normalizeTags(params.Tags),
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := s.readDataLocked()
	if err != nil {
		return nil, err
	}

	results := make([]Entry, 0, len(data.Entries))
	for _, entry := range data.Entries {
		if !matchesEntryFilters(entry, filters) {
			continue
		}
		if queryMatchesEntry(entry, normalizedQuery) {
			results = append(results, entry)
		}
	}
	sortEntries(results)
	return applyLimit(results, params.Limit), nil
}

func (s *service) List(ctx context.Context, params ListParams) ([]Entry, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	filters := entryFilters{
		Scope:    strings.TrimSpace(params.Scope),
		Category: strings.TrimSpace(params.Category),
		Type:     strings.TrimSpace(params.Type),
		Tags:     normalizeTags(params.Tags),
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := s.readDataLocked()
	if err != nil {
		return nil, err
	}

	results := make([]Entry, 0, len(data.Entries))
	for _, entry := range data.Entries {
		if !matchesEntryFilters(entry, filters) {
			continue
		}
		results = append(results, entry)
	}
	sortEntries(results)
	return applyLimit(results, params.Limit), nil
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
	for key, entry := range data.Entries {
		data.Entries[key] = normalizeEntry(entry, key)
	}
	return data, nil
}

func (s *service) writeDataLocked(data persistedData) error {
	if data.Entries == nil {
		data.Entries = map[string]Entry{}
	}
	for key, entry := range data.Entries {
		data.Entries[key] = normalizeEntry(entry, key)
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

func normalizeEntry(entry Entry, key string) Entry {
	normalizedKey := strings.TrimSpace(entry.Key)
	if normalizedKey == "" {
		normalizedKey = strings.TrimSpace(key)
	}
	return Entry{
		Key:       normalizedKey,
		Value:     entry.Value,
		Scope:     strings.TrimSpace(entry.Scope),
		Category:  strings.TrimSpace(entry.Category),
		Type:      strings.TrimSpace(entry.Type),
		Tags:      normalizeTags(entry.Tags),
		UpdatedAt: entry.UpdatedAt,
	}
}

func normalizeTags(tags []string) []string {
	if len(tags) == 0 {
		return nil
	}

	seen := make(map[string]string, len(tags))
	keys := make([]string, 0, len(tags))
	for _, tag := range tags {
		trimmed := strings.TrimSpace(tag)
		if trimmed == "" {
			continue
		}
		normalized := strings.ToLower(trimmed)
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = trimmed
		keys = append(keys, normalized)
	}
	if len(keys) == 0 {
		return nil
	}
	sort.Strings(keys)
	result := make([]string, 0, len(keys))
	for _, key := range keys {
		result = append(result, seen[key])
	}
	return result
}

func matchesEntryFilters(entry Entry, filters entryFilters) bool {
	if filters.Scope != "" && !strings.EqualFold(strings.TrimSpace(entry.Scope), filters.Scope) {
		return false
	}
	if filters.Category != "" && !strings.EqualFold(strings.TrimSpace(entry.Category), filters.Category) {
		return false
	}
	if filters.Type != "" && !strings.EqualFold(strings.TrimSpace(entry.Type), filters.Type) {
		return false
	}
	if len(filters.Tags) == 0 {
		return true
	}

	entryTags := make(map[string]struct{}, len(entry.Tags))
	for _, tag := range entry.Tags {
		entryTags[strings.ToLower(strings.TrimSpace(tag))] = struct{}{}
	}
	for _, tag := range filters.Tags {
		if _, ok := entryTags[strings.ToLower(tag)]; !ok {
			return false
		}
	}
	return true
}

func queryMatchesEntry(entry Entry, query string) bool {
	fields := []string{
		entry.Key,
		entry.Value,
		entry.Scope,
		entry.Category,
		entry.Type,
		strings.Join(entry.Tags, " "),
	}
	for _, field := range fields {
		if strings.Contains(strings.ToLower(field), query) {
			return true
		}
	}
	return false
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
