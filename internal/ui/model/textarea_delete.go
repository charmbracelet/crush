package model

import (
	"strings"
	"unicode"

	tea "charm.land/bubbletea/v2"
)

func isCoreIdentifierRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '$'
}

func isIdentifierSeparatorRune(r rune) bool {
	return r == '_'
}

func isIdentifierRune(r rune) bool {
	return isCoreIdentifierRune(r) || isIdentifierSeparatorRune(r)
}

func isSymbolRune(r rune) bool {
	return !unicode.IsSpace(r) && !isIdentifierRune(r)
}

func isCamelHumpStart(runes []rune, idx int) bool {
	if idx <= 0 || idx >= len(runes) {
		return false
	}
	prev := runes[idx-1]
	curr := runes[idx]
	if !isCoreIdentifierRune(prev) || !isCoreIdentifierRune(curr) {
		return false
	}
	if unicode.IsDigit(prev) != unicode.IsDigit(curr) {
		return true
	}
	if unicode.IsLower(prev) && unicode.IsUpper(curr) {
		return true
	}
	if unicode.IsUpper(prev) && unicode.IsUpper(curr) && idx+1 < len(runes) {
		next := runes[idx+1]
		if isCoreIdentifierRune(next) && unicode.IsLower(next) {
			return true
		}
	}
	return false
}

func moveGroupBackwardBoundary(runes []rune, idx int) int {
	if idx <= 0 {
		return 0
	}
	if isCamelHumpStart(runes, idx) {
		idx--
	}
	if unicode.IsSpace(runes[idx-1]) {
		for idx > 0 && unicode.IsSpace(runes[idx-1]) {
			idx--
		}
		return idx
	}
	if isSymbolRune(runes[idx-1]) {
		for idx > 0 && isSymbolRune(runes[idx-1]) {
			idx--
		}
		return idx
	}

	for idx > 0 && isIdentifierSeparatorRune(runes[idx-1]) {
		idx--
	}
	if idx <= 0 || !isCoreIdentifierRune(runes[idx-1]) {
		return idx
	}

	for idx > 0 && isCoreIdentifierRune(runes[idx-1]) && !isCamelHumpStart(runes, idx) {
		idx--
	}
	return idx
}

func moveGroupForwardBoundary(runes []rune, idx int) int {
	if idx >= len(runes) {
		return len(runes)
	}
	if unicode.IsSpace(runes[idx]) {
		for idx < len(runes) && unicode.IsSpace(runes[idx]) {
			idx++
		}
		return idx
	}
	if isSymbolRune(runes[idx]) {
		for idx < len(runes) && isSymbolRune(runes[idx]) {
			idx++
		}
		return idx
	}

	for idx < len(runes) && isIdentifierSeparatorRune(runes[idx]) {
		idx++
	}
	if idx >= len(runes) || !isCoreIdentifierRune(runes[idx]) {
		return idx
	}

	idx++
	for idx < len(runes) && isCoreIdentifierRune(runes[idx]) && !isCamelHumpStart(runes, idx) {
		idx++
	}
	return idx
}

func moveCursorBackwardBoundary(runes []rune, idx int) int {
	if isCamelHumpStart(runes, idx) {
		idx--
	}
	for idx > 0 && (unicode.IsSpace(runes[idx-1]) || isIdentifierSeparatorRune(runes[idx-1])) {
		idx--
	}
	if idx <= 0 {
		return 0
	}
	if isSymbolRune(runes[idx-1]) {
		for idx > 0 && isSymbolRune(runes[idx-1]) {
			idx--
		}
		return idx
	}
	for idx > 0 && isCoreIdentifierRune(runes[idx-1]) && !isCamelHumpStart(runes, idx) {
		idx--
	}
	return idx
}

func moveCursorForwardBoundary(runes []rune, idx int) int {
	for idx < len(runes) && (unicode.IsSpace(runes[idx]) || isIdentifierSeparatorRune(runes[idx])) {
		idx++
	}
	if idx >= len(runes) {
		return len(runes)
	}
	if isSymbolRune(runes[idx]) {
		for idx < len(runes) && isSymbolRune(runes[idx]) {
			idx++
		}
		return idx
	}
	idx++
	for idx < len(runes) && isCoreIdentifierRune(runes[idx]) && !isCamelHumpStart(runes, idx) {
		idx++
	}
	return idx
}

func textareaLineColToIndex(lines []string, line, col int) int {
	idx := 0
	for i := 0; i < line && i < len(lines); i++ {
		idx += len([]rune(lines[i])) + 1
	}
	if line >= len(lines) {
		return idx
	}
	return idx + min(col, len([]rune(lines[line])))
}

func textareaIndexToLineCol(lines []string, idx int) (int, int) {
	if idx <= 0 {
		return 0, 0
	}
	for line, text := range lines {
		lineLen := len([]rune(text))
		if idx <= lineLen {
			return line, idx
		}
		idx -= lineLen
		if idx == 0 {
			return line, lineLen
		}
		idx--
	}
	last := len(lines) - 1
	if last < 0 {
		return 0, 0
	}
	return last, len([]rune(lines[last]))
}

func (m *UI) restoreTextareaCursor(line, col int) {
	m.textarea.MoveToBegin()
	for range line {
		m.textarea.CursorDown()
	}
	m.textarea.SetCursorColumn(col)
}

func (m *UI) setTextareaCursorFromIndex(lines []string, idx int) {
	line, col := textareaIndexToLineCol(lines, idx)
	m.restoreTextareaCursor(line, col)
}

func (m *UI) replaceTextareaContent(lines []string, idx int, prevHeight int) tea.Cmd {
	m.textarea.SetValue(strings.Join(lines, "\n"))
	m.setTextareaCursorFromIndex(lines, idx)
	return m.handleTextareaHeightChange(prevHeight)
}

func (m *UI) moveTextareaGroupBackward() {
	lines := strings.Split(m.textarea.Value(), "\n")
	idx := textareaLineColToIndex(lines, m.textarea.Line(), m.textarea.Column())
	m.setTextareaCursorFromIndex(lines, moveCursorBackwardBoundary([]rune(m.textarea.Value()), idx))
}

func (m *UI) moveTextareaGroupForward() {
	lines := strings.Split(m.textarea.Value(), "\n")
	idx := textareaLineColToIndex(lines, m.textarea.Line(), m.textarea.Column())
	m.setTextareaCursorFromIndex(lines, moveCursorForwardBoundary([]rune(m.textarea.Value()), idx))
}

func (m *UI) deleteTextareaGroupBackward() tea.Cmd {
	prevHeight := m.textarea.Height()
	lines := strings.Split(m.textarea.Value(), "\n")
	runes := []rune(m.textarea.Value())
	idx := textareaLineColToIndex(lines, m.textarea.Line(), m.textarea.Column())
	start := moveGroupBackwardBoundary(runes, idx)
	if start == idx {
		return nil
	}
	return m.replaceTextareaContent(strings.Split(string(append(runes[:start], runes[idx:]...)), "\n"), start, prevHeight)
}

func (m *UI) deleteTextareaGroupForward() tea.Cmd {
	prevHeight := m.textarea.Height()
	lines := strings.Split(m.textarea.Value(), "\n")
	runes := []rune(m.textarea.Value())
	idx := textareaLineColToIndex(lines, m.textarea.Line(), m.textarea.Column())
	end := moveGroupForwardBoundary(runes, idx)
	if end == idx {
		return nil
	}
	return m.replaceTextareaContent(strings.Split(string(append(runes[:idx], runes[end:]...)), "\n"), idx, prevHeight)
}
