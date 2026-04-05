package memory

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	defaultLimit   = 20
	maxLimit       = 100
	maxMemoryFiles = 200
	indexFilename  = "MEMORY.md"
)

var ErrNotFound = errors.New("memory key not found")

type Entry struct {
	Key       string   `json:"key" yaml:"key"`
	Value     string   `json:"value" yaml:"value"`
	Scope     string   `json:"scope,omitempty" yaml:"scope,omitempty"`
	Category  string   `json:"category,omitempty" yaml:"category,omitempty"`
	Type      string   `json:"type,omitempty" yaml:"type,omitempty"`
	Tags      []string `json:"tags,omitempty" yaml:"tags,omitempty"`
	UpdatedAt int64    `json:"updated_at" yaml:"updated_at"`
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
	ReadIndex() (string, error)
	ListMemoryFiles() ([]MemoryFileInfo, error)
	ReadMemoryFileBody(string) (string, error)
}

type service struct {
	memoryDir string
	indexPath string
	mu        sync.Mutex
}

type memoryFrontmatter struct {
	Key         string   `yaml:"key"`
	Description string   `yaml:"description"`
	Scope       string   `yaml:"scope,omitempty"`
	Category    string   `yaml:"category,omitempty"`
	Type        string   `yaml:"type,omitempty"`
	Tags        []string `yaml:"tags,omitempty"`
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
		memoryDir: memoryDir,
		indexPath: filepath.Join(memoryDir, indexFilename),
	}
	if err := s.ensureIndexFile(); err != nil {
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

	fm := memoryFrontmatter{
		Key:         normalizedKey,
		Description: truncateForDescription(params.Value),
		Scope:       strings.TrimSpace(params.Scope),
		Category:    strings.TrimSpace(params.Category),
		Type:        strings.TrimSpace(params.Type),
		Tags:        normalizeTags(params.Tags),
	}

	filePath := s.entryFilePath(normalizedKey)
	content := buildMemoryFileContent(fm, params.Value)
	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("writing memory file: %w", err)
	}

	if err := s.rebuildIndexLocked(); err != nil {
		return fmt.Errorf("rebuilding index: %w", err)
	}
	return nil
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

	return s.readEntryLocked(normalizedKey)
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

	filePath := s.entryFilePath(normalizedKey)
	if _, err := os.Stat(filePath); err != nil {
		if os.IsNotExist(err) {
			return ErrNotFound
		}
		return fmt.Errorf("checking memory file: %w", err)
	}

	if err := os.Remove(filePath); err != nil {
		return fmt.Errorf("deleting memory file: %w", err)
	}

	return s.rebuildIndexLocked()
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

	entries, err := s.loadAllEntriesLocked()
	if err != nil {
		return nil, err
	}

	results := make([]Entry, 0, len(entries))
	for _, entry := range entries {
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

	entries, err := s.loadAllEntriesLocked()
	if err != nil {
		return nil, err
	}

	results := make([]Entry, 0, len(entries))
	for _, entry := range entries {
		if !matchesEntryFilters(entry, filters) {
			continue
		}
		results = append(results, entry)
	}
	sortEntries(results)
	return applyLimit(results, params.Limit), nil
}

func (s *service) ensureIndexFile() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := os.Stat(s.indexPath); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("checking memory index: %w", err)
	}

	return os.WriteFile(s.indexPath, []byte("# Memory Index\n\n"), 0o644)
}

func (s *service) entryFilePath(key string) string {
	safeKey := sanitizeFilename(key)
	if safeKey+".md" == indexFilename {
		safeKey = "_" + safeKey
	}
	filename := safeKey + ".md"
	return filepath.Join(s.memoryDir, filename)
}

func (s *service) readEntryLocked(key string) (Entry, error) {
	filePath := s.entryFilePath(key)
	content, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return Entry{}, ErrNotFound
		}
		return Entry{}, fmt.Errorf("reading memory file: %w", err)
	}

	fm, body, err := parseMemoryFile(content)
	if err != nil {
		return Entry{}, fmt.Errorf("parsing memory file: %w", err)
	}

	info, statErr := os.Stat(filePath)
	updatedAt := time.Now().UnixNano()
	if statErr == nil {
		updatedAt = info.ModTime().UnixNano()
	}

	return Entry{
		Key:       fm.Key,
		Value:     strings.TrimSpace(body),
		Scope:     fm.Scope,
		Category:  fm.Category,
		Type:      fm.Type,
		Tags:      normalizeTags(fm.Tags),
		UpdatedAt: updatedAt,
	}, nil
}

func (s *service) loadAllEntriesLocked() ([]Entry, error) {
	files, err := os.ReadDir(s.memoryDir)
	if err != nil {
		return nil, fmt.Errorf("reading memory directory: %w", err)
	}

	type fileStat struct {
		name    string
		modTime time.Time
	}
	var mdFiles []fileStat
	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".md") || f.Name() == indexFilename {
			continue
		}
		info, err := f.Info()
		if err != nil {
			continue
		}
		mdFiles = append(mdFiles, fileStat{name: f.Name(), modTime: info.ModTime()})
	}

	sort.Slice(mdFiles, func(i, j int) bool {
		return mdFiles[i].modTime.After(mdFiles[j].modTime)
	})

	if len(mdFiles) > maxMemoryFiles {
		mdFiles = mdFiles[:maxMemoryFiles]
	}

	entries := make([]Entry, 0, len(mdFiles))
	for _, fs := range mdFiles {
		content, err := os.ReadFile(filepath.Join(s.memoryDir, fs.name))
		if err != nil {
			continue
		}
		fm, body, err := parseMemoryFile(content)
		if err != nil {
			continue
		}
		entries = append(entries, Entry{
			Key:       fm.Key,
			Value:     strings.TrimSpace(body),
			Scope:     fm.Scope,
			Category:  fm.Category,
			Type:      fm.Type,
			Tags:      normalizeTags(fm.Tags),
			UpdatedAt: fs.modTime.UnixNano(),
		})
	}
	return entries, nil
}

func (s *service) rebuildIndexLocked() error {
	entries, err := s.loadAllEntriesLocked()
	if err != nil {
		return err
	}

	var sb strings.Builder
	sb.WriteString("# Memory Index\n\n")
	sb.WriteString("Auto-generated index of memory entries. Do not edit manually.\n\n")
	for _, entry := range entries {
		desc := truncateForDescription(entry.Value)
		fileName := sanitizeFilename(entry.Key) + ".md"
		fmt.Fprintf(&sb, "- [%s](%s) — %s\n", entry.Key, fileName, desc)
	}
	return os.WriteFile(s.indexPath, []byte(sb.String()), 0o644)
}

func parseMemoryFile(content []byte) (memoryFrontmatter, string, error) {
	text := string(content)
	if !strings.HasPrefix(text, "---\n") {
		return memoryFrontmatter{}, text, nil
	}

	endIdx := strings.Index(text[4:], "\n---\n")
	if endIdx < 0 {
		return memoryFrontmatter{}, text, nil
	}
	endIdx += 4

	fmText := text[4:endIdx]
	body := strings.TrimSpace(text[endIdx+5:])

	var fm memoryFrontmatter
	if err := yaml.Unmarshal([]byte(fmText), &fm); err != nil {
		return memoryFrontmatter{}, "", fmt.Errorf("parsing frontmatter: %w", err)
	}

	return fm, body, nil
}

func buildMemoryFileContent(fm memoryFrontmatter, body string) string {
	fmBytes, err := yaml.Marshal(fm)
	if err != nil {
		fmBytes = []byte(fmt.Sprintf("key: %s\n", fm.Key))
	}

	var sb strings.Builder
	sb.WriteString("---\n")
	sb.Write(fmBytes)
	sb.WriteString("---\n\n")
	sb.WriteString(body)
	if !strings.HasSuffix(body, "\n") {
		sb.WriteString("\n")
	}
	return sb.String()
}

func sanitizeFilename(key string) string {
	replacer := strings.NewReplacer(
		"/", "__",
		"\\", "__",
		" ", "_",
		":", "-",
		"*", "",
		"?", "",
		"\"", "",
		"<", "",
		">", "",
		"|", "",
	)
	safe := replacer.Replace(key)
	if safe == "" {
		safe = "unnamed"
	}

	hash := sha256.Sum256([]byte(key))
	hashStr := hex.EncodeToString(hash[:4])

	if len(safe) > 50 {
		safe = safe[:50]
	}
	return safe + "_" + hashStr
}

func truncateForDescription(value string) string {
	trimmed := strings.TrimSpace(value)
	if len([]rune(trimmed)) <= 120 {
		return trimmed
	}
	return string([]rune(trimmed)[:120]) + "…"
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

type entryFilters struct {
	Scope    string
	Category string
	Type     string
	Tags     []string
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

// ReadIndex returns the raw content of the MEMORY.md index file.
func (s *service) ReadIndex() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	content, err := os.ReadFile(s.indexPath)
	if err != nil {
		return "", fmt.Errorf("reading memory index: %w", err)
	}
	return string(content), nil
}

// MemoryFileInfo holds metadata about a memory file without loading full content.
type MemoryFileInfo struct {
	Key         string
	FileName    string
	Description string
	Scope       string
	Category    string
	Type        string
	Tags        []string
	UpdatedAt   int64
}

// ListMemoryFiles returns metadata about all memory files without loading full content.
func (s *service) ListMemoryFiles() ([]MemoryFileInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	files, err := os.ReadDir(s.memoryDir)
	if err != nil {
		return nil, fmt.Errorf("reading memory directory: %w", err)
	}

	var infos []MemoryFileInfo
	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".md") || f.Name() == indexFilename {
			continue
		}
		content, err := os.ReadFile(filepath.Join(s.memoryDir, f.Name()))
		if err != nil {
			continue
		}
		fm, _, err := parseMemoryFile(content)
		if err != nil {
			continue
		}
		info, statErr := f.Info()
		updatedAt := time.Now().UnixNano()
		if statErr == nil {
			updatedAt = info.ModTime().UnixNano()
		}
		infos = append(infos, MemoryFileInfo{
			Key:         fm.Key,
			FileName:    f.Name(),
			Description: fm.Description,
			Scope:       fm.Scope,
			Category:    fm.Category,
			Type:        fm.Type,
			Tags:        normalizeTags(fm.Tags),
			UpdatedAt:   updatedAt,
		})
	}

	sort.Slice(infos, func(i, j int) bool {
		return infos[i].UpdatedAt > infos[j].UpdatedAt
	})

	if len(infos) > maxMemoryFiles {
		infos = infos[:maxMemoryFiles]
	}
	return infos, nil
}

// ReadMemoryFileBody reads just the body content of a memory file by filename.
func (s *service) ReadMemoryFileBody(fileName string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	fullPath := filepath.Join(s.memoryDir, fileName)
	cleanPath := filepath.Clean(fullPath)
	cleanDir := filepath.Clean(s.memoryDir) + string(filepath.Separator)
	if !strings.HasPrefix(cleanPath, cleanDir) {
		return "", fmt.Errorf("invalid memory file path")
	}

	content, err := os.ReadFile(fullPath)
	if err != nil {
		return "", fmt.Errorf("reading memory file: %w", err)
	}
	_, body, err := parseMemoryFile(content)
	return strings.TrimSpace(body), err
}
