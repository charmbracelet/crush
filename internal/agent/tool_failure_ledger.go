package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

type toolFailure struct {
	ToolName  string
	Input     string
	Output    string
	Count     int
	UpdatedAt time.Time
}

type toolFailureLedger struct {
	mu       sync.Mutex
	failures map[string]map[string]toolFailure
	grounded map[string]bool
}

func newToolFailureLedger() *toolFailureLedger {
	return &toolFailureLedger{
		failures: make(map[string]map[string]toolFailure),
		grounded: make(map[string]bool),
	}
}

func (l *toolFailureLedger) markGrounded(sessionID, toolName string) {
	if l == nil || sessionID == "" || !isGroundingTool(toolName) {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.grounded[sessionID] = true
}

func (l *toolFailureLedger) hasGrounding(sessionID string) bool {
	if l == nil || sessionID == "" {
		return false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.grounded[sessionID]
}

func (l *toolFailureLedger) recordFailure(sessionID, toolName, input, output string) {
	if l == nil || sessionID == "" {
		return
	}
	key := toolFailureKey(toolName, input)
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.failures[sessionID] == nil {
		l.failures[sessionID] = make(map[string]toolFailure)
	}
	f := l.failures[sessionID][key]
	f.ToolName = toolName
	f.Input = canonicalToolInput(input)
	f.Output = trimToolOutput(output)
	f.Count++
	f.UpdatedAt = time.Now()
	l.failures[sessionID][key] = f
}

func (l *toolFailureLedger) clearFailure(sessionID, toolName, input string) {
	if l == nil || sessionID == "" {
		return
	}
	key := toolFailureKey(toolName, input)
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.failures[sessionID], key)
	if len(l.failures[sessionID]) == 0 {
		delete(l.failures, sessionID)
	}
}

func (l *toolFailureLedger) previousFailure(sessionID, toolName, input string) (toolFailure, bool) {
	if l == nil || sessionID == "" {
		return toolFailure{}, false
	}
	key := toolFailureKey(toolName, input)
	l.mu.Lock()
	defer l.mu.Unlock()
	f, ok := l.failures[sessionID][key]
	return f, ok
}

func toolFailureKey(toolName, input string) string {
	return toolName + "\x00" + canonicalToolInput(input)
}

func canonicalToolInput(input string) string {
	input = strings.TrimSpace(input)
	if input == "" {
		return "{}"
	}
	var buf bytes.Buffer
	if err := json.Compact(&buf, []byte(input)); err == nil {
		return buf.String()
	}
	return input
}

func trimToolOutput(output string) string {
	output = strings.TrimSpace(output)
	if len(output) <= 1200 {
		return output
	}
	return output[:1200] + "\n... output truncated ..."
}

func repeatFailureMessage(f toolFailure) string {
	return fmt.Sprintf(
		"Repeated failed tool call blocked. The exact %q call with the same input already failed %d time(s).\n\nPrevious failure output:\n%s\n\nDo not retry the same input. Inspect the tool schema/help/current state, change arguments, or switch tools before trying again.",
		f.ToolName,
		f.Count,
		f.Output,
	)
}

func isGroundingTool(toolName string) bool {
	switch toolName {
	case "bash", "view", "grep", "glob", "ls", "recode_info":
		return true
	}
	return strings.HasPrefix(toolName, "mcp_filesystem_") ||
		strings.HasPrefix(toolName, "mcp_github_") ||
		strings.HasPrefix(toolName, "mcp_context7_") ||
		strings.HasPrefix(toolName, "mcp_exa_")
}
