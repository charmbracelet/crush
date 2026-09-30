package list

import (
	"strings"

	uv "github.com/charmbracelet/ultraviolet"

	"github.com/charmbracelet/crush/internal/ui/styles"
)

// This file reconstructs raw markdown from rendered cells so that a
// selection copy (see [HighlightContent]) reproduces the source text
// instead of the rendered output. The mechanism mirrors the inline-code
// sentinel: the markdown style config (internal/ui/styles) renders
// invisible blank sentinel cells where raw syntax markers would sit, and
// the extractor here maps them back. Elements that carry their raw form
// in the render itself (headings H2-H6, ordered lists) or in cell
// metadata (hyperlink URLs on OSC 8 cells) are restored without
// sentinels.
//
// Known limitations, all accepted: emphasis markers normalize to the
// canonical * / ** forms and cannot be distinguished from escaped
// literals; copies are built from the selected cells, so a selection
// that starts inside a marker or a link yields a partial reconstruction;
// links and images spanning a line wrap copy as their rendered text;
// table alignment, column escaping, and cells containing a literal │ are
// not preserved; the code fence language is not recoverable.

// rowKind classifies an extracted row for table assembly.
type rowKind uint8

const (
	rowNormal rowKind = iota
	// rowTablePipe is a rendered table content row (contains column
	// separators).
	rowTablePipe
	// rowTableBorder is a rendered table border row (box-drawing only).
	rowTableBorder
)

type markdownRow struct {
	text string
	kind rowKind
}

// Box-drawing runes a rendered table is built from.
const boxRunes = "─│┼┌┐└┘├┤┬┴╭╮╰╯═║╔╗╚╝╠╣╦╩╬"

// Bare base runes of the whitespace-based sentinels (styles.SentinelFence,
// styles.SentinelStrike, styles.SentinelTask). Word wrap strips the
// variation selector from whitespace runs, so these are matched with or
// without it. They are safe to match bare because these code points are
// essentially never found in real message text.
const (
	fenceRune  = "\u2007"
	strikeRune = "\u2005"
	taskRune   = "\u2004"
)

// sentinelBase strips the text-presentation variation selector from a
// sentinel cell content, yielding the bare base rune. Only sentinels
// whose base can never appear in real text may be matched bare; the
// emphasis, strong, and codespan sentinels must always be matched with
// the selector included (see styles.SentinelEmph).
func sentinelBase(content string) string {
	return strings.TrimSuffix(content, "\ufe0e")
}

// cell is a trimmed-down view of a buffer cell for the row scan.
type cell struct {
	content string
	link    string
	bold    bool
	italic  bool
}

// rawExtractor carries markdown state across the rows of one
// [HighlightContent] call.
type rawExtractor struct {
	// inCode tracks whether the scan is currently inside a fenced code
	// block, entered and left via the fence sentinels.
	inCode bool

	// openMarkers tracks the emphasis/strong sentinel pairs opened but
	// not yet closed, across rows (emphasis can wrap). Each entry
	// records which sentinel content opened it and the marker its pair
	// emits, so the closing sentinel mirrors the opening one.
	openMarkers []openMarker
}

// openMarker is an emphasis or strong sentinel pair opened but not yet
// closed. emph records which sentinel content opened it.
type openMarker struct {
	emph   bool
	marker string
}

// isEmphasisSentinel reports whether the cell content is one of the
// emphasis/strong sentinel graphemes.
func isEmphasisSentinel(c cell) bool {
	return c.content == styles.SentinelEmph || c.content == styles.SentinelStrong
}

// writeEmphasisAt handles an emphasis or strong sentinel cell at
// cells[i] and returns the number of cells consumed (always 1).
//
// Glamour renders one sentinel pair per text run. For nested emphasis
// the outer element's sentinel wins every run's cell content, so a
// construct like "**a *b* c**" renders three identical-looking strong
// pairs, and the raw markers must be recovered from the cell attributes
// and the pair adjacency:
//
//   - A pair whose cells also carry the other element's attribute is a
//     boundary of that inner element. If it stands alone it is a
//     standalone bold-italic run and emits both markers folded (***);
//     if it is adjacent to another pair of the same content it is an
//     interior boundary and emits only the inner marker.
//   - A pair without the other element's attribute that is adjacent to
//     another pair of the same content is the outer element's marker
//     re-emitted mid-construct and emits nothing; the outer element's
//     real markers come from the first and last pairs.
func (ex *rawExtractor) writeEmphasisAt(b *rowsBuilder, cells []cell, i int) int {
	c := cells[i]
	emphSentinel := c.content == styles.SentinelEmph
	otherAttr := c.italic
	if emphSentinel {
		otherAttr = c.bold
	}
	adjacent := (i > 0 && isEmphasisSentinel(cells[i-1])) ||
		(i+1 < len(cells) && isEmphasisSentinel(cells[i+1]))

	var (
		marker   string
		pairEmph bool
	)
	switch {
	case otherAttr && adjacent:
		// Interior boundary of the inner element: the cell content is
		// the outer element's sentinel, so the inner element is the
		// other one.
		if emphSentinel {
			marker, pairEmph = "**", false
		} else {
			marker, pairEmph = "*", true
		}
	case otherAttr:
		// Standalone run wrapped by both elements.
		marker = "***"
		pairEmph = emphSentinel
	case adjacent:
		// The outer element's marker re-emitted mid-construct.
		return 1
	default:
		marker, pairEmph = "**", false
		if emphSentinel {
			marker, pairEmph = "*", true
		}
	}
	ex.emitPairedMarker(b, pairEmph, marker)
	return 1
}

// emitPairedMarker writes marker, pairing it with the matching open
// marker of the same element if one is open (the close mirrors the
// marker the open emitted).
func (ex *rawExtractor) emitPairedMarker(b *rowsBuilder, emph bool, marker string) {
	if n := len(ex.openMarkers); n > 0 && ex.openMarkers[n-1].emph == emph {
		b.WriteString(ex.openMarkers[n-1].marker)
		ex.openMarkers = ex.openMarkers[:n-1]
		return
	}
	ex.openMarkers = append(ex.openMarkers, openMarker{emph: emph, marker: marker})
	b.WriteString(marker)
}

// row extracts one buffer line into raw markdown text and classifies it.
func (ex *rawExtractor) row(line uv.Line, colStart, colEnd int) markdownRow {
	startedInCode := ex.inCode

	lastCellX := -1
	for x := colStart; x < colEnd; x++ {
		c := line.At(x)
		if c != nil && c.Content != "" {
			lastCellX = x
		}
	}
	if lastCellX < colStart {
		return markdownRow{kind: classifyRow("", !startedInCode)}
	}

	cells := make([]cell, 0, lastCellX+1-colStart)
	for x := colStart; x <= lastCellX; x++ {
		c := line.At(x)
		if c == nil || c.Content == "" {
			continue
		}
		cells = append(cells, cell{
			content: c.Content,
			link:    c.Link.URL,
			bold:    c.Style.Attrs&uv.AttrBold != 0,
			italic:  c.Style.Attrs&uv.AttrItalic != 0,
		})
	}

	text := ex.scanRow(cells)
	if !startedInCode {
		text = restoreLineStart(text)
		text = sanitizeSentinels(text)
		if isRuleRow(text) {
			text = "---"
		}
	}

	return markdownRow{text: text, kind: classifyRow(text, !startedInCode)}
}

// scanRow walks one row's cells and rebuilds its raw markdown text.
func (ex *rawExtractor) scanRow(cells []cell) string {
	var b rowsBuilder

	for i := 0; i < len(cells); {
		c := cells[i]

		if c.link != "" {
			i += ex.writeHyperlink(&b, cells, i)
			continue
		}

		switch {
		case c.content == styles.CodespanPadding:
			b.WriteString("`")
		case c.content == styles.SentinelStrong, c.content == styles.SentinelEmph:
			ex.writeEmphasisAt(&b, cells, i)
		case sentinelBase(c.content) == strikeRune:
			// Strikethrough sentinel: a blank cell rendered with the
			// strikethrough style.
			b.WriteString("~~")
		case sentinelBase(c.content) == fenceRune:
			ex.writeFence(&b)
		default:
			b.WriteString(c.content)
		}
		i++
	}
	return b.String()
}

// writeHyperlink reconstructs one hyperlink element starting at cells[i]
// and returns the number of cells consumed.
//
// Glamour renders "[text](url)" as the link text, then a blank untagged
// cell, then the URL itself, all inside one OSC 8 region, so the URL is
// available twice: rendered in cells and on their Link metadata. The
// rendered copy is dropped and the target rebuilt from the metadata.
// Images render as the untagged "Image: " label, the tagged alt text,
// an untagged "→", and then the tagged URL, and are rebuilt as
// "![alt](url)".
func (ex *rawExtractor) writeHyperlink(b *rowsBuilder, cells []cell, i int) int {
	url := cells[i].link

	var (
		text       strings.Builder // cells before the rendered URL
		shown      strings.Builder // cells rendering the URL itself
		gap        bool
		imageArrow bool
	)
	j := i
	for ; j < len(cells); j++ {
		c := cells[j]
		if c.link == url {
			if gap {
				shown.WriteString(c.content)
			} else {
				text.WriteString(c.content)
			}
			continue
		}
		if strings.TrimSpace(c.content) == "" {
			if shown.Len() > 0 {
				// Blanks after the rendered URL belong to the surrounding
				// text, not to the element.
				break
			}
			if text.Len() > 0 {
				if text.String() == url {
					// The run already rendered the full URL (an autolink,
					// or a selection that starts inside the URL): further
					// blanks are trailing text.
					break
				}
				gap = true
			}
			continue
		}
		if gap && c.content == "→" && strings.HasSuffix(b.String(), "Image: ") {
			// The image label's arrow: part of the element, skipped.
			imageArrow = true
			continue
		}
		break
	}

	consumed := j - i
	textStr := text.String()

	switch {
	case imageArrow && textStr != "":
		// Drop the "Image: " label written before the alt text and emit
		// the raw image syntax.
		b.trimSuffix("Image: ")
		b.WriteString("![" + textStr + "](" + url + ")")
	case textStr == url:
		// Autolink, or a selection that starts at the rendered URL: the
		// URL itself is the raw form.
		b.WriteString(url)
	case textStr == "":
		// Nothing but the rendered URL part was selected.
		b.WriteString(url)
	case gap:
		b.WriteString("[" + textStr + "](" + url + ")")
	default:
		// Unrecognized run (for example a table footer link): fall back
		// to the rendered text.
		b.WriteString(textStr)
		if shown.Len() > 0 {
			b.WriteString(" ")
			b.WriteString(shown.String())
		}
	}
	return consumed
}

// writeFence handles a fence sentinel cell. The opening sentinel starts
// the first code row, so any blank margin cells collected before it are
// dropped and the fence is emitted on its own line. The closing sentinel
// sits after the block's trailing newline (on a row of its own) or,
// in fallback renderings without a syntax highlighter, at the end of the
// last code row.
func (ex *rawExtractor) writeFence(b *rowsBuilder) {
	opening := !ex.inCode
	ex.inCode = opening
	if opening {
		if b.isBlank() {
			b.Reset()
			b.WriteString("```\n")
		} else {
			b.WriteString("\n```\n")
		}
		return
	}
	if b.isBlank() {
		b.Reset()
		b.WriteString("```")
	} else {
		b.WriteString("\n```")
	}
}

// rowsBuilder accumulates one row's text. It tracks whether any non-blank
// content has been collected and can retract a trailing suffix, which the
// image reconstruction needs for the "Image: " label.
type rowsBuilder struct {
	sb       strings.Builder
	nonBlank bool
}

func (b *rowsBuilder) WriteString(s string) {
	if strings.TrimSpace(s) != "" {
		b.nonBlank = true
	}
	b.sb.WriteString(s)
}

func (b *rowsBuilder) String() string { return b.sb.String() }

func (b *rowsBuilder) isBlank() bool { return !b.nonBlank }

func (b *rowsBuilder) Reset() {
	b.sb.Reset()
	b.nonBlank = false
}

func (b *rowsBuilder) trimSuffix(suffix string) {
	s := b.sb.String()
	if strings.HasSuffix(s, suffix) {
		b.sb.Reset()
		b.sb.WriteString(strings.TrimSuffix(s, suffix))
	}
}

// restoreLineStart rewrites line-start display markers back to their raw
// markdown form: bullets, blockquote bars, and task checkboxes. Task
// checkboxes carry a sentinel blank (styles.SentinelTask) so a plain
// text checklist like "[ ] fix bug" is never mistaken for a rendered
// task.
func restoreLineStart(text string) string {
	// Blockquote bars are written by glamour's indent writer at column
	// 0, never after leading spaces. Table cell continuation rows also
	// start with cell padding before their │ bar, so requiring column 0
	// here keeps those bars intact for the table reconstruction.
	var quotePrefix strings.Builder
	for strings.HasPrefix(text, "│ ") {
		quotePrefix.WriteString("> ")
		text = text[len("│ "):]
	}
	i := strings.IndexFunc(text, func(r rune) bool { return r != ' ' })
	if i < 0 {
		return quotePrefix.String() + text
	}
	prefix, rest := quotePrefix.String()+text[:i], text[i:]
	for changed := true; changed; {
		changed = false
		switch {
		case strings.HasPrefix(rest, "• "):
			prefix += "- "
			rest = rest[len("• "):]
			changed = true
		default:
			if r, ok := cutTaskCheckbox(rest, "[✓]"); ok {
				prefix += "- [x] "
				rest = r
				changed = true
			} else if r, ok := cutTaskCheckbox(rest, "[ ]"); ok {
				prefix += "- [ ] "
				rest = r
				changed = true
			}
		}
	}
	return prefix + rest
}

// cutTaskCheckbox strips a task checkbox (box plus sentinel blank) from
// the start of rest, reporting whether it matched.
func cutTaskCheckbox(rest, box string) (string, bool) {
	for _, sentinel := range []string{taskRune, styles.SentinelTask} {
		if p := box + sentinel; strings.HasPrefix(rest, p) {
			return rest[len(p):], true
		}
	}
	return rest, false
}

// sanitizeSentinels replaces any sentinel blanks that survived without
// their expected context with plain spaces, so no invisible runes leak
// into a copy.
func sanitizeSentinels(text string) string {
	if !strings.ContainsAny(text, "\u2004\u2005\u2007") {
		return text
	}
	text = strings.ReplaceAll(text, "\u2004", " ")
	text = strings.ReplaceAll(text, "\u2005", " ")
	return strings.ReplaceAll(text, "\u2007", " ")
}

// isRuleRow reports whether the row renders a horizontal rule: only
// dashes after any restored blockquote markers. A row like this cannot
// come from source text outside a code block, because goldmark turns
// dash rows into rules or setext headings before rendering.
func isRuleRow(text string) bool {
	for strings.HasPrefix(text, "> ") {
		text = text[len("> "):]
	}
	text = strings.TrimRight(text, " ")
	if len(text) < 3 {
		return false
	}
	return strings.Trim(text, "-") == ""
}

// classifyRow determines the table role of an extracted row. Rows inside
// a code block are always normal: code may legitimately contain │ or
// dash runs.
func classifyRow(text string, outsideCode bool) rowKind {
	if !outsideCode {
		return rowNormal
	}
	if isBoxRow(text) {
		return rowTableBorder
	}
	if strings.Contains(text, "│") {
		return rowTablePipe
	}
	return rowNormal
}

// isBoxRow reports whether the row consists solely of box-drawing runes
// and blanks.
func isBoxRow(text string) bool {
	seenBox := false
	for _, r := range text {
		if r == ' ' {
			continue
		}
		if !strings.ContainsRune(boxRunes, r) {
			return false
		}
		seenBox = true
	}
	return seenBox
}

// assembleMarkdownRows rebuilds raw pipe-table syntax from rendered
// table rows. A table is recognized structurally: content rows
// containing the │ column separator next to table structure — an
// all-box-drawing border row or another pipe row, so a selection that
// starts mid-table keeps reconstructing pipe syntax. Border rows are
// dropped, cells are recovered by splitting on │, wrapped-cell
// continuation rows are folded back into the cell they continue, and
// the header separator row is synthesized after the first reconstructed
// row. Rows of plain text that merely contain │ with no neighboring
// table structure pass through untouched.
func assembleMarkdownRows(rows []markdownRow) []string {
	out := make([]string, 0, len(rows))
	var table [][]string
	inTable := false

	isBorder := func(i int) bool {
		return i >= 0 && i < len(rows) && rows[i].kind == rowTableBorder
	}
	// isTableNeighbor reports whether the row at i is table structure a
	// pipe row can latch onto: a border row, or another pipe row (which
	// also lets a selection that starts mid-table keep reconstructing
	// pipe syntax, since the data rows chain together).
	isTableNeighbor := func(i int) bool {
		return isBorder(i) || (i >= 0 && i < len(rows) && rows[i].kind == rowTablePipe)
	}
	flush := func() {
		if len(table) == 0 {
			return
		}
		cols := max(len(table[0]), 1)
		out = append(out, pipeCells(table[0]), "|"+strings.Repeat("---|", cols))
		for _, row := range table[1:] {
			out = append(out, pipeCells(row))
		}
		table = nil
		inTable = false
	}

	for i, r := range rows {
		switch {
		case inTable:
			switch r.kind {
			case rowTableBorder:
				// Border row: structure only, dropped from the copy.
			case rowTablePipe:
				table = append(table, mergeContinuation(table, splitPipeCells(r.text)))
			default:
				flush()
				out = append(out, r.text)
			}
		case r.kind == rowTablePipe && (isTableNeighbor(i+1) || isTableNeighbor(i-1)):
			inTable = true
			table = append(table, splitPipeCells(r.text))
		default:
			out = append(out, r.text)
		}
	}
	flush()
	return out
}

// splitPipeCells splits one rendered table content row into its cell
// texts, trimmed of the cell padding.
func splitPipeCells(text string) []string {
	parts := strings.Split(text, "│")
	cells := make([]string, len(parts))
	for i, part := range parts {
		cells[i] = strings.TrimSpace(part)
	}
	return cells
}

// mergeContinuation folds a wrapped-cell continuation row into the row
// it continues. A wrapped cell renders its continuation on its own row
// with every other cell blank, so a row with exactly one non-blank cell
// is folded into that cell of the previous row. A genuinely empty cell
// in a real row matches the same shape and folds too; blank cells are
// rare next to wrapped ones.
func mergeContinuation(table [][]string, cells []string) []string {
	if len(table) == 0 {
		return cells
	}
	prev := table[len(table)-1]
	nonBlank := -1
	for i, c := range cells {
		if c == "" {
			continue
		}
		if nonBlank >= 0 {
			return cells // more than one cell carries text: a new row
		}
		nonBlank = i
	}
	if nonBlank < 0 || nonBlank >= len(prev) {
		return cells
	}
	if prev[nonBlank] == "" {
		return cells // continuation of an empty cell is a new row
	}
	prev[nonBlank] += " " + cells[nonBlank]
	return prev
}

// pipeCells formats one table row's cells as a raw pipe row.
func pipeCells(cells []string) string {
	return "| " + strings.Join(cells, " | ") + " |"
}
