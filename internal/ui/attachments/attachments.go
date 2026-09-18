package attachments

import (
	"fmt"
	"path/filepath"
	"slices"
	"strconv"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
)

const maxFilename = 15

type Keymap struct {
	DeleteMode,
	DeleteAll,
	Escape key.Binding
}

func New(renderer *Renderer, keyMap Keymap) *Attachments {
	return &Attachments{
		keyMap:   keyMap,
		renderer: renderer,
	}
}

type Attachments struct {
	renderer *Renderer
	keyMap   Keymap
	list     []message.Attachment
	deleting bool
}

func (m *Attachments) List() []message.Attachment { return m.list }
func (m *Attachments) Reset()                     { m.list = nil }

func (m *Attachments) Update(msg tea.Msg) bool {
	switch msg := msg.(type) {
	case message.Attachment:
		m.list = append(m.list, msg)
		return true
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.keyMap.DeleteMode):
			if len(m.list) > 0 {
				m.deleting = true
			}
			return true
		case m.deleting && key.Matches(msg, m.keyMap.Escape):
			m.deleting = false
			return true
		case m.deleting && key.Matches(msg, m.keyMap.DeleteAll):
			m.deleting = false
			m.list = nil
			return true
		case m.deleting:
			// Handle digit keys for individual attachment deletion.
			r := msg.Code
			if r >= '0' && r <= '9' {
				num := int(r - '0')
				if num < len(m.list) {
					m.list = slices.Delete(m.list, num, num+1)
				}
				m.deleting = false
			}
			return true
		}
	}
	return false
}

// HandleClick processes a mouse click at the given x offset within the
// attachment row. If the click lands on a remove button, the
// corresponding attachment is removed. It returns true if the click was
// handled.
func (m *Attachments) HandleClick(x int) bool {
	if m.deleting || len(m.list) == 0 {
		return false
	}
	idx := m.renderer.HitTestRemove(m.list, x)
	if idx >= 0 && idx < len(m.list) {
		m.list = slices.Delete(m.list, idx, idx+1)
		return true
	}
	return false
}

func (m *Attachments) Render(width int) string {
	// The editor is interactive, so the remove button is shown.
	return m.renderer.Render(m.list, m.deleting, true, width)
}

// SetStyles updates the chip styles used when rendering.
func (m *Attachments) SetStyles(s styles.AttachmentStyles) { m.renderer.SetStyles(s) }

func NewRenderer(s styles.AttachmentStyles) *Renderer {
	return &Renderer{styles: s}
}

// SetStyles updates the renderer styles in place.
func (r *Renderer) SetStyles(s styles.AttachmentStyles) { r.styles = s }

type Renderer struct {
	styles styles.AttachmentStyles
	// bounds stores the X-coordinate ranges of each chip's remove
	// button from the most recent Render call, for mouse hit-testing.
	bounds []chipBounds
}

// chipBounds holds the rendered strings and the X-coordinate range of
// each chip's remove button for hit-testing.
type chipBounds struct {
	startX    int
	removeEnd int // exclusive end X of the remove button (0 if none)
}

// Render renders the attachment chips. Each chip shows an icon and a
// filename; when showRemove is true a remove button (✕) follows on the
// right, and in deleting mode that slot shows the numeral to press
// instead, so toggling delete-mode doesn't shift the chips. showRemove
// should be false for attachments on already-posted messages, where
// removal is not possible.
func (r *Renderer) Render(attachments []message.Attachment, deleting, showRemove bool, width int) string {
	var chips []string
	r.bounds = r.bounds[:0]

	// Every chip ends with a trailing slot. In the editor it holds the
	// remove button; in delete-mode that same slot holds the numeral to
	// press, so toggling modes doesn't shift the chips; on a posted message
	// it is empty. Which of the three applies is fixed for the whole row,
	// so decide once here. Only the remove button is clickable, so removeW
	// doubles as the flag for whether chips need hit-test bounds.
	removeStr := ""
	removeW := 0
	if showRemove && !deleting {
		removeStr = r.styles.Remove.String()
		removeW = lipgloss.Width(removeStr)
	}

	var offset int
	for i, att := range attachments {
		filename := filepath.Base(att.FileName)
		// Truncate if needed.
		if ansi.StringWidth(filename) > maxFilename {
			filename = ansi.Truncate(filename, maxFilename, "…")
		}

		iconStr := r.icon(att).String()
		nameStyle := r.styles.Normal
		if !showRemove {
			// Without a remove button there is nothing to carry the
			// trailing margin that separates adjacent chips (the ✕'s
			// MarginRight does this on the editor path), so put it on the
			// filename instead. Otherwise posted messages with multiple
			// attachments render with their chip backgrounds touching.
			nameStyle = nameStyle.MarginRight(1)
		}
		nameStr := nameStyle.Render(filename)

		trailingStr := removeStr
		if deleting {
			trailingStr = r.styles.Deleting.Render(strconv.Itoa(i))
		}

		chipW := lipgloss.Width(iconStr) + lipgloss.Width(nameStr) + lipgloss.Width(trailingStr)

		// Chips are measured as they are laid out rather than budgeted at a
		// uniform maximum, so a short filename isn't counted as if it were
		// the full maxFilename. Placing this chip has to leave room for the
		// hint that will stand in for whatever follows it, except on the
		// last chip, which leaves nothing behind to announce.
		hidden := len(attachments) - i
		need := chipW
		if hidden > 1 {
			need += lipgloss.Width(r.more(hidden - 1))
		}
		placeable := offset+need <= width

		// The first chip is drawn even when it doesn't fit. A row of pure
		// metadata with no filename on it is worse than one that gives up
		// the count, and on a row too narrow for both the name is the more
		// useful half.
		if !placeable && i > 0 {
			chips = append(chips, r.more(hidden))
			break
		}

		chips = append(chips, iconStr, nameStr, trailingStr)
		if removeW > 0 {
			removeStart := offset + chipW - removeW
			// The button's trailing margin is the gap between chips, not
			// part of the button, so clicking it must not count as a hit.
			// Exclude it from the region.
			r.bounds = append(r.bounds, chipBounds{
				startX:    removeStart,
				removeEnd: removeStart + removeW - r.styles.Remove.GetHorizontalMargins(),
			})
		}
		offset += chipW

		if !placeable {
			// Drawing that first chip consumed the room the hint needed.
			break
		}
	}

	return lipgloss.JoinHorizontal(lipgloss.Left, chips...)
}

// more renders the "N more…" hint that stands in for the attachments that
// did not fit on the row.
func (r *Renderer) more(n int) string {
	return r.styles.More.Render(fmt.Sprintf("%d more…", n))
}

// HitTestRemove returns the index of the attachment whose remove button
// contains the given x coordinate, or -1 if none.
func (r *Renderer) HitTestRemove(_ []message.Attachment, x int) int {
	for i, b := range r.bounds {
		if x >= b.startX && x < b.removeEnd {
			return i
		}
	}
	return -1
}

func (r *Renderer) icon(a message.Attachment) lipgloss.Style {
	if a.IsImage() {
		return r.styles.Image
	}
	if a.IsMarkdown() {
		return r.styles.Skill
	}
	return r.styles.Text
}
