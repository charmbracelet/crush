package model

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/session"
)

// summaryExportPath returns the file path where the given session's
// summary is saved. The path is stable per session so repeated saves
// overwrite the previous summary instead of accumulating files.
func summaryExportPath(dataDirectory, sessionID string) string {
	return filepath.Join(dataDirectory, "summaries", sessionID+".md")
}

// saveSummaryExport writes the session's latest summary message to a
// markdown file inside dataDirectory and returns the written path.
func saveSummaryExport(dataDirectory string, sess session.Session, msgs []message.Message) (string, error) {
	if sess.SummaryMessageID == "" {
		return "", fmt.Errorf("no summary available yet, run \"Summarize Session\" first")
	}
	var summary *message.Message
	for i := range msgs {
		if msgs[i].ID == sess.SummaryMessageID {
			summary = &msgs[i]
			break
		}
	}
	if summary == nil {
		return "", fmt.Errorf("summary message not found for this session")
	}
	content := strings.TrimSpace(summary.Content().Text)
	if content == "" {
		return "", fmt.Errorf("summary message is empty")
	}
	path := summaryExportPath(dataDirectory, sess.ID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("failed to create summaries directory: %w", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("failed to write summary: %w", err)
	}
	return path, nil
}
