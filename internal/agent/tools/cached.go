package tools

import (
	"context"
	"fmt"
	"strings"

	"charm.land/fantasy"
)

// CachedTool wraps an AgentTool to cache large outputs and return truncated results.
// This allows agents to explore full outputs via output_head/output_tail/output_grep tools.
type CachedTool struct {
	tool fantasy.AgentTool
}

// WrapWithCaching wraps a tool to cache and truncate large outputs.
func WrapWithCaching(tool fantasy.AgentTool) fantasy.AgentTool {
	return &CachedTool{tool: tool}
}

// WrapAllWithCaching wraps all tools in the slice with output caching.
func WrapAllWithCaching(tools []fantasy.AgentTool) []fantasy.AgentTool {
	wrapped := make([]fantasy.AgentTool, len(tools))
	for i, tool := range tools {
		wrapped[i] = WrapWithCaching(tool)
	}
	return wrapped
}

// Info returns the tool info from the wrapped tool.
func (t *CachedTool) Info() fantasy.ToolInfo {
	return t.tool.Info()
}

// Run executes the wrapped tool with output caching.
func (t *CachedTool) Run(ctx context.Context, params fantasy.ToolCall) (fantasy.ToolResponse, error) {
	// Execute the tool.
	resp, err := t.tool.Run(ctx, params)
	if err != nil {
		return resp, err
	}

	// Don't cache error responses or empty responses.
	if resp.IsError || resp.Content == "" {
		return resp, nil
	}

	// Don't cache media/image responses.
	if len(resp.Data) > 0 {
		return resp, nil
	}

	// Check if output is large enough to cache.
	lines := strings.Split(resp.Content, "\n")
	totalLines := len(lines)

	if totalLines <= DefaultOutputLines {
		return resp, nil
	}

	// Cache the full output.
	sessionID := GetSessionFromContext(ctx)
	if sessionID == "" {
		// No session, can't cache - return original response.
		return resp, nil
	}

	GetOutputCache().Store(sessionID, params.ID, resp.Content)

	// Truncate to last N lines.
	start := totalLines - DefaultOutputLines
	truncatedContent := strings.Join(lines[start:], "\n")

	// Build response with truncation notice.
	var builder strings.Builder
	fmt.Fprintf(&builder, "[Showing last %d of %d lines. Use output_head/output_tail/output_grep with tool_call_id=%q to explore full output]\n\n",
		DefaultOutputLines, totalLines, params.ID)
	builder.WriteString(truncatedContent)

	// Create new response with truncated content.
	newResp := fantasy.ToolResponse{
		Type:      resp.Type,
		Content:   builder.String(),
		Metadata:  resp.Metadata,
		IsError:   resp.IsError,
		Data:      resp.Data,
		MediaType: resp.MediaType,
	}

	return newResp, nil
}

// ProviderOptions returns the provider options from the wrapped tool.
func (t *CachedTool) ProviderOptions() fantasy.ProviderOptions {
	return t.tool.ProviderOptions()
}

// SetProviderOptions sets provider options on the wrapped tool.
func (t *CachedTool) SetProviderOptions(opts fantasy.ProviderOptions) {
	t.tool.SetProviderOptions(opts)
}
