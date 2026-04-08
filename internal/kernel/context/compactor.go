package context

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"charm.land/fantasy"
)

// ForkSummarizeFunc is the callback type for fork summarization
// Returns summarized messages and summary text, called when L3FullCompact is triggered
type ForkSummarizeFunc func(ctx context.Context, messages []fantasy.Message, sessionID string) ([]fantasy.Message, string, error)

// String returns the compression level name
func (cl CompressionLevel) String() string {
	switch cl {
	case L1Microcompact:
		return "L1Microcompact"
	case L2AutoCompact:
		return "L2AutoCompact"
	case L3FullCompact:
		return "L3FullCompact"
	case L4SessionMemory:
		return "L4SessionMemory"
	default:
		return "Unknown"
	}
}

// CompactableTool represents a tool whose output can be compressed
type CompactableTool string

const (
	ToolRead     CompactableTool = "Read"
	ToolBash     CompactableTool = "Bash"
	ToolGrep     CompactableTool = "Grep"
	ToolGlob     CompactableTool = "Glob"
	ToolWebFetch CompactableTool = "WebFetch"
	ToolEdit     CompactableTool = "Edit"
	ToolWrite    CompactableTool = "Write"
)

// CompactorToolResult is the internal tool result tracking for ContextCompactor
type CompactorToolResult struct {
	ID        string
	ToolName  string
	Content   string
	Timestamp time.Time
	IsFrozen  bool
}

// ContextCompactor implements Claude Code's 4-tier compression system
type ContextCompactor struct {
	maxTokenBudget   int
	keepRecentCount  int
	thresholdRatio   float64
	fullCompactRatio float64
	compactableTools map[CompactableTool]bool

	ctxManager *ContextManager

	// SM Compression components
	memPool  *SessionMemoryPool
	hitCalc  *MemoryHitCalculator
	composer *SMComposer

	toolResults      []CompactorToolResult
	suppressedCount  int
	lastCompactTime  time.Time
	compressionLevel CompressionLevel
	totalCompactions int

	// ForkSummarizeCallback is called when L3FullCompact is triggered
	// This allows the sessionAgent to perform async LLM-based summarization
	ForkSummarizeCallback ForkSummarizeFunc
}

// NewContextCompactor creates a new context compactor with integrated ContextManager
func NewContextCompactor(maxTokenBudget int) *ContextCompactor {
	cc := &ContextCompactor{
		maxTokenBudget:   maxTokenBudget,
		keepRecentCount:  3,
		thresholdRatio:   0.85,
		fullCompactRatio: 0.95,
		compactableTools: map[CompactableTool]bool{
			ToolRead:     true,
			ToolBash:     true,
			ToolGrep:     true,
			ToolGlob:     true,
			ToolWebFetch: true,
			ToolEdit:     true,
			ToolWrite:    true,
		},
		toolResults: make([]CompactorToolResult, 0),
	}

	// Initialize integrated ContextManager
	cc.ctxManager = New(DefaultConfig())

	// Initialize SM Compression components
	cc.memPool = NewSessionMemoryPool(50, 100) // 50 blocks max, 100 msgs per block
	cc.hitCalc = NewMemoryHitCalculator(DefaultHitWeightConfig())
	cc.composer = NewSMComposer(DefaultComposerConfig())

	return cc
}

// IntegrateContextManager sets the ContextManager for core compression operations
func (cc *ContextCompactor) IntegrateContextManager(cm *ContextManager) {
	cc.ctxManager = cm
}

// GetContextManager returns the integrated ContextManager
func (cc *ContextCompactor) GetContextManager() *ContextManager {
	return cc.ctxManager
}

// IsCompactable checks if a tool can be microcompacted
func (cc *ContextCompactor) IsCompactable(toolName string) bool {
	return cc.compactableTools[CompactableTool(toolName)]
}

// RecordToolResult records a tool execution for potential compression
func (cc *ContextCompactor) RecordToolResult(id, toolName, content string) {
	tr := CompactorToolResult{
		ID:        id,
		ToolName:  toolName,
		Content:   content,
		Timestamp: time.Now(),
		IsFrozen:  false,
	}
	cc.toolResults = append(cc.toolResults, tr)

	// Also record in integrated ContextManager if available
	if cc.ctxManager != nil {
		cc.ctxManager.AddToolResult(&ToolResult{
			ID:       id,
			ToolName: toolName,
			Output:   content,
			Tier:     TierFresh,
			Frozen:   false,
		})
	}
}

// Freeze marks a tool result as frozen (cannot be compressed)
func (cc *ContextCompactor) Freeze(id string) {
	for i := range cc.toolResults {
		if cc.toolResults[i].ID == id {
			cc.toolResults[i].IsFrozen = true
			break
		}
	}

	// Also freeze in ContextManager
	if cc.ctxManager != nil {
		cc.ctxManager.Freeze(id)
	}
}

// GetCompressionLevel determines the appropriate compression level based on token usage
func (cc *ContextCompactor) GetCompressionLevel(currentTokens int) CompressionLevel {
	ratio := float64(currentTokens) / float64(cc.maxTokenBudget)

	switch {
	case ratio >= cc.fullCompactRatio:
		return L3FullCompact
	case ratio >= cc.thresholdRatio:
		// Check if we have session memory collapses available
		if cc.ctxManager != nil && len(cc.ctxManager.GetCollapses()) > 0 {
			return L4SessionMemory
		}
		return L2AutoCompact
	case len(cc.toolResults) > 20:
		return L1Microcompact
	default:
		return 0 // No compression needed
	}
}

// ShouldAutoCompact checks if automatic compaction should be triggered
func (cc *ContextCompactor) ShouldAutoCompact(currentTokens int) bool {
	return cc.GetCompressionLevel(currentTokens) > 0
}

// L1Microcompact performs rule-based cleanup of old tool results
// This is the fastest compression, targeting <1ms execution
func (cc *ContextCompactor) L1Microcompact(messages []fantasy.Message) []fantasy.Message {
	now := time.Now()
	var result []fantasy.Message
	suppressed := 0

	// Use ContextManager's tool budget if available
	if cc.ctxManager != nil {
		result = cc.ctxManager.ApplyToolBudget(messages)
		cc.lastCompactTime = now
		return result
	}

	// Fallback: simple rule-based cleanup
	for _, msg := range messages {
		if msg.Role == fantasy.MessageRoleTool {
			// Skip if we have too many tool results
			if suppressed >= 20 && !cc.shouldPreserveToolResult(msg) {
				suppressed++
				continue
			}
		}
		result = append(result, msg)
	}

	cc.suppressedCount = suppressed
	cc.lastCompactTime = now
	cc.compressionLevel = L1Microcompact
	return result
}

// shouldPreserveToolResult determines if a tool result should be preserved
func (cc *ContextCompactor) shouldPreserveToolResult(msg fantasy.Message) bool {
	// Preserve tool results that are frozen or important
	for _, part := range msg.Content {
		if toolResult, ok := fantasy.AsMessagePart[fantasy.ToolResultPart](part); ok {
			// Preserve short results
			if text, ok := fantasy.AsToolResultOutputType[fantasy.ToolResultOutputContentText](toolResult.Output); ok {
				return len(text.Text) < 500
			}
		}
	}
	return false
}

// L2AutoCompact performs threshold-triggered compression using summarization
func (cc *ContextCompactor) L2AutoCompact(messages []fantasy.Message, sessionID string) []fantasy.Message {
	// Use ContextManager's hook if available
	if cc.ctxManager != nil && cc.ctxManager.onCompactHook != nil {
		result := cc.ctxManager.RunCompactHook(messages)
		cc.compressionLevel = L2AutoCompact
		return result
	}

	// Fallback: basic summary-based compression
	return cc.performBasicCompression(messages, 4)
}

// EmergencyCompact performs aggressive compression for critical context overflow.
// It keeps only 2 recent messages and truncates large individual messages.
// This is used when context exceeds model limits and other compression has failed.
func (cc *ContextCompactor) EmergencyCompact(messages []fantasy.Message, sessionID string) []fantasy.Message {
	const (
		keepRecent    = 2
		maxMessageLen = 2000 // Max characters per message after truncation
	)

	if len(messages) <= keepRecent+1 {
		// Even the messages we would keep are too large, truncate them
		return cc.truncateMessages(messages, maxMessageLen)
	}

	var systemMsgs []fantasy.Message
	var recentMsgs []fantasy.Message
	var middleMsgs []fantasy.Message

	for i, msg := range messages {
		if i == 0 && msg.Role == fantasy.MessageRoleSystem {
			systemMsgs = append(systemMsgs, msg)
		} else if i >= len(messages)-keepRecent {
			recentMsgs = append(recentMsgs, msg)
		} else {
			middleMsgs = append(middleMsgs, msg)
		}
	}

	// Build summary from middle messages
	var summary strings.Builder
	summary.WriteString("## Previous Conversation Summary\n\n")
	summary.WriteString(cc.summarizeMessages(middleMsgs))
	summary.WriteString("\n---\n*Above conversation has been summarized due to length.*\n\n")

	// Rebuild messages with truncated content
	var result []fantasy.Message
	result = append(result, systemMsgs...)

	// Truncate summary if too long
	summaryStr := summary.String()
	if len(summaryStr) > maxMessageLen {
		summaryStr = summaryStr[:maxMessageLen] + "... [truncated]"
	}
	result = append(result, fantasy.NewUserMessage(summaryStr))

	// Truncate recent messages
	for _, msg := range recentMsgs {
		truncated := cc.truncateMessage(msg, maxMessageLen)
		result = append(result, truncated)
	}

	cc.totalCompactions++
	cc.compressionLevel = L2AutoCompact
	slog.Warn("Emergency compaction applied", "original_count", len(messages), "result_count", len(result))
	return result
}

// truncateMessages truncates all messages to maxLen characters
func (cc *ContextCompactor) truncateMessages(messages []fantasy.Message, maxLen int) []fantasy.Message {
	var result []fantasy.Message
	for _, msg := range messages {
		result = append(result, cc.truncateMessage(msg, maxLen))
	}
	return result
}

// truncateMessage truncates a single message's content to maxLen characters
func (cc *ContextCompactor) truncateMessage(msg fantasy.Message, maxLen int) fantasy.Message {
	// Extract current content
	content := extractTextContent(msg.Content)
	if len(content) <= maxLen {
		return msg
	}

	// Truncate content and create new message with truncated content
	truncatedContent := content[:maxLen] + "... [content truncated]"

	// Create appropriate message type based on role
	switch msg.Role {
	case fantasy.MessageRoleUser:
		return fantasy.NewUserMessage(truncatedContent)
	case fantasy.MessageRoleSystem:
		return fantasy.NewSystemMessage(truncatedContent)
	default:
		// For other roles, return user message with truncated content
		return fantasy.NewUserMessage(truncatedContent)
	}
}

// L3FullCompact performs fork agent summarization for severe context overflow
func (cc *ContextCompactor) L3FullCompact(messages []fantasy.Message, sessionID string, summary string) []fantasy.Message {
	// Record the collapse in ContextManager
	if cc.ctxManager != nil {
		commit := CollapseCommit{
			ID:        sessionID,
			Timestamp: time.Now(),
			Messages:  messages,
			Summary:   summary,
		}
		cc.ctxManager.AddCollapse(commit)
	}

	cc.compressionLevel = L3FullCompact
	cc.totalCompactions++
	return messages
}

// TriggerForkSummarize invokes the fork summarization callback if configured
// Returns the compressed messages and summary, or the original messages if no callback
func (cc *ContextCompactor) TriggerForkSummarize(ctx context.Context, messages []fantasy.Message, sessionID string) ([]fantasy.Message, string, error) {
	if cc.ForkSummarizeCallback == nil {
		// No callback configured, fall back to basic compression
		slog.Warn("L3ForkSummarize: No callback configured, falling back to L2")
		return cc.L2AutoCompact(messages, sessionID), "", nil
	}

	slog.Info("L3ForkSummarize: Triggering async fork summarization", "sessionID", sessionID)
	return cc.ForkSummarizeCallback(ctx, messages, sessionID)
}

// L4SessionMemory uses existing collapse summaries for projection
func (cc *ContextCompactor) L4SessionMemory(messages []fantasy.Message) []fantasy.Message {
	if cc.ctxManager != nil {
		result := cc.ctxManager.ProjectView(messages, true)
		cc.compressionLevel = L4SessionMemory
		return result
	}
	return messages
}

// performBasicCompression performs basic message compression
func (cc *ContextCompactor) performBasicCompression(messages []fantasy.Message, keepRecent int) []fantasy.Message {
	if len(messages) <= keepRecent+1 {
		return messages
	}

	var systemMsgs []fantasy.Message
	var recentMsgs []fantasy.Message
	var middleMsgs []fantasy.Message

	for i, msg := range messages {
		if i == 0 && msg.Role == fantasy.MessageRoleSystem {
			systemMsgs = append(systemMsgs, msg)
		} else if i >= len(messages)-keepRecent {
			recentMsgs = append(recentMsgs, msg)
		} else {
			middleMsgs = append(middleMsgs, msg)
		}
	}

	if len(middleMsgs) == 0 {
		return messages
	}

	// Build summary
	var summary strings.Builder
	summary.WriteString("## Previous Conversation Summary\n\n")
	summary.WriteString(cc.summarizeMessages(middleMsgs))
	summary.WriteString("\n---\n*Above conversation has been summarized due to length.*\n\n")

	// Rebuild messages
	var result []fantasy.Message
	result = append(result, systemMsgs...)
	result = append(result, fantasy.NewUserMessage(summary.String()))
	result = append(result, recentMsgs...)

	cc.totalCompactions++
	return result
}

// summarizeMessages creates a text summary of messages
func (cc *ContextCompactor) summarizeMessages(messages []fantasy.Message) string {
	var summary strings.Builder
	userCount := 0
	assistantCount := 0

	for _, msg := range messages {
		switch msg.Role {
		case fantasy.MessageRoleUser:
			userCount++
			if text := extractTextContent(msg.Content); len(text) > 0 {
				if len(text) > 200 {
					text = text[:200] + "..."
				}
				summary.WriteString(fmt.Sprintf("**User %d**: %s\n\n", userCount, text))
			}
		case fantasy.MessageRoleAssistant:
			assistantCount++
			hasToolCalls := false
			for _, part := range msg.Content {
				if _, ok := fantasy.AsMessagePart[fantasy.ToolResultPart](part); ok {
					hasToolCalls = true
					break
				}
			}
			if hasToolCalls {
				summary.WriteString(fmt.Sprintf("**Assistant %d**: [Used tools]\n\n", assistantCount))
			} else {
				summary.WriteString(fmt.Sprintf("**Assistant %d**: [Response omitted]\n\n", assistantCount))
			}
		}
	}

	return summary.String()
}

// GetCompactableToolResults returns tool results that can be compressed
func (cc *ContextCompactor) GetCompactableToolResults() []CompactorToolResult {
	var result []CompactorToolResult
	for _, tr := range cc.toolResults {
		if !tr.IsFrozen && cc.IsCompactable(tr.ToolName) {
			result = append(result, tr)
		}
	}
	return result
}

// GenerateCompactPrompt creates a prompt for LLM-based summarization
func (cc *ContextCompactor) GenerateCompactPrompt(toPreserve []fantasy.Message, context string) string {
	var sb strings.Builder
	sb.WriteString("## Context Compression Request\n\n")
	sb.WriteString("The conversation history is becoming too long. Please summarize the following messages ")
	sb.WriteString("while preserving critical information:\n\n")

	for _, msg := range toPreserve {
		switch msg.Role {
		case fantasy.MessageRoleUser:
			sb.WriteString("**User**: ")
			if content := extractTextContent(msg.Content); len(content) > 200 {
				sb.WriteString(content[:200] + "... [truncated]\n")
			} else {
				sb.WriteString(content + "\n")
			}
		case fantasy.MessageRoleAssistant:
			sb.WriteString("**Assistant**: [omitted for brevity]\n")
		case fantasy.MessageRoleTool:
			sb.WriteString("**Tool Result**: [summarized tool output]\n")
		}
	}

	sb.WriteString("\n## Context\n")
	sb.WriteString(context)
	sb.WriteString("\n\nPlease provide a concise summary that preserves key decisions, important code changes, and user preferences.")

	return sb.String()
}

// MaxTokenBudget returns the configured max token budget
func (cc *ContextCompactor) MaxTokenBudget() int {
	return cc.maxTokenBudget
}

// UpdateMaxTokenBudget updates the max token budget at runtime
// This is needed when models are switched and the budget must be recalculated
func (cc *ContextCompactor) UpdateMaxTokenBudget(newBudget int) {
	cc.maxTokenBudget = newBudget
}

// CompressionLevel returns the last compression level applied
func (cc *ContextCompactor) CompressionLevel() CompressionLevel {
	return cc.compressionLevel
}

// TotalCompactions returns the total number of compactions performed
func (cc *ContextCompactor) TotalCompactions() int {
	return cc.totalCompactions
}

// Metrics returns current compaction metrics
func (cc *ContextCompactor) Metrics() map[string]interface{} {
	metrics := map[string]interface{}{
		"total_tool_results":     len(cc.toolResults),
		"suppressed_count":       cc.suppressedCount,
		"compactable_tools":      len(cc.compactableTools),
		"last_compact_time":      cc.lastCompactTime,
		"last_compression_level": cc.compressionLevel.String(),
		"threshold_ratio":        cc.thresholdRatio,
		"full_compact_ratio":     cc.fullCompactRatio,
		"max_token_budget":       cc.maxTokenBudget,
		"total_compactions":      cc.totalCompactions,
	}

	// Add ContextManager metrics if available
	if cc.ctxManager != nil {
		metrics["context_manager"] = cc.ctxManager.Metrics()
	}

	// Add SM Compression metrics
	metrics["sm_compression"] = cc.SMMetrics()

	return metrics
}

// Reset clears the compactor state
func (cc *ContextCompactor) Reset() {
	cc.toolResults = make([]CompactorToolResult, 0)
	cc.suppressedCount = 0
	cc.lastCompactTime = time.Time{}
	cc.compressionLevel = 0
	// Note: We don't reset ContextManager here to preserve session memory

	// Reset SM components but preserve pool data
	if cc.hitCalc != nil {
		cc.hitCalc.Reset()
	}
	if cc.composer != nil {
		cc.composer.ClearCache()
	}
}

// FullReset clears all state including ContextManager and SM components
func (cc *ContextCompactor) FullReset() {
	cc.Reset()
	if cc.ctxManager != nil {
		cc.ctxManager = New(DefaultConfig())
	}

	// Full reset of SM components
	cc.memPool = NewSessionMemoryPool(50, 100)
	cc.hitCalc = NewMemoryHitCalculator(DefaultHitWeightConfig())
	cc.composer = NewSMComposer(DefaultComposerConfig())
}

// extractTextContent extracts text content from message parts
func extractTextContent(parts []fantasy.MessagePart) string {
	var sb strings.Builder
	for _, part := range parts {
		if textPart, ok := fantasy.AsMessagePart[fantasy.TextPart](part); ok {
			sb.WriteString(textPart.Text)
		}
		if toolResult, ok := fantasy.AsMessagePart[fantasy.ToolResultPart](part); ok {
			if text, ok := fantasy.AsToolResultOutputType[fantasy.ToolResultOutputContentText](toolResult.Output); ok {
				sb.WriteString(text.Text)
			}
		}
	}
	return sb.String()
}

// SM Compression Integration Methods

// GetMemoryPool returns the session memory pool
func (cc *ContextCompactor) GetMemoryPool() *SessionMemoryPool {
	return cc.memPool
}

// GetHitCalculator returns the memory hit calculator
func (cc *ContextCompactor) GetHitCalculator() *MemoryHitCalculator {
	return cc.hitCalc
}

// GetComposer returns the SM composer
func (cc *ContextCompactor) GetComposer() *SMComposer {
	return cc.composer
}

// RecordMemoryBlock records a session memory block for SM compression
func (cc *ContextCompactor) RecordMemoryBlock(block *SessionMemoryBlock) error {
	if cc.memPool == nil {
		return fmt.Errorf("memory pool not initialized")
	}
	return cc.memPool.AddBlock(block)
}

// GetRelevantBlocks retrieves blocks relevant to the given topics
func (cc *ContextCompactor) GetRelevantBlocks(topics []string, limit int) []*SessionMemoryBlock {
	if cc.memPool == nil || cc.hitCalc == nil {
		return nil
	}

	// Record topics for hit calculation
	cc.hitCalc.RecordTopics(topics)

	// Get all blocks
	blocks := cc.memPool.GetRecentBlocks(cc.memPool.Size())
	if len(blocks) == 0 {
		return nil
	}

	// Calculate hits and select top blocks
	selected := cc.hitCalc.SelectTopBlocks(blocks, topics, limit)

	return selected
}

// ComposeMemorySummary creates a summary from relevant memory blocks
func (cc *ContextCompactor) ComposeMemorySummary(topics []string, maxBlocks int) *CompositionResult {
	if cc.composer == nil {
		return nil
	}

	blocks := cc.GetRelevantBlocks(topics, maxBlocks)
	if len(blocks) == 0 {
		return &CompositionResult{
			Summary:    "",
			TokensUsed: 0,
			BlocksUsed: 0,
		}
	}

	return cc.composer.Compose(blocks, topics)
}

// SMCompact performs SM-based compression using session memory pool
// This is the fast path for L4 compression, targeting <10ms execution
func (cc *ContextCompactor) SMCompact(messages []fantasy.Message, topics []string) ([]fantasy.Message, string, error) {
	if cc.composer == nil || cc.memPool == nil {
		return messages, "", fmt.Errorf("SM components not initialized")
	}

	// Extract topics from messages if not provided
	if len(topics) == 0 {
		topics = cc.extractTopics(messages)
	}

	// Create memory block from messages
	block := &SessionMemoryBlock{
		ID:       fmt.Sprintf("block-%d", time.Now().UnixNano()),
		Messages: messages,
		Tags:     topics,
		Weight:   1.0, // Full weight for current context
	}

	// Add to pool
	if err := cc.memPool.AddBlock(block); err != nil {
		slog.Warn("Failed to add memory block", "error", err)
	}

	// Get relevant blocks for composition
	blocks := cc.GetRelevantBlocks(topics, 10)
	if len(blocks) == 0 {
		return messages, "", nil
	}

	// Compose summary
	result := cc.composer.Compose(blocks, topics)

	// Build compressed messages
	compressed := cc.buildCompressedMessages(messages, result.Summary)

	return compressed, result.Summary, nil
}

// extractTopics extracts topic tags from messages
func (cc *ContextCompactor) extractTopics(messages []fantasy.Message) []string {
	topicSet := make(map[string]bool)

	for _, msg := range messages {
		switch msg.Role {
		case fantasy.MessageRoleUser:
			text := extractTextContent(msg.Content)
			// Simple keyword extraction (in production, use NLP)
			keywords := []string{"code", "test", "bug", "feature", "api", "file", "error", "build", "run"}
			for _, kw := range keywords {
				if strings.Contains(strings.ToLower(text), kw) {
					topicSet[kw] = true
				}
			}
		case fantasy.MessageRoleTool:
			topicSet["tool-call"] = true
		}
	}

	topics := make([]string, 0, len(topicSet))
	for t := range topicSet {
		topics = append(topics, t)
	}

	return topics
}

// buildCompressedMessages rebuilds message list with summary
func (cc *ContextCompactor) buildCompressedMessages(original []fantasy.Message, summary string) []fantasy.Message {
	if summary == "" || len(original) == 0 {
		return original
	}

	var result []fantasy.Message
	var systemMsgs []fantasy.Message
	var recentMsgs []fantasy.Message
	var middleStartIdx int

	// Separate system, recent, and middle messages
	for i, msg := range original {
		if i == 0 && msg.Role == fantasy.MessageRoleSystem {
			systemMsgs = append(systemMsgs, msg)
		} else if i >= len(original)-cc.keepRecentCount {
			recentMsgs = append(recentMsgs, msg)
		} else {
			if middleStartIdx == 0 {
				middleStartIdx = i
			}
		}
	}

	// Build result with summary
	result = append(result, systemMsgs...)
	result = append(result, fantasy.NewSystemMessage("## Session Memory\n\n"+summary+"\n\n*Previous conversation summarized.*"))
	result = append(result, recentMsgs...)

	cc.totalCompactions++
	cc.compressionLevel = L4SessionMemory

	return result
}

// SMMetrics returns metrics for SM compression components
func (cc *ContextCompactor) SMMetrics() map[string]interface{} {
	metrics := map[string]interface{}{}

	if cc.memPool != nil {
		metrics["memory_pool"] = cc.memPool.Metrics()
	}

	if cc.hitCalc != nil {
		topicStats := cc.hitCalc.GetTopicStats()
		statsMap := make([]map[string]interface{}, len(topicStats))
		for i, s := range topicStats {
			statsMap[i] = map[string]interface{}{
				"topic":     s.Topic,
				"frequency": s.Frequency,
			}
		}
		metrics["topic_stats"] = statsMap
	}

	if cc.composer != nil {
		metrics["composer"] = cc.composer.Metrics()
	}

	return metrics
}
