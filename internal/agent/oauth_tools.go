// Package agent provides OAuth tool name remapping for Anthropic.
//
// Anthropic blocks specific lowercase tool names (bash, read, write, edit) when
// using OAuth tokens. This file provides utilities to remap tool names to
// Claude Code-style capitalized versions for OAuth compatibility.
package agent

import (
	"strings"

	"charm.land/fantasy"
)

// claudeCodeToolNames maps lowercase tool names to Claude Code-style capitalized versions.
// Source: https://cchistory.mariozechner.at/data/prompts-2.1.11.md
var claudeCodeToolNames = map[string]string{
	"bash":  "Bash",
	"read":  "Read",
	"write": "Write",
	"edit":  "Edit",
	"grep":  "Grep",
	"glob":  "Glob",
	"view":  "View", // crush uses 'view' instead of 'read'
}

// reverseLookup builds the reverse mapping for tool name resolution
var reverseToolNames = func() map[string]string {
	reverse := make(map[string]string, len(claudeCodeToolNames))
	for k, v := range claudeCodeToolNames {
		reverse[strings.ToLower(v)] = k
	}
	return reverse
}()

// ToClaudeCodeName converts a tool name to Claude Code-style casing if it's a blocked name.
func ToClaudeCodeName(name string) string {
	if ccName, ok := claudeCodeToolNames[strings.ToLower(name)]; ok {
		return ccName
	}
	return name
}

// FromClaudeCodeName converts a Claude Code-style tool name back to the original.
func FromClaudeCodeName(name string) string {
	if original, ok := reverseToolNames[strings.ToLower(name)]; ok {
		return original
	}
	return name
}

// oauthToolWrapper wraps an AgentTool with a renamed name for OAuth compatibility.
type oauthToolWrapper struct {
	fantasy.AgentTool
	renamedName string
}

// Info returns the tool info with the renamed name.
func (w *oauthToolWrapper) Info() fantasy.ToolInfo {
	info := w.AgentTool.Info()
	return fantasy.ToolInfo{
		Name:        w.renamedName,
		Description: info.Description,
		Parameters:  info.Parameters,
	}
}

// Name returns the renamed tool name.
func (w *oauthToolWrapper) Name() string {
	return w.renamedName
}

// WrapToolsForOAuth wraps tools with Claude Code-style names for Anthropic OAuth.
// This is necessary because Anthropic blocks lowercase tool names (bash, read, write, edit)
// when using OAuth tokens.
func WrapToolsForOAuth(tools []fantasy.AgentTool) []fantasy.AgentTool {
	wrapped := make([]fantasy.AgentTool, len(tools))
	for i, tool := range tools {
		originalName := tool.Info().Name
		renamedName := ToClaudeCodeName(originalName)
		if renamedName != originalName {
			wrapped[i] = &oauthToolWrapper{
				AgentTool:   tool,
				renamedName: renamedName,
			}
		} else {
			wrapped[i] = tool
		}
	}
	return wrapped
}
