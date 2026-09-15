package tools

import "encoding/json"

// WriteToolNames mutate files; a successful result supersedes earlier
// reads of the same path. The set is also the verifyingTool wrap set and
// the gate's metadata-scan set — lsp_rename/lsp_replace_symbol mutate
// via workspace edits, the canonical caller-breaker.
var WriteToolNames = map[string]bool{
	"edit": true, "write": true, "multiedit": true,
	"lsp_rename": true, "lsp_replace_symbol": true,
}

// ReadToolNames capture file content; their results go stale on writes.
var ReadToolNames = map[string]bool{"view": true, "read": true}

// CommandToolNames emit re-derivable output: once a result is old
// enough to leave the recency guard, or a re-run makes it redundant,
// a labeled stub suffices.
var CommandToolNames = map[string]bool{"bash": true, "grep": true, "glob": true, "ls": true}

// ToolCallFilePath extracts the file path from a tool call's JSON
// input, trying the conventional keys.
func ToolCallFilePath(input string) string {
	var fields map[string]any
	if err := json.Unmarshal([]byte(input), &fields); err != nil {
		return ""
	}
	for _, key := range []string{"file_path", "path", "file"} {
		if s, ok := fields[key].(string); ok && s != "" {
			return s
		}
	}
	return ""
}
