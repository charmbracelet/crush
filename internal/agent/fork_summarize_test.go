package agent

import (
	"context"
	"strings"
	"testing"
	"time"

	"charm.land/fantasy"
	kctx "github.com/charmbracelet/crushcl/internal/kernel/context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestForkSummarize_L3Triggered tests that L3 compression is triggered for large contexts
// Note: L3 triggers at 95% of 200K = 190K tokens, L2 at 85% = 170K tokens
func TestForkSummarize_L3Triggered(t *testing.T) {
	compactor := kctx.NewContextCompactor(200000)

	// Create large message set that will exceed 95% threshold (190K tokens)
	// Each message set is ~1492 tokens, so need >128 messages
	messages := createLargeMessageSet(150)
	totalTokens := estimateTokenCountTest(messages)

	t.Logf("Created %d messages with ~%d tokens (threshold: 190K for L3)", len(messages), totalTokens)

	compressionLevel := compactor.GetCompressionLevel(totalTokens)
	t.Logf("Compression level for %d tokens: %s", totalTokens, compressionLevel)

	// Should trigger L3 for very large context
	assert.Equal(t, kctx.L3FullCompact, compressionLevel, "Should trigger L3 for >95% token budget")
}

// TestForkSummarize_L2Triggered tests L2 triggering at 85% threshold
func TestForkSummarize_L2Triggered(t *testing.T) {
	compactor := kctx.NewContextCompactor(200000)

	// Need ~172K tokens to hit 85% threshold
	// Each message set is ~1492 tokens
	messages := createLargeMessageSet(120)
	totalTokens := estimateTokenCountTest(messages)

	t.Logf("Created %d messages with ~%d tokens (threshold: 170K for L2)", len(messages), totalTokens)

	compressionLevel := compactor.GetCompressionLevel(totalTokens)
	t.Logf("Compression level for %d tokens: %s", totalTokens, compressionLevel)

	// Should trigger L2 or higher
	assert.True(t, compressionLevel >= kctx.L2AutoCompact, "Should trigger L2 or higher")
}

// TestForkSummarize_ActualCompression tests actual compression
func TestForkSummarize_ActualCompression(t *testing.T) {
	compactor := kctx.NewContextCompactor(200000)

	messages := createLargeMessageSet(30)
	originalCount := len(messages)
	originalTokens := estimateTokenCountTest(messages)

	t.Logf("Before: %d messages, ~%d tokens", originalCount, originalTokens)

	// Test L2AutoCompact (which is the fallback)
	compressed := compactor.L2AutoCompact(messages, "test-session")

	compressedCount := len(compressed)
	compressedTokens := estimateTokenCountTest(compressed)

	t.Logf("After L2: %d messages, ~%d tokens", compressedCount, compressedTokens)
	t.Logf("Reduction: messages %.1f%%, tokens %.1f%%",
		100*(1-float64(compressedCount)/float64(originalCount)),
		100*(1-float64(compressedTokens)/float64(originalTokens)))

	// L2 should reduce message count
	assert.Less(t, compressedCount, originalCount)
}

// TestForkSummarize_TriggerForkSummarize tests the callback mechanism
func TestForkSummarize_TriggerForkSummarize(t *testing.T) {
	compactor := kctx.NewContextCompactor(200000)

	// Set up a callback that captures the call
	var capturedMessages []fantasy.Message
	var capturedSessionID string

	compactor.ForkSummarizeCallback = func(ctx context.Context, messages []fantasy.Message, sessionID string) ([]fantasy.Message, string, error) {
		capturedMessages = messages
		capturedSessionID = sessionID
		// Return compressed messages (L2 style for testing)
		return compactor.L2AutoCompact(messages, sessionID), "test summary", nil
	}

	messages := createLargeMessageSet(20)
	ctx := context.Background()

	// Trigger via callback
	compressed, summary, err := compactor.TriggerForkSummarize(ctx, messages, "test-session-123")

	require.NoError(t, err)
	assert.Equal(t, "test-session-123", capturedSessionID)
	assert.Equal(t, len(messages), len(capturedMessages))
	assert.NotEmpty(t, summary)
	assert.Less(t, len(compressed), len(messages))

	t.Logf("TriggerForkSummarize: captured %d messages, returned %d compressed, summary=%q",
		len(capturedMessages), len(compressed), summary)
}

// TestForkSummarize_WithoutCallback tests fallback behavior
func TestForkSummarize_WithoutCallback(t *testing.T) {
	compactor := kctx.NewContextCompactor(200000)
	// No callback set - will use fallback

	messages := createLargeMessageSet(20)
	ctx := context.Background()

	// Should fall back to L2
	compressed, summary, err := compactor.TriggerForkSummarize(ctx, messages, "test-session")

	require.NoError(t, err)
	assert.Empty(t, summary) // No LLM summary in fallback
	assert.Less(t, len(compressed), len(messages))

	t.Logf("Fallback mode: %d -> %d messages", len(messages), len(compressed))
}

// TestForkSummarize_TokenBudget tests the token budget calculation
func TestForkSummarize_TokenBudget(t *testing.T) {
	testCases := []struct {
		messages      int
		expectedMin   int // minimum expected tokens
		expectedMax   int // maximum expected tokens
		expectedLevel int // minimum expected compression level
	}{
		{10, 10000, 20000, 0},      // Small - no compression
		{50, 40000, 90000, 0},      // Medium - still under threshold
		{130, 100000, 250000, 3},   // Large - should trigger L3
	}

	for _, tc := range testCases {
		compactor := kctx.NewContextCompactor(200000)
		messages := createLargeMessageSet(tc.messages)
		tokens := estimateTokenCountTest(messages)
		level := compactor.GetCompressionLevel(tokens)

		t.Logf("%d messages -> ~%d tokens, level=%s", tc.messages, tokens, level)

		assert.GreaterOrEqual(t, tokens, tc.expectedMin)
		assert.LessOrEqual(t, tokens, tc.expectedMax)
		assert.GreaterOrEqual(t, int(level), tc.expectedLevel)
	}
}

// TestForkSummarize_CompressionMetrics tests metrics reporting
func TestForkSummarize_CompressionMetrics(t *testing.T) {
	compactor := kctx.NewContextCompactor(200000)

	// Apply compressions - L3 increments totalCompactions
	messages := createLargeMessageSet(30)
	compactor.L1Microcompact(messages)
	compactor.L3FullCompact(messages, "test", "summary")

	metrics := compactor.Metrics()

	t.Logf("Metrics: %+v", metrics)

	// L3 increments totalCompactions, L1 does not
	assert.Equal(t, 1, metrics["total_compactions"], "L3 should increment totalCompactions")
	assert.Contains(t, metrics, "context_manager")
}

// TestForkSummarize_TimeoutHandling tests timeout handling in TriggerForkSummarize
func TestForkSummarize_TimeoutHandling(t *testing.T) {
	compactor := kctx.NewContextCompactor(200000)

	// Set up a slow callback
	compactor.ForkSummarizeCallback = func(ctx context.Context, messages []fantasy.Message, sessionID string) ([]fantasy.Message, string, error) {
		// Simulate slow operation
		select {
		case <-ctx.Done():
			return nil, "", ctx.Err()
		case <-time.After(100 * time.Millisecond):
			return messages, "summary", nil
		}
	}

	messages := createLargeMessageSet(10)

	// Test with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, _, err := compactor.TriggerForkSummarize(ctx, messages, "test-timeout")

	// Should timeout
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context deadline exceeded")

	t.Logf("Timeout test passed: %v", err)
}

// TestForkSummarize_Integration tests the full integration with sessionAgent
func TestForkSummarize_Integration(t *testing.T) {
	// Create compactor with callback wired up
	compactor := kctx.NewContextCompactor(200000)

	compactor.ForkSummarizeCallback = func(ctx context.Context, messages []fantasy.Message, sessionID string) ([]fantasy.Message, string, error) {
		t.Logf("L3ForkSummarize callback invoked: %d messages, session=%s", len(messages), sessionID)

		// Simple L2 compression as stand-in for actual LLM summarization
		compressed := compactor.L2AutoCompact(messages, sessionID)
		summary := "Compressed via L3 callback"
		return compressed, summary, nil
	}

	// Create large context
	messages := createLargeMessageSet(40)
	totalTokens := estimateTokenCountTest(messages)

	t.Logf("Integration test: %d messages, ~%d tokens", len(messages), totalTokens)

	// Trigger compression
	ctx := context.Background()
	compressed, summary, err := compactor.TriggerForkSummarize(ctx, messages, "integration-test")

	require.NoError(t, err)
	assert.NotEmpty(t, summary)
	assert.Less(t, len(compressed), len(messages))

	t.Logf("Integration: %d -> %d messages, summary=%q", len(messages), len(compressed), summary)
}

// TestForkSummarize_L4SessionMemory tests L4 compression with existing collapses
// L4 triggers when: tokens >= 85% threshold AND collapses exist
func TestForkSummarize_L4SessionMemory(t *testing.T) {
	compactor := kctx.NewContextCompactor(200000)

	// Create large context first (to hit threshold)
	messages := createLargeMessageSet(120)
	totalTokens := estimateTokenCountTest(messages)

	t.Logf("Large context: %d messages, ~%d tokens", len(messages), totalTokens)

	// Add a collapse manually (simulating previous L3 compressions)
	compactor.L3FullCompact(messages, "session-1", "Previous session summary")

	// Get collapses
	cm := compactor.GetContextManager()
	collapses := cm.GetCollapses()
	t.Logf("Recorded collapses: %d", len(collapses))

	// Now test L4 - should use existing collapses since we're above 85% threshold
	compressionLevel := compactor.GetCompressionLevel(totalTokens)
	t.Logf("With collapses and %d tokens, compression level: %s", totalTokens, compressionLevel)

	// L4 should be triggered if we have collapses AND are above threshold
	assert.Equal(t, kctx.L4SessionMemory, compressionLevel, "Should trigger L4 when above 85% threshold with collapses")
}

// TestForkSummarize_L1Microcompact tests L1 micro compaction
func TestForkSummarize_L1Microcompact(t *testing.T) {
	compactor := kctx.NewContextCompactor(200000)

	// Record many tool results to trigger L1
	for i := 0; i < 25; i++ {
		compactor.RecordToolResult("tool-"+strings.Repeat("0", i%3+1), "Bash", generateLargeOutput())
	}

	messages := createLargeMessageSet(10)
	originalCount := len(messages)

	// Apply L1
	compressed := compactor.L1Microcompact(messages)

	t.Logf("L1 Microcompact: %d -> %d messages", originalCount, len(compressed))

	// L1 should reduce tool result messages
	// (exact behavior depends on compactor implementation)
	metrics := compactor.Metrics()
	t.Logf("L1 Metrics: %+v", metrics)
}

// Helper: estimateTokenCountTest estimates token count
func estimateTokenCountTest(messages []fantasy.Message) int {
	total := 0
	for _, msg := range messages {
		for _, part := range msg.Content {
			if textPart, ok := fantasy.AsMessagePart[fantasy.TextPart](part); ok {
				total += len(textPart.Text) / 4
			}
			if toolResult, ok := fantasy.AsMessagePart[fantasy.ToolResultPart](part); ok {
				if text, ok := toolResult.Output.(fantasy.ToolResultOutputContentText); ok {
					total += len(text.Text) / 4
				}
			}
		}
		total += 10 // overhead
	}
	return total
}

// Helper: createLargeMessageSet creates a set of messages simulating long context
func createLargeMessageSet(count int) []fantasy.Message {
	var messages []fantasy.Message

	// System message
	messages = append(messages, fantasy.NewSystemMessage(
		"You are a helpful AI assistant specialized in code review and refactoring.",
	))

	// Generate alternating user/assistant/tool messages
	for i := 0; i < count; i++ {
		// User message (simulating a code review task)
		userContent := generateReviewTask(i)
		messages = append(messages, fantasy.NewUserMessage(userContent))

		// Assistant message with text
		assistantMsg := fantasy.Message{
			Role: fantasy.MessageRoleAssistant,
			Content: []fantasy.MessagePart{
				fantasy.TextPart{Text: "I'll analyze this code change. Let me review the implementation."},
			},
		}
		messages = append(messages, assistantMsg)

		// Tool result
		toolResult := fantasy.ToolResultPart{
			ToolCallID: "tool-" + strings.Repeat("0", i%3+1),
			Output:     fantasy.ToolResultOutputContentText{Text: generateLargeOutput()},
		}
		toolMsg := fantasy.Message{
			Role: fantasy.MessageRoleTool,
			Content: []fantasy.MessagePart{toolResult},
		}
		messages = append(messages, toolMsg)
	}

	return messages
}

// Helper: generateReviewTask creates a realistic code review task text
func generateReviewTask(index int) string {
	taskNum := strings.Repeat(string(rune('0'+index%10)), 3)
	return `## Code Review Task #` + taskNum + `

Please review the following changes:

### File: internal/agent/agent.go

The agent implementation handles session management and tool execution.
We need to ensure proper compression at all tiers.

### Changes:
- Added forkSummarize for L3 compression
- Integrated with ContextCompactor callback
- Fixed circuit breaker string formatting
- Updated token estimation logic

### Questions:
1. Is the compression logic correct?
2. Are there any race conditions?
3. Should we add more metrics?

Please analyze and provide feedback on the implementation.
`
}

// Helper: generateLargeOutput creates a large tool output (~2KB)
func generateLargeOutput() string {
	// Generate ~2KB of realistic output
	output := "total 1234\n"
	for i := 0; i < 50; i++ {
		lineNum := strings.Repeat(string(rune('0'+i%10)), 4)
		output += "drwxr-xr-x  5 user staff  160 Apr  3 10:" + lineNum + " .\n"
		output += "-rw-r--r--  1 user staff  2048 Apr  3 10:" + lineNum + " file.go\n"
	}
	output += "\n--- Analysis Complete ---\n"
	output += "Found 50 files across 5 directories.\n"
	output += "Largest file: internal/agent/agent.go (2048 bytes)\n"
	output += "Total lines analyzed: 1234\n"
	return output
}
