package toolcoach

import (
	"os"
	"regexp"
	"strings"
	"time"
)

// repeatedViewThreshold is the minimum time between views of the same file
// before the repeated_view pattern fires. Overridable in tests.
var repeatedViewThreshold = 30 * time.Second

// Severity levels for coaching tips.
const (
	SeverityHint     = "hint"
	SeverityWarning  = "warning"
	SeverityCritical = "critical"
)

// pattern is a single anti-pattern detector.
type pattern struct {
	ID        string
	Name      string
	AppliesTo []string // tool names; empty = all tools
	Detect    func(state *sessionState, toolName, input string) bool
	Suggest   func(state *sessionState, toolName, input string) string
	// Validate checks whether a subsequent tool call represents the agent
	// acting on this pattern's tip. If nil, the default expected-tool check
	// is used (see coach_metrics.go).
	Validate func(state *sessionState, toolName, input string, tip pendingTip) bool
	// FixInput optionally returns an improved tool input for guided retry.
	// If nil, no auto-retry is attempted for this pattern.
	FixInput func(state *sessionState, toolName, input string) string
	Severity string
}

// Pre-compiled regexes used by pattern detectors.
var (
	rmRfRegex     = regexp.MustCompile(`\brm\s+-rf\s+\b`)
	concreteRegex = regexp.MustCompile(`[a-zA-Z0-9_]{3,}`)
)

// defaultPatterns is the built-in set of anti-pattern detectors.
// They are ordered by priority: earlier patterns are checked first.
var defaultPatterns = []pattern{
	destructiveBashPattern,
	writeOverExistingPattern,
	editWithoutViewPattern,
	repeatedViewPattern,
	broadGrepPattern,
	missingMultieditPattern,
}

// toolsWithPatterns is the set of tool names that have at least one pattern.
// This is used for a fast-path short-circuit before any JSON parsing.
var toolsWithPatterns = map[string]struct{}{
	"bash":      {},
	"edit":      {},
	"view":      {},
	"write":     {},
	"grep":      {},
	"multiedit": {},
}

// hasPatterns reports whether the given tool name has any registered patterns.
func hasPatterns(toolName string) bool {
	_, ok := toolsWithPatterns[toolName]
	return ok
}

// patternByID returns the built-in pattern with the given ID, or nil.
func patternByID(id string) *pattern {
	for i := range defaultPatterns {
		if defaultPatterns[i].ID == id {
			return &defaultPatterns[i]
		}
	}
	return nil
}

// replaceJSONField replaces the string value of a single field in flat JSON.
// It is best-effort and assumes the field appears exactly once.
func replaceJSONField(input, key, newValue string) string {
	// Find `"key"` in the input.
	prefix := `"` + key + `"`
	idx := strings.Index(input, prefix)
	if idx < 0 {
		return ""
	}
	// Move past the key and any whitespace/colon.
	rest := input[idx+len(prefix):]
	start := 0
	for start < len(rest) && (rest[start] == ' ' || rest[start] == ':' || rest[start] == '\t') {
		start++
	}
	if start >= len(rest) || rest[start] != '"' {
		return ""
	}
	// Find the closing quote, handling escaped quotes.
	quoteStart := start + 1
	end := quoteStart
	for end < len(rest) {
		if rest[end] == '\\' && end+1 < len(rest) {
			end += 2
			continue
		}
		if rest[end] == '"' {
			break
		}
		end++
	}
	if end >= len(rest) {
		return ""
	}
	// Build the replacement.
	var sb strings.Builder
	sb.WriteString(input[:idx+len(prefix)+start+1])
	sb.WriteString(newValue)
	sb.WriteString(rest[end:])
	return sb.String()
}

// severityAllowedByIntensity reports whether a pattern with the given severity
// should fire under the current coaching intensity.
func severityAllowedByIntensity(severity string, intensity CoachingIntensity) bool {
	switch intensity {
	case CoachingTutor:
		return true
	case CoachingBalanced:
		return severity != SeverityHint
	case CoachingMinimal:
		return severity == SeverityCritical
	default:
		return true
	}
}

// destructiveBashPattern detects dangerous bash commands.
var destructiveBashPattern = pattern{
	ID:        "destructive_bash",
	Name:      "Destructive Bash Command",
	AppliesTo: []string{"bash"},
	Detect: func(_ *sessionState, _ string, input string) bool {
		cmd, ok := jsonpeek(input, "command")
		if !ok {
			return false
		}
		cmd = strings.ToLower(strings.TrimSpace(cmd))
		if cmd == "" {
			return false
		}
		dangerous := []string{
			"rm -rf /", "rm -rf ~", ":(){ :|:& };:", "mkfs.",
			"dd if=/dev/zero of=/dev/sd",
		}
		for _, d := range dangerous {
			if strings.Contains(cmd, d) {
				return true
			}
		}
		if rmRfRegex.MatchString(cmd) {
			after := strings.TrimSpace(strings.TrimPrefix(cmd, "rm -rf"))
			if after == "." || after == "" || after == "/" || after == "~" {
				return true
			}
		}
		return false
	},
	Suggest: func(_ *sessionState, _ string, input string) string {
		cmd, _ := jsonpeek(input, "command")
		return "Command '" + cmd + "' looks destructive. Consider using the edit or write tool for safer file changes."
	},
	Severity: SeverityCritical,
}

// writeOverExistingPattern warns when writing over an existing file.
var writeOverExistingPattern = pattern{
	ID:        "write_over_existing",
	Name:      "Write Over Existing File",
	AppliesTo: []string{"write"},
	Detect: func(_ *sessionState, _ string, input string) bool {
		filePath, ok := jsonpeek(input, "file_path")
		if !ok || filePath == "" {
			return false
		}
		_, err := os.Stat(filePath)
		return err == nil
	},
	Suggest: func(_ *sessionState, _ string, input string) string {
		filePath, _ := jsonpeek(input, "file_path")
		return "File '" + filePath + "' already exists. Consider using the edit tool to preserve existing content."
	},
	Severity: SeverityWarning,
}

// editWithoutViewPattern detects edits on files that were not previously viewed
// and whose content is not in the session cache.
var editWithoutViewPattern = pattern{
	ID:        "edit_without_view",
	Name:      "Edit Without View",
	AppliesTo: []string{"edit"},
	Detect: func(state *sessionState, _ string, input string) bool {
		filePath, ok := jsonpeek(input, "file_path")
		if !ok || filePath == "" {
			return false
		}
		// If we have cached content for this file, the agent already knows it.
		if state.hasCachedContent(filePath) {
			oldStr, _ := jsonpeek(input, "old_string")
			if oldStr != "" && state.cachedContentContains(filePath, oldStr) {
				return false
			}
		}
		return !state.hasViewed(filePath)
	},
	Suggest: func(_ *sessionState, _ string, input string) string {
		filePath, _ := jsonpeek(input, "file_path")
		return "Consider viewing '" + filePath + "' first to ensure the find string matches exactly."
	},
	Severity: SeverityHint,
}

// repeatedViewPattern detects viewing the same file multiple times without editing.
var repeatedViewPattern = pattern{
	ID:        "repeated_view",
	Name:      "Repeated View",
	AppliesTo: []string{"view"},
	Detect: func(state *sessionState, _ string, input string) bool {
		filePath, ok := jsonpeek(input, "file_path")
		if !ok || filePath == "" {
			return false
		}
		// viewCount includes the current call, so > 1 means a repeat.
		if state.viewCount(filePath) <= 1 {
			return false
		}
		if state.hasEdited(filePath) {
			return false
		}
		// Only fire if the last view was more than the threshold ago.
		lastView := state.lastViewTime(filePath)
		return time.Since(lastView) > repeatedViewThreshold
	},
	Suggest: func(_ *sessionState, _ string, input string) string {
		filePath, _ := jsonpeek(input, "file_path")
		return "You already viewed '" + filePath + "'. Consider using edit instead of re-reading."
	},
	Severity: SeverityHint,
}

// broadGrepPattern detects overly broad grep patterns.
var broadGrepPattern = pattern{
	ID:        "broad_grep",
	Name:      "Broad Grep Pattern",
	AppliesTo: []string{"grep"},
	Detect: func(_ *sessionState, _ string, input string) bool {
		pat, ok := jsonpeek(input, "pattern")
		if !ok {
			return false
		}
		pat = strings.TrimSpace(pat)
		if pat == "" {
			return false
		}
		if len(pat) < 3 {
			return true
		}
		if pat == ".*" || pat == ".+" || pat == "." || pat == ".*?" {
			return true
		}
		return !concreteRegex.MatchString(pat)
	},
	Suggest: func(_ *sessionState, _ string, input string) string {
		pat, _ := jsonpeek(input, "pattern")
		return "Pattern '" + pat + "' looks broad. Try a more specific search to reduce noise."
	},
	Validate: func(_ *sessionState, toolName, input string, tip pendingTip) bool {
		if toolName != "grep" {
			return false
		}
		pat, ok := jsonpeek(input, "pattern")
		if !ok {
			return false
		}
		// Success if the new pattern is longer and contains concrete characters.
		pat = strings.TrimSpace(pat)
		if len(pat) <= 2 {
			return false
		}
		return concreteRegex.MatchString(pat)
	},
	FixInput: func(_ *sessionState, _ string, input string) string {
		pat, ok := jsonpeek(input, "pattern")
		if !ok {
			return ""
		}
		pat = strings.TrimSpace(pat)
		var better string
		switch pat {
		case ".*", ".+", ".":
			better = `\b\w+\b`
		default:
			if len(pat) < 3 {
				better = `\b` + pat + `\b`
			} else {
				return ""
			}
		}
		return replaceJSONField(input, "pattern", better)
	},
	Severity: SeverityHint,
}

// missingMultieditPattern suggests multiedit after multiple sequential edits on
// the same file.
var missingMultieditPattern = pattern{
	ID:        "missing_multiedit",
	Name:      "Missing Multiedit",
	AppliesTo: []string{"edit"},
	Detect: func(state *sessionState, _ string, input string) bool {
		filePath, ok := jsonpeek(input, "file_path")
		if !ok || filePath == "" {
			return false
		}
		return state.consecutiveEdits(filePath) >= 2
	},
	Suggest: func(_ *sessionState, _ string, input string) string {
		filePath, _ := jsonpeek(input, "file_path")
		return "You made multiple edits to '" + filePath + "'. Consider using multiedit to batch changes in one call."
	},
	Severity: SeverityHint,
}
