package compact

import (
	"encoding/json"
	"strings"

	"github.com/charmbracelet/crush/internal/message"
)

// toolUse is a tool call paired with its result by tool call ID.
type toolUse struct {
	name    string
	input   toolInput
	result  string
	isError bool
}

// toolInput holds the tool arguments the extractors read.
type toolInput struct {
	FilePath string `json:"file_path"`
	Command  string `json:"command"`
	Pattern  string `json:"pattern"`
	Path     string `json:"path"`
}

func newToolUse(c message.ToolCall, r message.ToolResult) *toolUse {
	use := &toolUse{name: c.Name, result: r.Content, isError: r.IsError}
	// Invalid or partial input leaves the fields empty.
	_ = json.Unmarshal([]byte(c.Input), &use.input)
	return use
}

// toolResults indexes every tool result by the call it answers.
func toolResults(msgs []message.Message) map[string]message.ToolResult {
	byCall := make(map[string]message.ToolResult)
	for _, m := range msgs {
		for _, r := range m.ToolResults() {
			byCall[r.ToolCallID] = r
		}
	}
	return byCall
}

// previousSummary is the text of the earlier summary the history starts with.
func previousSummary(msgs []message.Message) string {
	for _, m := range msgs {
		if m.IsSummaryMessage {
			return m.Content().Text
		}
	}
	return ""
}

func visibleText(parts []message.ContentPart) string {
	var texts []string
	for _, p := range parts {
		if t, ok := p.(message.TextContent); ok && !t.Hidden {
			if s := strings.TrimSpace(t.Text); s != "" {
				texts = append(texts, s)
			}
		}
	}
	return strings.Join(texts, "\n")
}
