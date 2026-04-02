package tools

import (
	"context"
	"time"

	"github.com/charmbracelet/crush/internal/toolruntime"
)

func reportToolRuntime(
	ctx context.Context,
	toolCallID string,
	toolName string,
	status toolruntime.Status,
	snapshot string,
	clientMetadata map[string]any,
) {
	sessionID := GetSessionFromContext(ctx)
	if sessionID == "" {
		sessionID = toolruntime.SessionIDFromContext(ctx)
	}
	if toolCallID == "" {
		toolCallID = GetToolCallIDFromContext(ctx)
	}
	if toolCallID == "" {
		toolCallID = toolruntime.ToolCallIDFromContext(ctx)
	}
	if sessionID == "" || toolCallID == "" || toolName == "" {
		return
	}

	if clientMetadata == nil {
		clientMetadata = map[string]any{}
	}
	if _, ok := clientMetadata["duration_ms"]; !ok {
		if start, ok := clientMetadata["started_at_unix_ms"].(int64); ok && start > 0 {
			duration := time.Now().UnixMilli() - start
			if duration < 0 {
				duration = 0
			}
			clientMetadata["duration_ms"] = duration
		}
	}

	toolruntime.Report(ctx, toolruntime.State{
		SessionID:      sessionID,
		ToolCallID:     toolCallID,
		ToolName:       toolName,
		Status:         status,
		SnapshotText:   snapshot,
		ClientMetadata: clientMetadata,
	})
}

func detachedToolRuntimeContext(ctx context.Context) context.Context {
	service := toolruntime.ServiceFromContext(ctx)
	if service == nil {
		return context.Background()
	}
	return toolruntime.WithService(context.Background(), service)
}
