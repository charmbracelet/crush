package agent

import (
	"errors"
	"regexp"
	"strings"

	"charm.land/fantasy"
)

var errToolProtocol = errors.New("model returned tool-call markup as text after one native tool-call retry")

func attemptedTextToolNames(result *fantasy.AgentResult, availableTools []fantasy.AgentTool) []string {
	if result == nil || len(result.Steps) == 0 {
		return nil
	}
	last := result.Steps[len(result.Steps)-1]
	if last.FinishReason != fantasy.FinishReasonStop || len(last.Content.ToolCalls()) != 0 {
		return nil
	}

	text := strings.ToLower(strings.TrimSpace(last.Content.Text()))
	if text == "" || !containsToolCallMarkup(text) {
		return nil
	}
	var names []string
	for _, tool := range availableTools {
		name := strings.TrimSpace(tool.Info().Name)
		if name != "" && mentionsToolCall(text, name) {
			names = append(names, name)
		}
	}
	return names
}

func mentionsToolCall(text, name string) bool {
	pattern := `(?:^|[^a-z0-9_-])` + regexp.QuoteMeta(strings.ToLower(name)) + `\s*(?:\(|\{|$)`
	return regexp.MustCompile(pattern).MatchString(text)
}

func containsToolCallMarkup(text string) bool {
	for _, marker := range []string{
		"<tool_code",
		"<tool_call",
		"<function=",
		"```tool",
	} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}
