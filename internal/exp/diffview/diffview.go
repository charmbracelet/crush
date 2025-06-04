package diffview

import (
	"os"
	"strings"

	"github.com/aymanbagabas/go-udiff"
	"github.com/aymanbagabas/go-udiff/myers"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/charmtone"
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

type LineStyle struct {
	Symbol lipgloss.Style
	Code   lipgloss.Style
}

type Style struct {
	EqualLine  LineStyle
	InsertLine LineStyle
	DeleteLine LineStyle
}

var DefaultLightStyle = Style{
	EqualLine: LineStyle{
		Code: lipgloss.NewStyle().
			Foreground(charmtone.Pepper).
			Background(charmtone.Salt),
	},
	InsertLine: LineStyle{
		Symbol: lipgloss.NewStyle().
			Foreground(charmtone.Turtle).
			Background(lipgloss.Color("#e8f5e9")),
		Code: lipgloss.NewStyle().
			Foreground(charmtone.Pepper).
			Background(lipgloss.Color("#e8f5e9")),
	},
	DeleteLine: LineStyle{
		Symbol: lipgloss.NewStyle().
			Foreground(charmtone.Cherry).
			Background(lipgloss.Color("#ffebee")),
		Code: lipgloss.NewStyle().
			Foreground(charmtone.Pepper).
			Background(lipgloss.Color("#ffebee")),
	},
}

var DefaultDarkStyle = Style{
	EqualLine: LineStyle{
		Code: lipgloss.NewStyle().
			Foreground(charmtone.Salt).
			Background(charmtone.Pepper),
	},
	InsertLine: LineStyle{
		Symbol: lipgloss.NewStyle().
			Foreground(charmtone.Turtle).
			Background(lipgloss.Color("#303a30")),
		Code: lipgloss.NewStyle().
			Foreground(charmtone.Salt).
			Background(lipgloss.Color("#303a30")),
	},
	DeleteLine: LineStyle{
		Symbol: lipgloss.NewStyle().
			Foreground(charmtone.Cherry).
			Background(lipgloss.Color("#3a3030")),
		Code: lipgloss.NewStyle().
			Foreground(charmtone.Salt).
			Background(lipgloss.Color("#3a3030")),
	},
}

// DiffView represents a view for displaying differences between two files.
type DiffView struct {
	layout       layout
	before       file
	after        file
	contextLines int
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
	if err := dv.computeDiff(); err != nil {
		return err.Error()
	}
	dv.detectWidth()

	var b strings.Builder

	for _, h := range dv.unified.Hunks {
		for _, l := range h.Lines {
			content := strings.TrimSuffix(l.Content, "\n")
			width := dv.width - leadingSymbolsSize

			switch l.Kind {
			case udiff.Insert:
				b.WriteString(dv.style.InsertLine.Symbol.Render("+ "))
				b.WriteString(dv.style.InsertLine.Code.Width(width).Render(content))
			case udiff.Delete:
				b.WriteString(dv.style.DeleteLine.Symbol.Render("- "))
				b.WriteString(dv.style.DeleteLine.Code.Width(width).Render(content))
			case udiff.Equal:
				b.WriteString(dv.style.EqualLine.Code.Width(width + leadingSymbolsSize).Render("  " + content))
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
