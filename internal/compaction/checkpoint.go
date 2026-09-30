package compaction

import (
	"fmt"
	"strings"
)

const (
	// infoStart and infoEnd delimit the deterministic footer appended to a
	// checkpoint. The footer is generated, never model-written, so the
	// numbers in it can be trusted and the model can be pointed at the
	// displaced transcript.
	infoStart = "<compaction_info>"
	infoEnd   = "</compaction_info>"
)

// Info is the machine-written footer that accompanies a checkpoint summary.
type Info struct {
	// ReplacedMessages and ReplacedTokens describe the region the checkpoint
	// stands in for.
	ReplacedMessages int
	ReplacedTokens   int64
	// KeptMessages and KeptTokens describe the transcript that still follows
	// the checkpoint verbatim.
	KeptMessages int
	KeptTokens   int64
	// TranscriptPath, when non-empty, holds the full text of the replaced
	// region outside the context window.
	TranscriptPath string
}

// Render returns the footer, without any surrounding model output.
func (i Info) Render() string {
	var lines []string
	lines = append(lines, fmt.Sprintf(
		"This checkpoint replaces %d earlier messages (~%d tokens).",
		i.ReplacedMessages, i.ReplacedTokens,
	))
	if i.KeptMessages > 0 {
		lines = append(lines, fmt.Sprintf(
			"The %d most recent messages (~%d tokens) were not summarized and follow verbatim.",
			i.KeptMessages, i.KeptTokens,
		))
	}
	if i.TranscriptPath != "" {
		lines = append(lines,
			fmt.Sprintf("Full text of the replaced region: %s", i.TranscriptPath),
			"When you need an exact quote, error message, or file body from before this checkpoint, read that file instead of guessing at it.",
		)
	}
	return infoStart + "\n" + strings.Join(lines, "\n") + "\n" + infoEnd
}

// Body returns summary with any footer removed, so a later compaction can
// merge the prose it wrote without re-summarizing bookkeeping about where the
// transcript went.
func Body(summary string) string {
	start := strings.Index(summary, infoStart)
	if start == -1 {
		return summary
	}
	end := strings.Index(summary[start:], infoEnd)
	if end == -1 {
		return summary
	}
	return strings.TrimSpace(summary[:start] + summary[start+end+len(infoEnd):])
}
