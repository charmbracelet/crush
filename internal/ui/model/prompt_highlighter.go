package model

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/textarea"
)

// promptHighlighter implements textarea.LineHighlighter for the prompt: it
// marks @file tokens and /skill tokens as they are typed. Tokenization
// itself lives in common.ScanPromptTokens, shared with the posted-message
// renderer so a token looks the same before and after you hit enter; this
// type only maps tokens onto the editor's styles and caches the result.
type promptHighlighter struct {
	fileStyle  lipgloss.Style
	skillStyle lipgloss.Style

	// lines caches the tokenized logical lines from the last Rescan.
	lines [][]lipgloss.Range
}

var _ textarea.LineHighlighter = (*promptHighlighter)(nil)

func newPromptHighlighter(fileStyle, skillStyle lipgloss.Style) *promptHighlighter {
	return &promptHighlighter{
		fileStyle:  fileStyle,
		skillStyle: skillStyle,
	}
}

// SetStyles re-pushes the token styles after a theme change.
//
// The styles are value copies taken at construction, so without this the
// editor keeps drawing @file and /skill tokens in the old theme's colours
// while the posted message — which reads the styles live — draws them in the
// new one, and the same token appears in two colours above and below the
// editor. The cached ranges hold offsets only and stay valid.
func (h *promptHighlighter) SetStyles(fileStyle, skillStyle lipgloss.Style) {
	h.fileStyle = fileStyle
	h.skillStyle = skillStyle
	// The cached ranges carry the old styles, so retokenize on next use.
	h.lines = nil
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

// scanLine turns one logical line's tokens into styled rune ranges.
func (h *promptHighlighter) scanLine(line []rune) []lipgloss.Range {
	tokens := common.ScanPromptTokens(line)
	if len(tokens) == 0 {
		return nil
	}
	ranges := make([]lipgloss.Range, 0, len(tokens))
	for _, tok := range tokens {
		ranges = append(ranges, lipgloss.NewRange(tok.Start, tok.End, h.tokenStyle(tok.Kind)))
	}
	return ranges
}

func (h *promptHighlighter) tokenStyle(kind common.PromptTokenKind) lipgloss.Style {
	if kind == common.PromptTokenSkill {
		return h.skillStyle
	}
	return h.fileStyle
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

// refreshSkillNames re-primes the known-skill set that validates "/" tokens.
// Called from every path that can change the effective skill list.
func (m *UI) refreshSkillNames() {
	common.SetPromptSkillNames(m.skillNames())
}
