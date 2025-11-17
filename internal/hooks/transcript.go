package hooks

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/crush/internal/message"
)

// TranscriptMessage represents a message in the exported transcript.
type TranscriptMessage struct {
	ID          string                 `json:"id"`
	Role        string                 `json:"role"`
	Content     string                 `json:"content,omitempty"`
	ToolCalls   []TranscriptToolCall   `json:"tool_calls,omitempty"`
	ToolResults []TranscriptToolResult `json:"tool_results,omitempty"`
	Timestamp   string                 `json:"timestamp"`
}

// TranscriptToolCall represents a tool call in the transcript.
type TranscriptToolCall struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Input string `json:"input"`
}

// TranscriptToolResult represents a tool result in the transcript.
type TranscriptToolResult struct {
	ToolCallID string `json:"tool_call_id"`
	Name       string `json:"name"`
	Content    string `json:"content"`
	IsError    bool   `json:"is_error"`
}

// Transcript represents the complete transcript structure.
type Transcript struct {
	SessionID string              `json:"session_id"`
	Messages  []TranscriptMessage `json:"messages"`
}

// exportTranscript exports session messages to a temporary JSON file.
func exportTranscript(
	ctx context.Context,
	messages message.Service,
	sessionID string,
) (string, error) {
	// Get all messages for the session
	msgs, err := messages.List(ctx, sessionID)
	if err != nil {
		return "", fmt.Errorf("failed to list messages: %w", err)
	}

	// Convert to transcript format
	transcript := Transcript{
		SessionID: sessionID,
		Messages:  make([]TranscriptMessage, 0, len(msgs)),
	}

	for _, msg := range msgs {
		tm := TranscriptMessage{
			ID:        msg.ID,
			Role:      string(msg.Role),
			Timestamp: time.Unix(msg.CreatedAt, 0).Format("2006-01-02T15:04:05Z07:00"),
		}

		// Extract content
		for _, part := range msg.Parts {
			if text, ok := part.(message.TextContent); ok {
				if tm.Content != "" {
					tm.Content += "\n"
				}
				tm.Content += text.Text
			}
		}

		// Extract tool calls
		if msg.Role == message.Assistant {
			toolCalls := msg.ToolCalls()
			for _, tc := range toolCalls {
				tm.ToolCalls = append(tm.ToolCalls, TranscriptToolCall{
					ID:    tc.ID,
					Name:  tc.Name,
					Input: tc.Input,
				})
			}
		}

		// Extract tool results
		if msg.Role == message.Tool {
			toolResults := msg.ToolResults()
			for _, tr := range toolResults {
				tm.ToolResults = append(tm.ToolResults, TranscriptToolResult{
					ToolCallID: tr.ToolCallID,
					Name:       tr.Name,
					Content:    tr.Content,
					IsError:    tr.IsError,
				})
			}
		}

		transcript.Messages = append(transcript.Messages, tm)
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(transcript, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal transcript: %w", err)
	}

	// Write to temporary file
	tmpDir := os.TempDir()
	filename := fmt.Sprintf("crush-transcript-%s.json", sessionID)
	path := filepath.Join(tmpDir, filename)

	// Use restrictive permissions (0600)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", fmt.Errorf("failed to write transcript file: %w", err)
	}

	return path, nil
}

func cleanupTranscript(path string) {
	if path != "" {
		if err := os.Remove(path); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to cleanup transcript file %s: %v\n", path, err)
		}
	}
}
