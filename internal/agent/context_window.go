package agent

import (
	"context"
	"errors"
	"fmt"
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
			if !ok || tr.IsError {
				continue
			}
			contentRunes := []rune(tr.Content)
			if len(contentRunes) <= contextWindowToolResultMaxChars {
				continue
			}
			omitted := len(contentRunes) - contextWindowToolResultMaxChars
			tr.Content = fmt.Sprintf(
				"%s\n\n[%d characters omitted — output exceeded the context window limit. "+
					"Use a more targeted query (e.g. add WHERE/LIMIT clauses, avoid large columns) to retrieve less data.]",
				string(contentRunes[:contextWindowToolResultMaxChars]),
				omitted,
			)
			msg.Parts[i] = tr
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
