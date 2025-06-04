package diffview

import (
	"os"
	"strconv"
	"strings"

	"github.com/aymanbagabas/go-udiff"
	"github.com/aymanbagabas/go-udiff/myers"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

const leadingSymbolsSize = 2

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
	lineNumbers  bool
	highlight    bool
	height       int
	width        int
	style        Style

	isComputed bool
	err        error
	unified    udiff.UnifiedDiff
	edits      []udiff.Edit
}

// New creates a new DiffView with default settings.
func New() *DiffView {
	dv := &DiffView{
		layout:       layoutUnified,
		contextLines: udiff.DefaultContextLines,
		lineNumbers:  true,
	}
	if lipgloss.HasDarkBackground(os.Stdin, os.Stdout) {
		dv.style = DefaultDarkStyle
	} else {
		dv.style = DefaultLightStyle
	}
	return dv
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

// Style sets the style for the DiffView.
func (dv *DiffView) Style(style Style) *DiffView {
	dv.style = style
	return dv
}

// LineNumbers sets whether to display line numbers in the DiffView.
func (dv *DiffView) LineNumbers(lineNumbers bool) *DiffView {
	dv.lineNumbers = lineNumbers
	return dv
}

// SyntaxHightlight sets whether to enable syntax highlighting in the DiffView.
func (dv *DiffView) SyntaxHightlight(highlight bool) *DiffView {
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
	if err := dv.computeDiff(); err != nil {
		return err.Error()
	}
	dv.detectWidth()

	lineNumberWidth := func(start, num int) int {
		return len(strconv.Itoa(start + num))
	}

	var b strings.Builder

	for _, h := range dv.unified.Hunks {
		beforeLine := h.FromLine
		afterLine := h.ToLine

		beforeNumDigits := lineNumberWidth(h.FromLine, len(h.Lines))
		afterNumDigits := lineNumberWidth(h.ToLine, len(h.Lines))

		for _, l := range h.Lines {
			content := strings.TrimSuffix(l.Content, "\n")
			codeWidth := dv.width - leadingSymbolsSize

			switch l.Kind {
			case udiff.Equal:
				if dv.lineNumbers {
					b.WriteString(dv.style.EqualLine.LineNumber.Render(pad(beforeLine, beforeNumDigits)))
					b.WriteString(dv.style.EqualLine.LineNumber.Render(pad(afterLine, afterNumDigits)))
				}
				b.WriteString(dv.style.EqualLine.Code.Width(codeWidth + leadingSymbolsSize).Render("  " + content))
				beforeLine++
				afterLine++
			case udiff.Insert:
				if dv.lineNumbers {
					b.WriteString(dv.style.InsertLine.LineNumber.Render(pad(" ", beforeNumDigits)))
					b.WriteString(dv.style.InsertLine.LineNumber.Render(pad(afterLine, afterNumDigits)))
				}
				b.WriteString(dv.style.InsertLine.Symbol.Render("+ "))
				b.WriteString(dv.style.InsertLine.Code.Width(codeWidth).Render(content))
				afterLine++
			case udiff.Delete:
				if dv.lineNumbers {
					b.WriteString(dv.style.DeleteLine.LineNumber.Render(pad(beforeLine, beforeNumDigits)))
					b.WriteString(dv.style.DeleteLine.LineNumber.Render(pad(" ", afterNumDigits)))
				}
				b.WriteString(dv.style.DeleteLine.Symbol.Render("- "))
				b.WriteString(dv.style.DeleteLine.Code.Width(codeWidth).Render(content))
				beforeLine++
			}
			b.WriteRune('\n')
		}
	}

	return b.String()
}

func (dv *DiffView) computeDiff() error {
	if dv.isComputed {
		return dv.err
	}
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
	return dv.err
}

func (dv *DiffView) detectWidth() {
	if dv.width > 0 {
		return
	}

	for _, h := range dv.unified.Hunks {
		for _, l := range h.Lines {
			lineWidth := ansi.StringWidth(strings.TrimSuffix(l.Content, "\n"))
			lineWidth += leadingSymbolsSize
			dv.width = max(dv.width, lineWidth)
		}
	}
}
