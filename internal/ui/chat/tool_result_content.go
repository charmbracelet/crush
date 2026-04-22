package chat

import (
	"encoding/json"
	"strings"

	"github.com/charmbracelet/crush/internal/diffdetect"
	"github.com/charmbracelet/crush/internal/stringext"
	"github.com/charmbracelet/crush/internal/ui/styles"
)

type toolResultContentWidths struct {
	Body int
	Diff int
}

func humanizedToolName(name string) string {
	name = strings.ReplaceAll(name, "_", " ")
	name = strings.ReplaceAll(name, "-", " ")
	return stringext.Capitalize(name)
}

func looksLikeMarkdown(content string) bool {
	patterns := []string{
		"# ",
		"## ",
		"**",
		"```",
		"- ",
		"1. ",
		"> ",
		"---",
		"***",
	}
	for _, p := range patterns {
		if strings.Contains(content, p) {
			return true
		}
	}
	return false
}

func renderToolResultTextContent(sty *styles.Styles, content string, widths toolResultContentWidths, expanded bool) string {
	var result json.RawMessage
	if err := json.Unmarshal([]byte(content), &result); err == nil {
		prettyResult, err := json.MarshalIndent(result, "", "  ")
		if err == nil {
			return sty.Tool.Body.Render(toolOutputCodeContent(sty, "result.json", string(prettyResult), 0, widths.Body, expanded))
		}
		return sty.Tool.Body.Render(toolOutputPlainContent(sty, content, widths.Body, expanded))
	}
	if diffdetect.IsUnifiedDiff(content) {
		return toolOutputDiffContentFromUnified(sty, content, widths.Diff, expanded)
	}
	if looksLikeMarkdown(content) {
		return sty.Tool.Body.Render(toolOutputCodeContent(sty, "result.md", content, 0, widths.Body, expanded))
	}
	return sty.Tool.Body.Render(toolOutputPlainContent(sty, content, widths.Body, expanded))
}
