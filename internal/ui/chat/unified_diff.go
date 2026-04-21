package chat

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/crush/internal/diffdetect"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
)

type parsedDiffFile struct {
	path   string
	before string
	after  string
}

func looksLikeDiff(content string) bool {
	return diffdetect.IsUnifiedDiff(content)
}

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

	for i, line := range lines {
		if strings.HasPrefix(line, "diff --git ") {
			inHunk = false
			parts := strings.SplitN(line, " ", 4)
			if len(parts) >= 4 {
				files = append(files, fileBuilder{path: strings.TrimPrefix(parts[3], "b/")})
				currentIdx = len(files) - 1
			}
			continue
		}

		if strings.HasPrefix(line, "@@") {
			inHunk = true
			continue
		}

		if strings.HasPrefix(line, "index ") || strings.HasPrefix(line, "new file") || strings.HasPrefix(line, "deleted file") {
			inHunk = false
			continue
		}

		nextIsPlusHeader := i+1 < len(lines) && strings.HasPrefix(lines[i+1], "+++ ")
		if strings.HasPrefix(line, "--- ") && (!inHunk || nextIsPlusHeader) {
			startedNewFileFromHunk := inHunk && nextIsPlusHeader
			inHunk = false
			p := strings.TrimPrefix(line, "--- ")
			p = strings.TrimPrefix(p, "a/")
			if idx := strings.Index(p, "\t"); idx >= 0 {
				p = p[:idx]
			}
			if currentIdx < 0 || startedNewFileFromHunk {
				files = append(files, fileBuilder{path: p})
				currentIdx = len(files) - 1
				continue
			}
			if p != "/dev/null" {
				files[currentIdx].path = p
			}
			continue
		}

		if strings.HasPrefix(line, "+++ ") && !inHunk {
			p := strings.TrimPrefix(line, "+++ ")
			p = strings.TrimPrefix(p, "b/")
			if idx := strings.Index(p, "\t"); idx >= 0 {
				p = p[:idx]
			}
			if currentIdx < 0 {
				if p != "/dev/null" {
					files = append(files, fileBuilder{path: p})
					currentIdx = len(files) - 1
				}
				continue
			}
			if p != "/dev/null" && (files[currentIdx].path == "" || strings.HasPrefix(files[currentIdx].path, "/dev/null")) {
				files[currentIdx].path = p
			}
			continue
		}

		if currentIdx < 0 {
			continue
		}

		if strings.HasPrefix(line, "-") {
			inHunk = true
			files[currentIdx].before.WriteString(line[1:])
			files[currentIdx].before.WriteByte('\n')
			continue
		}

		if strings.HasPrefix(line, "+") {
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

	result := make([]parsedDiffFile, 0, len(files))
	for _, f := range files {
		result = append(result, parsedDiffFile{
			path:   f.path,
			before: strings.TrimSuffix(f.before.String(), "\n"),
			after:  strings.TrimSuffix(f.after.String(), "\n"),
		})
	}
	return result
}

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
