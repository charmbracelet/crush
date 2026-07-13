package agent

import (
	"encoding/json"
	"strings"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/tools"
)

func needsMCPCompletionEvidence(call SessionAgentCall, steps []fantasy.StepResult, finalText string) bool {
	if call.mcpCompletionCheck || !isMCPSetupIntent(originalIntent(call)) || !claimsMCPSetupSuccess(finalText) {
		return false
	}
	if !attemptedMCPSetup(steps) {
		return false
	}
	return !hasSuccessfulNamedMCPConnection(steps, originalIntent(call), finalText)
}

func isMCPSetupIntent(intent string) bool {
	intent = strings.ToLower(intent)
	for clause := range strings.SplitSeq(splitMCPClauses(intent), "\n") {
		if !strings.Contains(clause, "mcp") || containsAny(clause,
			"do not", "don't", "must not", "never", "no need to",
			"should not", "shouldn't", "without adding", "without installing",
		) {
			continue
		}
		if containsAny(clause, "add", "install", "configure", "enable", "connect", "set up", "setup") {
			return true
		}
	}
	return false
}

func claimsMCPSetupSuccess(text string) bool {
	text = strings.ToLower(text)
	clauses := splitMCPClauses(text)
	for clause := range strings.SplitSeq(clauses, "\n") {
		if !strings.Contains(clause, "mcp") || containsAny(clause,
			"could not", "did not", "didn't", "does not", "doesn't",
			"failed", "failure", "not connected", "not configured",
			"not installed", "not ready", "unable", "error",
		) {
			continue
		}
		if containsAny(clause, "added", "configured", "connected", "installed", "appears", "ready", "successful", "successfully") {
			return true
		}
	}
	return false
}

func splitMCPClauses(text string) string {
	return strings.NewReplacer(
		"\r", "\n",
		".", "\n",
		"!", "\n",
		"?", "\n",
		";", "\n",
		" but ", "\n",
		" however ", "\n",
		" yet ", "\n",
		" and do not ", "\ndo not ",
		" and don't ", "\ndon't ",
	).Replace(text)
}

func containsAny(text string, values ...string) bool {
	for _, value := range values {
		if strings.Contains(text, value) {
			return true
		}
	}
	return false
}

func attemptedMCPSetup(steps []fantasy.StepResult) bool {
	for _, step := range steps {
		for _, call := range step.Content.ToolCalls() {
			if call.ToolName == tools.MCPAddToolName || call.ToolName == tools.MCPRefreshToolName || configMutationTargetsCrush(call) {
				return true
			}
		}
	}
	return false
}

func configMutationTargetsCrush(call fantasy.ToolCallContent) bool {
	if call.ToolName != tools.WriteToolName && call.ToolName != tools.EditToolName && call.ToolName != tools.MultiEditToolName {
		return false
	}
	var input struct {
		FilePath string `json:"file_path"`
	}
	if json.Unmarshal([]byte(call.Input), &input) != nil {
		return false
	}
	path := strings.ToLower(strings.ReplaceAll(input.FilePath, "\\", "/"))
	return strings.HasSuffix(path, "/crush.json") || strings.HasSuffix(path, "/crush.project.json") || path == "crush.json" || path == "crush.project.json"
}

func hasSuccessfulNamedMCPConnection(steps []fantasy.StepResult, intent, finalText string) bool {
	claimedTarget := strings.ToLower(intent + "\n" + finalText)
	for _, step := range steps {
		names := make(map[string]string)
		for _, call := range step.Content.ToolCalls() {
			switch call.ToolName {
			case tools.MCPAddToolName:
				var params tools.MCPAddParams
				if json.Unmarshal([]byte(call.Input), &params) == nil && strings.TrimSpace(params.Name) != "" {
					names[call.ToolCallID] = strings.TrimSpace(params.Name)
				}
			case tools.MCPRefreshToolName:
				var params tools.MCPRefreshParams
				if json.Unmarshal([]byte(call.Input), &params) == nil && strings.TrimSpace(params.Name) != "" {
					names[call.ToolCallID] = strings.TrimSpace(params.Name)
				}
			}
		}
		for _, result := range step.Content.ToolResults() {
			name, ok := names[result.ToolCallID]
			if !ok || !mentionsExactMCPServerName(claimedTarget, name) {
				continue
			}
			if _, isError := fantasy.AsToolResultOutputType[fantasy.ToolResultOutputContentError](result.Result); isError {
				continue
			}
			output := strings.ToLower(toolResultOutputString(result.Result))
			if strings.Contains(output, strings.ToLower(name)+": connected") {
				return true
			}
		}
	}
	return false
}

func mentionsExactMCPServerName(text, name string) bool {
	text = strings.ToLower(text)
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return false
	}
	for offset := 0; offset < len(text); {
		index := strings.Index(text[offset:], name)
		if index < 0 {
			return false
		}
		start := offset + index
		end := start + len(name)
		if (start == 0 || !isMCPNameCharacter(text[start-1])) &&
			(end == len(text) || !isMCPNameCharacter(text[end])) {
			return true
		}
		offset = end
	}
	return false
}

func isMCPNameCharacter(value byte) bool {
	return value >= 'a' && value <= 'z' || value >= '0' && value <= '9' || value == '_' || value == '-'
}
