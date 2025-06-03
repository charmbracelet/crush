package diffview

import (
	"github.com/aymanbagabas/go-udiff"
	"github.com/aymanbagabas/go-udiff/myers"
	"github.com/charmbracelet/lipgloss/v2"
)

type file struct {
	path    string
	content string
}

type layout int

const (
	layoutUnified layout = iota + 1
	layoutSplit
)

// DiffView represents a view for displaying differences between two files.
type DiffView struct {
	layout       layout
	before       file
	after        file
	contextLines int
	baseStyle    lipgloss.Style
	highlight    bool
	height       int
	width        int

	isComputed bool
	err        error
	unified    udiff.UnifiedDiff
	edits      []udiff.Edit
}

// New creates a new DiffView with default settings.
func New() *DiffView {
	return &DiffView{
		layout:       layoutUnified,
		contextLines: udiff.DefaultContextLines,
	}
}

// Unified sets the layout of the DiffView to unified.
func (dv *DiffView) Unified() *DiffView {
	dv.layout = layoutUnified
	return dv
}

// Split sets the layout of the DiffView to split (side-by-side).
func (dv *DiffView) Split() *DiffView {
	dv.layout = layoutSplit
	return dv
}

// Before sets the "before" file for the DiffView.
func (dv *DiffView) Before(path, content string) *DiffView {
	dv.before = file{path: path, content: content}
	return dv
}

// After sets the "after" file for the DiffView.
func (dv *DiffView) After(path, content string) *DiffView {
	dv.after = file{path: path, content: content}
	return dv
}

// ContextLines sets the number of context lines for the DiffView.
func (dv *DiffView) ContextLines(contextLines int) *DiffView {
	dv.contextLines = contextLines
	return dv
}

// BaseStyle sets the base style for the DiffView.
// This is useful for setting a custom background color, for example.
func (dv *DiffView) BaseStyle(baseStyle lipgloss.Style) *DiffView {
	dv.baseStyle = baseStyle
	return dv
}

// SyntaxHighlight sets whether to enable syntax highlighting in the DiffView.
func (dv *DiffView) SyntaxHighlight(highlight bool) *DiffView {
	dv.highlight = highlight
	return dv
}

// Height sets the height of the DiffView.
func (dv *DiffView) Height(height int) *DiffView {
	dv.height = height
	return dv
}

// Width sets the width of the DiffView.
func (dv *DiffView) Width(width int) *DiffView {
	dv.width = width
	return dv
}

// String returns the string representation of the DiffView.
func (dv *DiffView) String() string {
	if !dv.isComputed {
		dv.compute()
	}
	if dv.err != nil {
		return dv.err.Error()
	}
	return dv.unified.String()
}

func (dv *DiffView) compute() {
	dv.isComputed = true
	dv.edits = myers.ComputeEdits(
		dv.before.content,
		dv.after.content,
	)
	dv.unified, dv.err = udiff.ToUnifiedDiff(
		dv.before.path,
		dv.after.path,
		dv.before.content,
		dv.edits,
		dv.contextLines,
	)
}
