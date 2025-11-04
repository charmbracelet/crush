package editor

import (
	"math"

	"github.com/charmbracelet/bubbles/v2/textarea"
)

// Selection represents text selection in the editor.
type Selection struct {
	Start  int  // Character position where selection starts
	End    int  // Character position where selection ends
	Active bool // Whether selection is being actively created (drag)
}

// NewSelection creates a new selection with the given bounds.
func NewSelection(start, end int) Selection {
	return Selection{
		Start:  start,
		End:    end,
		Active: false,
	}
}

// IsActive returns whether there is an active selection.
func (s Selection) IsActive() bool {
	return s.Start != s.End && s.Start >= 0 && s.End >= 0
}

// Length returns the length of the selection.
func (s Selection) Length() int {
	if !s.IsActive() {
		return 0
	}
	return int(math.Abs(float64(s.End - s.Start)))
}

// Bounds returns the selection bounds normalized (start <= end).
func (s Selection) Bounds() (start, end int) {
	if !s.IsActive() {
		return 0, 0
	}
	start = int(math.Min(float64(s.Start), float64(s.End)))
	end = int(math.Max(float64(s.Start), float64(s.End)))
	return start, end
}

// Contains checks if the given position is within the selection.
func (s Selection) Contains(pos int) bool {
	if !s.IsActive() {
		return false
	}
	start, end := s.Bounds()
	return pos >= start && pos < end
}

// Clear removes the selection.
func (s *Selection) Clear() {
	s.Start = -1
	s.End = -1
	s.Active = false
}

// SelectAll creates a selection that encompasses all text.
func (s *Selection) SelectAll(text string) {
	s.Start = 0
	s.End = len(text)
	s.Active = false // Not actively selecting, just selected
}

// GetText returns the selected portion of the given text.
func (s Selection) GetText(text string) string {
	if !s.IsActive() {
		return ""
	}
	start, end := s.Bounds()
	if start < 0 || end > len(text) || start >= end {
		return ""
	}
	return text[start:end]
}

// SelectionManager manages text selection for a textarea.
type SelectionManager struct {
	selection Selection
	textarea  *textarea.Model
}

// NewSelectionManager creates a new selection manager for the given textarea.
func NewSelectionManager(ta *textarea.Model) *SelectionManager {
	return &SelectionManager{
		selection: NewSelection(-1, -1),
		textarea:  ta,
	}
}

// SelectAll selects all text in the textarea.
func (sm *SelectionManager) SelectAll() {
	sm.selection.SelectAll(sm.textarea.Value())
}

// Clear clears the current selection.
func (sm *SelectionManager) Clear() {
	sm.selection.Clear()
}

// GetSelectedText returns the currently selected text.
func (sm *SelectionManager) GetSelectedText() string {
	return sm.selection.GetText(sm.textarea.Value())
}

// HasSelection returns whether there is an active selection.
func (sm *SelectionManager) HasSelection() bool {
	return sm.selection.IsActive()
}

// SetSelection sets the selection to the specified bounds.
func (sm *SelectionManager) SetSelection(start, end int) {
	sm.selection.Start = start
	sm.selection.End = end
	sm.selection.Active = false
}

// GetSelection returns the current selection.
func (sm *SelectionManager) GetSelection() Selection {
	return sm.selection
}