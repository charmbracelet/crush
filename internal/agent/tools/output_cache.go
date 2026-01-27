package tools

import (
	"regexp"
	"strings"
	"sync"
	"time"
)

// OutputCache stores tool outputs for later retrieval via head/tail/grep operations.
// Outputs are stored per-session and expire after a retention period.
type OutputCache struct {
	mu      sync.RWMutex
	entries map[string]*outputEntry // keyed by "sessionID:toolCallID"
}

type outputEntry struct {
	output    string
	lines     []string // pre-split lines for efficient pagination
	createdAt time.Time
}

const (
	// OutputCacheRetention is how long to keep cached outputs (30 minutes).
	OutputCacheRetention = 30 * time.Minute
	// DefaultOutputLines is the default number of lines to return.
	DefaultOutputLines = 100
	// MaxOutputLines is the maximum number of lines that can be requested at once.
	MaxOutputLines = 500
)

var (
	globalOutputCache     *OutputCache
	globalOutputCacheOnce sync.Once
)

// GetOutputCache returns the singleton output cache.
func GetOutputCache() *OutputCache {
	globalOutputCacheOnce.Do(func() {
		globalOutputCache = &OutputCache{
			entries: make(map[string]*outputEntry),
		}
	})
	return globalOutputCache
}

// Store saves tool output for later retrieval.
func (c *OutputCache) Store(sessionID, toolCallID, output string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := sessionID + ":" + toolCallID
	c.entries[key] = &outputEntry{
		output:    output,
		lines:     strings.Split(output, "\n"),
		createdAt: time.Now(),
	}
}

// Get retrieves raw output for a tool call.
func (c *OutputCache) Get(sessionID, toolCallID string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	key := sessionID + ":" + toolCallID
	entry, ok := c.entries[key]
	if !ok {
		return "", false
	}
	return entry.output, true
}

// Head returns the first n lines of the cached output.
func (c *OutputCache) Head(sessionID, toolCallID string, n, offset int) (result string, totalLines int, hasMore bool, ok bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	key := sessionID + ":" + toolCallID
	entry, exists := c.entries[key]
	if !exists {
		return "", 0, false, false
	}

	totalLines = len(entry.lines)
	if offset >= totalLines {
		return "", totalLines, false, true
	}

	end := offset + n
	if end > totalLines {
		end = totalLines
	}

	result = strings.Join(entry.lines[offset:end], "\n")
	hasMore = end < totalLines
	return result, totalLines, hasMore, true
}

// Tail returns the last n lines of the cached output.
func (c *OutputCache) Tail(sessionID, toolCallID string, n, offset int) (result string, totalLines int, hasMore bool, ok bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	key := sessionID + ":" + toolCallID
	entry, exists := c.entries[key]
	if !exists {
		return "", 0, false, false
	}

	totalLines = len(entry.lines)

	// offset from end: 0 = last n lines, 1 = skip last line, etc.
	end := totalLines - offset
	if end <= 0 {
		return "", totalLines, false, true
	}

	start := end - n
	if start < 0 {
		start = 0
	}

	result = strings.Join(entry.lines[start:end], "\n")
	hasMore = start > 0
	return result, totalLines, hasMore, true
}

// GrepResult represents a single grep match.
type GrepResult struct {
	LineNum int
	Line    string
}

// Grep searches for a pattern in the cached output.
func (c *OutputCache) Grep(sessionID, toolCallID, pattern string, contextLines int) (matches []GrepResult, totalLines int, ok bool, err error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	key := sessionID + ":" + toolCallID
	entry, exists := c.entries[key]
	if !exists {
		return nil, 0, false, nil
	}

	regex, err := regexp.Compile(pattern)
	if err != nil {
		return nil, 0, true, err
	}

	totalLines = len(entry.lines)
	matches = make([]GrepResult, 0)

	for i, line := range entry.lines {
		if regex.MatchString(line) {
			matches = append(matches, GrepResult{
				LineNum: i + 1, // 1-indexed
				Line:    line,
			})
			// Limit matches to prevent huge responses
			if len(matches) >= 100 {
				break
			}
		}
	}

	return matches, totalLines, true, nil
}

// TotalLines returns the total number of lines in the cached output.
func (c *OutputCache) TotalLines(sessionID, toolCallID string) (int, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	key := sessionID + ":" + toolCallID
	entry, ok := c.entries[key]
	if !ok {
		return 0, false
	}
	return len(entry.lines), true
}

// Cleanup removes expired entries from the cache.
func (c *OutputCache) Cleanup() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	removed := 0

	for key, entry := range c.entries {
		if now.Sub(entry.createdAt) > OutputCacheRetention {
			delete(c.entries, key)
			removed++
		}
	}

	return removed
}

// Clear removes all entries for a session.
func (c *OutputCache) Clear(sessionID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	prefix := sessionID + ":"
	for key := range c.entries {
		if strings.HasPrefix(key, prefix) {
			delete(c.entries, key)
		}
	}
}

// tailOutput returns the last n lines of content and whether it was truncated.
// This is used by the CachedTool wrapper to truncate large outputs.
func tailOutput(content string, n int) (result string, truncated bool) {
	if content == "" {
		return "", false
	}

	lines := strings.Split(content, "\n")
	totalLines := len(lines)

	if totalLines <= n {
		return content, false
	}

	// Return last n lines.
	start := totalLines - n
	return strings.Join(lines[start:], "\n"), true
}
