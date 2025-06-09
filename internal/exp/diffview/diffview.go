package diffview

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/aymanbagabas/go-udiff"
	"github.com/aymanbagabas/go-udiff/myers"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

const (
	leadingSymbolsSize = 2
	lineNumPadding     = 1
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
	lineNumbers  bool
	highlight    bool
	height       int
	width        int
	style        Style

	isComputed bool
	err        error
	unified    udiff.UnifiedDiff
	edits      []udiff.Edit

	splitHunks []splitHunk

	codeWidth       int
	fullCodeWidth   int // with leading symbols
	beforeNumDigits int
	afterNumDigits  int
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
	dv.convertDiffToSplit()
	dv.adjustStyles()
	dv.detectNumDigits()
	dv.detectCodeWidth()

	switch dv.layout {
	case layoutUnified:
		return dv.renderUnified()
	case layoutSplit:
		return dv.renderSplit()
	default:
		panic("unknown diffview layout")
	}
}

// computeDiff computes the differences between the "before" and "after" files.
func (dv *DiffView) computeDiff() error {
	if dv.isComputed {
		return dv.err
	}
	dv.isComputed = true
	dv.edits = myers.ComputeEdits( //nolint:staticcheck
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

// convertDiffToSplit converts the unified diff to a split diff if the layout is
// set to split.
func (dv *DiffView) convertDiffToSplit() {
	if dv.layout != layoutSplit {
		return
	}

	dv.splitHunks = make([]splitHunk, len(dv.unified.Hunks))
	for i, h := range dv.unified.Hunks {
		dv.splitHunks[i] = hunkToSplit(h)
	}
}

// adjustStyles adjusts adds padding and alignment to the styles.
func (dv *DiffView) adjustStyles() {
	dv.style.MissingLine.LineNumber = setPadding(dv.style.MissingLine.LineNumber)
	dv.style.DividerLine.LineNumber = setPadding(dv.style.DividerLine.LineNumber)
	dv.style.EqualLine.LineNumber = setPadding(dv.style.EqualLine.LineNumber)
	dv.style.InsertLine.LineNumber = setPadding(dv.style.InsertLine.LineNumber)
	dv.style.DeleteLine.LineNumber = setPadding(dv.style.DeleteLine.LineNumber)
}

// detectNumDigits calculates the maximum number of digits needed for before and
// after line numbers.
func (dv *DiffView) detectNumDigits() {
	dv.beforeNumDigits = 0
	dv.afterNumDigits = 0

	for _, h := range dv.unified.Hunks {
		dv.beforeNumDigits = max(dv.beforeNumDigits, len(strconv.Itoa(h.FromLine+len(h.Lines))))
		dv.afterNumDigits = max(dv.afterNumDigits, len(strconv.Itoa(h.ToLine+len(h.Lines))))
	}
}

func setPadding(s lipgloss.Style) lipgloss.Style {
	return s.Padding(0, lineNumPadding).Align(lipgloss.Right)
}

// detectCodeWidth calculates the maximum width of code lines in the diff view.
func (dv *DiffView) detectCodeWidth() {
	switch dv.layout {
	case layoutUnified:
		dv.detectUnifiedCodeWidth()
	case layoutSplit:
		dv.detectSplitCodeWidth()
	}
	dv.fullCodeWidth = dv.codeWidth + leadingSymbolsSize
}

// detectUnifiedCodeWidth calculates the maximum width of code lines in a
// unified diff.
func (dv *DiffView) detectUnifiedCodeWidth() {
	dv.codeWidth = 0

	for _, h := range dv.unified.Hunks {
		shownLines := ansi.StringWidth(dv.hunkLineFor(h))

		for _, l := range h.Lines {
			lineWidth := ansi.StringWidth(strings.TrimSuffix(l.Content, "\n")) + 1
			dv.codeWidth = max(dv.codeWidth, lineWidth, shownLines)
		}
	}
}

// detectSplitCodeWidth calculates the maximum width of code lines in a
// split diff.
func (dv *DiffView) detectSplitCodeWidth() {
	dv.codeWidth = 0

	for i, h := range dv.splitHunks {
		shownLines := ansi.StringWidth(dv.hunkLineFor(dv.unified.Hunks[i]))

		for _, l := range h.lines {
			if l.before != nil {
				codeWidth := ansi.StringWidth(strings.TrimSuffix(l.before.Content, "\n")) + 1
				dv.codeWidth = max(dv.codeWidth, codeWidth, shownLines)
			}
			if l.after != nil {
				codeWidth := ansi.StringWidth(strings.TrimSuffix(l.after.Content, "\n")) + 1
				dv.codeWidth = max(dv.codeWidth, codeWidth, shownLines)
			}
		}
	}
}

// renderUnified renders the unified diff view as a string.
func (dv *DiffView) renderUnified() string {
	var b strings.Builder

	for _, h := range dv.unified.Hunks {
		if dv.lineNumbers {
			b.WriteString(dv.style.DividerLine.LineNumber.Render(pad("…", dv.beforeNumDigits)))
			b.WriteString(dv.style.DividerLine.LineNumber.Render(pad("…", dv.afterNumDigits)))
		}
		b.WriteString(dv.style.DividerLine.Code.Width(dv.fullCodeWidth).Render(dv.hunkLineFor(h)))
		b.WriteRune('\n')

		beforeLine := h.FromLine
		afterLine := h.ToLine

		for _, l := range h.Lines {
			content := strings.TrimSuffix(l.Content, "\n")

			switch l.Kind {
			case udiff.Equal:
				if dv.lineNumbers {
					b.WriteString(dv.style.EqualLine.LineNumber.Render(pad(beforeLine, dv.beforeNumDigits)))
					b.WriteString(dv.style.EqualLine.LineNumber.Render(pad(afterLine, dv.afterNumDigits)))
				}
				b.WriteString(dv.style.EqualLine.Code.Width(dv.fullCodeWidth).Render("  " + content))
				beforeLine++
				afterLine++
			case udiff.Insert:
				if dv.lineNumbers {
					b.WriteString(dv.style.InsertLine.LineNumber.Render(pad(" ", dv.beforeNumDigits)))
					b.WriteString(dv.style.InsertLine.LineNumber.Render(pad(afterLine, dv.afterNumDigits)))
				}
				b.WriteString(dv.style.InsertLine.Symbol.Render("+ "))
				b.WriteString(dv.style.InsertLine.Code.Width(dv.codeWidth).Render(content))
				afterLine++
			case udiff.Delete:
				if dv.lineNumbers {
					b.WriteString(dv.style.DeleteLine.LineNumber.Render(pad(beforeLine, dv.beforeNumDigits)))
					b.WriteString(dv.style.DeleteLine.LineNumber.Render(pad(" ", dv.afterNumDigits)))
				}
				b.WriteString(dv.style.DeleteLine.Symbol.Render("- "))
				b.WriteString(dv.style.DeleteLine.Code.Width(dv.codeWidth).Render(content))
				beforeLine++
			}
			b.WriteRune('\n')
		}
	}

	return b.String()
}

// renderSplit renders the split (side-by-side) diff view as a string.
func (dv *DiffView) renderSplit() string {
	var b strings.Builder

	for i, h := range dv.splitHunks {
		if dv.lineNumbers {
			b.WriteString(dv.style.DividerLine.LineNumber.Render(pad("…", dv.beforeNumDigits)))
		}
		b.WriteString(dv.style.DividerLine.Code.Width(dv.fullCodeWidth).Render(dv.hunkLineFor(dv.unified.Hunks[i])))
		if dv.lineNumbers {
			b.WriteString(dv.style.DividerLine.LineNumber.Render(pad("…", dv.afterNumDigits)))
		}
		b.WriteString(dv.style.DividerLine.Code.Width(dv.fullCodeWidth).Render(" "))
		b.WriteRune('\n')

		beforeLine := h.fromLine
		afterLine := h.toLine

		for _, l := range h.lines {
			var beforeContent string
			var afterContent string
			if l.before != nil {
				beforeContent = strings.TrimSuffix(l.before.Content, "\n")
			}
			if l.after != nil {
				afterContent = strings.TrimSuffix(l.after.Content, "\n")
			}

			switch {
			case l.before == nil:
				if dv.lineNumbers {
					b.WriteString(dv.style.MissingLine.LineNumber.Render(pad(" ", dv.beforeNumDigits)))
				}
				b.WriteString(dv.style.MissingLine.Code.Width(dv.fullCodeWidth).Render("  "))
			case l.before.Kind == udiff.Equal:
				if dv.lineNumbers {
					b.WriteString(dv.style.EqualLine.LineNumber.Render(pad(beforeLine, dv.beforeNumDigits)))
				}
				b.WriteString(dv.style.EqualLine.Code.Width(dv.fullCodeWidth).Render("  " + beforeContent))
				beforeLine++
			case l.before.Kind == udiff.Delete:
				if dv.lineNumbers {
					b.WriteString(dv.style.DeleteLine.LineNumber.Render(pad(beforeLine, dv.beforeNumDigits)))
				}
				b.WriteString(dv.style.DeleteLine.Symbol.Render("- "))
				b.WriteString(dv.style.DeleteLine.Code.Width(dv.codeWidth).Render(beforeContent))
				beforeLine++
			}

			switch {
			case l.after == nil:
				if dv.lineNumbers {
					b.WriteString(dv.style.MissingLine.LineNumber.Render(pad(" ", dv.afterNumDigits)))
				}
				b.WriteString(dv.style.MissingLine.Code.Width(dv.fullCodeWidth).Render("  "))
			case l.after.Kind == udiff.Equal:
				if dv.lineNumbers {
					b.WriteString(dv.style.EqualLine.LineNumber.Render(pad(afterLine, dv.afterNumDigits)))
				}
				b.WriteString(dv.style.EqualLine.Code.Width(dv.fullCodeWidth).Render("  " + afterContent))
				afterLine++
			case l.after.Kind == udiff.Insert:
				if dv.lineNumbers {
					b.WriteString(dv.style.InsertLine.LineNumber.Render(pad(afterLine, dv.afterNumDigits)))
				}
				b.WriteString(dv.style.InsertLine.Symbol.Render("+ "))
				b.WriteString(dv.style.InsertLine.Code.Width(dv.codeWidth).Render(afterContent))
				afterLine++
			}

			b.WriteRune('\n')
		}
	}

	return b.String()
}

// hunkLineFor formats the header line for a hunk in the unified diff view.
func (dv *DiffView) hunkLineFor(h *udiff.Hunk) string {
	beforeShownLines, afterShownLines := dv.hunkShownLines(h)

	return fmt.Sprintf(
		"  @@ -%d,%d +%d,%d @@ ",
		h.FromLine,
		beforeShownLines,
		h.ToLine,
		afterShownLines,
	)
}

// hunkShownLines calculates the number of lines shown in a hunk for both before
// and after versions.
func (dv *DiffView) hunkShownLines(h *udiff.Hunk) (before, after int) {
	for _, l := range h.Lines {
		switch l.Kind {
		case udiff.Equal:
			before++
			after++
		case udiff.Insert:
			after++
		case udiff.Delete:
			before++
		}
	}
	return
}
