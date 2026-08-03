package model

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/ui/textarea"
)

// promptHighlighter implements textarea.LineHighlighter for the prompt:
// it marks @file tokens and /skill tokens as they are typed. Tokens are
// recognized at word boundaries (start of line or after whitespace) and run
// to the next whitespace — they never span lines, which keeps the
// textarea's per-line range contract trivially satisfiable.
//
// A @token is always highlighted (the file may not exist yet, and the
// completion popup is the discovery path). A /token is only highlighted
// when it is a prefix of a known skill name, so arbitrary slash-words in
// prose don't get styled as skills.
type promptHighlighter struct {
	fileStyle  lipgloss.Style
	skillStyle lipgloss.Style

	// skillNames is the set of known skill names used to validate /tokens.
	skillNames []string

	// lines caches the tokenized logical lines from the last Rescan.
	lines [][]lipgloss.Range
}

var _ textarea.LineHighlighter = (*promptHighlighter)(nil)

func newPromptHighlighter(fileStyle, skillStyle lipgloss.Style, skillNames []string) *promptHighlighter {
	return &promptHighlighter{
		fileStyle:  fileStyle,
		skillStyle: skillStyle,
		skillNames: skillNames,
	}
}

// setSkillNames updates the known skill set (e.g. after a skills reload).
func (h *promptHighlighter) setSkillNames(names []string) {
	h.skillNames = names
}

// Rescan retokenizes the full prompt value. Called whenever the textarea
// value changes; cheap (linear scan) and keeps Highlight a pure lookup.
func (h *promptHighlighter) Rescan(value string) {
	rawLines := strings.Split(value, "\n")
	h.lines = make([][]lipgloss.Range, len(rawLines))
	for i, line := range rawLines {
		h.lines[i] = h.scanLine([]rune(line))
	}
}

// Highlight implements textarea.LineHighlighter.
func (h *promptHighlighter) Highlight(lineIdx int, _ []rune) []lipgloss.Range {
	if lineIdx < 0 || lineIdx >= len(h.lines) {
		return nil
	}
	return h.lines[lineIdx]
}

// scanLine finds all tokens in a single logical line.
func (h *promptHighlighter) scanLine(line []rune) []lipgloss.Range {
	var ranges []lipgloss.Range
	i := 0
	for i < len(line) {
		atWordStart := i == 0 || isSpaceRune(line[i-1])
		if atWordStart && (line[i] == '@' || line[i] == '/') && i+1 < len(line) && !isSpaceRune(line[i+1]) {
			end := i + 1
			for end < len(line) && !isSpaceRune(line[end]) {
				end++
			}
			if style, ok := h.tokenStyle(line[i], line[i+1:end]); ok {
				ranges = append(ranges, lipgloss.NewRange(i, end, style))
			}
			i = end
			continue
		}
		i++
	}
	return ranges
}

// tokenStyle decides whether a candidate token gets styled, and with which
// style. trigger is '@' or '/'; word is the token body without the trigger.
func (h *promptHighlighter) tokenStyle(trigger rune, word []rune) (lipgloss.Style, bool) {
	if trigger == '@' {
		return h.fileStyle, true
	}
	if len(word) == 0 {
		return lipgloss.Style{}, false
	}
	name := string(word)
	for _, s := range h.skillNames {
		if strings.HasPrefix(s, name) {
			return h.skillStyle, true
		}
	}
	return lipgloss.Style{}, false
}

func isSpaceRune(r rune) bool {
	return r == ' ' || r == '\t'
}

// skillNames returns the names of the skills a /token may refer to. It
// shares skillCompletionValues' source of truth — the effective catalog,
// falling back to discovery states before the first catalog load — so a
// token highlights exactly when the popup would have offered it.
func (m *UI) skillNames() []string {
	values := m.skillCompletionValues()
	names := make([]string, 0, len(values))
	for _, v := range values {
		names = append(names, v.Name)
	}
	return names
}

// refreshSkillNames re-primes the prompt highlighter's known-skill set.
// Called from every path that can change the effective skill list.
func (m *UI) refreshSkillNames() {
	if m.promptHighlighter == nil {
		return
	}
	m.promptHighlighter.setSkillNames(m.skillNames())
}
