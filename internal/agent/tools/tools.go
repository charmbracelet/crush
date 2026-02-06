package tools

import (
	"context"
	"fmt"
	"strings"
)

const (
	// DefaultMaxToolOutputLength is the fallback maximum length when context info
	// is unavailable. ~20KB is roughly 5000-7500 tokens.
	DefaultMaxToolOutputLength = 20_000

	// MinToolOutputLength ensures tools can always return some useful content.
	MinToolOutputLength = 4_000

	// ContextBufferRatio is the percentage of context window to keep free
	// for model responses after tool output.
	ContextBufferRatio = 0.15

	// CharsPerToken is a rough estimate for token calculation.
	// Most tokenizers average 3-4 chars per token.
	CharsPerToken = 4
)

type (
	sessionIDContextKey    string
	messageIDContextKey    string
	supportsImagesKey      string
	modelNameKey           string
	contextWindowKey       string
	usedTokensKey          string
	contextWindowAvailable string
)

const (
	// SessionIDContextKey is the key for the session ID in the context.
	SessionIDContextKey sessionIDContextKey = "session_id"
	// MessageIDContextKey is the key for the message ID in the context.
	MessageIDContextKey messageIDContextKey = "message_id"
	// SupportsImagesContextKey is the key for the model's image support capability.
	SupportsImagesContextKey supportsImagesKey = "supports_images"
	// ModelNameContextKey is the key for the model name in the context.
	ModelNameContextKey modelNameKey = "model_name"
	// ContextWindowSizeKey is the key for the model's context window size in tokens.
	ContextWindowSizeKey contextWindowKey = "context_window_size"
	// UsedTokensKey is the key for the current token usage in the context.
	UsedTokensKey usedTokensKey = "used_tokens"
	// ContextWindowAvailableKey is the key for the available context window in chars.
	ContextWindowAvailableKey contextWindowAvailable = "context_window_available"
)

// GetSessionFromContext retrieves the session ID from the context.
func GetSessionFromContext(ctx context.Context) string {
	sessionID := ctx.Value(SessionIDContextKey)
	if sessionID == nil {
		return ""
	}
	s, ok := sessionID.(string)
	if !ok {
		return ""
	}
	return s
}

// GetMessageFromContext retrieves the message ID from the context.
func GetMessageFromContext(ctx context.Context) string {
	messageID := ctx.Value(MessageIDContextKey)
	if messageID == nil {
		return ""
	}
	s, ok := messageID.(string)
	if !ok {
		return ""
	}
	return s
}

// GetSupportsImagesFromContext retrieves whether the model supports images from the context.
func GetSupportsImagesFromContext(ctx context.Context) bool {
	supportsImages := ctx.Value(SupportsImagesContextKey)
	if supportsImages == nil {
		return false
	}
	if supports, ok := supportsImages.(bool); ok {
		return supports
	}
	return false
}

// GetModelNameFromContext retrieves the model name from the context.
func GetModelNameFromContext(ctx context.Context) string {
	modelName := ctx.Value(ModelNameContextKey)
	if modelName == nil {
		return ""
	}
	s, ok := modelName.(string)
	if !ok {
		return ""
	}
	return s
}

// GetContextWindowAvailable retrieves the available context window in chars from context.
func GetContextWindowAvailable(ctx context.Context) int {
	available := ctx.Value(ContextWindowAvailableKey)
	if available == nil {
		return 0
	}
	if v, ok := available.(int); ok {
		return v
	}
	return 0
}

// MaxOutputLength calculates the maximum tool output length based on context.
// If context info is available, it calculates dynamically to leave 15% buffer.
// Otherwise, it falls back to DefaultMaxToolOutputLength.
func MaxOutputLength(ctx context.Context) int {
	available := GetContextWindowAvailable(ctx)
	if available <= 0 {
		return DefaultMaxToolOutputLength
	}

	// Use 85% of available space (leave 15% for model response).
	maxLen := int(float64(available) * (1 - ContextBufferRatio))

	// Clamp to reasonable bounds.
	if maxLen < MinToolOutputLength {
		return MinToolOutputLength
	}
	if maxLen > DefaultMaxToolOutputLength {
		return DefaultMaxToolOutputLength
	}
	return maxLen
}

// TruncateOutput truncates tool output to fit within context limits.
// It keeps the beginning and end to preserve context.
func TruncateOutput(content string) string {
	return TruncateOutputWithLimit(content, DefaultMaxToolOutputLength)
}

// TruncateOutputCtx truncates tool output based on available context window.
func TruncateOutputCtx(ctx context.Context, content string) string {
	return TruncateOutputWithLimit(content, MaxOutputLength(ctx))
}

// TruncateOutputWithLimit truncates tool output to the specified limit.
func TruncateOutputWithLimit(content string, maxLen int) string {
	if len(content) <= maxLen {
		return content
	}

	halfLength := maxLen / 2
	start := content[:halfLength]
	end := content[len(content)-halfLength:]

	// Count lines in the truncated middle section.
	middle := content[halfLength : len(content)-halfLength]
	truncatedLines := strings.Count(middle, "\n") + 1
	truncatedBytes := len(middle)

	return fmt.Sprintf(
		"%s\n\n[... %d lines (%d bytes) truncated ...]\n\n%s",
		start, truncatedLines, truncatedBytes, end,
	)
}
