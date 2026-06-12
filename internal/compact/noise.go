package compact

import (
	"regexp"
	"strings"
)

// Noise tools that carry no semantic value for compaction.
var noiseTools = map[string]bool{
	"todos":       true,
	"crush_info":  true,
	"crush_logs":  true,
	"lsp_restart": true,
}

// XML wrapper patterns injected by the system that should be stripped.
// Go regexp doesn't support backreferences, so we match each tag explicitly.
var xmlWrapperRe = regexp.MustCompile(
	`<(?:system-reminder|ide_opened_file|command-message|context-window-usage)[^>]*>[\s\S]*?</(?:system-reminder|ide_opened_file|command-message|context-window-usage)>`,
)

// Noise strings that indicate boilerplate user messages.
var noiseStrings = []string{
	"Continue from where you left off.",
	"No response requested.",
}

// FilterNoise removes thinking blocks, noise tool calls/results, and
// boilerplate user messages from the block stream.
func FilterNoise(blocks []NormalizedBlock) []NormalizedBlock {
	out := make([]NormalizedBlock, 0, len(blocks))
	for _, b := range blocks {
		switch b.Kind {
		case BlockToolCall:
			if noiseTools[b.Name] {
				continue
			}
		case BlockToolResult:
			if noiseTools[b.Name] {
				continue
			}
		case BlockUser:
			if isNoiseUser(b.Text) {
				continue
			}
			cleaned := cleanUserText(b.Text)
			if cleaned == "" {
				continue
			}
			b.Text = cleaned
		}
		out = append(out, b)
	}
	return out
}

func isNoiseUser(text string) bool {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return true
	}
	for _, s := range noiseStrings {
		if strings.Contains(trimmed, s) {
			return true
		}
	}
	stripped := strings.TrimSpace(xmlWrapperRe.ReplaceAllString(trimmed, ""))
	return stripped == ""
}

func cleanUserText(text string) string {
	return strings.TrimSpace(xmlWrapperRe.ReplaceAllString(text, ""))
}
