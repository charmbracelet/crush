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
	xOffset      int
	yOffset      int
	style        Style

	isComputed bool
	err        error
	unified    udiff.UnifiedDiff
	edits      []udiff.Edit

	splitHunks []splitHunk

	codeWidth       int
	fullCodeWidth   int  // with leading symbols
	extraColOnAfter bool // add extra column on after panel
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

// XOffset sets the horizontal offset for the DiffView.
func (dv *DiffView) XOffset(xOffset int) *DiffView {
	dv.xOffset = xOffset
	return dv
}

// YOffset sets the vertical offset for the DiffView.
func (dv *DiffView) YOffset(yOffset int) *DiffView {
	dv.yOffset = yOffset
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

	if dv.width <= 0 {
		dv.detectCodeWidth()
	} else {
		dv.resizeCodeWidth()
	}

	style := lipgloss.NewStyle()
	if dv.width > 0 {
		style = style.MaxWidth(dv.width)
	}
	if dv.height > 0 {
		style = style.MaxHeight(dv.height)
	}

	switch dv.layout {
	case layoutUnified:
		return style.Render(strings.TrimSuffix(dv.renderUnified(), "\n"))
	case layoutSplit:
		return style.Render(strings.TrimSuffix(dv.renderSplit(), "\n"))
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
	setPadding := func(s lipgloss.Style) lipgloss.Style {
		return s.Padding(0, lineNumPadding).Align(lipgloss.Right)
	}
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

// resizeCodeWidth resizes the code width to fit within the specified width.
func (dv *DiffView) resizeCodeWidth() {
	fullNumWidth := dv.beforeNumDigits + dv.afterNumDigits
	fullNumWidth += lineNumPadding * 4 // left and right padding for both line numbers

	switch dv.layout {
	case layoutUnified:
		dv.codeWidth = dv.width - fullNumWidth - leadingSymbolsSize
	case layoutSplit:
		remainingWidth := dv.width - fullNumWidth - leadingSymbolsSize*2
		dv.codeWidth = remainingWidth / 2
		dv.extraColOnAfter = isOdd(remainingWidth)
	}

	dv.fullCodeWidth = dv.codeWidth + leadingSymbolsSize
}

// renderUnified renders the unified diff view as a string.
func (dv *DiffView) renderUnified() string {
	var b strings.Builder

	fullContentStyle := lipgloss.NewStyle().MaxWidth(dv.fullCodeWidth)
	printedLines := -dv.yOffset

	write := func(s string) {
		if printedLines >= 0 {
			b.WriteString(s)
		}
	}

outer:
	for i, h := range dv.unified.Hunks {
		if dv.lineNumbers {
			write(dv.style.DividerLine.LineNumber.Render(pad("…", dv.beforeNumDigits)))
			write(dv.style.DividerLine.LineNumber.Render(pad("…", dv.afterNumDigits)))
		}
		content := ansi.Truncate(dv.hunkLineFor(h), dv.fullCodeWidth, "…")
		write(dv.style.DividerLine.Code.Width(dv.fullCodeWidth).Render(content))
		write("\n")
		printedLines++

		beforeLine := h.FromLine
		afterLine := h.ToLine

		for j, l := range h.Lines {
			// print ellipis if we don't have enough space to print the rest of the diff
			hasReachedHeight := dv.height > 0 && printedLines+1 == dv.height
			isLastHunk := i+1 == len(dv.unified.Hunks)
			isLastLine := j+1 == len(h.Lines)
			if hasReachedHeight && (!isLastHunk || !isLastLine) {
				lineStyle := dv.lineStyleForType(l.Kind)
				if dv.lineNumbers {
					write(lineStyle.LineNumber.Render(pad("…", dv.beforeNumDigits)))
					write(lineStyle.LineNumber.Render(pad("…", dv.afterNumDigits)))
				}
				write(fullContentStyle.Render(
					lineStyle.Code.Width(dv.fullCodeWidth).Render("  …"),
				))
				write("\n")
				break outer
			}

			content := strings.TrimSuffix(l.Content, "\n")
			content = ansi.GraphemeWidth.Cut(content, dv.xOffset, len(content))
			content = ansi.Truncate(content, dv.codeWidth, "…")

			leadingEllipsis := dv.xOffset > 0 && strings.TrimSpace(content) != ""

			switch l.Kind {
			case udiff.Equal:
				if dv.lineNumbers {
					write(dv.style.EqualLine.LineNumber.Render(pad(beforeLine, dv.beforeNumDigits)))
					write(dv.style.EqualLine.LineNumber.Render(pad(afterLine, dv.afterNumDigits)))
				}
				write(fullContentStyle.Render(
					dv.style.EqualLine.Code.Width(dv.fullCodeWidth).Render(ternary(leadingEllipsis, " …", "  ") + content),
				))
				beforeLine++
				afterLine++
			case udiff.Insert:
				if dv.lineNumbers {
					write(dv.style.InsertLine.LineNumber.Render(pad(" ", dv.beforeNumDigits)))
					write(dv.style.InsertLine.LineNumber.Render(pad(afterLine, dv.afterNumDigits)))
				}
				write(fullContentStyle.Render(
					dv.style.InsertLine.Symbol.Render(ternary(leadingEllipsis, "+…", "+ ")) +
						dv.style.InsertLine.Code.Width(dv.codeWidth).Render(content),
				))
				afterLine++
			case udiff.Delete:
				if dv.lineNumbers {
					write(dv.style.DeleteLine.LineNumber.Render(pad(beforeLine, dv.beforeNumDigits)))
					write(dv.style.DeleteLine.LineNumber.Render(pad(" ", dv.afterNumDigits)))
				}
				write(fullContentStyle.Render(
					dv.style.DeleteLine.Symbol.Render(ternary(leadingEllipsis, "-…", "- ")) +
						dv.style.DeleteLine.Code.Width(dv.codeWidth).Render(content),
				))
				beforeLine++
			}
			write("\n")

			printedLines++
		}
	}

	for printedLines < dv.height {
		if dv.lineNumbers {
			write(dv.style.MissingLine.LineNumber.Render(pad(" ", dv.beforeNumDigits)))
			write(dv.style.MissingLine.LineNumber.Render(pad(" ", dv.afterNumDigits)))
		}
		write(dv.style.MissingLine.Code.Width(dv.fullCodeWidth).Render("  "))
		write("\n")
		printedLines++
	}

	return b.String()
}

// renderSplit renders the split (side-by-side) diff view as a string.
func (dv *DiffView) renderSplit() string {
	var b strings.Builder

	beforeFullContentStyle := lipgloss.NewStyle().MaxWidth(dv.fullCodeWidth)
	afterFullContentStyle := lipgloss.NewStyle().MaxWidth(dv.fullCodeWidth + btoi(dv.extraColOnAfter))
	printedLines := -dv.yOffset

	write := func(s string) {
		if printedLines >= 0 {
			b.WriteString(s)
		}
	}

outer:
	for i, h := range dv.splitHunks {
		if dv.lineNumbers {
			write(dv.style.DividerLine.LineNumber.Render(pad("…", dv.beforeNumDigits)))
		}
		content := ansi.Truncate(dv.hunkLineFor(dv.unified.Hunks[i]), dv.fullCodeWidth, "…")
		write(dv.style.DividerLine.Code.Width(dv.fullCodeWidth).Render(content))
		if dv.lineNumbers {
			write(dv.style.DividerLine.LineNumber.Render(pad("…", dv.afterNumDigits)))
		}
		write(dv.style.DividerLine.Code.Width(dv.fullCodeWidth + btoi(dv.extraColOnAfter)).Render(" "))
		write("\n")
		printedLines++

		beforeLine := h.fromLine
		afterLine := h.toLine

		for j, l := range h.lines {
			// print ellipis if we don't have enough space to print the rest of the diff
			hasReachedHeight := dv.height > 0 && printedLines+1 == dv.height
			isLastHunk := i+1 == len(dv.unified.Hunks)
			isLastLine := j+1 == len(h.lines)
			if hasReachedHeight && (!isLastHunk || !isLastLine) {
				lineStyle := dv.style.MissingLine
				if l.before != nil {
					lineStyle = dv.lineStyleForType(l.before.Kind)
				}
				if dv.lineNumbers {
					write(lineStyle.LineNumber.Render(pad("…", dv.beforeNumDigits)))
				}
				write(beforeFullContentStyle.Render(
					lineStyle.Code.Width(dv.fullCodeWidth).Render("  …"),
				))
				lineStyle = dv.style.MissingLine
				if l.after != nil {
					lineStyle = dv.lineStyleForType(l.after.Kind)
				}
				if dv.lineNumbers {
					write(lineStyle.LineNumber.Render(pad("…", dv.afterNumDigits)))
				}
				write(afterFullContentStyle.Render(
					lineStyle.Code.Width(dv.fullCodeWidth).Render("  …"),
				))
				write("\n")
				break outer
			}

			var beforeContent string
			var afterContent string
			if l.before != nil {
				beforeContent = strings.TrimSuffix(l.before.Content, "\n")
				beforeContent = ansi.GraphemeWidth.Cut(beforeContent, dv.xOffset, len(beforeContent))
				beforeContent = ansi.Truncate(beforeContent, dv.codeWidth, "…")
			}
			if l.after != nil {
				afterContent = strings.TrimSuffix(l.after.Content, "\n")
				afterContent = ansi.GraphemeWidth.Cut(afterContent, dv.xOffset, len(afterContent))
				afterContent = ansi.Truncate(afterContent, dv.codeWidth+btoi(dv.extraColOnAfter), "…")
			}

			leadingBeforeEllipsis := dv.xOffset > 0 && strings.TrimSpace(beforeContent) != ""
			leadingAfterEllipsis := dv.xOffset > 0 && strings.TrimSpace(afterContent) != ""

			switch {
			case l.before == nil:
				if dv.lineNumbers {
					write(dv.style.MissingLine.LineNumber.Render(pad(" ", dv.beforeNumDigits)))
				}
				write(beforeFullContentStyle.Render(
					dv.style.MissingLine.Code.Width(dv.fullCodeWidth).Render("  "),
				))
			case l.before.Kind == udiff.Equal:
				if dv.lineNumbers {
					write(dv.style.EqualLine.LineNumber.Render(pad(beforeLine, dv.beforeNumDigits)))
				}
				write(beforeFullContentStyle.Render(
					dv.style.EqualLine.Code.Width(dv.fullCodeWidth).Render(ternary(leadingBeforeEllipsis, " …", "  ") + beforeContent),
				))
				beforeLine++
			case l.before.Kind == udiff.Delete:
				if dv.lineNumbers {
					write(dv.style.DeleteLine.LineNumber.Render(pad(beforeLine, dv.beforeNumDigits)))
				}
				write(beforeFullContentStyle.Render(
					dv.style.DeleteLine.Symbol.Render(ternary(leadingBeforeEllipsis, "-…", "- ")) +
						dv.style.DeleteLine.Code.Width(dv.codeWidth).Render(beforeContent),
				))
				beforeLine++
			}

			switch {
			case l.after == nil:
				if dv.lineNumbers {
					write(dv.style.MissingLine.LineNumber.Render(pad(" ", dv.afterNumDigits)))
				}
				write(afterFullContentStyle.Render(
					dv.style.MissingLine.Code.Width(dv.fullCodeWidth + btoi(dv.extraColOnAfter)).Render("  "),
				))
			case l.after.Kind == udiff.Equal:
				if dv.lineNumbers {
					write(dv.style.EqualLine.LineNumber.Render(pad(afterLine, dv.afterNumDigits)))
				}
				write(afterFullContentStyle.Render(
					dv.style.EqualLine.Code.Width(dv.fullCodeWidth + btoi(dv.extraColOnAfter)).Render(ternary(leadingAfterEllipsis, " …", "  ") + afterContent),
				))
				afterLine++
			case l.after.Kind == udiff.Insert:
				if dv.lineNumbers {
					write(dv.style.InsertLine.LineNumber.Render(pad(afterLine, dv.afterNumDigits)))
				}
				write(afterFullContentStyle.Render(
					dv.style.InsertLine.Symbol.Render(ternary(leadingAfterEllipsis, "+…", "+ ")) +
						dv.style.InsertLine.Code.Width(dv.codeWidth+btoi(dv.extraColOnAfter)).Render(afterContent),
				))
				afterLine++
			}

			write("\n")

			printedLines++
		}
	}

	for printedLines < dv.height {
		if dv.lineNumbers {
			write(dv.style.MissingLine.LineNumber.Render(pad("…", dv.beforeNumDigits)))
		}
		write(dv.style.MissingLine.Code.Width(dv.fullCodeWidth).Render(" "))
		if dv.lineNumbers {
			write(dv.style.MissingLine.LineNumber.Render(pad("…", dv.afterNumDigits)))
		}
		write(dv.style.MissingLine.Code.Width(dv.fullCodeWidth + btoi(dv.extraColOnAfter)).Render(" "))
		write("\n")
		printedLines++
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

func (dv *DiffView) lineStyleForType(t udiff.OpKind) LineStyle {
	switch t {
	case udiff.Equal:
		return dv.style.EqualLine
	case udiff.Insert:
		return dv.style.InsertLine
	case udiff.Delete:
		return dv.style.DeleteLine
	default:
		return dv.style.MissingLine
	}
}
