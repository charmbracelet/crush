package chat

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/crush/internal/diffdetect"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/stringext"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
)

// MCPToolMessageItem is a message item that represents a bash tool call.
type MCPToolMessageItem struct {
	*baseToolMessageItem
}

var _ ToolMessageItem = (*MCPToolMessageItem)(nil)

// NewMCPToolMessageItem creates a new [MCPToolMessageItem].
func NewMCPToolMessageItem(
	sty *styles.Styles,
	toolCall message.ToolCall,
	result *message.ToolResult,
	canceled bool,
) ToolMessageItem {
	return newBaseToolMessageItem(sty, toolCall, result, &MCPToolRenderContext{}, canceled)
}

// MCPToolRenderContext renders bash tool messages.
type MCPToolRenderContext struct{}

// RenderTool implements the [ToolRenderer] interface.
func (b *MCPToolRenderContext) RenderTool(sty *styles.Styles, width int, opts *ToolRenderOpts) string {
	cappedWidth := cappedMessageWidth(width)
	toolNameParts := strings.SplitN(opts.ToolCall.Name, "_", 3)
	if len(toolNameParts) != 3 {
		return toolErrorContent(sty, &message.ToolResult{Content: "Invalid tool name"}, cappedWidth)
	}
	mcpName := prettyName(toolNameParts[1])
	toolName := prettyName(toolNameParts[2])

	mcpName = sty.Tool.MCPName.Render(mcpName)
	toolName = sty.Tool.MCPToolName.Render(toolName)

	name := fmt.Sprintf("%s %s %s", mcpName, sty.Tool.MCPArrow.String(), toolName)

	if opts.IsPending() {
		return pendingTool(sty, name, opts.Anim, opts.Compact)
	}

	var params map[string]any
	if err := json.Unmarshal([]byte(opts.ToolCall.Input), &params); err != nil {
		return toolErrorContent(sty, &message.ToolResult{Content: "Invalid parameters"}, cappedWidth)
	}

	var toolParams []string
	if len(params) > 0 {
		parsed, _ := json.Marshal(params)
		toolParams = append(toolParams, string(parsed))
	}

	header := toolHeader(sty, opts.Status, name, cappedWidth, opts.Compact, toolParams...)
	if opts.Compact {
		return header
	}

	if earlyState, ok := toolEarlyStateContent(sty, opts, cappedWidth); ok {
		return joinToolParts(header, earlyState)
	}

	if !opts.HasResult() || opts.Result.Content == "" {
		return header
	}

	bodyWidth := cappedWidth - toolBodyLeftPaddingTotal
	// see if the result is json
	var result json.RawMessage
	var body string
	if err := json.Unmarshal([]byte(opts.Result.Content), &result); err == nil {
		prettyResult, err := json.MarshalIndent(result, "", "  ")
		if err == nil {
			body = sty.Tool.Body.Render(toolOutputCodeContent(sty, "result.json", string(prettyResult), 0, bodyWidth, opts.ExpandedContent))
		} else {
			body = sty.Tool.Body.Render(toolOutputPlainContent(sty, opts.Result.Content, bodyWidth, opts.ExpandedContent))
		}
	} else if looksLikeDiff(opts.Result.Content) {
		body = toolOutputDiffContentFromUnified(sty, opts.Result.Content, cappedWidth, opts.ExpandedContent)
	} else if looksLikeMarkdown(opts.Result.Content) {
		body = sty.Tool.Body.Render(toolOutputCodeContent(sty, "result.md", opts.Result.Content, 0, bodyWidth, opts.ExpandedContent))
	} else {
		body = sty.Tool.Body.Render(toolOutputPlainContent(sty, opts.Result.Content, bodyWidth, opts.ExpandedContent))
	}
	return joinToolParts(header, body)
}

func prettyName(name string) string {
	name = strings.ReplaceAll(name, "_", " ")
	name = strings.ReplaceAll(name, "-", " ")
	return stringext.Capitalize(name)
}

// looksLikeMarkdown checks if content appears to be markdown by looking for
// common markdown patterns.
func looksLikeMarkdown(content string) bool {
	patterns := []string{
		"# ",  // headers
		"## ", // headers
		"**",  // bold
		"```", // code fence
		"- ",  // unordered list
		"1. ", // ordered list
		"> ",  // blockquote
		"---", // horizontal rule
		"***", // horizontal rule
	}
	for _, p := range patterns {
		if strings.Contains(content, p) {
			return true
		}
	}
	return false
}

// looksLikeDiff reports whether content appears to be a unified diff.
func looksLikeDiff(content string) bool {
	return diffdetect.IsUnifiedDiff(content)
}

// parsedDiffFile holds the before and after content extracted from one file in
// a unified diff.
type parsedDiffFile struct {
	path   string
	before string
	after  string
}

// parseUnifiedDiff extracts before and after file contents from a unified diff
// string. It returns one entry per file in the diff.
func parseUnifiedDiff(content string) []parsedDiffFile {
	type fileBuilder struct {
		path   string
		before strings.Builder
		after  strings.Builder
	}
	var files []fileBuilder
	currentIdx := -1
	inHunk := false

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "diff --git ") {
			inHunk = false
			parts := strings.SplitN(line, " ", 4)
			if len(parts) >= 4 {
				f := fileBuilder{path: strings.TrimPrefix(parts[3], "b/")}
				files = append(files, f)
				currentIdx = len(files) - 1
			}
			continue
		}

		if strings.HasPrefix(line, "--- ") {
			if currentIdx < 0 {
				// Non-git unified patch: start a new file entry.
				// Path will be refined by the +++ line if needed.
				p := strings.TrimPrefix(line, "--- ")
				p = strings.TrimPrefix(p, "a/")
				if idx := strings.Index(p, "\t"); idx >= 0 {
					p = p[:idx]
				}
				files = append(files, fileBuilder{path: p})
				currentIdx = len(files) - 1
				continue
			}
			if !inHunk {
				// File header: update the path on the current entry.
				p := strings.TrimPrefix(line, "--- ")
				p = strings.TrimPrefix(p, "a/")
				if idx := strings.Index(p, "\t"); idx >= 0 {
					p = p[:idx]
				}
				if p != "/dev/null" {
					files[currentIdx].path = p
				}
			}
			continue
		}

		if strings.HasPrefix(line, "+++ ") {
			if currentIdx < 0 {
				// Non-git new-file patch: seed entry from +++ when ---
				// was /dev/null or missing.
				p := strings.TrimPrefix(line, "+++ ")
				p = strings.TrimPrefix(p, "b/")
				if idx := strings.Index(p, "\t"); idx >= 0 {
					p = p[:idx]
				}
				if p != "/dev/null" {
					files = append(files, fileBuilder{path: p})
					currentIdx = len(files) - 1
				}
				continue
			}
			if !inHunk {
				p := strings.TrimPrefix(line, "+++ ")
				p = strings.TrimPrefix(p, "b/")
				if idx := strings.Index(p, "\t"); idx >= 0 {
					p = p[:idx]
				}
				if p != "/dev/null" && (files[currentIdx].path == "" || strings.HasPrefix(files[currentIdx].path, "/dev/null")) {
					files[currentIdx].path = p
				}
			}
			continue
		}

		if strings.HasPrefix(line, "@@") {
			inHunk = true
			continue
		}

		if strings.HasPrefix(line, "diff ") || strings.HasPrefix(line, "index ") || strings.HasPrefix(line, "new file") || strings.HasPrefix(line, "deleted file") {
			continue
		}

		if currentIdx < 0 {
			continue
		}

		if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "--- ") {
			inHunk = true
			files[currentIdx].before.WriteString(line[1:])
			files[currentIdx].before.WriteByte('\n')
			continue
		}

		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++ ") {
			inHunk = true
			files[currentIdx].after.WriteString(line[1:])
			files[currentIdx].after.WriteByte('\n')
			continue
		}

		if strings.HasPrefix(line, " ") {
			inHunk = true
			lineContent := line[1:]
			files[currentIdx].before.WriteString(lineContent)
			files[currentIdx].before.WriteByte('\n')
			files[currentIdx].after.WriteString(lineContent)
			files[currentIdx].after.WriteByte('\n')
		}
	}

	var result []parsedDiffFile
	for _, f := range files {
		result = append(result, parsedDiffFile{
			path:   f.path,
			before: strings.TrimSuffix(f.before.String(), "\n"),
			after:  strings.TrimSuffix(f.after.String(), "\n"),
		})
	}
	return result
}

// toolOutputDiffContentFromUnified renders a raw unified diff string using the
// diff viewer by parsing it into before/after content. Each file in the diff
// gets its own diff block; if parsing yields no files, falls back to a
// syntax-highlighted code block. Truncation is applied globally across all
// file blocks, not per-file, so multi-file diffs show a single "click to
// expand" indicator.
func toolOutputDiffContentFromUnified(sty *styles.Styles, content string, width int, expanded bool) string {
	files := parseUnifiedDiff(content)
	if len(files) == 0 {
		bodyWidth := width - toolBodyLeftPaddingTotal
		return sty.Tool.Body.Render(toolOutputCodeContent(sty, "result.diff", content, 0, bodyWidth, expanded))
	}
	bodyWidth := width - toolBodyLeftPaddingTotal
	var blocks []string
	for i, f := range files {
		formatter := common.DiffFormatter(sty).
			Before(f.path, f.before).
			After(f.path, f.after).
			Width(bodyWidth)
		if len(files) > 1 {
			formatter = formatter.FileName(f.path)
		}
		if width > maxTextWidth {
			formatter = formatter.Split()
		}
		formatted := formatter.String()
		if i < len(files)-1 {
			formatted += "\n"
		}
		blocks = append(blocks, formatted)
	}
	combined := strings.Join(blocks, "\n")
	lines := strings.Split(combined, "\n")
	maxLines := responseContextHeight
	if expanded {
		maxLines = len(lines)
	}
	if len(lines) > maxLines && !expanded {
		truncMsg := sty.Tool.DiffTruncation.
			Width(bodyWidth).
			Render(fmt.Sprintf(assistantMessageTruncateFormat, len(lines)-maxLines))
		combined = strings.Join(lines[:maxLines], "\n") + "\n" + truncMsg
	}
	return sty.Tool.Body.Render(combined)
}
