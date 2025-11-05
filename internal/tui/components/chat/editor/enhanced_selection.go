package editor

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/v2/textarea"
)

// Position represents a cursor position in text using line/column coordinates
// This is more robust than character-based positions
type Position struct {
	Line int // 0-based line number
	Col  int // 0-based column number (character position, not visual)
}

// String returns a string representation of the position
func (p Position) String() string {
	return fmt.Sprintf("L%d:C%d", p.Line, p.Col)
}

// IsZero returns true if position is at (0,0)
func (p Position) IsZero() bool {
	return p.Line == 0 && p.Col == 0
}

// Compare returns -1 if p < q, 0 if p == q, 1 if p > q
func (p Position) Compare(q Position) int {
	if p.Line < q.Line {
		return -1
	}
	if p.Line > q.Line {
		return 1
	}
	if p.Col < q.Col {
		return -1
	}
	if p.Col > q.Col {
		return 1
	}
	return 0
}

// SelectionRange represents a text selection using position-based coordinates
type SelectionRange struct {
	Start Position // Start of selection (inclusive)
	End   Position // End of selection (exclusive)
}

// String returns string representation
func (sr SelectionRange) String() string {
	return fmt.Sprintf("[%s-%s]", sr.Start, sr.End)
}

// IsEmpty returns true if selection has zero length
func (sr SelectionRange) IsEmpty() bool {
	return sr.Start.Compare(sr.End) == 0
}

// IsNormalized returns true if Start <= End
func (sr SelectionRange) IsNormalized() bool {
	return sr.Start.Compare(sr.End) <= 0
}

// Normalize returns a selection where Start <= End
func (sr SelectionRange) Normalize() SelectionRange {
	if sr.IsNormalized() {
		return sr
	}
	return SelectionRange{
		Start: sr.End,
		End:   sr.Start,
	}
}

// Length returns the length of selection in characters
func (sr SelectionRange) Length(text string) int {
	if sr.IsEmpty() {
		return 0
	}
	normalized := sr.Normalize()
	return normalized.End.CharPosition(text) - normalized.Start.CharPosition(text)
}

// CharPosition converts position to character index in text
func (p Position) CharPosition(text string) int {
	lines := strings.Split(text, "\n")
	if p.Line >= len(lines) {
		return len(text)
	}

	charPos := 0
	for i := 0; i < p.Line && i < len(lines); i++ {
		charPos += len(lines[i]) + 1 // +1 for \n
	}

	if p.Line < len(lines) {
		line := lines[p.Line]
		if p.Col > len(line) {
			charPos += len(line)
		} else {
			charPos += p.Col
		}
	}

	return charPos
}

// FromCharPosition creates a Position from character index
func FromCharPosition(text string, charPos int) Position {
	lines := strings.Split(text, "\n")
	currentPos := 0
	line := 0
	col := 0

	for i, lineContent := range lines {
		if currentPos+len(lineContent) >= charPos {
			line = i
			col = charPos - currentPos
			break
		}
		currentPos += len(lineContent) + 1 // +1 for \n
	}

	return Position{Line: line, Col: col}
}

// EnhancedSelectionManager manages selection using position-based coordinates
type EnhancedSelectionManager struct {
	selection SelectionRange
	textarea  *textarea.Model
	text      string // Cached text content
}

// NewEnhancedSelectionManager creates a new selection manager
func NewEnhancedSelectionManager(ta *textarea.Model) *EnhancedSelectionManager {
	return &EnhancedSelectionManager{
		selection: SelectionRange{
			Start: Position{Line: 0, Col: 0},
			End:   Position{Line: 0, Col: 0},
		},
		textarea: ta,
		text:     ta.Value(),
	}
}

// SelectAll selects all text using position-based selection
func (esm *EnhancedSelectionManager) SelectAll() {
	text := esm.textarea.Value()
	if text == "" {
		esm.selection = SelectionRange{}
		esm.text = text
		return
	}

	esm.selection = SelectionRange{
		Start: Position{Line: 0, Col: 0},
		End:   FromCharPosition(text, len(text)),
	}
	esm.text = text
}

// Clear removes the selection
func (esm *EnhancedSelectionManager) Clear() {
	esm.selection = SelectionRange{}
}

// GetSelectedText returns the selected portion of text
func (esm *EnhancedSelectionManager) GetSelectedText() string {
	if esm.selection.IsEmpty() {
		return ""
	}

	text := esm.textarea.Value()
	start := esm.selection.Start.CharPosition(text)
	end := esm.selection.End.CharPosition(text)

	if start < 0 || end > len(text) || start >= end {
		return ""
	}

	return text[start:end]
}

// HasSelection returns true if there is an active selection
func (esm *EnhancedSelectionManager) HasSelection() bool {
	return !esm.selection.IsEmpty() &&
		esm.selection.Start.CharPosition(esm.text) >= 0 &&
		esm.selection.End.CharPosition(esm.text) >= 0
}

// SetSelection sets selection using character positions (converted to position-based)
func (esm *EnhancedSelectionManager) SetSelection(startChar, endChar int) {
	text := esm.textarea.Value()
	esm.selection = SelectionRange{
		Start: FromCharPosition(text, startChar),
		End:   FromCharPosition(text, endChar),
	}
	esm.text = text
}

// GetSelection returns the current selection range
func (esm *EnhancedSelectionManager) GetSelection() SelectionRange {
	return esm.selection
}

// Sync synchronizes selection with textarea content changes
func (esm *EnhancedSelectionManager) Sync() {
	newText := esm.textarea.Value()
	if newText != esm.text {
		// Text changed, clear selection or adjust as needed
		if esm.HasSelection() {
			// For now, clear on text change (could be enhanced)
			esm.Clear()
		}
		esm.text = newText
	}
}
