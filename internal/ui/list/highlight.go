package list

import (
	"image"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/stringext"
	"github.com/charmbracelet/crush/internal/ui/styles"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

// DefaultHighlighter is the default highlighter function that applies inverse style.
var DefaultHighlighter Highlighter = func(x, y int, c *uv.Cell) *uv.Cell {
	if c == nil {
		return c
	}
	c.Style.Attrs |= uv.AttrReverse
	return c
}

// Highlighter represents a function that defines how to highlight text.
type Highlighter func(x, y int, c *uv.Cell) *uv.Cell

// HighlightContent returns the content with highlighted regions based on the specified parameters.
func HighlightContent(content string, area image.Rectangle, startLine, startCol, endLine, endCol int) string {
	return highlightContent(content, area, startLine, startCol, endLine, endCol, false)
}

// HighlightContentMarkdown is [HighlightContent] for content that is
// terminal-rendered markdown (glamour): in addition to the raw text it
// reconstructs the inline markers the renderer dropped, so the clipboard
// holds valid markdown. Bold, italic, and strikethrough runs become **, *,
// and ~~ again, the same way codespan padding becomes backticks.
func HighlightContentMarkdown(content string, area image.Rectangle, startLine, startCol, endLine, endCol int) string {
	return highlightContent(content, area, startLine, startCol, endLine, endCol, true)
}

func highlightContent(content string, area image.Rectangle, startLine, startCol, endLine, endCol int, markdown bool) string {
	content = stringext.NormalizeSpace(content)

	if startLine < 0 || startCol < 0 {
		return ""
	}

	width, height := area.Dx(), area.Dy()
	buf := renderBuffer(content, area, width, height)

	// Treat -1 as "end of content".
	if endLine < 0 {
		endLine = height - 1
	}
	if endCol < 0 {
		endCol = width
	}

	rows := extractRows(buf, startLine, startCol, endLine, endCol, height, markdown)
	return joinRows(rows, width) + "\n"
}

// renderBuffer draws content into a screen buffer of the given dimensions.
func renderBuffer(content string, area image.Rectangle, width, height int) uv.ScreenBuffer {
	buf := uv.NewScreenBuffer(width, height)
	styled := uv.NewStyledString(content)
	styled.Draw(&buf, area)
	return buf
}

// extractedRow holds one extracted screen row: text is the copy text with
// any markdown markers restored, plain the same row without markers. The
// wrap heuristics in joinRows measure plain because restored markers would
// inflate the row width and skew the word-wrap threshold.
type extractedRow struct {
	text  string
	plain string
}

// extractRows extracts the text of the selected region from the buffer,
// one entry per row, trimmed to the last cell holding content.
func extractRows(buf uv.ScreenBuffer, startLine, startCol, endLine, endCol, height int, markdown bool) []extractedRow {
	rows := make([]extractedRow, 0, endLine-startLine+1)
	for y := startLine; y <= endLine && y < height; y++ {
		if y >= buf.Height() {
			break
		}

		line := buf.Line(y)
		colStart := 0
		if y == startLine {
			colStart = min(startCol, len(line))
		}
		colEnd := len(line)
		if y == endLine {
			colEnd = min(endCol, len(line))
		}

		rows = append(rows, extractRow(line, colStart, colEnd, markdown))
	}
	return rows
}

// extractRow returns the text of a single buffer line between colStart and
// colEnd, trimmed to the last cell holding any content (including explicit
// spaces: renderers like glamour pad rows with real space cells, so content
// usually reaches the full width).
//
// Codespan padding cells are converted back into backticks: markdown inline
// code renders blank padding in place of its backticks
// ([styles.CodespanPadding]), and a copy of a selection must reproduce the
// original source text, not the rendered blank padding. The sentinel is a
// no-break space tagged with a variation selector, so a real no-break space
// in the message text does not match it and is copied verbatim.
//
// In markdown mode, runs of bold, italic, and strikethrough cells
// additionally become **, *, and ~~ markers again (see cellEmphasis). A
// space inside an open run is held back until the next non-space cell so a
// trailing space lands outside the closing marker: "**bold **" is not valid
// emphasis, while "**bold** " renders the same as the source.
func extractRow(line uv.Line, colStart, colEnd int, markdown bool) extractedRow {
	lastCellX := -1
	for x := colStart; x < colEnd; x++ {
		cell := line.At(x)
		if cell != nil && cell.Content != "" {
			lastCellX = x
		}
	}

	heading := markdown && isHeadingRow(line)

	var row, plain strings.Builder
	var cur emphasis
	held := 0
	flushHeld := func() {
		for range held {
			row.WriteByte(' ')
			plain.WriteByte(' ')
		}
		held = 0
	}
	for x := colStart; x <= lastCellX; x++ {
		cell := line.At(x)
		if cell == nil || cell.Content == "" {
			continue
		}

		content := cell.Content
		if content == styles.CodespanPadding {
			content = "`"
		}

		e := emphasis(0)
		if markdown && !heading {
			e = cellEmphasis(cell)
		}

		switch {
		case cur != 0 && e == cur && content == " ":
			held++
		case e == cur:
			flushHeld()
			row.WriteString(content)
			plain.WriteString(content)
		default:
			if lost := cur &^ e; lost != 0 {
				row.WriteString(closeMarkers(lost))
			}
			flushHeld()
			if gained := e &^ cur; gained != 0 {
				row.WriteString(openMarkers(gained))
			}
			if content == " " && e != 0 {
				held++
			} else {
				row.WriteString(content)
				plain.WriteString(content)
			}
			cur = e
		}
	}
	if cur != 0 {
		row.WriteString(closeMarkers(cur))
	}
	flushHeld()

	return extractedRow{text: row.String(), plain: plain.String()}
}

// emphasis is a bitmask of the inline markdown emphases a cell displays.
type emphasis uint8

const (
	emphBold emphasis = 1 << iota
	emphItalic
	emphStrikethrough
)

// cellEmphasis maps a cell's display attributes back to the markdown
// emphasis they were rendered from. Cells styled for reasons other than
// markdown emphasis are excluded: hyperlinks (OSC8 link), cells with a
// background (codespan pills, H1 headings), and underlined cells (link
// URLs, images, syntax-highlighted class names in fenced code), none of
// which come from *, **, or ~~ in the source.
func cellEmphasis(cell *uv.Cell) emphasis {
	if cell.Link.URL != "" || cell.Style.Bg != nil || cell.Style.Underline != uv.UnderlineNone {
		return 0
	}
	var e emphasis
	if cell.Style.Attrs&uv.AttrBold != 0 {
		e |= emphBold
	}
	if cell.Style.Attrs&uv.AttrItalic != 0 {
		e |= emphItalic
	}
	if cell.Style.Attrs&uv.AttrStrikethrough != 0 {
		e |= emphStrikethrough
	}
	return e
}

// isHeadingRow reports whether the buffer row is a markdown heading. H2-H6
// render their literal "## " prefix and title all in bold, so without the
// check a copied heading would gain bogus ** markers around its title. H1
// needs no check: it paints a background color, which cellEmphasis already
// rejects.
func isHeadingRow(line uv.Line) bool {
	cell := line.At(0)
	return cell != nil && strings.HasPrefix(cell.Content, "#")
}

// openMarkers returns the markdown markers opening the given emphasis
// flags, bold outermost, so bold-italic text opens as ***.
func openMarkers(e emphasis) string {
	var s strings.Builder
	if e&emphBold != 0 {
		s.WriteString("**")
	}
	if e&emphItalic != 0 {
		s.WriteString("*")
	}
	if e&emphStrikethrough != 0 {
		s.WriteString("~~")
	}
	return s.String()
}

// closeMarkers returns the markdown markers closing the given emphasis
// flags, unwinding openMarkers in reverse, so bold-italic text closes as
// ***.
func closeMarkers(e emphasis) string {
	var s strings.Builder
	if e&emphStrikethrough != 0 {
		s.WriteString("~~")
	}
	if e&emphItalic != 0 {
		s.WriteString("*")
	}
	if e&emphBold != 0 {
		s.WriteString("**")
	}
	return s.String()
}

// joinRows stitches screen rows back into text, deciding per row boundary
// whether it was a real newline or a word wrap. Blank rows are paragraph
// breaks; otherwise isWordWrap decides.
func joinRows(rows []extractedRow, width int) string {
	var sb strings.Builder
	for i, row := range rows {
		text := strings.TrimRight(row.text, " ")
		plain := strings.TrimRight(row.plain, " ")
		sb.WriteString(text)
		if i == len(rows)-1 {
			break
		}

		next := rows[i+1].plain
		switch {
		case strings.TrimSpace(next) == "":
			// Blank row: paragraph break.
			sb.WriteString("\n")
		case isWordWrap(plain, next, width):
			sb.WriteString(" ")
		default:
			sb.WriteString("\n")
		}
	}
	return strings.TrimSpace(sb.String())
}

// isWordWrap reports whether the boundary between the current row and the
// next is a renderer word wrap rather than a real newline.
//
// Renderers like glamour word-wrap paragraphs, filling each wrapped row
// close to the full width; a row whose text reaches past the wrap
// threshold therefore continues on the next row, while a shorter row ends
// a block (a heading, a list item, the last line of a paragraph). The
// threshold is generous because glamour fills wrapped rows to roughly 80%
// or more of the width.
//
// Two overrides apply: a blank next row is a paragraph break (handled by
// the caller), and a next row that starts a new markdown block — a bullet
// or a heading — always begins on its own line, even if the current row is
// full-width (a list item can itself wrap right up to the width before the
// following item).
func isWordWrap(text, next string, width int) bool {
	if startsBlock(next) {
		return false
	}
	return width > 0 && ansi.StringWidth(text) >= width*3/5
}

// startsBlock reports whether a row begins a new markdown block such as a
// list item or heading, rather than continuing a wrapped paragraph.
// Indented rows are continuations of nested content (e.g. the second line
// of a list item), not new blocks.
func startsBlock(row string) bool {
	if row != strings.TrimLeft(row, " ") {
		return false
	}
	switch {
	case strings.HasPrefix(row, "- "), strings.HasPrefix(row, "* "),
		strings.HasPrefix(row, "+ "), strings.HasPrefix(row, "• "),
		strings.HasPrefix(row, "#"):
		return true
	}
	if i := strings.IndexAny(row, ".)"); i > 0 && i < 4 {
		for j := range i {
			if row[j] < '0' || row[j] > '9' {
				return false
			}
		}
		return true
	}
	return false
}

// Highlight highlights a region of text within the given content and region.
func Highlight(content string, area image.Rectangle, startLine, startCol, endLine, endCol int, highlighter Highlighter) string {
	buf := HighlightBuffer(content, area, startLine, startCol, endLine, endCol, highlighter)
	if buf == nil {
		return content
	}
	return buf.Render()
}

// HighlightBuffer highlights a region of text within the given content and
// region, returning a [uv.ScreenBuffer].
func HighlightBuffer(content string, area image.Rectangle, startLine, startCol, endLine, endCol int, highlighter Highlighter) *uv.ScreenBuffer {
	content = stringext.NormalizeSpace(content)

	if startLine < 0 || startCol < 0 {
		return nil
	}

	if highlighter == nil {
		highlighter = DefaultHighlighter
	}

	width, height := area.Dx(), area.Dy()
	buf := uv.NewScreenBuffer(width, height)
	styled := uv.NewStyledString(content)
	styled.Draw(&buf, area)

	// Treat -1 as "end of content"
	if endLine < 0 {
		endLine = height - 1
	}
	if endCol < 0 {
		endCol = width
	}

	for y := startLine; y <= endLine && y < height; y++ {
		if y >= buf.Height() {
			break
		}

		line := buf.Line(y)

		// Determine column range for this line
		colStart := 0
		if y == startLine {
			colStart = min(startCol, len(line))
		}

		colEnd := len(line)
		if y == endLine {
			colEnd = min(endCol, len(line))
		}

		// Track last non-empty position as we go
		lastContentX := -1

		// Single pass: check content and track last non-empty position
		for x := colStart; x < colEnd; x++ {
			cell := line.At(x)
			if cell == nil {
				continue
			}

			// Update last content position if non-empty
			if cell.Content != "" && cell.Content != " " {
				lastContentX = x
			}
		}

		// Only apply highlight up to last content position
		highlightEnd := colEnd
		if lastContentX >= 0 {
			highlightEnd = lastContentX + 1
		} else if lastContentX == -1 {
			highlightEnd = colStart // No content on this line
		}

		// Apply highlight style only to cells with content
		for x := colStart; x < highlightEnd; x++ {
			if !image.Pt(x, y).In(area) {
				continue
			}
			cell := line.At(x)
			if cell != nil {
				highlighter(x, y, cell)
			}
		}
	}

	return &buf
}

// ToHighlighter converts a [lipgloss.Style] to a [Highlighter].
func ToHighlighter(lgStyle lipgloss.Style) Highlighter {
	return func(_ int, _ int, c *uv.Cell) *uv.Cell {
		if c != nil {
			c.Style = ToStyle(lgStyle)
		}
		return c
	}
}

// ToStyle converts an inline [lipgloss.Style] to a [uv.Style].
func ToStyle(lgStyle lipgloss.Style) uv.Style {
	var uvStyle uv.Style

	// Colors are already color.Color
	uvStyle.Fg = lgStyle.GetForeground()
	uvStyle.Bg = lgStyle.GetBackground()

	// Build attributes using bitwise OR
	var attrs uint8

	if lgStyle.GetBold() {
		attrs |= uv.AttrBold
	}

	if lgStyle.GetItalic() {
		attrs |= uv.AttrItalic
	}

	if lgStyle.GetUnderline() {
		uvStyle.Underline = uv.UnderlineSingle
	}

	if lgStyle.GetStrikethrough() {
		attrs |= uv.AttrStrikethrough
	}

	if lgStyle.GetFaint() {
		attrs |= uv.AttrFaint
	}

	if lgStyle.GetBlink() {
		attrs |= uv.AttrBlink
	}

	if lgStyle.GetReverse() {
		attrs |= uv.AttrReverse
	}

	uvStyle.Attrs = attrs

	return uvStyle
}

// AdjustArea adjusts the given area rectangle by subtracting margins, borders,
// and padding from the style.
func AdjustArea(area image.Rectangle, style lipgloss.Style) image.Rectangle {
	topMargin, rightMargin, bottomMargin, leftMargin := style.GetMargin()
	topBorder, rightBorder, bottomBorder, leftBorder := style.GetBorderTopSize(),
		style.GetBorderRightSize(),
		style.GetBorderBottomSize(),
		style.GetBorderLeftSize()
	topPadding, rightPadding, bottomPadding, leftPadding := style.GetPadding()

	return image.Rectangle{
		Min: image.Point{
			X: area.Min.X + leftMargin + leftBorder + leftPadding,
			Y: area.Min.Y + topMargin + topBorder + topPadding,
		},
		Max: image.Point{
			X: area.Max.X - (rightMargin + rightBorder + rightPadding),
			Y: area.Max.Y - (bottomMargin + bottomBorder + bottomPadding),
		},
	}
}
