package dialog

import (
	"image"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
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
// and an empty title is left out. cur, relative to the body, comes back
// relative to the whole view.
func authFrame(t *styles.Styles, width int, title, body, help string, cur *tea.Cursor, onboarding bool) (string, *tea.Cursor) {
	rc := NewRenderContext(t, width)
	rc.IsOnboarding = onboarding
	if !onboarding {
		rc.Title = title
	} else if title != "" {
		rc.AddPart(t.Dialog.PrimaryText.Render(title))
	}
	if cur != nil {
		origin := nextPartOrigin(rc)
		cur.X += origin.X
		cur.Y += origin.Y
	}
	rc.AddPart(body)
	rc.Help = help
	return rc.Render(), cur
}

// nextPartOrigin is where the next part added to rc lands in its rendered
// view: below the title and the parts so far, inside the frame. It
// assumes no gap between parts, as the sign-in dialogs use.
func nextPartOrigin(rc *RenderContext) image.Point {
	above := *rc
	above.IsOnboarding = true
	above.Help = ""
	origin := tea.NewCursor(0, 0)
	if s := above.Render(); s != "" {
		origin.Y = lipgloss.Height(s)
	}
	if !rc.IsOnboarding {
		origin = offsetByFrame(rc.ViewStyle, origin)
	}
	return image.Pt(origin.X, origin.Y)
}

// offsetByFrame moves cur, relative to the content of a block drawn in
// style, to be relative to the block itself: past its top and left
// margin, border and padding.
func offsetByFrame(style lipgloss.Style, cur *tea.Cursor) *tea.Cursor {
	if cur == nil {
		return nil
	}
	cur.X += style.GetMarginLeft() + style.GetBorderLeftSize() + style.GetPaddingLeft()
	cur.Y += style.GetMarginTop() + style.GetBorderTopSize() + style.GetPaddingTop()
	return cur
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

// authEscKey is esc in a sign-in dialog: it cancels while a sign-in is
// still running, and once it is done (signed in, failed, or never
// started) there is nothing left to cancel, so it just exits.
func authEscKey(done bool) key.Binding {
	if done {
		return CloseKey
	}
	return cancelKey
}

// finishKey acknowledges a finished sign-in. Enter and esc both do it,
// since moving on is all that is left.
func finishKey(label string) key.Binding {
	return key.NewBinding(
		key.WithKeys("enter", "ctrl+y", "esc", "alt+esc"),
		key.WithHelp("enter", label),
	)
}

// authSpinnerLine renders a sign-in progress row: the spinner followed by
// msg, sized to width and inset by one cell.
func authSpinnerLine(t *styles.Styles, sp spinner.Model, width int, msg string) string {
	status := t.Dialog.OAuth.StatusText
	return status.
		Width(width).
		Padding(0, 1).
		Render(t.Dialog.OAuth.Success.Render(sp.View()) + status.Render(msg))
}

// authSuccessBlock renders the sign-in confirmation, sized to width and
// padded by one cell on every side.
func authSuccessBlock(t *styles.Styles, width int) string {
	return t.Dialog.OAuth.Success.
		Width(width).
		Padding(1).
		Render("Authentication successful!")
}

// authErrorBlock renders a failed sign-in: an "Authentication failed."
// header and, when there is one, the reason below it, flattened onto one
// line and wrapped to width. Both are inset by one cell and framed by
// blank lines.
func authErrorBlock(t *styles.Styles, width int, reason string) string {
	header := t.Dialog.OAuth.ErrorText.
		Width(width).
		Padding(0, 1).
		Render("Authentication failed.")
	reason = strings.Join(strings.Fields(reason), " ")
	if reason == "" {
		return lipgloss.JoinVertical(lipgloss.Left, "", header, "")
	}
	detail := t.Dialog.OAuth.StatusText.
		Width(width).
		Padding(0, 1).
		Render(reason)
	return lipgloss.JoinVertical(lipgloss.Left, "", header, "", detail, "")
}

// errText is the message of err, or "" when there is none.
func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// authLinkBlock renders url as a hyperlink broken to fit width, inset by
// one cell, with label on the line above when there is one. id names the
// hyperlink so terminals treat its lines as one link. An empty url
// renders nothing.
func authLinkBlock(t *styles.Styles, width int, url, id, label string) string {
	if url == "" {
		return ""
	}
	const inset = 1
	text := wrappedLink(t.Dialog.OAuth.Link, url, "id="+id, width-2*inset)
	style := lipgloss.NewStyle()
	if label != "" {
		text = label + "\n" + text
		style = t.Dialog.OAuth.StatusText
	}
	return style.Width(width).Padding(0, inset).Render(text)
}

// wrappedLink renders url in style as a hyperlink broken at fixed
// character widths of at most width cells, rather than at hyphens, so it
// reads as one unbroken token. Each line carries its own hyperlink to
// the full URL, so clicking any of them opens it. params are the OSC 8
// hyperlink parameters, such as "id=oauth-verify".
func wrappedLink(style lipgloss.Style, url, params string, width int) string {
	link := style.Hyperlink(url, params)
	lines := strings.Split(ansi.Hardwrap(url, max(1, width), false), "\n")
	for i, line := range lines {
		lines[i] = link.Render(line)
	}
	return strings.Join(lines, "\n")
}
