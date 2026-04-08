package memory

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// MemoryLayer represents the tier of memory
type MemoryLayer int

const (
	LayerSession    MemoryLayer = iota + 1 // Current session only, temporary
	LayerPersistent                       // Cross-session, per-project, permanent
	LayerTeam                             // Cross-user, per-repo, remote + local
)

// MemoryType represents the category of memory
type MemoryType string

const (
	MemoryTypeUser     MemoryType = "user"
	MemoryTypeFeedback MemoryType = "feedback"
	MemoryTypeProject  MemoryType = "project"
	MemoryTypeReference MemoryType = "reference"
)

// MemoryEntry represents a single memory item
type MemoryEntry struct {
	ID          string
	Type        MemoryType
	Name        string
	Description string
	Content     string
	Layer       MemoryLayer
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Why         string // Why this was saved
	HowToApply  string // How to apply this memory
}

// MemoryStore implements Claude Code's 3-layer memory architecture
type MemoryStore struct {
	mu sync.RWMutex

	// Layer 1: Session memory (in-memory, temporary)
	sessionMemory []MemoryEntry

	// Layer 2: Persistent memory (file-based, permanent)
	persistentPath string

	// Layer 3: Team memory (remote sync + local mirror)
	teamPath  string
	localPath string
}

// NewMemoryStore creates a new memory store
func NewMemoryStore(dataDir string) *MemoryStore {
	store := &MemoryStore{
		sessionMemory: make([]MemoryEntry, 0),
		persistentPath: filepath.Join(dataDir, "memory"),
		teamPath:      filepath.Join(dataDir, "memory", "team"),
		localPath:     filepath.Join(dataDir, "memory", "local"),
	}

	// Ensure directories exist
	os.MkdirAll(store.persistentPath, 0o700)
	os.MkdirAll(store.teamPath, 0o700)
	os.MkdirAll(store.localPath, 0o700)

	return store
}

// AddSessionMemory adds a memory entry to the session layer
func (ms *MemoryStore) AddSessionMemory(entry MemoryEntry) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	entry.Layer = LayerSession
	entry.CreatedAt = time.Now()
	entry.UpdatedAt = time.Now()

	ms.sessionMemory = append(ms.sessionMemory, entry)
}

// AddPersistentMemory saves memory to disk (Layer 2)
func (ms *MemoryStore) AddPersistentMemory(entry MemoryEntry) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	entry.Layer = LayerPersistent
	entry.CreatedAt = time.Now()
	entry.UpdatedAt = time.Now()

	// Generate filename from name
	filename := sanitizeFilename(entry.Name) + ".md"
	filepath := filepath.Join(ms.persistentPath, filename)

	// Write memory file with frontmatter
	content := formatMemoryFile(entry)
	return os.WriteFile(filepath, []byte(content), 0o600)
}

// AddTeamMemory saves memory to team layer (Layer 3)
func (ms *MemoryStore) AddTeamMemory(entry MemoryEntry) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	entry.Layer = LayerTeam
	entry.CreatedAt = time.Now()
	entry.UpdatedAt = time.Now()

	filename := sanitizeFilename(entry.Name) + ".md"
	filepath := filepath.Join(ms.teamPath, filename)

	content := formatMemoryFile(entry)
	return os.WriteFile(filepath, []byte(content), 0o644)
}

// GetMemory retrieves memory entries from all layers
func (ms *MemoryStore) GetMemory(memoryType MemoryType, limit int) []MemoryEntry {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	var results []MemoryEntry

	// Layer 1: Session (most recent first)
	for i := len(ms.sessionMemory) - 1; i >= 0 && len(results) < limit; i-- {
		if memoryType == "" || ms.sessionMemory[i].Type == memoryType {
			results = append(results, ms.sessionMemory[i])
		}
	}

	// Layer 2: Persistent (read from files)
	entries, _ := ms.readMemoryFiles(ms.persistentPath, memoryType)
	for _, e := range entries {
		if len(results) >= limit {
			break
		}
		results = append(results, e)
	}

	return results
}

// GetContextSummary returns a formatted summary for system prompts
func (ms *MemoryStore) GetContextSummary() string {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	var sb strings.Builder
	sb.WriteString("\n\n## Memory Context\n\n")

	// Session memories
	if len(ms.sessionMemory) > 0 {
		sb.WriteString("### Current Session\n")
		for _, m := range ms.sessionMemory {
			sb.WriteString(fmt.Sprintf("- %s: %s\n", m.Name, m.Description))
		}
		sb.WriteString("\n")
	}

	// Persistent memories (from index)
	indexContent, err := os.ReadFile(filepath.Join(ms.persistentPath, "MEMORY.md"))
	if err == nil {
		lines := strings.Split(string(indexContent), "\n")
		for i, line := range lines {
			if i > 50 { // Limit to first 50 lines
				sb.WriteString("- ... [more memories]\n")
				break
			}
			if strings.HasPrefix(line, "- [") {
				sb.WriteString(line + "\n")
			}
		}
	}

	return sb.String()
}

// readMemoryFiles reads all memory files from a directory
func (ms *MemoryStore) readMemoryFiles(dir string, memoryType MemoryType) ([]MemoryEntry, error) {
	var entries []MemoryEntry

	files, err := os.ReadDir(dir)
	if err != nil {
		return entries, err
	}

	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".md") {
			continue
		}

		filepath := filepath.Join(dir, file.Name())
		content, err := os.ReadFile(filepath)
		if err != nil {
			continue
		}

		entry := parseMemoryFile(string(content))
		if memoryType == "" || entry.Type == memoryType {
			entries = append(entries, entry)
		}
	}

	return entries, nil
}

// formatMemoryFile formats a memory entry as Markdown with YAML frontmatter
func formatMemoryFile(entry MemoryEntry) string {
	var sb strings.Builder
	sb.WriteString("---\n")
	sb.WriteString(fmt.Sprintf("name: %s\n", entry.Name))
	sb.WriteString(fmt.Sprintf("description: %s\n", entry.Description))
	sb.WriteString(fmt.Sprintf("type: %s\n", entry.Type))
	sb.WriteString(fmt.Sprintf("created: %s\n", entry.CreatedAt.Format(time.RFC3339)))
	sb.WriteString("---\n\n")
	sb.WriteString(entry.Content)
	if entry.Why != "" {
		sb.WriteString("\n\n**Why:** ")
		sb.WriteString(entry.Why)
	}
	if entry.HowToApply != "" {
		sb.WriteString("\n**How to apply:** ")
		sb.WriteString(entry.HowToApply)
	}
	return sb.String()
}

// parseMemoryFile parses a memory file back into an entry
func parseMemoryFile(content string) MemoryEntry {
	entry := MemoryEntry{}

	lines := strings.Split(content, "\n")
	inFrontmatter := false
	var bodyLines []string

	for _, line := range lines {
		if strings.TrimSpace(line) == "---" {
			if !inFrontmatter {
				inFrontmatter = true
				continue
			}
			inFrontmatter = false
			continue
		}

		if inFrontmatter {
			if strings.HasPrefix(line, "name: ") {
				entry.Name = strings.TrimPrefix(line, "name: ")
			} else if strings.HasPrefix(line, "description: ") {
				entry.Description = strings.TrimPrefix(line, "description: ")
			} else if strings.HasPrefix(line, "type: ") {
				entry.Type = MemoryType(strings.TrimPrefix(line, "type: "))
			}
		} else {
			bodyLines = append(bodyLines, line)
		}
	}

	entry.Content = strings.Join(bodyLines, "\n")

	// Parse Why/HowToApply from content
	if idx := strings.Index(entry.Content, "**Why:**"); idx >= 0 {
		endIdx := strings.Index(entry.Content[idx:], "\n")
		if endIdx > 0 {
			entry.Why = entry.Content[idx+len("**Why:**") : idx+endIdx]
		}
	}
	if idx := strings.Index(entry.Content, "**How to apply:**"); idx >= 0 {
		endIdx := strings.Index(entry.Content[idx:], "\n")
		if endIdx > 0 {
			entry.HowToApply = entry.Content[idx+len("**How to apply:**") : idx+endIdx]
		}
	}

	return entry
}

// sanitizeFilename creates a safe filename from a memory name
func sanitizeFilename(name string) string {
	// Replace spaces with underscores, remove special chars
	result := strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			return r
		}
		return '_'
	}, name)

	// Limit length
	if len(result) > 50 {
		result = result[:50]
	}

	return result
}

// ClearSession clears session memory only
func (ms *MemoryStore) ClearSession() {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.sessionMemory = make([]MemoryEntry, 0)
}

// Metrics returns memory statistics
func (ms *MemoryStore) Metrics() map[string]interface{} {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	return map[string]interface{}{
		"session_count":    len(ms.sessionMemory),
		"persistent_path": ms.persistentPath,
		"team_path":        ms.teamPath,
	}
}

// WeibullDecayConfig holds parameters for Weibull decay calculation
type WeibullDecayConfig struct {
	Shape    float64 // k (kappa) - shape parameter, controls decay curve
	Scale    float64 // λ (lambda) - scale parameter, controls characteristic lifetime
}

// DefaultWeibullConfig returns default Weibull parameters
// Shape < 1: memory fades quickly at first
// Shape = 1: exponential decay
// Shape > 1: memory persists longer then fades
func DefaultWeibullConfig() WeibullDecayConfig {
	return WeibullDecayConfig{
		Shape: 0.8,  // Slightly less than 1 for gentle early decay
		Scale: 24 * 7, // Scale factor (hours) - about 1 week characteristic time
	}
}

// CalculateDecay computes the Weibull decay factor for a given age
// Returns value between 0 (forgotten) and 1 (fully remembered)
func (wdc WeibullDecayConfig) CalculateDecay(ageHours float64) float64 {
	if ageHours <= 0 {
		return 1.0
	}
	// Weibull survival function: exp(-(t/λ)^k)
	normalizedAge := ageHours / wdc.Scale
	if normalizedAge > 700 { // prevent overflow for very old memories
		normalizedAge = 700
	}
	decay := math.Exp(-math.Pow(normalizedAge, wdc.Shape))
	return decay
}

// CalculateMemoryRelevance returns a relevance score for a memory entry
// based on its age using Weibull decay
func (ms *MemoryStore) CalculateMemoryRelevance(entry MemoryEntry) float64 {
	config := DefaultWeibullConfig()
	ageHours := time.Since(entry.CreatedAt).Hours()
	return config.CalculateDecay(ageHours)
}

// GetMemoryWithDecay retrieves memories sorted by relevance (decay-adjusted)
func (ms *MemoryStore) GetMemoryWithDecay(memoryType MemoryType, limit int) []MemoryEntry {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	type scoredEntry struct {
		entry MemoryEntry
		score float64
	}

	var scored []scoredEntry

	// Score session memories
	for _, e := range ms.sessionMemory {
		if memoryType == "" || e.Type == memoryType {
			ageHours := time.Since(e.CreatedAt).Hours()
			decay := DefaultWeibullConfig().CalculateDecay(ageHours)
			scored = append(scored, scoredEntry{entry: e, score: decay})
		}
	}

	// Read persistent memories
	entries, _ := ms.readMemoryFiles(ms.persistentPath, memoryType)
	for _, e := range entries {
		ageHours := time.Since(e.CreatedAt).Hours()
		decay := DefaultWeibullConfig().CalculateDecay(ageHours)
		scored = append(scored, scoredEntry{entry: e, score: decay})
	}

	// Sort by score descending (most relevant first)
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	// Take top results
	result := make([]MemoryEntry, 0, limit)
	for i := 0; i < len(scored) && i < limit; i++ {
		result = append(result, scored[i].entry)
	}

	return result
}
