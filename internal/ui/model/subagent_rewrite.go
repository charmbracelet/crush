package model

import "strings"

// rewriteSubagentPrompt detects the pattern `@name rest` at the start of
// content and rewrites it to a delegation instruction when name is a known
// active subagent. Returns content unchanged if the pattern doesn't match.
func rewriteSubagentPrompt(content string, activeNames map[string]bool) string {
	if !strings.HasPrefix(content, "@") {
		return content
	}
	name, prompt, ok := strings.Cut(content[1:], " ")
	if !ok {
		return content
	}
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return content
	}
	if !activeNames[name] {
		return content
	}
	return `Use the agent tool with subagent_type="` + name + `" to handle this request: ` + prompt
}
