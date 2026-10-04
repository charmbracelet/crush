package compaction

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const (
	// infoStart and infoEnd delimit the deterministic footer appended to a
	// checkpoint. The footer is generated, never model-written, so the numbers
	// in it can be trusted and the model can be pointed at the displaced
	// transcript. infoStart is a prefix: the opening tag carries the same
	// counts as attributes for anything reading the session back.
	infoStart = "<compaction_info"
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
//
// The counts appear twice on purpose: as attributes, which the interface reads,
// and as sentences, which the model reads. Neither should have to infer from
// the other.
func (i Info) Render() string {
	tag := fmt.Sprintf(
		`<compaction_info replaced_messages="%d" replaced_tokens="%d" kept_messages="%d" kept_tokens="%d"`,
		i.ReplacedMessages, i.ReplacedTokens, i.KeptMessages, i.KeptTokens,
	)
	if i.TranscriptPath != "" {
		tag += fmt.Sprintf(` transcript_path=%q`, i.TranscriptPath)
	}
	lines := []string{fmt.Sprintf(
		"This checkpoint replaces %d earlier messages (~%d tokens).",
		i.ReplacedMessages, i.ReplacedTokens,
	)}
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
	return tag + ">\n" + strings.Join(lines, "\n") + "\n" + infoEnd
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

// ParseInfo recovers the footer's counts from checkpoint text, reporting
// whether a footer was present at all.
//
// Sessions written before the tag carried attributes are read from the same
// sentences a model reads, because a checkpoint card that silently lost their
// counts would look like a bug in the card rather than in the file's vintage.
func ParseInfo(summary string) (Info, bool) {
	start := strings.Index(summary, infoStart)
	if start == -1 {
		return Info{}, false
	}
	rest := summary[start+len(infoStart):]
	lineEnd := strings.IndexByte(rest, '\n')
	if lineEnd == -1 {
		lineEnd = len(rest)
	}
	tag, body := rest[:lineEnd], rest[lineEnd:]
	if info, ok := parseAttrs(tag); ok {
		return info, true
	}
	return parseProse(body), true
}

func parseAttrs(tag string) (Info, bool) {
	var (
		info  Info
		found bool
	)
	for {
		equals := strings.IndexByte(tag, '=')
		if equals < 0 {
			break
		}
		key := strings.TrimSpace(tag[:equals])
		value := strings.TrimLeft(tag[equals+1:], " ")
		if !strings.HasPrefix(value, `"`) {
			break
		}
		value = value[1:]
		closing := strings.IndexByte(value, '"')
		if closing < 0 {
			break
		}
		value, tag = value[:closing], value[closing+1:]

		number, err := strconv.ParseInt(value, 10, 64)
		switch {
		case err != nil && key == "transcript_path":
			info.TranscriptPath, found = value, true
		case err != nil:
		case key == "replaced_messages":
			info.ReplacedMessages, found = int(number), true
		case key == "replaced_tokens":
			info.ReplacedTokens, found = number, true
		case key == "kept_messages":
			info.KeptMessages, found = int(number), true
		case key == "kept_tokens":
			info.KeptTokens, found = number, true
		}
	}
	return info, found
}

var (
	replacedRe = regexp.MustCompile(`replaces (\d+) earlier messages \(~(\d+) tokens\)`)
	keptRe     = regexp.MustCompile(`The (\d+) most recent messages \(~(\d+) tokens\)`)
	regionRe   = regexp.MustCompile(`Full text of the replaced region: (\S+)`)
)

func parseProse(body string) Info {
	var info Info
	if m := replacedRe.FindStringSubmatch(body); m != nil {
		info.ReplacedMessages = atoi(m[1])
		info.ReplacedTokens = atoi64(m[2])
	}
	if m := keptRe.FindStringSubmatch(body); m != nil {
		info.KeptMessages = atoi(m[1])
		info.KeptTokens = atoi64(m[2])
	}
	if m := regionRe.FindStringSubmatch(body); m != nil {
		info.TranscriptPath = m[1]
	}
	return info
}

func atoi(s string) int { return int(atoi64(s)) }

func atoi64(s string) int64 {
	n, _ := strconv.ParseInt(s, 10, 64)
	return n
}
