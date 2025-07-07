package mcp

import (
	"context"
	"fmt"
)

// TypedToolHandlerFunc is a function that handles a tool call with typed arguments
type TypedToolHandlerFunc[T any] func(ctx context.Context, request CallToolRequest, args T) (*CallToolResult, error)

// NewTypedToolHandler creates a ToolHandlerFunc that automatically binds arguments to a typed struct
func NewTypedToolHandler[T any](handler TypedToolHandlerFunc[T]) func(ctx context.Context, request CallToolRequest) (*CallToolResult, error) {
	return func(ctx context.Context, request CallToolRequest) (*CallToolResult, error) {
		var args T
		if err := request.BindArguments(&args); err != nil {
			return NewToolResultError(fmt.Sprintf("failed to bind arguments: %v", err)), nil
		}
		return handler(ctx, request, args)
	}
}
