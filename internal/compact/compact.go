// Package compact extracts a short, deterministic brief from a Crush session
// so the summary model polishes facts instead of rereading the history.
package compact

import (
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/crush/internal/message"
)

// maxBytes bounds the brief: about 3K tokens fits a ~1.5K token previous
// summary plus fresh facts while staying cheap for the summary call.
const maxBytes = 12 << 10

// maxPreviousBytes bounds the verbatim previous summary.
const maxPreviousBytes = maxBytes / 2

type sectionID int

const (
	goalSection sectionID = iota
	latestSection
	modifiedSection
	failedSection
	readSection
	searchSection
	transcriptSection
	numSections
)

// sections lists every section in render order with its item cap. When the
// brief is over maxBytes, the oldest items of the section with the lowest
// positive drop rank are removed first; rank 0 is never dropped.
var sections = [numSections]struct {
	title string
	cap   int
	drop  int
}{
	goalSection:       {"Session goal", 1, 0},
	latestSection:     {"Latest request", 1, 0},
	modifiedSection:   {"Files modified", 25, 6},
	failedSection:     {"Failed tool calls", 10, 5},
	readSection:       {"Files read", 15, 2},
	searchSection:     {"Searches with no results", 5, 1},
	transcriptSection: {"Recent transcript", 8, 3},
}

// brief holds every section's items, oldest first.
type brief [numSections][]string

// Compact returns a brief of the history (oldest first, possibly starting at
// an earlier summary) for the summary model, or "" when there is neither a
// user prompt nor a previous summary to work from.
func Compact(msgs []message.Message) string {
	b := extract(msgs)
	prev := clip(previousSummary(msgs), maxPreviousBytes)
	if prev == "" && len(b[goalSection]) == 0 {
		return ""
	}

	var shown [numSections]int
	for i, items := range b {
		shown[i] = min(len(items), sections[i].cap)
	}
	out := render(prev, &b, shown)
	for len(out) > maxBytes {
		i := nextDrop(shown)
		if i < 0 {
			break
		}
		shown[i]--
		out = render(prev, &b, shown)
	}
	return clip(out, maxBytes)
}

// nextDrop picks the section to lose an item, or -1 when none can.
func nextDrop(shown [numSections]int) int {
	best := -1
	for i, s := range sections {
		if s.drop == 0 || shown[i] == 0 {
			continue
		}
		if best < 0 || s.drop < sections[best].drop {
			best = i
		}
	}
	return best
}

func render(prev string, b *brief, shown [numSections]int) string {
	var sb strings.Builder
	if prev != "" {
		sb.WriteString("[Previous summary]\n")
		sb.WriteString(prev)
		sb.WriteString("\n")
	}
	for i, items := range b {
		if shown[i] == 0 {
			continue
		}
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString("[" + sections[i].title + "]\n")
		if more := len(items) - shown[i]; more > 0 {
			sb.WriteString("- (+" + strconv.Itoa(more) + " more)\n")
		}
		for _, item := range items[len(items)-shown[i]:] {
			sb.WriteString("- ")
			sb.WriteString(strings.ReplaceAll(item, "\n", "\n  "))
			sb.WriteString("\n")
		}
	}
	return strings.TrimSpace(sb.String())
}

// clip trims s and shortens it to at most n bytes without splitting a rune,
// marking a cut with an ellipsis.
func clip(s string, n int) string {
	const ellipsis = "…"
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	cut := max(n-len(ellipsis), 0)
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return strings.TrimSpace(s[:cut]) + ellipsis
}
