package chat

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
)

// highlightPromptTokens restyles the @file and /skill tokens in an
// already-rendered user message so a token keeps the colour it had in the
// prompt editor after the message is posted.
//
// It runs over the rendered output rather than the Markdown source because
// that output is what the reader sees: Glamour has already wrapped, padded
// and coloured the text, and re-deriving offsets from the source would not
// survive any of that. Each line is scanned in its ANSI-stripped form and
// the token spans are converted to display cells, which is exactly what
// lipgloss.StyleRanges wants. A token split across a soft wrap highlights
// on the segment it starts in, which is the same thing the textarea does.
func highlightPromptTokens(content string, sty *styles.Styles) string {
	if content == "" {
		return content
	}

	lines := strings.Split(content, "\n")
	changed := false
	for i, line := range lines {
		plain := []rune(ansi.Strip(line))
		tokens := common.ScanPromptTokens(plain)
		if len(tokens) == 0 {
			continue
		}

		ranges := make([]lipgloss.Range, 0, len(tokens))
		for _, tok := range tokens {
			// Rune offsets are not cell offsets once wide runes or combining
			// marks are in play, and StyleRanges counts cells.
			start := ansi.StringWidth(string(plain[:tok.Start]))
			end := start + ansi.StringWidth(string(plain[tok.Start:tok.End]))
			style := sty.Editor.TokenFile
			if tok.Kind == common.PromptTokenSkill {
				style = sty.Editor.TokenSkill
			}
			ranges = append(ranges, lipgloss.NewRange(start, end, style))
		}
		lines[i] = lipgloss.StyleRanges(line, ranges...)
		changed = true
	}
	if !changed {
		return content
	}
	return strings.Join(lines, "\n")
}
