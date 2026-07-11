package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"strings"

	"charm.land/fantasy"
)

const (
	loopDetectionWindowSize = 10
	loopDetectionMaxRepeats = 2
)

// hasRepeatedToolCalls checks whether the agent is stuck in a loop by looking
// at recent steps. It examines the last windowSize steps and returns true if
// any tool-call signature appears more than maxRepeats times.
func hasRepeatedToolCalls(steps []fantasy.StepResult, windowSize, maxRepeats int) bool {
	if len(steps) == 0 || windowSize <= 0 || maxRepeats < 1 {
		return false
	}

	start := max(0, len(steps)-windowSize)
	window := steps[start:]
	counts := make(map[string]int)

	for _, step := range window {
		sig := getToolInteractionSignature(step.Content)
		if sig == "" {
			continue
		}
		counts[sig]++
		if counts[sig] > maxRepeats {
			return true
		}
	}

	return false
}

// hasRepeatedFailureClass catches strategy loops whose inputs differ while the
// underlying tool failure remains the same.
func hasRepeatedFailureClass(steps []fantasy.StepResult, windowSize, maxRepeats int) bool {
	if len(steps) == 0 || windowSize <= 0 || maxRepeats < 1 {
		return false
	}
	start := max(0, len(steps)-windowSize)
	counts := make(map[string]int)
	for _, step := range steps[start:] {
		seenThisStep := make(map[string]bool)
		for _, result := range step.Content.ToolResults() {
			class := failureClass(toolResultOutputString(result.Result))
			if class == "" || seenThisStep[class] {
				continue
			}
			seenThisStep[class] = true
			counts[class]++
			if counts[class] >= maxRepeats {
				return true
			}
		}
	}
	return false
}

func failureClass(output string) string {
	s := strings.ToLower(output)
	switch {
	case strings.Contains(s, "npm error code e404") || strings.Contains(s, "404 not found"):
		return "package-not-found"
	case strings.Contains(s, "executable file not found") || strings.Contains(s, "is not recognized as an internal or external command"):
		return "executable-not-found"
	case strings.Contains(s, "syntaxerror:") && strings.Contains(s, "json"):
		return "invalid-json"
	case strings.Contains(s, "context deadline exceeded") || strings.Contains(s, "timed out"):
		return "timeout"
	default:
		return ""
	}
}

// getToolInteractionSignature computes a hash signature for the tool
// interactions in a single step's content. It pairs tool calls with their
// results (matched by ToolCallID) and returns a hex-encoded SHA-256 hash.
// If the step contains no tool calls, it returns "".
func getToolInteractionSignature(content fantasy.ResponseContent) string {
	toolCalls := content.ToolCalls()
	if len(toolCalls) == 0 {
		return ""
	}

	// Index tool results by their ToolCallID for fast lookup.
	resultsByID := make(map[string]fantasy.ToolResultContent)
	for _, tr := range content.ToolResults() {
		resultsByID[tr.ToolCallID] = tr
	}

	h := sha256.New()
	for _, tc := range toolCalls {
		output := ""
		if tr, ok := resultsByID[tc.ToolCallID]; ok {
			output = toolResultOutputString(tr.Result)
		}
		io.WriteString(h, tc.ToolName)
		io.WriteString(h, "\x00")
		io.WriteString(h, tc.Input)
		io.WriteString(h, "\x00")
		io.WriteString(h, output)
		io.WriteString(h, "\x00")
	}
	return hex.EncodeToString(h.Sum(nil))
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
