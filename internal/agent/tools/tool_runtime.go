package tools

import (
	"context"

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
	if sessionID == "" || toolCallID == "" || toolName == "" {
		return
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
