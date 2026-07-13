package agent

import (
	"strings"

	"charm.land/fantasy"
)

func failureClass(output string) string {
	s := strings.ToLower(output)
	switch {
	case strings.Contains(s, "npm error code e404") ||
		(strings.Contains(s, "npm error 404") && strings.Contains(s, "package")):
		return "package-not-found"
	case strings.Contains(s, "404 not found") ||
		strings.Contains(s, "status code: 404") ||
		strings.Contains(s, "status code 404") ||
		strings.Contains(s, "returned 404"):
		return "resource-not-found"
	case strings.Contains(s, "executable file not found") || strings.Contains(s, "is not recognized as an internal or external command"):
		return "executable-not-found"
	case strings.Contains(s, "syntaxerror:") && strings.Contains(s, "json"):
		return "invalid-json"
	case strings.Contains(s, "unsupported mcp type") || strings.Contains(s, "unsupported transport") || strings.Contains(s, "unknown field"):
		return "unsupported-schema"
	case strings.Contains(s, "context deadline exceeded") || strings.Contains(s, "timed out"):
		return "timeout"
	default:
		return ""
	}
}

// toolResultOutputString converts a ToolResultOutputContent to a stable string
// representation for signature comparison.
func toolResultOutputString(result fantasy.ToolResultOutputContent) string {
	if result == nil {
		return ""
	}
	if text, ok := fantasy.AsToolResultOutputType[fantasy.ToolResultOutputContentText](result); ok {
		return text.Text
	}
	if errResult, ok := fantasy.AsToolResultOutputType[fantasy.ToolResultOutputContentError](result); ok {
		if errResult.Error != nil {
			return errResult.Error.Error()
		}
		return ""
	}
	if media, ok := fantasy.AsToolResultOutputType[fantasy.ToolResultOutputContentMedia](result); ok {
		return media.Data
	}
	return ""
}
