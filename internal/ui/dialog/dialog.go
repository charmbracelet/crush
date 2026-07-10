package dialog

import (
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
	uv "github.com/charmbracelet/ultraviolet"
)

// Dialog sizing constants.
const (
	// defaultDialogMaxWidth is the maximum width for standard dialogs.
	defaultDialogMaxWidth = 70
	// defaultDialogHeight is the default height for standard dialogs.
	defaultDialogHeight = 20
	// titleContentHeight is the height of the title content line.
	titleContentHeight = 1
	// inputContentHeight is the height of the input content line.
	inputContentHeight = 1
	// mouseScrollLines is the number of lines to scroll on mouse wheel events.
	mouseScrollLines = 3
)

// CloseKey is the default key binding to close dialogs.
var CloseKey = key.NewBinding(
	key.WithKeys("esc", "alt+esc"),
	key.WithHelp("esc", "exit"),
)

// Action represents an action taken in a dialog after handling a message.
type Action any

// Dialog is a component that can be displayed on top of the UI.
type Dialog interface {
	// ID returns the unique identifier of the dialog.
	ID() string
	// HandleMsg processes a message and returns an action. An [Action] can be
	// anything and the caller is responsible for handling it appropriately.
	HandleMsg(msg tea.Msg) Action
	// Draw draws the dialog onto the provided screen within the specified area
	// and returns the desired cursor position on the screen.
	Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor
}

// LoadingDialog is a dialog that can show a loading state.
type LoadingDialog interface {
	StartLoading() tea.Cmd
	StopLoading()
}

// Grace period constants for dialogs that open asynchronously and may
// receive in-flight keystrokes from a previously focused component.
const (
	// graceQuietPeriod is how long input must be quiet before the dialog
	// arms. Each absorbed keystroke resets this timer.
	graceQuietPeriod = 200 * time.Millisecond
	// graceMaxDelay is the absolute ceiling: the dialog always arms after
	// this duration regardless of input activity. Prevents auto-repeat
	// from keeping the dialog disarmed indefinitely.
	graceMaxDelay = 1500 * time.Millisecond
)

// Overlay manages multiple dialogs as an overlay.
type Overlay struct {
	dialogs []Dialog

	// Grace period state for the front dialog. Only active when the
	// dialog was opened via OpenDialogWithGrace.
	graceOpenedAt    time.Time
	graceLastInputAt time.Time

	// Track recently closed dialog IDs so that reopening the same
	// dialog type can skip the grace period. This prevents rapid
	// successive dialogs (e.g. multiple permission prompts) from
	// each eating a keystroke.
	lastClosedID string
	lastClosedAt time.Time
}

// NewOverlay creates a new [Overlay] instance.
func NewOverlay(dialogs ...Dialog) *Overlay {
	return &Overlay{
		dialogs: dialogs,
	}
}

// HasDialogs checks if there are any active dialogs.
func (d *Overlay) HasDialogs() bool {
	return len(d.dialogs) > 0
}

// ContainsDialog checks if a dialog with the specified ID exists.
func (d *Overlay) ContainsDialog(dialogID string) bool {
	for _, dialog := range d.dialogs {
		if dialog.ID() == dialogID {
			return true
		}
	}
	return false
}

// OpenDialog opens a new dialog to the stack.
func (d *Overlay) OpenDialog(dialog Dialog) {
	d.dialogs = append(d.dialogs, dialog)
	d.graceOpenedAt = time.Time{}
	d.graceLastInputAt = time.Time{}
}

// OpenDialogWithGrace opens a dialog with an input grace period. All
// keystrokes are absorbed until either the input has been quiet for
// graceQuietPeriod or graceMaxDelay has elapsed since opening, whichever
// comes first. Use this for dialogs that open asynchronously (e.g.
// permission prompts) where in-flight keystrokes from a previously
// focused component could act on the dialog before the user sees it.
//
// If the same dialog ID was just closed (within reopenGraceWindow),
// the grace period is skipped — the user is already focused on this
// dialog type and rapid successive prompts should not eat keystrokes.
func (d *Overlay) OpenDialogWithGrace(dialog Dialog) {
	now := time.Now()
	d.dialogs = append(d.dialogs, dialog)

	// Skip grace when reopening the same dialog type immediately.
	if dialog.ID() == d.lastClosedID && now.Sub(d.lastClosedAt) < reopenGraceWindow {
		d.graceOpenedAt = time.Time{}
		d.graceLastInputAt = time.Time{}
		return
	}

	d.graceOpenedAt = now
	d.graceLastInputAt = now
}

// inGracePeriod reports whether the front dialog is still within its
// input grace period. Returns false if no grace period is active.
func (d *Overlay) inGracePeriod() bool {
	if d.graceOpenedAt.IsZero() {
		return false
	}
	if time.Since(d.graceOpenedAt) >= graceMaxDelay {
		return false
	}
	if time.Since(d.graceLastInputAt) >= graceQuietPeriod {
		return false
	}
	return true
}

// CloseDialog closes the dialog with the specified ID from the stack.
func (d *Overlay) CloseDialog(dialogID string) {
	for i, dialog := range d.dialogs {
		if dialog.ID() == dialogID {
			d.removeDialog(i)
			return
		}
	}
}

// CloseFrontDialog closes the front dialog in the stack.
func (d *Overlay) CloseFrontDialog() {
	if len(d.dialogs) == 0 {
		return
	}
	d.removeDialog(len(d.dialogs) - 1)
}

// reopenGraceWindow is how long after closing a dialog we consider
// a reopen of the same dialog ID to be "immediate" and skip grace.
const reopenGraceWindow = 500 * time.Millisecond

func (d *Overlay) removeDialog(idx int) {
	if idx == len(d.dialogs)-1 {
		d.lastClosedID = d.dialogs[idx].ID()
		d.lastClosedAt = time.Now()
	}
	d.dialogs = append(d.dialogs[:idx], d.dialogs[idx+1:]...)
	// Clear grace state when the front dialog changes.
	if idx == len(d.dialogs) {
		d.graceOpenedAt = time.Time{}
		d.graceLastInputAt = time.Time{}
	}
}

// Dialog returns the dialog with the specified ID, or nil if not found.
func (d *Overlay) Dialog(dialogID string) Dialog {
	for _, dialog := range d.dialogs {
		if dialog.ID() == dialogID {
			return dialog
		}
	}
	return nil
}

// DialogLast returns the front dialog, or nil if there are no dialogs.
func (d *Overlay) DialogLast() Dialog {
	if len(d.dialogs) == 0 {
		return nil
	}
	return d.dialogs[len(d.dialogs)-1]
}

// BringToFront brings the dialog with the specified ID to the front.
func (d *Overlay) BringToFront(dialogID string) {
	for i, dialog := range d.dialogs {
		if dialog.ID() == dialogID {
			// Move the dialog to the end of the slice
			d.dialogs = append(d.dialogs[:i], d.dialogs[i+1:]...)
			d.dialogs = append(d.dialogs, dialog)
			return
		}
	}
}

// Update handles dialog updates.
func (d *Overlay) Update(msg tea.Msg) tea.Msg {
	if len(d.dialogs) == 0 {
		return nil
	}

	// Absorb keystrokes during the grace period for async dialogs.
	if _, ok := msg.(tea.KeyPressMsg); ok && d.inGracePeriod() {
		d.graceLastInputAt = time.Now()
		return nil
	}

	idx := len(d.dialogs) - 1 // active dialog is the last one
	dialog := d.dialogs[idx]
	if dialog == nil {
		return nil
	}

	return dialog.HandleMsg(msg)
}

// StartLoading starts the loading state for the front dialog if it
// implements [LoadingDialog].
func (d *Overlay) StartLoading() tea.Cmd {
	dialog := d.DialogLast()
	if ld, ok := dialog.(LoadingDialog); ok {
		return ld.StartLoading()
	}
	return nil
}

// StopLoading stops the loading state for the front dialog if it
// implements [LoadingDialog].
func (d *Overlay) StopLoading() {
	dialog := d.DialogLast()
	if ld, ok := dialog.(LoadingDialog); ok {
		ld.StopLoading()
	}
}

// DrawCenterCursor draws the given string view centered in the screen area and
// adjusts the cursor position accordingly.
func DrawCenterCursor(scr uv.Screen, area uv.Rectangle, view string, cur *tea.Cursor) {
	width, height := lipgloss.Size(view)
	center := common.CenterRect(area, width, height)
	if cur != nil {
		cur.X += center.Min.X
		cur.Y += center.Min.Y
	}
	uv.NewStyledString(view).Draw(scr, center)
}

// DrawCenter draws the given string view centered in the screen area.
func DrawCenter(scr uv.Screen, area uv.Rectangle, view string) {
	DrawCenterCursor(scr, area, view, nil)
}

// DrawOnboarding draws the given string view centered in the screen area.
func DrawOnboarding(scr uv.Screen, area uv.Rectangle, view string) {
	DrawOnboardingCursor(scr, area, view, nil)
}

// DrawOnboardingCursor draws the given string view positioned at the bottom
// left area of the screen.
func DrawOnboardingCursor(scr uv.Screen, area uv.Rectangle, view string, cur *tea.Cursor) {
	width, height := lipgloss.Size(view)
	bottomLeft := common.BottomLeftRect(area, width, height)
	if cur != nil {
		cur.X += bottomLeft.Min.X
		cur.Y += bottomLeft.Min.Y
	}
	uv.NewStyledString(view).Draw(scr, bottomLeft)
}

// Draw renders the overlay and its dialogs.
func (d *Overlay) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	var cur *tea.Cursor
	for _, dialog := range d.dialogs {
		cur = dialog.Draw(scr, area)
	}
	return cur
}
