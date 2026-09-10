package notebook

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/google/uuid"
)

// significantReadThreshold is the minimum output size (in bytes) for a
// file read to be considered significant. Reads below this threshold
// are grouped into a trivial exploration mini-entry.
const significantReadThreshold = 1000

// isSignificant returns true if a tool call warrants its own notebook
// entry.
func isSignificant(toolCall message.ToolCall, toolResult *message.ToolResult) bool {
	switch toolCall.Name {
	case "view", "read":
		if toolResult != nil {
			return len(toolResult.Content) > significantReadThreshold
		}
		return false
	case "edit", "write", "multiedit":
		return true
	case "bash":
		return true
	case "grep", "glob", "ls":
		return false
	default:
		return true
	}
}

// classifyEvents partitions the tool calls in a turn's messages into
// significant events (each gets its own entry) and trivial events
// (grouped into one exploration mini-entry).
func classifyEvents(msgs []message.Message) (significant []EntryInput, trivial []EntryInput) {
	for _, msg := range msgs {
		switch msg.Role {
		case message.Assistant:
			for _, tc := range msg.ToolCalls() {
				if !tc.Finished {
					continue
				}
				result := findToolResult(msgs, tc.ID)
				input := EntryInput{
					ToolCall:   &tc,
					ToolResult: result,
				}
				input.EventType = eventTypeForTool(tc.Name)
				input.Title = titleForTool(tc)
				input.Description = describeToolCall(tc, result)
				if isSignificant(tc, result) {
					significant = append(significant, input)
				} else {
					trivial = append(trivial, input)
				}
			}
		}
	}
	return significant, trivial
}

// findToolResult searches the tool messages for a result matching the
// given tool call ID.
func findToolResult(msgs []message.Message, toolCallID string) *message.ToolResult {
	for _, msg := range msgs {
		if msg.Role != message.Tool {
			continue
		}
		for _, tr := range msg.ToolResults() {
			if tr.ToolCallID == toolCallID {
				result := tr // Copy to avoid pointer to range variable.
				return &result
			}
		}
	}
	return nil
}

// eventTypeForTool maps a tool name to a notebook event type.
func eventTypeForTool(name string) string {
	switch name {
	case "view", "read":
		return EventFileRead
	case "edit", "write", "multiedit":
		return EventFileEdit
	case "bash":
		return EventCommand
	case "grep", "glob", "ls":
		return EventExploration
	default:
		return EventGeneral
	}
}

// titleForTool generates a short title for a tool call.
func titleForTool(tc message.ToolCall) string {
	switch tc.Name {
	case "view", "read":
		path := extractPathFromInput(tc.Input)
		if path != "" {
			return "Read " + path
		}
		return "Read file"
	case "edit":
		path := extractPathFromInput(tc.Input)
		if path != "" {
			return "Edit " + path
		}
		return "Edit file"
	case "write":
		path := extractPathFromInput(tc.Input)
		if path != "" {
			return "Write " + path
		}
		return "Write file"
	case "multiedit":
		path := extractPathFromInput(tc.Input)
		if path != "" {
			return "Multi-edit " + path
		}
		return "Multi-edit file"
	case "bash":
		cmd := extractCommandFromInput(tc.Input)
		if cmd != "" {
			return "Run: " + truncate(cmd, 60)
		}
		return "Run command"
	case "grep":
		return "Grep search"
	case "glob":
		return "Glob search"
	case "ls":
		return "List directory"
	default:
		return tc.Name
	}
}

// describeToolCall produces a human-readable description of a tool call
// and its result for the generator.
func describeToolCall(tc message.ToolCall, result *message.ToolResult) string {
	var sb strings.Builder
	sb.WriteString("Tool: ")
	sb.WriteString(tc.Name)
	sb.WriteString("\nInput: ")
	sb.WriteString(truncate(tc.Input, 2000))
	if result != nil {
		sb.WriteString("\nResult: ")
		if result.IsError {
			sb.WriteString("[ERROR] ")
		}
		sb.WriteString(truncate(result.Content, 2000))
	}
	return sb.String()
}

// extractPathFromInput attempts to extract a file path from a tool
// call's JSON input.
func extractPathFromInput(input string) string {
	// Simple JSON field extraction for common path keys.
	for _, key := range []string{`"file_path"`, `"path"`, `"file"`} {
		if val := extractJSONString(input, key); val != "" {
			return val
		}
	}
	return ""
}

// extractCommandFromInput attempts to extract a command from a bash
// tool call's JSON input.
func extractCommandFromInput(input string) string {
	return extractJSONString(input, `"command"`)
}

// extractJSONString extracts a string value for a JSON key from a
// JSON input string. This is a lightweight parser that avoids full JSON
// unmarshalling for simple flat objects.
func extractJSONString(input, key string) string {
	idx := strings.Index(input, key)
	if idx == -1 {
		return ""
	}
	// Move past the key and the colon.
	rest := input[idx+len(key):]
	rest = strings.TrimLeft(rest, " \t:")
	// Find the opening quote.
	if len(rest) == 0 || rest[0] != '"' {
		return ""
	}
	rest = rest[1:]
	// Find the closing quote (handle escaped quotes).
	var sb strings.Builder
	for i := 0; i < len(rest); i++ {
		if rest[i] == '\\' && i+1 < len(rest) {
			sb.WriteByte(rest[i+1])
			i++
			continue
		}
		if rest[i] == '"' {
			break
		}
		sb.WriteByte(rest[i])
	}
	return sb.String()
}

// truncate clips a string to maxLen characters, appending an ellipsis.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-1] + "…"
}

// hasDecision checks if the assistant's text response contains a
// decision (heuristic: contains keywords like "decided", "chose",
// "will defer", "let's go with").
func hasDecision(msgs []message.Message) bool {
	keywords := []string{"decided", "chose", "will defer", "let's go with", "going with", "opted for"}
	for _, msg := range msgs {
		if msg.Role != message.Assistant {
			continue
		}
		text := strings.ToLower(msg.Content().Text)
		for _, kw := range keywords {
			if strings.Contains(text, kw) {
				return true
			}
		}
	}
	return false
}

// buildTrivialExplorationEntry creates a mini-entry for grouped trivial
// tool calls (grep, glob, ls).
func buildTrivialExplorationEntry(trivial []EntryInput) GeneratedEntry {
	var sb strings.Builder
	sb.WriteString("## Trivial exploration\n")
	for _, t := range trivial {
		if t.ToolCall != nil {
			sb.WriteString("- ")
			sb.WriteString(t.ToolCall.Name)
			if t.ToolResult != nil && t.ToolResult.Content != "" {
				sb.WriteString(" → ")
				sb.WriteString(truncate(t.ToolResult.Content, 100))
			}
			sb.WriteString("\n")
		}
	}
	sb.WriteString("#phase:exploration")
	return GeneratedEntry{
		EventType: EventExploration,
		Title:     "Trivial exploration",
		Text:      sb.String(),
		Tags:      []string{"phase:exploration"},
	}
}

// storeEntry persists a generated entry to the database.
func (s *service) storeEntry(ctx context.Context, sessionID string, turnNumber, eventNumber int64, entry GeneratedEntry) error {
	tokenCount := estimateTokens(entry.Text)
	id := uuid.New().String()
	now := time.Now().Unix()

	_, err := s.q.CreateNotebookEntry(ctx, db.CreateNotebookEntryParams{
		ID:               id,
		SessionID:        sessionID,
		TurnNumber:       turnNumber,
		EventNumber:      eventNumber,
		EventType:        entry.EventType,
		Title:            entry.Title,
		EntryText:        entry.Text,
		EntryTextFull:    sql.NullString{String: entry.Text, Valid: true},
		TokenCount:       tokenCount,
		CompressionLevel: CompressionFull,
		CreatedAt:        now,
	})
	if err != nil {
		return fmt.Errorf("failed to create notebook entry: %w", err)
	}

	for _, tag := range entry.Tags {
		if err := s.q.CreateNotebookTag(ctx, db.CreateNotebookTagParams{
			EntryID: id,
			Tag:     tag,
		}); err != nil {
			slog.Error("Failed to create notebook tag", "error", err, "tag", tag)
		}
	}
	return nil
}

// estimateTokens provides a rough token estimate (~4 chars per token).
func estimateTokens(text string) int64 {
	return int64(len(text) / 4)
}

// GenerateEntries implements the Service interface.
func (s *service) GenerateEntries(ctx context.Context, sessionID string, turnNumber int64, msgs []message.Message) error {
	significant, trivial := classifyEvents(msgs)

	// Check for a decision in the assistant response. Decisions are
	// significant events in their own right, alongside file edits/reads.
	// A decision entry is added even when there are other significant
	// events, because the decision context is distinct from the tool
	// events.
	if hasDecision(msgs) {
		significant = append(significant, EntryInput{
			EventType:   EventDecision,
			Title:       "Decision",
			Description: extractAssistantText(msgs),
		})
	}

	// If there are no significant events, no trivial events, and no
	// decision, this is a trivial turn — skip entirely.
	if len(significant) == 0 && len(trivial) == 0 {
		return nil
	}

	// Store trivial exploration mini-entry.
	if len(trivial) > 0 {
		entry := buildTrivialExplorationEntry(trivial)
		if err := s.storeEntry(ctx, sessionID, turnNumber, 0, entry); err != nil {
			slog.Error("Failed to store trivial exploration entry", "error", err)
		}
	}

	// Generate significant entries via the small model (batched).
	if len(significant) == 0 {
		return nil
	}

	entries, err := s.generator.Generate(ctx, sessionID, significant)
	if err != nil {
		return fmt.Errorf("failed to generate notebook entries: %w", err)
	}

	for i, entry := range entries {
		// Enforce max entry token cap via truncation.
		if estimateTokens(entry.Text) > s.opts.MaxEntryTokens {
			entry.Text = truncateEntry(entry.Text, s.opts.MaxEntryTokens)
		}
		if err := s.storeEntry(ctx, sessionID, turnNumber, int64(i+1), entry); err != nil {
			slog.Error("Failed to store notebook entry", "error", err)
		}
	}

	// Compact if needed.
	if err := s.Compact(ctx, sessionID); err != nil {
		slog.Error("Failed to compact notebook", "error", err)
	}

	return nil
}

// truncateEntry clips an entry to the max token budget, preserving
// tags at the bottom.
func truncateEntry(text string, maxTokens int64) string {
	maxChars := int(maxTokens * 4)
	if len(text) <= maxChars {
		return text
	}
	// Try to preserve the last few lines (tags section).
	lines := strings.Split(text, "\n")
	var tagLines []string
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "#") {
			tagLines = append([]string{lines[i]}, tagLines...)
			continue
		}
		break
	}
	body := strings.Join(lines[:len(lines)-len(tagLines)], "\n")
	body = body[:maxChars-len(strings.Join(tagLines, "\n"))-50]
	return body + "\n[Entry truncated. Use recall tool for full details.]\n" + strings.Join(tagLines, "\n")
}

// extractAssistantText returns the concatenated text of all assistant
// messages in the turn.
func extractAssistantText(msgs []message.Message) string {
	var sb strings.Builder
	for _, msg := range msgs {
		if msg.Role == message.Assistant {
			sb.WriteString(msg.Content().Text)
			sb.WriteString("\n")
		}
	}
	return sb.String()
}
