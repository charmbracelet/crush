package dialog

import (
	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	uv "github.com/charmbracelet/ultraviolet"
)

// authDialogMaxWidth is the widest a sign-in dialog grows, frame included.
const authDialogMaxWidth = 60

// authBase is the state every sign-in dialog shares: the common UI
// context, the progress spinner and the help bar.
type authBase struct {
	com     *common.Common
	spinner spinner.Model
	help    help.Model
}

// newAuthBase sets up the shared sign-in state with its spinner drawn in
// spinnerStyle.
func newAuthBase(com *common.Common, spinnerStyle lipgloss.Style) authBase {
	h := help.New()
	h.Styles = com.Styles.DialogHelpStyles()
	return authBase{
		com: com,
		spinner: spinner.New(
			spinner.WithSpinner(spinner.Dot),
			spinner.WithStyle(spinnerStyle),
		),
		help: h,
	}
}

// tickSpinner advances the spinner and schedules its next frame.
func (b *authBase) tickSpinner(msg spinner.TickMsg) Action {
	var cmd tea.Cmd
	b.spinner, cmd = b.spinner.Update(msg)
	if cmd == nil {
		return nil
	}
	return ActionCmd{cmd}
}

// authWidths returns a sign-in dialog's total width for area, capped at
// maxWidth, and its inner width: what is left inside the frame.
func authWidths(t *styles.Styles, area uv.Rectangle, maxWidth int) (width, inner int) {
	width = max(0, min(maxWidth, area.Dx()-t.Dialog.View.GetHorizontalBorderSize()))
	return width, width - t.Dialog.View.GetHorizontalFrameSize()
}

// authFrame lays out a sign-in dialog width cells wide: the title, the
// body and the help line on the dialog frame. During onboarding there is
// no frame, so the title is plain text instead of the gradient title bar,
// and an empty title is left out.
func authFrame(t *styles.Styles, width int, title, body, help string, onboarding bool) string {
	rc := NewRenderContext(t, width)
	rc.IsOnboarding = onboarding
	if !onboarding {
		rc.Title = title
	} else if title != "" {
		rc.AddPart(t.Dialog.PrimaryText.Render(title))
	}
	rc.AddPart(body)
	rc.Help = help
	return rc.Render()
}

// authDraw places a rendered sign-in dialog: bottom left during
// onboarding, centered otherwise. cur, relative to the view, is moved
// with it.
func authDraw(scr uv.Screen, area uv.Rectangle, view string, cur *tea.Cursor, onboarding bool) {
	if onboarding {
		DrawOnboardingCursor(scr, area, view, cur)
		return
	}
	DrawCenterCursor(scr, area, view, cur)
}
