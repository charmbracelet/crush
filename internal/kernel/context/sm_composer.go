package context

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"charm.land/fantasy"
)

// SMComposer constructs SM (Session Memory) summary text from blocks
// It combines multiple session memory blocks into a coherent summary
type SMComposer struct {
	mu sync.RWMutex

	config ComposerConfig

	// Cached compositions for performance
	cache map[string]*CompositionCache
}

// ComposerConfig holds configuration for composition
type ComposerConfig struct {
	// Max summary length in characters
	MaxSummaryLength int

	// Include key facts in summary
	IncludeKeyFacts bool

	// Include topic tags in summary
	IncludeTags bool

	// Format for timestamps
	TimestampFormat string

	// Template for composition
	Template string

	// Token budget for the summary (approximate chars = tokens * 4)
	TokenBudget int
}

// DefaultComposerConfig returns the default composition config
func DefaultComposerConfig() ComposerConfig {
	return ComposerConfig{
		MaxSummaryLength:  4000,  // ~1000 tokens
		IncludeKeyFacts:   true,
		IncludeTags:       true,
		TimestampFormat:   "2006-01-02 15:04",
		Template:          "## Session Memory Summary\n\n{{.Timestamp}}\n\n{{.Tags}}\n\n{{.Summary}}\n\n{{.KeyFacts}}\n\n{{.Metadata}}",
		TokenBudget:       1000,
	}
}

// CompositionCache caches composed summaries for reuse
type CompositionCache struct {
	Summary     string
	BlockIDs    []string
	CreatedAt   time.Time
	ExpiresAt   time.Time
	TokensUsed  int
}

// NewSMComposer creates a new SM composer
func NewSMComposer(config ComposerConfig) *SMComposer {
	if config.MaxSummaryLength == 0 {
		config = DefaultComposerConfig()
	}
	return &SMComposer{
		config: config,
		cache:  make(map[string]*CompositionCache),
	}
}

// CompositionResult contains the result of composition
type CompositionResult struct {
	Summary    string
	TokensUsed int
	BlocksUsed int
	Truncated  bool
	Metadata   CompositionMetadata
}

// CompositionMetadata contains metadata about the composition
type CompositionMetadata struct {
	BlockIDs      []string
	TimeRange    string
	TopicsCovered []string
	CompressionRatio float64
}

// Compose creates a summary from selected blocks
func (c *SMComposer) Compose(blocks []*SessionMemoryBlock, contextTopics []string) *CompositionResult {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(blocks) == 0 {
		return &CompositionResult{
			Summary:    "",
			TokensUsed: 0,
			BlocksUsed: 0,
		}
	}

	result := &CompositionResult{
		BlocksUsed: len(blocks),
		Metadata: CompositionMetadata{
			BlockIDs:      make([]string, len(blocks)),
			TopicsCovered: contextTopics,
		},
	}

	// Copy block IDs
	for i, block := range blocks {
		result.Metadata.BlockIDs[i] = block.ID
	}

	// Build summary parts
	var parts []string

	// Header
	header := c.buildHeader(blocks)
	parts = append(parts, header)

	// Tags section
	if c.config.IncludeTags && len(contextTopics) > 0 {
		tagsSection := c.buildTagsSection(contextTopics)
		parts = append(parts, tagsSection)
	}

	// Summary section
	summarySection := c.buildSummarySection(blocks)
	parts = append(parts, summarySection)

	// Key facts section
	if c.config.IncludeKeyFacts {
		keyFactsSection := c.buildKeyFactsSection(blocks)
		parts = append(parts, keyFactsSection)
	}

	// Combine and truncate if needed
	fullSummary := strings.Join(parts, "\n\n")
	
	// Check token budget
	result.TokensUsed = len(fullSummary) / 4 // Rough estimate
	if result.TokensUsed > c.config.TokenBudget {
		fullSummary = c.truncateToBudget(fullSummary)
		result.Truncated = true
		result.TokensUsed = c.config.TokenBudget
	}

	// Check hard limit
	if len(fullSummary) > c.config.MaxSummaryLength {
		fullSummary = fullSummary[:c.config.MaxSummaryLength-100] + "\n\n*[truncated]*"
		result.Truncated = true
	}

	result.Summary = fullSummary

	// Calculate compression ratio
	originalTokens := c.calculateOriginalTokens(blocks)
	if originalTokens > 0 {
		result.Metadata.CompressionRatio = float64(result.TokensUsed) / float64(originalTokens)
	}

	// Update time range
	result.Metadata.TimeRange = c.buildTimeRange(blocks)

	return result
}

// buildHeader creates the header section
func (c *SMComposer) buildHeader(blocks []*SessionMemoryBlock) string {
	var sb strings.Builder

	earliest := blocks[0].CreatedAt
	latest := blocks[0].CreatedAt

	for _, block := range blocks {
		if block.CreatedAt.Before(earliest) {
			earliest = block.CreatedAt
		}
		if block.CreatedAt.After(latest) {
			latest = block.CreatedAt
		}
	}

	sb.WriteString(fmt.Sprintf("## Session Memory Summary (%d blocks)", len(blocks)))
	sb.WriteString(fmt.Sprintf("\n\n*Period: %s - %s*",
		earliest.Format(c.config.TimestampFormat),
		latest.Format(c.config.TimestampFormat)))

	return sb.String()
}

// buildTagsSection creates the topic tags section
func (c *SMComposer) buildTagsSection(topics []string) string {
	if len(topics) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("### Topics\n")
	sb.WriteString("`")
	sb.WriteString(strings.Join(topics, "`, `"))
	sb.WriteString("`\n")

	return sb.String()
}

// buildSummarySection creates the main summary section
func (c *SMComposer) buildSummarySection(blocks []*SessionMemoryBlock) string {
	var sb strings.Builder
	sb.WriteString("### Summary\n")

	for i, block := range blocks {
		if block.Summary != "" {
			sb.WriteString(fmt.Sprintf("\n**Block %d (%s):**\n", i+1, block.ID))
			sb.WriteString(block.Summary)
		} else if len(block.Messages) > 0 {
			// Fallback: generate summary from messages
			sb.WriteString(fmt.Sprintf("\n**Block %d (%s):**\n", i+1, block.ID))
			sb.WriteString(c.summarizeMessages(block.Messages))
		}
	}

	return sb.String()
}

// buildKeyFactsSection creates the key facts section
func (c *SMComposer) buildKeyFactsSection(blocks []*SessionMemoryBlock) string {
	var sb strings.Builder
	var allFacts []string

	for _, block := range blocks {
		allFacts = append(allFacts, block.KeyFacts...)
	}

	if len(allFacts) == 0 {
		return ""
	}

	sb.WriteString("\n### Key Facts\n")
	for i, fact := range allFacts {
		if i >= 20 {
			sb.WriteString(fmt.Sprintf("\n*... and %d more facts", len(allFacts)-20))
			break
		}
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, fact))
	}

	return sb.String()
}

// buildTimeRange creates a time range string
func (c *SMComposer) buildTimeRange(blocks []*SessionMemoryBlock) string {
	if len(blocks) == 0 {
		return ""
	}

	earliest := blocks[0].CreatedAt
	latest := blocks[0].CreatedAt

	for _, block := range blocks {
		if block.CreatedAt.Before(earliest) {
			earliest = block.CreatedAt
		}
		if block.CreatedAt.After(latest) {
			latest = block.CreatedAt
		}
	}

	return fmt.Sprintf("%s to %s",
		earliest.Format(c.config.TimestampFormat),
		latest.Format(c.config.TimestampFormat))
}

// summarizeMessages generates a summary from messages
func (c *SMComposer) summarizeMessages(messages []fantasy.Message) string {
	if len(messages) == 0 {
		return "[No messages]"
	}

	var sb strings.Builder
	userCount := 0
	assistantCount := 0
	toolCount := 0

	for _, msg := range messages {
		switch msg.Role {
		case fantasy.MessageRoleUser:
			userCount++
			if text := extractTextContent(msg.Content); len(text) > 0 {
				if len(text) > 100 {
					text = text[:100] + "..."
				}
				sb.WriteString(fmt.Sprintf("- User %d: %s\n", userCount, text))
			}
		case fantasy.MessageRoleAssistant:
			assistantCount++
			sb.WriteString(fmt.Sprintf("- Assistant %d: [response]\n", assistantCount))
		case fantasy.MessageRoleTool:
			toolCount++
		}
	}

	if sb.Len() == 0 {
		sb.WriteString(fmt.Sprintf("[%d user messages, %d assistant responses, %d tool calls]",
			userCount, assistantCount, toolCount))
	}

	return sb.String()
}

// truncateToBudget truncates summary to fit token budget
func (c *SMComposer) truncateToBudget(summary string) string {
	maxChars := c.config.TokenBudget * 4 // Conservative: 4 chars per token

	if len(summary) <= maxChars {
		return summary
	}

	// Find a good break point (end of sentence or paragraph)
	breakPoint := maxChars
	for i := maxChars; i > maxChars-200 && i > 0; i-- {
		if summary[i] == '.' || summary[i] == '\n' {
			breakPoint = i + 1
			break
		}
	}

	return summary[:breakPoint] + "\n\n*[summary truncated to fit budget]*"
}

// calculateOriginalTokens estimates original token count from blocks
func (c *SMComposer) calculateOriginalTokens(blocks []*SessionMemoryBlock) int {
	total := 0
	for _, block := range blocks {
		for _, msg := range block.Messages {
			// Rough estimate: count content parts
			for _, part := range msg.Content {
				if textPart, ok := fantasy.AsMessagePart[fantasy.TextPart](part); ok {
					total += len(textPart.Text) / 4
				}
				if toolResult, ok := fantasy.AsMessagePart[fantasy.ToolResultPart](part); ok {
					// Extract text from tool result based on output type
					if text, ok := fantasy.AsToolResultOutputType[fantasy.ToolResultOutputContentText](toolResult.Output); ok {
						total += len(text.Text) / 4
					}
				}
			}
		}
	}
	return total
}

// CacheComposition caches a composed summary
func (c *SMComposer) CacheComposition(key string, summary string, blockIDs []string, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache[key] = &CompositionCache{
		Summary:   summary,
		BlockIDs:  blockIDs,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(ttl),
		TokensUsed: len(summary) / 4,
	}
}

// GetCachedComposition retrieves a cached composition if valid
func (c *SMComposer) GetCachedComposition(key string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	cached, ok := c.cache[key]
	if !ok {
		return "", false
	}

	if time.Now().After(cached.ExpiresAt) {
		return "", false
	}

	return cached.Summary, true
}

// ClearCache removes all cached compositions
func (c *SMComposer) ClearCache() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache = make(map[string]*CompositionCache)
}

// Metrics returns composer metrics
func (c *SMComposer) Metrics() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	validCache := 0
	for _, cached := range c.cache {
		if time.Now().Before(cached.ExpiresAt) {
			validCache++
		}
	}

	return map[string]interface{}{
		"cache_entries": len(c.cache),
		"valid_cache":   validCache,
		"token_budget":  c.config.TokenBudget,
		"max_length":    c.config.MaxSummaryLength,
	}
}
