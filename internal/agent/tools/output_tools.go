package tools

import (
	"context"
	_ "embed"
	"fmt"
	"strings"

	"charm.land/fantasy"
)

//go:embed output_head.md
var outputHeadDescription []byte

//go:embed output_tail.md
var outputTailDescription []byte

//go:embed output_grep.md
var outputGrepDescription []byte

const (
	OutputHeadToolName = "output_head"
	OutputTailToolName = "output_tail"
	OutputGrepToolName = "output_grep"
)

// OutputHeadParams are the parameters for the output_head tool.
type OutputHeadParams struct {
	ToolCallID string `json:"tool_call_id" description:"The tool call ID whose output to retrieve (from a previous bash, grep, or other tool result)"`
	Lines      int    `json:"lines,omitempty" description:"Number of lines to return from the beginning (default: 100, max: 500)"`
	Offset     int    `json:"offset,omitempty" description:"Line offset to start from (default: 0). Use for pagination."`
}

// OutputTailParams are the parameters for the output_tail tool.
type OutputTailParams struct {
	ToolCallID string `json:"tool_call_id" description:"The tool call ID whose output to retrieve (from a previous bash, grep, or other tool result)"`
	Lines      int    `json:"lines,omitempty" description:"Number of lines to return from the end (default: 100, max: 500)"`
	Offset     int    `json:"offset,omitempty" description:"Line offset from the end (default: 0). Use for pagination to see earlier lines."`
}

// OutputGrepParams are the parameters for the output_grep tool.
type OutputGrepParams struct {
	ToolCallID string `json:"tool_call_id" description:"The tool call ID whose output to search"`
	Pattern    string `json:"pattern" description:"Regex pattern to search for in the cached output"`
}

// OutputResponseMetadata contains metadata about the output retrieval.
type OutputResponseMetadata struct {
	ToolCallID string `json:"tool_call_id"`
	TotalLines int    `json:"total_lines"`
	HasMore    bool   `json:"has_more"`
	LinesShown int    `json:"lines_shown"`
}

// NewOutputHeadTool creates the output_head tool for viewing the beginning of cached output.
func NewOutputHeadTool() fantasy.AgentTool {
	return fantasy.NewAgentTool(
		OutputHeadToolName,
		string(outputHeadDescription),
		func(ctx context.Context, params OutputHeadParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if params.ToolCallID == "" {
				return fantasy.NewTextErrorResponse("tool_call_id is required"), nil
			}

			sessionID := GetSessionFromContext(ctx)
			if sessionID == "" {
				return fantasy.NewTextErrorResponse("session context not available"), nil
			}

			lines := params.Lines
			if lines <= 0 {
				lines = DefaultOutputLines
			}
			if lines > MaxOutputLines {
				lines = MaxOutputLines
			}

			cache := GetOutputCache()
			result, totalLines, hasMore, ok := cache.Head(sessionID, params.ToolCallID, lines, params.Offset)
			if !ok {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("No cached output found for tool call ID: %s. Output cache expires after %v.", params.ToolCallID, OutputCacheRetention)), nil
			}

			linesShown := len(strings.Split(result, "\n"))
			if result == "" {
				linesShown = 0
			}

			var output strings.Builder
			fmt.Fprintf(&output, "Lines %d-%d of %d total", params.Offset+1, params.Offset+linesShown, totalLines)
			if hasMore {
				fmt.Fprintf(&output, " (use offset=%d to see more)", params.Offset+linesShown)
			}
			output.WriteString("\n\n")
			output.WriteString(result)

			return fantasy.WithResponseMetadata(
				fantasy.NewTextResponse(output.String()),
				OutputResponseMetadata{
					ToolCallID: params.ToolCallID,
					TotalLines: totalLines,
					HasMore:    hasMore,
					LinesShown: linesShown,
				},
			), nil
		})
}

// NewOutputTailTool creates the output_tail tool for viewing the end of cached output.
func NewOutputTailTool() fantasy.AgentTool {
	return fantasy.NewAgentTool(
		OutputTailToolName,
		string(outputTailDescription),
		func(ctx context.Context, params OutputTailParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if params.ToolCallID == "" {
				return fantasy.NewTextErrorResponse("tool_call_id is required"), nil
			}

			sessionID := GetSessionFromContext(ctx)
			if sessionID == "" {
				return fantasy.NewTextErrorResponse("session context not available"), nil
			}

			lines := params.Lines
			if lines <= 0 {
				lines = DefaultOutputLines
			}
			if lines > MaxOutputLines {
				lines = MaxOutputLines
			}

			cache := GetOutputCache()
			result, totalLines, hasMore, ok := cache.Tail(sessionID, params.ToolCallID, lines, params.Offset)
			if !ok {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("No cached output found for tool call ID: %s. Output cache expires after %v.", params.ToolCallID, OutputCacheRetention)), nil
			}

			linesShown := len(strings.Split(result, "\n"))
			if result == "" {
				linesShown = 0
			}

			// Calculate the actual line range shown
			endLine := totalLines - params.Offset
			startLine := endLine - linesShown + 1
			if startLine < 1 {
				startLine = 1
			}

			var output strings.Builder
			fmt.Fprintf(&output, "Lines %d-%d of %d total", startLine, endLine, totalLines)
			if hasMore {
				fmt.Fprintf(&output, " (use offset=%d to see earlier)", params.Offset+linesShown)
			}
			output.WriteString("\n\n")
			output.WriteString(result)

			return fantasy.WithResponseMetadata(
				fantasy.NewTextResponse(output.String()),
				OutputResponseMetadata{
					ToolCallID: params.ToolCallID,
					TotalLines: totalLines,
					HasMore:    hasMore,
					LinesShown: linesShown,
				},
			), nil
		})
}

// NewOutputGrepTool creates the output_grep tool for searching cached output.
func NewOutputGrepTool() fantasy.AgentTool {
	return fantasy.NewAgentTool(
		OutputGrepToolName,
		string(outputGrepDescription),
		func(ctx context.Context, params OutputGrepParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if params.ToolCallID == "" {
				return fantasy.NewTextErrorResponse("tool_call_id is required"), nil
			}
			if params.Pattern == "" {
				return fantasy.NewTextErrorResponse("pattern is required"), nil
			}

			sessionID := GetSessionFromContext(ctx)
			if sessionID == "" {
				return fantasy.NewTextErrorResponse("session context not available"), nil
			}

			cache := GetOutputCache()
			matches, totalLines, ok, err := cache.Grep(sessionID, params.ToolCallID, params.Pattern, 0)
			if err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("Invalid regex pattern: %v", err)), nil
			}
			if !ok {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("No cached output found for tool call ID: %s. Output cache expires after %v.", params.ToolCallID, OutputCacheRetention)), nil
			}

			var output strings.Builder
			if len(matches) == 0 {
				fmt.Fprintf(&output, "No matches found for pattern '%s' in %d lines", params.Pattern, totalLines)
			} else {
				fmt.Fprintf(&output, "Found %d matches in %d total lines\n\n", len(matches), totalLines)
				for _, match := range matches {
					fmt.Fprintf(&output, "Line %d: %s\n", match.LineNum, match.Line)
				}
				if len(matches) >= 100 {
					output.WriteString("\n(Results truncated to 100 matches)")
				}
			}

			return fantasy.WithResponseMetadata(
				fantasy.NewTextResponse(output.String()),
				OutputResponseMetadata{
					ToolCallID: params.ToolCallID,
					TotalLines: totalLines,
					LinesShown: len(matches),
				},
			), nil
		})
}
