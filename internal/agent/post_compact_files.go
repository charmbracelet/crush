package agent

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

const (
	postCompactMaxFiles      = 5
	postCompactMaxPerFile    = 5_000
	postCompactMaxTotalChars = 50_000
)

func (a *sessionAgent) buildRecentFileContext(ctx context.Context, sessionID string, contextWindow int64) []string {
	paths, err := a.filetracker.ListReadFiles(ctx, sessionID)
	if err != nil {
		slog.Warn("Failed to list read files for post-compact injection", "error", err)
		return nil
	}

	if len(paths) == 0 {
		return nil
	}

	if len(paths) > postCompactMaxFiles {
		paths = paths[len(paths)-postCompactMaxFiles:]
	}

	var result []string
	totalChars := 0
	for _, absPath := range paths {
		if totalChars >= postCompactMaxTotalChars {
			break
		}

		content, err := os.ReadFile(absPath)
		if err != nil {
			continue
		}

		text := string(content)
		remaining := postCompactMaxTotalChars - totalChars
		maxForFile := postCompactMaxPerFile
		if remaining < maxForFile {
			maxForFile = remaining
		}

		runes := []rune(text)
		if len(runes) > maxForFile {
			text = string(runes[:maxForFile]) + fmt.Sprintf("\n... [truncated, %d chars total]", len(runes))
		}

		relPath := absPath
		if a.workingDir != "" {
			if rel, err := filepath.Rel(a.workingDir, absPath); err == nil {
				relPath = rel
			}
		}

		result = append(result, fmt.Sprintf("Recently read file `%s`:\n```\n%s\n```", filepath.ToSlash(relPath), text))
		totalChars += len(runes)
	}

	if len(result) > 0 {
		slog.Info("Injecting recently-read files into summary context", "count", len(result), "total_chars", totalChars)
	}
	return result
}
