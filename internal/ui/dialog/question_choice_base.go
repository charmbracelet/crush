package dialog

import (
	"fmt"
	"image"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/question"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

// choiceListMaxWidth is the maximum content width for choice
// question components.
const choiceListMaxWidth = 120

// questionIconPrompt returns the themed question icon based on
// focus state. Shared by all question component types.
func questionIconPrompt(sty *styles.Styles, focused bool) string {
	if focused {
		return sty.Editor.PromptQuestionIconFocused.Render()
	}
	return sty.Editor.PromptQuestionIconBlurred.Render()
}

// choiceList is the shared base for single-choice and multi-choice
// question components. It embeds questionEditor for fill-in, notes,
// and editor handling. Concrete types embed it and only implement
// selection semantics.
// fillInTop is the first content-line index of the fill-in row,
// computed during buildLines. -1 when no fill-in rows exist.
// Used by clampBounds to keep the fill-in visible during wheel
// scrolling without scanning lines on every event.
type choiceList struct {
	questionEditor
	Request question.Question

	cursorIdx        int
	scrollOffset     int // lines scrolled past the top of the viewport
	focused          bool
	lastWidth        int
	choiceCompositor *lipgloss.Compositor
	suppressScroll   bool // skip scroll clamping after mouse click
	wheelActive      bool // wheel-scroll mode: skip cursor snap until next keyboard nav
	hoverX, hoverY   int  // current mouse position for hover highlight
	hoveredChoice    int  // choice index under mouse, or -1
	mouseActive      bool // true when last interaction was mouse (hover mode)

	// Cached layout for wheel-scroll bounds checking.
	lastLines    []contentLine // last rendered lines from drawContent
	lastViewport int           // last viewport height from drawContent
	fillInTop    int           // first fill-in row index, or -1
	fillInBottom int           // last fill-in row index, or -1

	// styleFillInAsSelected controls whether non-empty fill-in text
	// gets the selected (pink) style. True for single-choice where
	// the fill-in IS the answer; false for multi-choice.
	styleFillInAsSelected bool

	keyUp    key.Binding
	keyDown  key.Binding
	keyClose key.Binding
}

// numberKeyIndex returns the zero-based choice index for a number
// key press (1-9), or -1 if the key is not a valid shortcut for
// the current choices.
func (c *choiceList) numberKeyIndex(msg tea.KeyPressMsg) int {
	if len(msg.Text) != 1 {
		return -1
	}
	n, err := strconv.Atoi(msg.Text)
	if err != nil || n < 1 || n > len(c.Request.Choices) {
		return -1
	}
	return n - 1
}

// newChoiceList creates a choiceList with a configured fill-in
// textarea and navigation bindings.
func newChoiceList(sty *styles.Styles, req question.Question) choiceList {
	return choiceList{
		questionEditor: newQuestionEditor(sty),
		Request:        req,
		hoveredChoice:  -1,
		hoverX:         -1,
		hoverY:         -1,
		fillInTop:      -1,
		fillInBottom:   -1,
		keyUp:          key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑", "up")),
		keyDown:        key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓", "down")),
		keyClose:       CloseKey,
	}
}

func (c *choiceList) itemCount() int {
	return len(c.Request.Choices) + 1 // +1 for fill-in
}

func (c *choiceList) isFillIn() bool {
	return c.cursorIdx == len(c.Request.Choices)
}

// moveUp moves the cursor up, wrapping around. Closes any active
// note editor since the note context changes with the cursor.
func (c *choiceList) moveUp() {
	c.wheelActive = false
	if c.mouseActive {
		if c.hoveredChoice >= 0 {
			c.cursorIdx = c.hoveredChoice
		} else {
			c.cursorIdx = 0
		}
	}
	c.mouseActive = false
	c.fillIn.Blur()
	if c.activeNoteKey != "" {
		c.closeNote(c.noteKey())
	}
	c.cursorIdx--
	if c.cursorIdx < 0 {
		c.cursorIdx = c.itemCount() - 1
	}
}

// moveDown moves the cursor down, wrapping around. Closes any
// active note editor since the note context changes with the cursor.
func (c *choiceList) moveDown() {
	c.wheelActive = false
	if c.mouseActive {
		if c.hoveredChoice >= 0 {
			c.cursorIdx = c.hoveredChoice - 1 // will become hoveredChoice after increment
		} else {
			c.cursorIdx = -1 // will become 0 after increment
		}
	}
	c.mouseActive = false
	c.fillIn.Blur()
	if c.activeNoteKey != "" {
		c.closeNote(c.noteKey())
	}
	c.cursorIdx++
	if c.cursorIdx >= c.itemCount() {
		c.cursorIdx = 0
	}
}

// handleFillInKey processes keys when the fill-in textarea is
// focused. Returns (cmd, handled). When handled is true the
// caller should not process the key further.
func (c *choiceList) handleFillInKey(msg tea.KeyPressMsg) (tea.Cmd, bool) {
	switch {
	case key.Matches(msg, c.keyClose):
		c.fillIn.Blur()
		return nil, true
	case key.Matches(msg, c.navUp):
		c.moveUp()
		if c.isFillIn() {
			c.fillIn.Focus()
			return c.fillIn.Focus(), true
		}
		return nil, true
	case key.Matches(msg, c.navDown):
		c.moveDown()
		if c.isFillIn() {
			c.fillIn.Focus()
			return c.fillIn.Focus(), true
		}
		return nil, true
	default:
		c.wheelActive = false
		var cmd tea.Cmd
		c.fillIn, cmd = c.fillIn.Update(msg)
		return cmd, true
	}
}

// handleNavKey processes up/down navigation keys when the
// fill-in is NOT focused. Returns true if the key was consumed.
func (c *choiceList) handleNavKey(msg tea.KeyPressMsg) bool {
	switch {
	case key.Matches(msg, c.keyUp):
		c.moveUp()
		if c.isFillIn() {
			c.fillIn.Focus()
		}
		return true
	case key.Matches(msg, c.keyDown):
		c.moveDown()
		if c.isFillIn() {
			c.fillIn.Focus()
		}
		return true
	}
	return false
}

// noteKey returns the map key for the currently focused item's
// note. Choices use their ID; the question itself uses "_question".
func (c *choiceList) noteKey() string {
	if c.isFillIn() || c.cursorIdx >= len(c.Request.Choices) {
		return "_question"
	}
	return c.Request.Choices[c.cursorIdx].ID
}

// contentLine is one visual row of the choice list. text is the
// pre-rendered, pre-styled string for the row. fillInRow marks the
// first row of the focused fill-in textarea, where the hardware
// cursor is placed. noteRow marks the first row of the focused
// note editor.
type contentLine struct {
	text       string
	fillInRow  bool
	noteRow    bool
	cursorItem bool // belongs to the currently selected item
	choiceIdx  int  // zero-based choice index, or -1 if not a choice row
}

// newContentLine creates a contentLine with choiceIdx initialized
// to -1 so non-choice rows don't accidentally match choice_0.
func newContentLine(text string) contentLine {
	return contentLine{text: text, choiceIdx: -1}
}

// sectionHeight returns the visual line count of a text block
// wrapped at width.
func sectionHeight(text string, width int) int {
	if text == "" {
		return 0
	}
	return strings.Count(ansi.Wrap(text, width, ""), "\n") + 1
}

// wrapIndent wraps text at width and prefixes every continuation
// line with indent so multi-line content aligns under the first
// line's content rather than flush left.
func wrapIndent(text string, width int, indent string) string {
	wrapped := ansi.Wrap(text, width, "")
	lines := strings.Split(wrapped, "\n")
	for i := 1; i < len(lines); i++ {
		lines[i] = indent + lines[i]
	}
	return strings.Join(lines, "\n")
}

// drawStyledText blits an ANSI-styled string into area and returns
// the number of visual lines it occupies.
func drawStyledText(scr uv.Screen, area uv.Rectangle, text string) int {
	if text == "" {
		return 0
	}
	uv.NewStyledString(text).Draw(scr, area)
	return strings.Count(text, "\n") + 1
}

// buildLines renders the entire choice list into a flat slice of
// rows. This is the single source of truth: height is len(lines),
// scrolling is index math over the slice, and drawing blits a
// window of it. itemFn renders a choice's label row(s) as a string.
//
// The final row is always a blank line, giving the list one line of
// bottom padding as real content rather than a phantom offset.
func (c *choiceList) buildLines(innerWidth int, fillInPrefix string, itemFn choiceItemRenderer) []contentLine {
	bodyStyle := c.Styles.Editor.QuestionBody
	barActive := c.Styles.Editor.QuestionCursorBar.Render("┃ ")
	const barInactive = "  "

	var lines []contentLine
	push := func(text string, flags ...bool) {
		cl := newContentLine(text)
		if len(flags) > 0 {
			cl.fillInRow = flags[0]
		}
		if len(flags) > 1 {
			cl.cursorItem = flags[1]
		}
		// Split multi-line strings into one row each.
		for ln := range strings.SplitSeq(text, "\n") {
			row := cl
			row.text = ln
			lines = append(lines, row)
		}
	}

	// Question header + blank separator.
	icon := c.iconPrompt()
	iconWidth := lipgloss.Width(icon)
	qIndent := strings.Repeat(" ", iconWidth)
	push(icon + c.Styles.Editor.QuestionUnselected.Render(wrapIndent(c.Request.Text, innerWidth-iconWidth, qIndent)))
	push("")

	// Optional markdown description + blank separator.
	if c.Request.Description != "" {
		push(c.renderDescription(innerWidth))
		push("")
	}

	// Choices: label row(s), optional wrapped description, note, blank.
	for i, ch := range c.Request.Choices {
		active := i == c.cursorIdx && !c.mouseActive
		hovered := i == c.hoveredChoice && c.mouseActive
		bar := barInactive
		if active || hovered {
			bar = barActive
		}
		content := itemFn(i, ch, active, innerWidth)
		// Prepend bar to every line so continuation lines also
		// show the selection indicator.
		for j, ln := range strings.Split(content, "\n") {
			b := bar
			if j > 0 && !active {
				b = barInactive
			}
			lines = append(lines, contentLine{text: b + ln, cursorItem: active, choiceIdx: i})
		}

		if ch.Description != "" {
			descContent := bodyStyle.Render(wrapIndent(ch.Description, innerWidth-lipgloss.Width(bar), ""))
			for j, ln := range strings.Split(descContent, "\n") {
				b := bar
				if j > 0 && !active {
					b = barInactive
				}
				lines = append(lines, contentLine{text: b + ln, cursorItem: active, choiceIdx: i})
			}
		}

		// Inline note editor or saved note for this choice.
		c.drawNote(&lines, innerWidth, bar, barInactive, ch.ID, active)

		// Blank separator — tag with current choice index so it's
		// part of the clickable/hoverable zone.
		lines = append(lines, contentLine{text: "", choiceIdx: i})
	}

	// Fill-in: live textarea when focused, otherwise placeholder.
	// Show active gutter only when focused or has content.
	hasFillInText := strings.TrimSpace(c.fillIn.Value()) != ""
	fillActive := c.isFillIn() && (c.fillIn.Focused() || hasFillInText)
	fillPrefix := c.Styles.Editor.QuestionBody.Render("> ")
	if c.styleFillInAsSelected && hasFillInText {
		fillPrefix = c.Styles.Editor.QuestionSelected.Render("> ")
	}
	fillBar := barInactive
	if fillActive {
		fillBar = barActive
	}
	linesBeforeFillIn := len(lines)
	c.drawFillIn(&lines, innerWidth, fillBar, barInactive, fillPrefix, c.isFillIn(), false)

	// Record fill-in row range for wheel-scroll bounds checking.
	c.fillInTop = -1
	c.fillInBottom = -1
	for i := linesBeforeFillIn; i < len(lines); i++ {
		if c.fillInTop < 0 {
			c.fillInTop = i
		}
		c.fillInBottom = i
	}

	// Tag fill-in rows with the fill-in item index so clicks can
	// navigate to it.
	fillInIdx := len(c.Request.Choices)
	for i := linesBeforeFillIn; i < len(lines); i++ {
		lines[i].choiceIdx = fillInIdx
	}

	// Trailing blank line for bottom padding.
	push("")

	return lines
}

// renderDescription renders the markdown description at width.
func (c *choiceList) renderDescription(width int) string {
	r := common.MarkdownRenderer(c.Styles, width)
	mu := common.LockMarkdownRenderer(r)
	mu.Lock()
	out, err := r.Render(c.Request.Description)
	mu.Unlock()
	if err != nil {
		return c.Request.Description
	}
	return strings.TrimSuffix(out, "\n")
}

// choiceItemRenderer renders a choice's label content as a string.
// The bar prefix is applied by buildLines so that continuation
// lines also receive it. innerWidth is the available content
// width for this particular render pass (may differ between
// overflow-test and final render).
type choiceItemRenderer func(index int, choice question.Choice, active bool, innerWidth int) string

// height returns the total visual height at the given width. It is
// len(buildLines), the single source of truth for layout, and a
// pure function of the width passed in so it always agrees with
// the width drawContent will use this frame.
func (c *choiceList) height(width int) int {
	if width <= 0 {
		width = c.lastWidth
	}
	innerWidth := min(width-4, choiceListMaxWidth)
	return len(c.buildLines(innerWidth, "> ", func(int, question.Choice, bool, int) string {
		return "x" // single-line placeholder; only count matters
	}))
}

func (c *choiceList) heightChanged() bool {
	return false // height is deterministic
}

func (c *choiceList) setFocused(focused bool) {
	c.focused = focused
}

// setHover updates the hover position and resolves which choice
// is under the cursor using the compositor.
func (c *choiceList) setHover(x, y int) {
	c.hoverX = x
	c.hoverY = y
	c.mouseActive = true
	c.hoveredChoice = -1
	if c.choiceCompositor == nil {
		return
	}
	hit := c.choiceCompositor.Hit(x, y)
	if !hit.Empty() {
		var idx int
		if _, err := fmt.Sscanf(hit.ID(), "choice_%d", &idx); err == nil {
			c.hoveredChoice = idx
		}
	}
}

// iconPrompt returns the themed question icon based on focus.
func (c *choiceList) iconPrompt() string {
	return questionIconPrompt(c.Styles, c.focused)
}

// drawContent renders the choice list with scroll support. It
// builds the full line list, clamps the scroll offset to keep the
// cursor visible, then blits the visible window. Returns the
// hardware cursor position, or nil.
func (c *choiceList) drawContent(scr uv.Screen, area uv.Rectangle, fillInPrefix string, itemFn choiceItemRenderer) *tea.Cursor {
	c.lastWidth = area.Dx()
	viewport := area.Dy()

	// Build lines at the wide width first (matching height(),
	// which the form uses to size our viewport). Only reserve a
	// scrollbar column and rebuild narrow if the wide layout
	// actually overflows. Testing narrow first would over-report
	// overflow by one line at boundary widths, causing the
	// scrollbar to flicker during horizontal resize.
	contentWidth := area.Dx()
	innerNarrow := min(contentWidth-1-4, choiceListMaxWidth)
	innerWide := min(contentWidth-4, choiceListMaxWidth)

	lines := c.buildLines(innerWide, fillInPrefix, itemFn)
	overflow := viewport > 0 && len(lines) > viewport
	if overflow && innerNarrow != innerWide {
		lines = c.buildLines(innerNarrow, fillInPrefix, itemFn)
	}

	if overflow {
		contentWidth--
	}
	c.lastLines = lines
	c.lastViewport = viewport
	c.clampScroll(lines, viewport)

	// Blit the visible window.
	var cur *tea.Cursor
	for screenRow := range viewport {
		idx := c.scrollOffset + screenRow
		if idx >= len(lines) {
			break
		}
		ln := lines[idx]
		y := area.Min.Y + screenRow
		if ln.text != "" {
			uv.NewStyledString(ln.text).Draw(scr, image.Rect(area.Min.X, y, area.Min.X+contentWidth, y+1))
		}
		if ln.fillInRow {
			fillPrefix := c.Styles.Editor.QuestionBody.Render("> ")
			if tc := c.fillInCursor(screenRow, area.Min.X, lipgloss.Width(fillPrefix)); tc != nil {
				cur = tc
			}
		}
		if ln.noteRow {
			const notePrefix = "> "
			if tc := c.noteCursor(screenRow, area.Min.X, lipgloss.Width(notePrefix)); tc != nil {
				cur = tc
			}
		}
	}

	// Clamp cursor to visible area to prevent overflow.
	if cur != nil {
		if cur.Y < 0 {
			cur.Y = 0
		} else if cur.Y >= viewport {
			cur.Y = viewport - 1
		}
		if cur.X < 0 {
			cur.X = 0
		} else if cur.X >= area.Dx() {
			cur.X = area.Dx() - 1
		}
	}

	// Scrollbar.
	if overflow {
		sb := common.Scrollbar(c.Styles, viewport, len(lines), viewport, c.scrollOffset)
		if sb != "" {
			x := area.Max.X - 1
			uv.NewStyledString(sb).Draw(scr, image.Rect(x, area.Min.Y, x+1, area.Min.Y+viewport))
		}
	}

	// Build hit layers for choice rows.
	c.buildChoiceCompositor(lines, area, contentWidth)

	return cur
}

// buildChoiceCompositor creates hit layers for each visible choice
// row so that mouse clicks can select choices directly. Each choice
// gets a single layer spanning all its visible rows.
func (c *choiceList) buildChoiceCompositor(lines []contentLine, area uv.Rectangle, contentWidth int) {
	// Collect the screen-row range for each choice index.
	type rowRange struct{ min, max int }
	ranges := make(map[int]*rowRange)
	for screenRow := range area.Dy() {
		idx := c.scrollOffset + screenRow
		if idx >= len(lines) {
			break
		}
		ln := lines[idx]
		if ln.choiceIdx < 0 {
			continue
		}
		r, ok := ranges[ln.choiceIdx]
		if !ok {
			r = &rowRange{min: screenRow, max: screenRow}
			ranges[ln.choiceIdx] = r
		} else {
			if screenRow < r.min {
				r.min = screenRow
			}
			if screenRow > r.max {
				r.max = screenRow
			}
		}
	}

	var layers []*lipgloss.Layer
	for choiceIdx, r := range ranges {
		height := r.max - r.min + 1
		hitStr := strings.Repeat(strings.Repeat(" ", contentWidth)+"\n", height-1) + strings.Repeat(" ", contentWidth)
		y := area.Min.Y + r.min
		layers = append(layers, lipgloss.NewLayer(hitStr).X(area.Min.X).Y(y).ID(fmt.Sprintf("choice_%d", choiceIdx)))
	}
	if len(layers) > 0 {
		c.choiceCompositor = lipgloss.NewCompositor(layers...)
	} else {
		c.choiceCompositor = nil
	}
}

// clampScroll keeps the cursor item visible using a sliding
// window: the cursor moves freely within the visible region and
// only pushes the window when it reaches an edge. Going down pushes
// the bottom; going up pushes the top until the start (header and
// HandleWheel scrolls the choice list vertically. Satisfies
// WheelScrollableEditor so the form can delegate wheel events.
func (c *choiceList) HandleWheel(deltaX, deltaY float64) {
	if deltaY == 0 {
		return
	}
	c.scrollOffset += int(deltaY)
	c.wheelActive = true
	c.clampToBounds(c.lastLines, c.lastViewport)
}

// clampToBounds enforces scroll bounds [0, max] and keeps the
// fill-in at least partially visible when active. It does NOT
// snap to the cursor, making it safe for wheel-driven scrolling.
func (c *choiceList) clampToBounds(lines []contentLine, viewport int) {
	limit := max(0, len(lines)-viewport)
	c.scrollOffset = min(max(0, c.scrollOffset), limit)
	// When fill-in is active, ensure at least one fill-in line
	// remains visible in the viewport.
	if c.isFillIn() && c.fillInTop >= 0 {
		fillInBottom := c.fillInBottom
		if fillInBottom < 0 {
			fillInBottom = c.fillInTop
		}
		// If scrolled past the fill-in entirely, pull back so
		// the last fill-in line is at the bottom of the viewport.
		if c.scrollOffset > fillInBottom {
			c.scrollOffset = max(0, fillInBottom-viewport+1)
		}
		// If scrolled above the fill-in start while fill-in is
		// taller than the viewport, pin to fill-in start.
		if c.scrollOffset+viewport <= c.fillInTop && fillInBottom-c.fillInTop+1 >= viewport {
			c.scrollOffset = c.fillInTop
		}
	}
}

// clampScroll keeps the cursor item visible using a sliding
// window: the cursor moves freely within the visible region and
// only pushes the window when it reaches an edge. Going down pushes
// the bottom; going up pushes the top until the start (header and
// description) comes back into view.
func (c *choiceList) clampScroll(lines []contentLine, viewport int) {
	if c.suppressScroll {
		c.suppressScroll = false
		return
	}
	// After wheel scroll, only enforce bounds instead of snapping
	// to cursor. Wheel mode persists until the next keyboard nav.
	if c.wheelActive {
		c.clampToBounds(lines, viewport)
		return
	}
	limit := max(0, len(lines)-viewport)
	if limit == 0 {
		c.scrollOffset = 0
		return
	}

	// Row range of the cursor item.
	cursorTop, cursorBottom := -1, -1
	for i, ln := range lines {
		if ln.cursorItem {
			if cursorTop < 0 {
				cursorTop = i
			}
			cursorBottom = i
		}
	}
	// When fill-in is focused, narrow the cursor target to the
	// specific textarea cursor line so typing auto-scrolls.
	if c.isFillIn() && c.fillIn.Focused() && cursorTop >= 0 {
		if tc := c.fillIn.Cursor(); tc != nil {
			targetLine := cursorTop + tc.Y
			if targetLine >= cursorTop && targetLine <= cursorBottom {
				cursorTop = targetLine
				cursorBottom = targetLine
			}
		}
	}
	if cursorTop < 0 {
		c.scrollOffset = min(max(0, c.scrollOffset), limit)
		return
	}

	// Keep one line below the cursor visible (trailing pad on the
	// last item, a separator otherwise) so the selection is never
	// flush against the bottom edge.
	below := min(cursorBottom+1, len(lines)-1)

	// On the first selectable item, prefer the top so the header
	// and description (nothing selectable sits above them) come
	// into view.
	if c.cursorIdx == 0 {
		c.scrollOffset = 0
	}
	// Push the window down if the cursor's bottom fell below it.
	if below >= c.scrollOffset+viewport {
		c.scrollOffset = below - viewport + 1
	}
	// Push the window up if the cursor's top rose above it.
	if cursorTop < c.scrollOffset {
		c.scrollOffset = cursorTop
	}

	c.scrollOffset = min(max(0, c.scrollOffset), limit)
}

// handleFillInFocused processes keys when the fill-in textarea is
// focused. onClose is called for the close key, onDone for the
// done key. Returns (done, cmd, handled). When handled is false
// the caller should process the key itself.
func (c *choiceList) handleFillInFocused(
	msg tea.KeyPressMsg,
	doneKey key.Binding,
	onClose func() (bool, tea.Cmd),
	onDone func() (bool, tea.Cmd),
) (bool, tea.Cmd, bool) {
	if !c.isFillIn() || !c.fillIn.Focused() {
		return false, nil, false
	}
	if key.Matches(msg, c.keyClose) {
		done, cmd := onClose()
		return done, cmd, true
	}
	if key.Matches(msg, doneKey) {
		done, cmd := onDone()
		return done, cmd, true
	}
	cmd, handled := c.handleFillInKey(msg)
	return false, cmd, handled
}
