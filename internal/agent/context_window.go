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
	contextWindowToolResultMaxChars       = 20_000
	contextWindowStepToolResultCharsLimit = 40_000
	contextWindowTruncationDir            = ".crush/truncation"

	// contextWindowMessageToolResultCharsLimit is the aggregate character budget
	// for all tool results within a single API-level message. When multiple tool
	// steps run in parallel, the per-step budget alone cannot prevent the total
	// from exceeding the context window. This limit caps the combined size.
	contextWindowMessageToolResultCharsLimit = 200_000

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
		strings.Contains(msg, "prompt is too long") ||
		strings.Contains(msg, "request body too large")
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

func (a *sessionAgent) enforceStepToolResultBudget(sessionID string, tr message.ToolResult, used *int) message.ToolResult {
	if tr.IsError || tr.Data != "" || tr.MIMEType != "" || used == nil {
		return tr
	}
	remaining := contextWindowStepToolResultCharsLimit - *used
	if remaining <= 0 {
		tr.Content = fmt.Sprintf("[Step tool-result budget exhausted. This result was omitted to keep the conversation within the context window. Re-run the tool with a narrower scope if you still need it. %d characters omitted.]", len([]rune(tr.Content)))
		return tr
	}
	contentRunes := []rune(tr.Content)
	if len(contentRunes) <= remaining {
		*used += len(contentRunes)
		return tr
	}
	fullOutputPath := ""
	if a != nil {
		persistedPath, err := a.persistToolResultContent(sessionID, tr)
		if err != nil {
			slog.Warn("Failed to persist step-budget tool result", "error", err, "session_id", sessionID, "tool_name", tr.Name)
		} else {
			fullOutputPath = persistedPath
		}
	}
	keep := remaining
	for range 3 {
		notice := truncatedToolResultNotice(len(contentRunes)-keep, fullOutputPath)
		keep = remaining - len([]rune(notice))
		if keep < 0 {
			keep = 0
		}
		if keep > len(contentRunes) {
			keep = len(contentRunes)
		}
	}
	tr.Content = string(contentRunes[:keep]) + truncatedToolResultNotice(len(contentRunes)-keep, fullOutputPath)
	*used = contextWindowStepToolResultCharsLimit
	return tr
}

// enforceMessageToolResultBudget applies an aggregate character budget across
// all tool results in the pending message. It should be called after individual
// tool results have been collected for the current turn but before they are sent
// to the LLM. Results are processed in order; once the budget is exhausted,
// remaining results are replaced with a notice.
//
// The function works on a slice of tool results and returns a new slice with
// oversized results replaced by truncated versions. It prioritizes keeping
// earlier (typically more important) results intact.
func (a *sessionAgent) enforceMessageToolResultBudget(sessionID string, results []message.ToolResult) []message.ToolResult {
	totalUsed := 0
	out := make([]message.ToolResult, len(results))
	copy(out, results)

	for i, tr := range out {
		if tr.IsError || tr.Data != "" || tr.MIMEType != "" {
			continue
		}

		contentRunes := []rune(tr.Content)
		remaining := contextWindowMessageToolResultCharsLimit - totalUsed

		if remaining <= 0 {
			out[i].Content = fmt.Sprintf(
				"[Message tool-result budget exhausted (%d/%d chars used). "+
					"This result was omitted to keep the conversation within the context window. "+
					"Re-run the tool with a narrower scope if you still need it. %d characters omitted.]",
				totalUsed, contextWindowMessageToolResultCharsLimit, len(contentRunes))
			continue
		}

		if len(contentRunes) <= remaining {
			totalUsed += len(contentRunes)
			continue
		}

		// Need to truncate this result to fit within the message budget.
		fullOutputPath := ""
		if a != nil {
			persistedPath, err := a.persistToolResultContent(sessionID, tr)
			if err != nil {
				slog.Warn("Failed to persist message-budget tool result",
					"error", err, "session_id", sessionID, "tool_name", tr.Name)
			} else {
				fullOutputPath = persistedPath
			}
		}

		keep := remaining
		for range 3 {
			notice := truncatedToolResultNotice(len(contentRunes)-keep, fullOutputPath)
			keep = remaining - len([]rune(notice))
			if keep < 0 {
				keep = 0
			}
			if keep > len(contentRunes) {
				keep = len(contentRunes)
			}
		}

		out[i].Content = string(contentRunes[:keep]) + truncatedToolResultNotice(len(contentRunes)-keep, fullOutputPath)
		totalUsed = contextWindowMessageToolResultCharsLimit
	}

	return out
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

	// Also enforce aggregate message-level budget across tool result groups.
	if err := a.enforceMessageToolResultBudgets(ctx, sessionID); err != nil {
		return fmt.Errorf("enforcing message tool result budget: %w", err)
	}
	return nil
}

// enforceMessageToolResultBudgets scans all tool messages in the session and
// applies the aggregate message-level budget to groups of tool results that
// belong to the same API-level message (consecutive tool messages).
func (a *sessionAgent) enforceMessageToolResultBudgets(ctx context.Context, sessionID string) error {
	msgs, err := a.messages.List(ctx, sessionID)
	if err != nil {
		return err
	}

	// Process consecutive groups of tool messages (these form a single API message).
	i := 0
	for i < len(msgs) {
		if msgs[i].Role != message.Tool {
			i++
			continue
		}

		// Collect consecutive tool messages into a group.
		groupStart := i
		var allResults []message.ToolResult
		var resultLocations []struct{ msgIdx, partIdx int }

		for i < len(msgs) && msgs[i].Role == message.Tool {
			for j, part := range msgs[i].Parts {
				if tr, ok := part.(message.ToolResult); ok && !tr.IsError && tr.Data == "" && tr.MIMEType == "" {
					allResults = append(allResults, tr)
					resultLocations = append(resultLocations, struct{ msgIdx, partIdx int }{i, j})
				}
			}
			i++
		}

		// Check if this group exceeds the message budget.
		totalChars := 0
		for _, tr := range allResults {
			totalChars += len([]rune(tr.Content))
		}
		if totalChars <= contextWindowMessageToolResultCharsLimit {
			continue
		}

		// Apply budget enforcement.
		budgeted := a.enforceMessageToolResultBudget(sessionID, allResults)
		modified := false
		for k, loc := range resultLocations {
			if budgeted[k].Content != allResults[k].Content {
				msgs[loc.msgIdx].Parts[loc.partIdx] = budgeted[k]
				modified = true
			}
		}

		// Persist modified messages.
		if modified {
			for idx := groupStart; idx < i; idx++ {
				if updateErr := a.messages.Update(ctx, msgs[idx]); updateErr != nil {
					return updateErr
				}
			}
		}
	}
	return nil
}
