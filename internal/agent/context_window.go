package agent

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/message"
)

const (
	// contextWindowToolResultMaxChars is the maximum number of characters
	// kept in a single tool-result Content field when recovering from a
	// context-window-exceeded error.  Results larger than this are truncated
	// in-place so that the subsequent summarization call does not also hit
	// the limit.
	contextWindowToolResultMaxChars = 20_000
	contextWindowTruncationDir      = ".crush/truncation"

	// contextWindowResumePromptPrefix is prepended to the original user
	// prompt when re-queuing the task after a forced summarization.  It
	// tells the LLM why the session was interrupted and asks it to reduce
	// the volume of data it requests from tools.
	contextWindowResumePromptPrefix = "The previous session was interrupted because a tool returned too much data, which pushed the conversation history over this model's context window limit. " +
		"To avoid this again, please reduce the scope of your tool calls — for example: add WHERE clauses and LIMIT/TOP constraints to SQL queries, avoid selecting large geometry or blob columns, " +
		"and prefer targeted lookups over broad scans. " +
		"The initial user request was: `"
)

// isContextWindowExceededError reports whether err is a provider error caused
// by the input exceeding the model's context window.
func isContextWindowExceededError(err error) bool {
	var providerErr *fantasy.ProviderError
	if !errors.As(err, &providerErr) || providerErr == nil {
		return false
	}
	if providerErr.StatusCode != 400 {
		return false
	}
	msg := strings.ToLower(providerErr.Message)
	return strings.Contains(msg, "context window") ||
		strings.Contains(msg, "context length") ||
		strings.Contains(msg, "maximum context") ||
		strings.Contains(msg, "input exceeds") ||
		strings.Contains(msg, "input length should be") ||
		strings.Contains(msg, "range of input length should be") ||
		strings.Contains(msg, "too many tokens") ||
		strings.Contains(msg, "prompt is too long")
}

func (a *sessionAgent) truncateToolResult(sessionID string, tr message.ToolResult) (message.ToolResult, bool) {
	if tr.IsError || tr.Data != "" || tr.MIMEType != "" {
		return tr, false
	}
	contentRunes := []rune(tr.Content)
	if len(contentRunes) <= contextWindowToolResultMaxChars {
		return tr, false
	}

	fullOutputPath := ""
	if a != nil {
		persistedPath, err := a.persistToolResultContent(sessionID, tr)
		if err != nil {
			slog.Warn("Failed to persist oversized tool result", "error", err, "session_id", sessionID, "tool_name", tr.Name)
		} else {
			fullOutputPath = persistedPath
		}
	}

	keep := contextWindowToolResultMaxChars
	for range 3 {
		notice := truncatedToolResultNotice(len(contentRunes)-keep, fullOutputPath)
		keep = contextWindowToolResultMaxChars - len([]rune(notice))
		if keep < 0 {
			keep = 0
		}
		if keep > len(contentRunes) {
			keep = len(contentRunes)
		}
	}

	tr.Content = string(contentRunes[:keep]) + truncatedToolResultNotice(len(contentRunes)-keep, fullOutputPath)
	return tr, true
}

func truncatedToolResultNotice(omitted int, fullOutputPath string) string {
	if fullOutputPath != "" {
		return fmt.Sprintf("\n\n[%d characters omitted — output exceeded the context window limit. This excerpt is incomplete; do not assume it contains the full result. The full output was saved to `%s`. Use the view tool to inspect that file with offset/limit.]", omitted, filepath.ToSlash(fullOutputPath))
	}
	return fmt.Sprintf("\n\n[%d characters omitted — output exceeded the context window limit. This excerpt is incomplete; do not assume it contains the full result. If you need more detail, rerun the tool with a narrower scope or use a precise read with offsets/limits when available.]", omitted)
}

func (a *sessionAgent) persistToolResultContent(sessionID string, tr message.ToolResult) (string, error) {
	if a == nil || a.workingDir == "" || tr.Content == "" {
		return "", nil
	}
	truncationDir := filepath.Join(a.workingDir, contextWindowTruncationDir)
	if err := os.MkdirAll(truncationDir, 0o755); err != nil {
		return "", err
	}
	pattern := fmt.Sprintf("%s-%s-*.txt", sanitizeTruncationPathPart(sessionID), sanitizeTruncationPathPart(tr.Name))
	file, err := os.CreateTemp(truncationDir, pattern)
	if err != nil {
		return "", err
	}
	defer file.Close()
	if _, err = file.WriteString(tr.Content); err != nil {
		return "", err
	}
	path := file.Name()
	if relPath, err := filepath.Rel(a.workingDir, path); err == nil {
		return relPath, nil
	}
	return path, nil
}

func sanitizeTruncationPathPart(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "tool"
	}
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= 'A' && r <= 'Z':
			return r
		case r >= '0' && r <= '9':
			return r
		case r == '-', r == '_':
			return r
		default:
			return '_'
		}
	}, trimmed)
}

// truncateOversizedToolResults scans all tool messages in the session and
// truncates any ToolResult whose Content field exceeds
// contextWindowToolResultMaxChars.  This is called before summarization when
// a context-window-exceeded error occurs, so the summarize request itself does
// not also hit the limit.
//
// The truncated text is replaced with the kept prefix plus a human-readable
// notice explaining how many characters were omitted and why.
func (a *sessionAgent) truncateOversizedToolResults(ctx context.Context, sessionID string) error {
	msgs, err := a.messages.List(ctx, sessionID)
	if err != nil {
		return err
	}
	for _, msg := range msgs {
		if msg.Role != message.Tool {
			continue
		}
		modified := false
		for i, part := range msg.Parts {
			tr, ok := part.(message.ToolResult)
			if !ok {
				continue
			}
			truncated, ok := a.truncateToolResult(sessionID, tr)
			if !ok {
				continue
			}
			msg.Parts[i] = truncated
			modified = true
		}
		if modified {
			if updateErr := a.messages.Update(ctx, msg); updateErr != nil {
				return updateErr
			}
		}
	}
	return nil
}
