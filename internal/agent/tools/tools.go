package tools

import (
	"context"

	"github.com/charmbracelet/crush/internal/session"
)

type (
	sessionIDContextKey       string
	messageIDContextKey       string
	supportsImagesKey         string
	modelNameKey              string
	workingDirContextKey      string
	sessionServiceContextKey  string
	toolCallIDContextKey      string
	agentMemoryContextKey     string
	agentIsolationContextKey  string
	agentBackgroundContextKey string
)

type sessionLookupService interface {
	Get(context.Context, string) (session.Session, error)
}

const (
	// SessionIDContextKey is the key for the session ID in the context.
	SessionIDContextKey sessionIDContextKey = "session_id"
	// MessageIDContextKey is the key for the message ID in the context.
	MessageIDContextKey messageIDContextKey = "message_id"
	// SupportsImagesContextKey is the key for the model's image support capability.
	SupportsImagesContextKey supportsImagesKey = "supports_images"
	// ModelNameContextKey is the key for the model name in the context.
	ModelNameContextKey modelNameKey = "model_name"
	// WorkingDirContextKey is the key for the session-specific working directory.
	WorkingDirContextKey      workingDirContextKey      = "working_dir"
	SessionServiceContextKey  sessionServiceContextKey  = "session_service"
	ToolCallIDContextKey      toolCallIDContextKey      = "tool_call_id"
	AgentMemoryContextKey     agentMemoryContextKey     = "agent_memory"
	AgentIsolationContextKey  agentIsolationContextKey  = "agent_isolation"
	AgentBackgroundContextKey agentBackgroundContextKey = "agent_background"
)

// getContextValue is a generic helper that retrieves a typed value from context.
// If the value is not found or has the wrong type, it returns the default value.
func getContextValue[T any](ctx context.Context, key any, defaultValue T) T {
	value := ctx.Value(key)
	if value == nil {
		return defaultValue
	}
	if typedValue, ok := value.(T); ok {
		return typedValue
	}
	return defaultValue
}

// GetSessionFromContext retrieves the session ID from the context.
func GetSessionFromContext(ctx context.Context) string {
	return getContextValue(ctx, SessionIDContextKey, "")
}

// GetMessageFromContext retrieves the message ID from the context.
func GetMessageFromContext(ctx context.Context) string {
	return getContextValue(ctx, MessageIDContextKey, "")
}

// GetSupportsImagesFromContext retrieves whether the model supports images from the context.
func GetSupportsImagesFromContext(ctx context.Context) bool {
	return getContextValue(ctx, SupportsImagesContextKey, false)
}

// GetModelNameFromContext retrieves the model name from the context.
func GetModelNameFromContext(ctx context.Context) string {
	return getContextValue(ctx, ModelNameContextKey, "")
}

// GetWorkingDirFromContext retrieves the session-specific working directory from context.
// Returns empty string if not set, in which case tools should fall back to the global working dir.
func GetWorkingDirFromContext(ctx context.Context) string {
	return getContextValue(ctx, WorkingDirContextKey, "")
}

func GetSessionServiceFromContext(ctx context.Context) sessionLookupService {
	return getContextValue(ctx, SessionServiceContextKey, sessionLookupService(nil))
}

func GetToolCallIDFromContext(ctx context.Context) string {
	return getContextValue(ctx, ToolCallIDContextKey, "")
}

func GetAgentMemoryFromContext(ctx context.Context) string {
	return getContextValue(ctx, AgentMemoryContextKey, "")
}

func GetAgentIsolationFromContext(ctx context.Context) string {
	return getContextValue(ctx, AgentIsolationContextKey, "")
}

func GetAgentBackgroundFromContext(ctx context.Context) bool {
	return getContextValue(ctx, AgentBackgroundContextKey, false)
}
